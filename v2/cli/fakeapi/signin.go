package fakeapi

import (
	"crypto/rand"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/mindfries/v2/cli/internal/api"
)

// Browser sign-in, the contract's POST /sessions/device and
// /sessions/device/token: `mindfries start` with no code asks for a sign-in,
// opens the browser at the fake's /device page, and polls until the
// candidate confirms there and picks the assessment they were invited to.
//
// The fake has no accounts: every invitation is offered to whoever signs
// in, with any email. Invitations can be connected any number of times, so
// a restarted deployment (which seeds them again) is always usable.

// invitation is an assessment a candidate was invited to.
type invitation struct {
	id   string
	opts SessionOptions
}

// signIn is one pending browser sign-in.
type signIn struct {
	deviceCode string
	userCode   string
	device     api.Device
	createdAt  time.Time
	expiresAt  time.Time
	interval   time.Duration
	lastPoll   time.Time

	approved *invitation
	email    string
	denied   bool
	session  string // the session it started, once the CLI collected it
	// GitHub, when authorised on the page before the session exists.
	ghToken, ghErr string
	used           bool
}

// Invite adds an assessment that signed-in candidates can connect to, as a
// recruiter's invitation does, and returns its ID.
func (s *Server) Invite(o SessionOptions) string {
	o = o.withDefaults()
	s.mu.Lock()
	defer s.mu.Unlock()
	inv := &invitation{id: randomID("inv_", 6), opts: o}
	s.invites = append(s.invites, inv)
	return inv.id
}

// PendingSignIns returns the user codes of sign-ins waiting for the
// browser, for tests.
func (s *Server) PendingSignIns() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []string
	for _, si := range s.signIns {
		if si.approved == nil && !si.denied && s.now().Before(si.expiresAt) {
			out = append(out, si.userCode)
		}
	}
	return out
}

// ApproveSignIn confirms a sign-in as the browser page does. An empty
// invitationID picks the first invitation.
func (s *Server) ApproveSignIn(userCode, invitationID, email string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.approveLocked(userCode, invitationID, email)
}

// DenySignIn refuses a sign-in, as "This wasn't me" on the page does.
func (s *Server) DenySignIn(userCode string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	si, err := s.pendingLocked(userCode)
	if err != nil {
		return err
	}
	si.denied = true
	return nil
}

var (
	errNoSignIn      = errors.New("there's no sign-in waiting with that code")
	errSignInExpired = errors.New("that code has expired; run mindfries start again")
	errSignInDone    = errors.New("that code has already been used")
	errNoInvitation  = errors.New("there's no such assessment")
)

func (s *Server) pendingLocked(userCode string) (*signIn, error) {
	userCode = normalizeUserCode(userCode)
	for _, si := range s.signIns {
		if si.userCode != userCode {
			continue
		}
		switch {
		case si.approved != nil || si.denied:
			return nil, errSignInDone
		case !s.now().Before(si.expiresAt):
			return nil, errSignInExpired
		}
		return si, nil
	}
	return nil, errNoSignIn
}

func (s *Server) approveLocked(userCode, invitationID, email string) error {
	si, err := s.pendingLocked(userCode)
	if err != nil {
		return err
	}
	var inv *invitation
	for _, i := range s.invites {
		if invitationID == "" || i.id == invitationID {
			inv = i
			break
		}
	}
	if inv == nil {
		return errNoInvitation
	}
	si.approved = inv
	si.email = email
	return nil
}

// userAlphabet leaves out vowels, so a user code never spells a word, and
// 0/O and 1/I.
const userAlphabet = "BCDFGHJKLMNPQRSTVWXZ23456789"

func newUserCode() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	var sb strings.Builder
	for i, c := range b {
		if i == 4 {
			sb.WriteByte('-')
		}
		sb.WriteByte(userAlphabet[int(c)%len(userAlphabet)])
	}
	return sb.String()
}

func normalizeUserCode(c string) string {
	c = strings.ToUpper(strings.Join(strings.Fields(c), ""))
	c = strings.ReplaceAll(c, "-", "")
	if len(c) == 8 {
		c = c[:4] + "-" + c[4:]
	}
	return c
}

