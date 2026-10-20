package auth

import (
	"regexp"
	"strings"
	"testing"
	"time"
)

func TestCodesLookRight(t *testing.T) {
	setup := regexp.MustCompile(`^[A-Z0-9]{4}(-[A-Z0-9]{4}){2}$`) // the contract's pattern
	user := regexp.MustCompile(`^[A-Z0-9]{4}-[A-Z0-9]{4}$`)
	seen := map[string]bool{}
	for i := 0; i < 500; i++ {
		c := NewSetupCode()
		if !setup.MatchString(c) || seen[c] {
			t.Fatalf("bad or repeated setup code %q", c)
		}
		seen[c] = true
		if u := NewUserCode(); !user.MatchString(u) || strings.ContainsAny(u, "AEIOU015SBL8") {
			t.Fatalf("user code %q has a vowel or a look-alike", u)
		}
	}
	if len(NewSecret()) != 64 || NewSecret() == NewSecret() {
		t.Fatal("secrets are 256 random bits")
	}
}

func TestNormaliseCode(t *testing.T) {
	if got := NormaliseCode(" ab cd-ef gh "); got != "ABCD-EFGH" {
		t.Fatal(got)
	}
}

func TestPanelToken(t *testing.T) {
	key := []byte("0123456789abcdef0123456789abcdef")
	now := time.Unix(1_800_000_000, 0)
	tok := IssuePanelToken(key, "ses_1", ScopePanel, time.Minute, now)

	if id, err := CheckPanelToken(key, tok, []string{ScopePanel}, now.Add(30*time.Second)); err != nil || id != "ses_1" {
		t.Fatalf("a fresh token must verify: %v %q", err, id)
	}
	if _, err := CheckPanelToken(key, tok, []string{ScopePanel}, now.Add(2*time.Minute)); err == nil {
		t.Error("an expired token must fail")
	}
	if _, err := CheckPanelToken(key, tok, []string{ScopeFeed}, now); err == nil {
		t.Error("a token for one scope must not open another")
	}
	if _, err := CheckPanelToken([]byte("a different key, also 32 bytes!!"), tok, []string{ScopePanel}, now); err == nil {
		t.Error("a token signed with another key must fail")
	}
	bad := tok[:len(tok)-2] + "xx"
	if _, err := CheckPanelToken(key, bad, []string{ScopePanel}, now); err == nil {
		t.Error("an altered signature must fail")
	}
	// Changing the session id in the body without re-signing must fail.
	forged := "eyJzIjoic2VzXzIiLCJwIjoicGFuZWwiLCJlIjo5OTk5OTk5OTk5fQ" + tok[strings.IndexByte(tok, '.'):]
	if _, err := CheckPanelToken(key, forged, []string{ScopePanel}, now); err == nil {
		t.Error("a forged body must fail")
	}
	if _, err := CheckPanelToken(nil, tok, []string{ScopePanel}, now); err == nil {
		t.Error("with no key nothing verifies")
	}
	if _, err := CheckPanelToken(key, "garbage", []string{ScopePanel}, now); err == nil {
		t.Error("garbage must fail")
	}
}
