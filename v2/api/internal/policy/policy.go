// Package policy is the per-assessment configuration: how long, what the AI
// assistant may do, who reviews the candidate's repository, and which of
// camera, screen and microphone the panel must be sharing. It is set per
// assessment by the company or the Mindfries admin (never by an environment
// variable) and read live, so a reviewer added mid-session is invited at the
// next opportunity.
package policy

import (
	"regexp"
	"sort"
	"strings"
)

// AssistantMode is how much the coding assistant (`mindfries agent`) may do.
type AssistantMode string

const (
	Off         AssistantMode = "off"
	ExplainOnly AssistantMode = "explain_only"
	CodeAllowed AssistantMode = "code_allowed"
)

// Valid reports whether m is one of the three modes.
func (m AssistantMode) Valid() bool { return m == Off || m == ExplainOnly || m == CodeAllowed }

// Config is one assessment's settings.
type Config struct {
	AssessmentID         string        `json:"assessment_id"`
	DurationMinutes      int           `json:"duration_minutes"`
	AssistantMode        AssistantMode `json:"assistant_mode"`
	OutsideAgentsAllowed bool          `json:"outside_agents_allowed"`
	Reviewers            []string      `json:"reviewers"` // GitHub logins
	RequireCamera        bool          `json:"require_camera"`
	RequireScreen        bool          `json:"require_screen"`
	RequireAudio         bool          `json:"require_audio"`
	AvatarEnabled        bool          `json:"avatar_enabled"`
	TabsOverride         []string      `json:"tabs_override,omitempty"`
	JD                   string        `json:"jd"`
}

// Default is what an assessment gets when nobody has configured it: 90
// minutes, the assistant may explain but not write code, and the panel must
// share camera, screen and microphone (the PRD's default).
func Default() Config {
	return Config{
		DurationMinutes: 90, AssistantMode: ExplainOnly,
		RequireCamera: true, RequireScreen: true, RequireAudio: true, AvatarEnabled: true,
		Reviewers: []string{},
	}
}

var githubLogin = regexp.MustCompile(`^[A-Za-z0-9](?:[A-Za-z0-9]|-(?:[A-Za-z0-9])){0,38}$`)

// ValidLogin reports whether s can be a GitHub username.
func ValidLogin(s string) bool { return githubLogin.MatchString(s) }

// NormaliseReviewers trims, drops a leading @, removes duplicates (GitHub
// logins are case-insensitive) and returns them sorted. Anything that isn't a
// valid username comes back in bad, so the caller can say which.
func NormaliseReviewers(in []string) (ok, bad []string) {
	seen := map[string]bool{}
	for _, r := range in {
		r = strings.TrimPrefix(strings.TrimSpace(r), "@")
		if r == "" {
			continue
		}
		if !ValidLogin(r) {
			bad = append(bad, r)
			continue
		}
		k := strings.ToLower(r)
		if seen[k] {
			continue
		}
		seen[k] = true
		ok = append(ok, r)
	}
	sort.Slice(ok, func(i, j int) bool { return strings.ToLower(ok[i]) < strings.ToLower(ok[j]) })
	if ok == nil {
		ok = []string{}
	}
	return ok, bad
}

// Problem says what is wrong with c, or "" when it can be saved.
func (c Config) Problem() string {
	if c.DurationMinutes < 10 || c.DurationMinutes > 600 {
		return "duration_minutes must be between 10 and 600"
	}
	if !c.AssistantMode.Valid() {
		return "assistant_mode must be off, explain_only or code_allowed"
	}
	if _, bad := NormaliseReviewers(c.Reviewers); len(bad) > 0 {
		return "not a GitHub username: " + strings.Join(bad, ", ")
	}
	return ""
}