func init() {
	register(func(s *Server, route func(string, http.HandlerFunc)) {
		route("POST /sessions/device", s.handleDeviceStart)
		route("POST /sessions/device/token", s.handleDeviceToken)
	})
	registerAdmin(func(s *Server, mux *http.ServeMux) {
		mux.HandleFunc("POST /__admin/invitations", func(w http.ResponseWriter, r *http.Request) {
			var in struct {
				Title           string   `json:"title"`
				DurationMinutes int      `json:"duration_minutes"`
				AssistantMode   string   `json:"assistant_mode"`
				Reviewers       []string `json:"reviewers"`
			}
			if r.ContentLength != 0 && !decode(w, r, &in) {
				return
			}
			reviewers, err := CleanReviewers(in.Reviewers)
			if err != nil {
				problem(w, http.StatusBadRequest, api.CodeMalformed, sentence(err), "")
				return
			}
			o := SessionOptions{Title: in.Title, Duration: time.Duration(in.DurationMinutes) * time.Minute, Reviewers: reviewers}
			o.Policy.AssistantMode = in.AssistantMode
			writeJSON(w, http.StatusCreated, map[string]string{"invitation_id": s.Invite(o)})
		})
	})
}

// publicBase is the fake's address as the browser sees it.
func (s *Server) publicBase(r *http.Request) string {
	if s.cfg.PublicURL != "" {
		return strings.TrimRight(s.cfg.PublicURL, "/")
	}
	scheme := "http"
	if r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https") {
		scheme = "https"
	}
	return scheme + "://" + r.Host
}

func (s *Server) handleDeviceStart(w http.ResponseWriter, r *http.Request) {
	var in api.DeviceRequest
	if !decode(w, r, &in) {
		return
	}
	if in.Device.OS == "" || in.Device.Arch == "" {
		problem(w, http.StatusBadRequest, api.CodeMalformed, "device is required", "")
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	si := &signIn{
		deviceCode: randomID("mfd_", 24),
		userCode:   newUserCode(),
		device:     in.Device,
		createdAt:  now,
		expiresAt:  now.Add(10 * time.Minute),
		interval:   s.cfg.SignInInterval,
	}
	s.signIns[si.deviceCode] = si
	page := s.publicBase(r) + "/device"
	secs := int(si.interval / time.Second)
	if secs < 1 {
		secs = 1
	}
	writeJSON(w, http.StatusOK, api.DeviceAuthorization{
		DeviceCode:              si.deviceCode,
		UserCode:                si.userCode,
		VerificationURI:         page,
		VerificationURIComplete: page + "?code=" + si.userCode,
		ExpiresAt:               si.expiresAt,
		IntervalSeconds:         secs,
	})
}

func (s *Server) handleDeviceToken(w http.ResponseWriter, r *http.Request) {
	var in api.DeviceTokenRequest
	if !decode(w, r, &in) {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	si := s.signIns[in.DeviceCode]
	now := s.now()
	switch {
	case si == nil:
		problem(w, http.StatusGone, api.CodeDeviceCodeInvalid, "This sign-in isn't recognised", "")
		return
	case si.used:
		problem(w, http.StatusGone, api.CodeDeviceCodeUsed, "This sign-in has already been used", "")
		return
	case si.denied:
		problem(w, http.StatusForbidden, api.CodeAccessDenied, "The sign-in was refused in the browser", "")
		return
	case si.approved == nil && !now.Before(si.expiresAt):
		problem(w, http.StatusGone, api.CodeDeviceCodeExpired, "This sign-in has expired", "")
		return
	}
	if si.approved == nil {
		// Polling faster than half the interval earns slow_down and a
		// longer interval, as RFC 8628 has it.
		tooFast := !si.lastPoll.IsZero() && now.Sub(si.lastPoll) < si.interval/2
		si.lastPoll = now
		if tooFast {
			si.interval += 5 * time.Second
			problem(w, http.StatusBadRequest, api.CodeSlowDown, "Polling too fast", fmt.Sprintf("wait %d seconds between requests", int(si.interval/time.Second)))
			return
		}
		problem(w, http.StatusBadRequest, api.CodeAuthorizationPending, "Waiting for the candidate to confirm in the browser", "")
		return
	}
	si.used = true
	sess := s.newSession(si.approved.opts, s.publicBase(r))
	sess.invitation = si.approved.id
	sess.ext.ghToken, sess.ext.ghErr = si.ghToken, si.ghErr
	si.ghToken = ""
	si.session = sess.ID
	writeJSON(w, http.StatusOK, api.ExchangeResponse{SessionToken: sess.token, ExpiresAt: sess.EndsAt, Session: sess.view()})
}

// pageRoutes are what a browser opens: the sign-in page and, with
// Config.DistDir, the installers and CLI builds.
func (s *Server) pageRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /device", s.handleDevicePage)
	mux.HandleFunc("POST /device", s.handleDeviceForm)
	s.githubRoutes(mux)
	s.monitorRoutes(mux)
	s.dashboardRoutes(mux)
	s.distRoutes(mux)
}
