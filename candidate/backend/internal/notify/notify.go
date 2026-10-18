// Package notify sends the few operational emails the backend raises itself
// — right now, one: an alert when a candidate's task could not be generated,
// so the company or Mindfries can regenerate it or assign one by hand rather
// than the candidate sitting on an assessment that never prepares.
//
// It goes over Resend's HTTP API, the same provider the three frontends send
// with, so the backend takes on no mail SDK for a single message. An
// unconfigured Mailer (any of key/from/to missing) sends nothing and says so
// — the same "real, or an honest failure" rule the rest of the backend holds.
package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// Mailer sends alert mail through Resend.
type Mailer struct {
	apiKey  string
	from    string
	to      string
	baseURL string // "" → Resend; a stand-in sets it in tests
	client  *http.Client
}

// New builds a Mailer. Any of apiKey/from/to empty leaves it unconfigured,
// which Configured reports and Send refuses — callers log and move on.
func New(apiKey, from, to, baseURL string) *Mailer {
	return &Mailer{apiKey: apiKey, from: from, to: to, baseURL: baseURL, client: &http.Client{Timeout: 15 * time.Second}}
}

// Configured reports whether there is a key, a from and a to to send with.
func (m *Mailer) Configured() bool {
	return m != nil && m.apiKey != "" && m.from != "" && m.to != ""
}

func (m *Mailer) send(ctx context.Context, subject, text string) error {
	if !m.Configured() {
		return fmt.Errorf("notify: not configured (set RESEND_API_KEY, RESEND_FROM and TASK_ALERT_EMAIL)")
	}
	base := m.baseURL
	if base == "" {
		base = "https://api.resend.com"
	}
	body, err := json.Marshal(map[string]any{"from": m.from, "to": m.to, "subject": subject, "text": text})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/emails", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+m.apiKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := m.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("notify: the mail provider refused the message (http %d)", resp.StatusCode)
	}
	return nil
}

// TaskFailure alerts that a candidate's task could not be generated, naming
// who is blocked and why, so a person can step in.
func (m *Mailer) TaskFailure(ctx context.Context, invitationID, company, role, candidateEmail, reason string) error {
	who := candidateEmail
	if who == "" {
		who = "a candidate"
	}
	subject := fmt.Sprintf("Assessment task failed to generate — %s", orElse(role, "a role"))
	var b strings.Builder
	fmt.Fprintf(&b, "A candidate's assessment task could not be generated, and it will not be retried automatically.\n\n")
	fmt.Fprintf(&b, "Candidate: %s\n", who)
	if company != "" {
		fmt.Fprintf(&b, "Company:   %s\n", company)
	}
	fmt.Fprintf(&b, "Role:      %s\n", orElse(role, "(unspecified)"))
	fmt.Fprintf(&b, "Invitation: %s\n\n", invitationID)
	fmt.Fprintf(&b, "Reason: %s\n\n", reason)
	b.WriteString("What to do: regenerate the task for this invitation, or assign one, so the candidate can start. Until then their assessment stays in preparing.")
	return m.send(ctx, subject, b.String())
}

func orElse(s, fallback string) string {
	if strings.TrimSpace(s) == "" {
		return fallback
	}
	return s
}
