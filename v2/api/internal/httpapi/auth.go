package httpapi

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/mindfries/v2/api/internal/auth"
	"github.com/mindfries/v2/api/internal/session"
)

// Three kinds of caller, three ways in:
//
//	the CLI     a session bearer token (authed, authedLive)
//	a portal    the portals' servers, with V2_PORTAL_SECRET (portalOnly)
//	an admin    Mindfries ops, with V2_ADMIN_TOKEN (adminOnly)
//
// plus the monitoring panel's short-lived token (panelAuth, in panel.go).
// A browser never holds a portal secret or a session token.

const versionHeader = "Mindfries-CLI-Version"

// authed finds the session for the request's bearer token, whatever its
// status. It also enforces the CLI version.
func (s *Server) authed(w http.ResponseWriter, r *http.Request) (session.Session, bool) {
	if !s.checkVersion(w, r) {
		return session.Session{}, false
	}
	tok := bearer(r)
	if tok == "" {
		problem(w, http.StatusUnauthorized, "token_missing", "no session token")
		return session.Session{}, false
	}
	sess, err := s.sessions.SessionByToken(r.Context(), tok)
	if err != nil {
		problem(w, http.StatusUnauthorized, "token_invalid", "unknown session token")
		return session.Session{}, false
	}
	if sess.Status == "revoked" {
		problem(w, http.StatusUnauthorized, "token_revoked", "the session was ended")
		return session.Session{}, false
	}
	return sess, true
}

// authedLive allows a session that is set up (ready) or running (active):
// the bundle download and the evidence both happen during setup, before the
// clock starts.
func (s *Server) authedLive(w http.ResponseWriter, r *http.Request) (session.Session, bool) {
	sess, ok := s.authed(w, r)
	if !ok {
		return session.Session{}, false
	}
	if !sess.Live() {
		problemWith(w, http.StatusConflict, "session_not_active", "the session has ended",
			map[string]any{"session_status": sess.Status})
		return session.Session{}, false
	}
	return sess, true
}

// checkVersion requires the CLI's version header and refuses a CLI older
// than the minimum with the contract's 426.
func (s *Server) checkVersion(w http.ResponseWriter, r *http.Request) bool {
	v := r.Header.Get(versionHeader)
	if v == "" {
		problem(w, http.StatusBadRequest, "malformed", "missing "+versionHeader)
		return false
	}
	if min := s.cfg.MinCLIVersion; min != "" && CompareVersions(v, min) < 0 {
		problemWith(w, http.StatusUpgradeRequired, "cli_too_old", "update mindfries to continue",
			map[string]any{"minimum_version": min})
		return false
	}
	return true
}

// CompareVersions compares two dotted versions (1.2.3, an optional leading v
// and any -suffix ignored). It returns -1, 0 or 1. A version it can't read
// counts as 0.0.0.
func CompareVersions(a, b string) int {
	pa, pb := parseVersion(a), parseVersion(b)
	for i := 0; i < 3; i++ {
		switch {
		case pa[i] < pb[i]:
			return -1
		case pa[i] > pb[i]:
			return 1
		}
	}
	return 0
}

func parseVersion(v string) [3]int {
	v = strings.TrimPrefix(strings.TrimSpace(v), "v")
	if i := strings.IndexAny(v, "-+"); i >= 0 {
		v = v[:i]
	}
	var out [3]int
	for i, part := range strings.SplitN(v, ".", 3) {
		n, err := strconv.Atoi(part)
		if err != nil {
			return [3]int{}
		}
		out[i] = n
	}
	return out
}

func bearer(r *http.Request) string {
	h := r.Header.Get("Authorization")
	const p = "Bearer "
	if len(h) > len(p) && strings.EqualFold(h[:len(p)], p) {
		return strings.TrimSpace(h[len(p):])
	}
	return ""
}

// portalOnly lets through only the portals' servers. With no secret
// configured the routes are off, never open.
func (s *Server) portalOnly(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.cfg.PortalSecret == "" {
			problem(w, http.StatusServiceUnavailable, "not_configured", "V2_PORTAL_SECRET is not set")
			return
		}
		if !auth.Same(r.Header.Get("X-Portal-Secret"), s.cfg.PortalSecret) {
			problem(w, http.StatusForbidden, "forbidden", "a portal secret is required")
			return
		}
		h(w, r)
	}
}

// adminOnly lets through Mindfries ops with the admin token.
func (s *Server) adminOnly(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.cfg.AdminToken == "" {
			problem(w, http.StatusServiceUnavailable, "not_configured", "V2_ADMIN_TOKEN is not set")
			return
		}
		if !auth.Same(r.Header.Get("X-Admin-Token"), s.cfg.AdminToken) {
			problem(w, http.StatusForbidden, "forbidden", "valid X-Admin-Token required")
			return
		}
		h(w, r)
	}
}

// publicBase is this API's own address as the outside world sees it, for the
// links in setup commands and sign-in pages.
func (s *Server) publicBase(r *http.Request) string {
	if s.cfg.PublicURL != "" {
		return s.cfg.PublicURL
	}
	scheme := "http"
	if r.TLS != nil || (s.cfg.TrustProxy && r.Header.Get("X-Forwarded-Proto") == "https") {
		scheme = "https"
	}
	return scheme + "://" + r.Host
}
