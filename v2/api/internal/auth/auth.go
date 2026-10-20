// Package auth makes and checks the secrets the API hands out: setup codes,
// browser sign-in codes (RFC 8628), session tokens, and the short-lived
// token the monitoring panel and the live feed use. Only hashes of secrets
// are stored; a database leak doesn't hand out working credentials.
package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/mindfries/v2/api/internal/session"
)

// alphabet has no vowels (so no accidental words) and none of the characters
// people mix up: 0/O, 1/I/L, 5/S, 8/B-less. 28 symbols.
const alphabet = "CDFGHJKMNPQRTVWXYZ234679"

func group(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	out := make([]byte, n)
	for i, v := range b {
		out[i] = alphabet[int(v)%len(alphabet)]
	}
	return string(out)
}

// NewSetupCode is the code in the dashboard's command: XXXX-XXXX-XXXX.
func NewSetupCode() string { return group(4) + "-" + group(4) + "-" + group(4) }

// NewUserCode is the short code the candidate compares on the sign-in page.
func NewUserCode() string { return group(4) + "-" + group(4) }

// NewSecret is 256 random bits, hex: a session token or a device code.
func NewSecret() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// NewID is a prefixed random ID such as ses_3f9a….
func NewID(prefix string, bytes int) string {
	b := make([]byte, bytes)
	_, _ = rand.Read(b)
	return prefix + "_" + hex.EncodeToString(b)
}

// NewUUID is a random (version 4) UUID, for tests and for ids that must look
// like V1's.
func NewUUID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	h := hex.EncodeToString(b)
	return h[0:8] + "-" + h[8:12] + "-" + h[12:16] + "-" + h[16:20] + "-" + h[20:]
}

// Hash is what is stored for a secret.
func Hash(secret string) string { return session.HashToken(secret) }

// Same compares two secrets in constant time.
func Same(a, b string) bool {
	return len(a) == len(b) && subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

// NormaliseCode canonicalises a code the way the CLI does before it sends
// it: upper case, no spaces, and the dashes put back in an 8- or 12-character
// code typed without them. What a person types then matches what was shown.
func NormaliseCode(s string) string {
	c := strings.ToUpper(strings.Join(strings.Fields(s), ""))
	if !strings.Contains(c, "-") {
		switch len(c) {
		case 8:
			c = c[0:4] + "-" + c[4:8]
		case 12:
			c = c[0:4] + "-" + c[4:8] + "-" + c[8:12]
		}
	}
	return c
}

// DeviceInfo is what the CLI says about the computer, shown on the sign-in
// page so the candidate can see which machine is asking.
type DeviceInfo struct {
	OS        string `json:"os"`
	OSVersion string `json:"os_version,omitempty"`
	Arch      string `json:"arch"`
}

// Device is one browser sign-in in progress.
type Device struct {
	Info         DeviceInfo
	CodeHash     string
	UserCode     string
	ExpiresAt    time.Time
	Interval     int
	LastPolledAt *time.Time
	Status       string // pending | approved | denied | used
	CandidateID  string
	Grant        *session.Grant
	SessionID    string
}

// Panel tokens are issued to the browser for one session. They are an HMAC
// over {session, scope, expiry}, so nothing is stored and a stolen one dies
// quickly. The portal's server gets one only after checking the signed-in
// candidate owns the session.

// Panel scopes.
const (
	ScopePanel  = "panel"  // the panel's uploads and status
	ScopeFeed   = "feed"   // reading the live feed and artifacts
	ScopeGitHub = "github" // the CLI's connect link for one session
)

type panelClaims struct {
	S string `json:"s"` // session id
	P string `json:"p"` // scope
	E int64  `json:"e"` // expiry, unix seconds
}

// ErrBadToken is returned for any token that doesn't check out.
var ErrBadToken = errors.New("invalid panel token")

// IssuePanelToken signs a token for sessionID and scope that lasts ttl.
func IssuePanelToken(key []byte, sessionID, scope string, ttl time.Duration, now time.Time) string {
	body, _ := json.Marshal(panelClaims{S: sessionID, P: scope, E: now.Add(ttl).Unix()})
	b := base64.RawURLEncoding.EncodeToString(body)
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(b))
	return b + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

// CheckPanelToken verifies tok and returns its session id. A token for one
// scope doesn't open another; an expired or altered one fails.
func CheckPanelToken(key []byte, tok string, scopes []string, now time.Time) (string, error) {
	if len(key) == 0 {
		return "", ErrBadToken
	}
	i := strings.IndexByte(tok, '.')
	if i < 1 {
		return "", ErrBadToken
	}
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(tok[:i]))
	want := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	if !Same(want, tok[i+1:]) {
		return "", ErrBadToken
	}
	raw, err := base64.RawURLEncoding.DecodeString(tok[:i])
	if err != nil {
		return "", ErrBadToken
	}
	var c panelClaims
	if json.Unmarshal(raw, &c) != nil || c.S == "" || now.Unix() >= c.E {
		return "", ErrBadToken
	}
	for _, s := range scopes {
		if s == c.P {
			return c.S, nil
		}
	}
	return "", ErrBadToken
}
