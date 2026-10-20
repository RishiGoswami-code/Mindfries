package assistant

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// The AI avatar (SPEC section 12): a voice in the panel that asks the
// candidate about their decisions. The browser talks to Gemini Live directly
// over a WebSocket, using a short-lived token this server mints, so the
// server's key never leaves it. The avatar's instructions are locked into the
// token: a page can't loosen them. What is said is posted back and kept as
// evidence in the same record as the terminal assistant's.
//
// The avatar is bounded by design. It asks and listens; it doesn't judge,
// score, accuse or help solve the task.

// AvatarName is what the avatar calls itself.
const AvatarName = "Aria"

// AvatarContext is what the avatar knows when the conversation starts.
type AvatarContext struct {
	TaskTitle     string
	Brief         string
	Role          string
	ElapsedMin    int
	RemainingMin  int
	RecentNotes   []string // neutral lines from the tracker (never flags)
	CandidateName string
}

// AvatarSystem is the avatar's locked instructions.
func AvatarSystem(c AvatarContext) string {
	var b strings.Builder
	fmt.Fprintf(&b, "You are %s, a friendly interviewer's voice inside a timed technical assessment for a %s role. ", AvatarName, c.Role)
	b.WriteString("Speak briefly and naturally, one question at a time, like a thoughtful colleague. You are not the one who decides how the candidate did.\n\n")
	b.WriteString("Your job: understand how the candidate is thinking. Ask about their decisions and trade-offs, what they tried, why they chose an approach, what they would do with more time, and how they would know it works. Follow up on what they say.\n\n")
	b.WriteString("You must never:\n")
	b.WriteString("- score, grade, rank or say whether they are doing well or badly;\n")
	b.WriteString("- say or imply that the candidate has done something wrong, or comment on anything that looks like a rule being broken;\n")
	b.WriteString("- give the solution, write their code, reveal hidden tests or grading criteria;\n")
	b.WriteString("- ask about their personal life, health, family, age, background or anything unrelated to the work.\n")
	b.WriteString("If they ask for help solving it, say you can't help with the solution here, and ask what they have tried instead. If they are silent or stuck, give them time; don't fill every pause.\n\n")
	if c.CandidateName != "" {
		fmt.Fprintf(&b, "The candidate's first name is %s.\n", c.CandidateName)
	}
	fmt.Fprintf(&b, "Task: %s\n%s\n\n", c.TaskTitle, c.Brief)
	fmt.Fprintf(&b, "Time: %d minutes in, %d minutes left.\n", c.ElapsedMin, c.RemainingMin)
	if len(c.RecentNotes) > 0 {
		b.WriteString("Recent neutral activity you may refer to if useful:\n")
		for _, n := range c.RecentNotes {
			b.WriteString("- " + n + "\n")
		}
	}
	return b.String()
}

// LiveTokens mints ephemeral Gemini Live tokens.
type LiveTokens struct {
	APIKey  string
	Model   string // the Live model, e.g. gemini-3.8-live
	BaseURL string // https://generativelanguage.googleapis.com
	HTTP    *http.Client
}

// LiveToken is what the browser needs to open a Live session.
type LiveToken struct {
	Token     string    `json:"token"`
	Model     string    `json:"model"`
	WSURL     string    `json:"ws_url"`
	ExpiresAt time.Time `json:"expires_at"`
}

func (l LiveTokens) base() string {
	if l.BaseURL != "" {
		return strings.TrimRight(l.BaseURL, "/")
	}
	return "https://generativelanguage.googleapis.com"
}

func (l LiveTokens) client() *http.Client {
	if l.HTTP != nil {
		return l.HTTP
	}
	return &http.Client{Timeout: 15 * time.Second}
}

// Mint asks Gemini for a single-use token, good for one Live session that
// must start within two minutes and lasts at most ttl. The model, the voice
// settings and the instructions are locked in the token.
func (l LiveTokens) Mint(ctx context.Context, system string, ttl time.Duration, now time.Time) (LiveToken, error) {
	exp := now.Add(ttl)
	body, _ := json.Marshal(map[string]any{
		"uses":                 1,
		"expireTime":           exp.UTC().Format(time.RFC3339),
		"newSessionExpireTime": now.Add(2 * time.Minute).UTC().Format(time.RFC3339),
		"liveConnectConstraints": map[string]any{
			"model": "models/" + strings.TrimPrefix(l.Model, "models/"),
			"config": map[string]any{
				"responseModalities":       []string{"AUDIO"},
				"systemInstruction":        map[string]any{"parts": []map[string]string{{"text": system}}},
				"inputAudioTranscription":  map[string]any{},
				"outputAudioTranscription": map[string]any{},
			},
		},
	})
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, l.base()+"/v1alpha/auth_tokens", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-goog-api-key", l.APIKey)
	res, err := l.client().Do(req)
	if err != nil {
		return LiveToken{}, fmt.Errorf("%w (%v)", ErrUnavailable, err)
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(res.Body, 64<<10))
	if res.StatusCode == 429 || res.StatusCode >= 500 {
		return LiveToken{}, ErrUnavailable
	}
	if res.StatusCode != http.StatusOK {
		return LiveToken{}, fmt.Errorf("couldn't get a Live token (%d): %s", res.StatusCode, truncate(string(raw), 200))
	}
	var out struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(raw, &out); err != nil || out.Name == "" {
		return LiveToken{}, errors.New("the Live token answer wasn't understood")
	}
	ws := strings.Replace(strings.Replace(l.base(), "https://", "wss://", 1), "http://", "ws://", 1) +
		"/ws/google.ai.generativelanguage.v1alpha.GenerativeService.BidiGenerateContentConstrained"
	return LiveToken{Token: out.Name, Model: l.Model, WSURL: ws, ExpiresAt: exp}, nil
}

// Utterance is one thing said in the avatar conversation.
type Utterance struct {
	Seq  int       `json:"seq"`
	Role string    `json:"role"` // candidate | avatar
	Text string    `json:"text"`
	At   time.Time `json:"at"`
}

// CleanUtterances drops what can't be kept: empty, over-long, or out-of-range
// entries, and an unknown speaker.
func CleanUtterances(in []Utterance) []Utterance {
	var out []Utterance
	for _, u := range in {
		u.Text = strings.TrimSpace(u.Text)
		if u.Seq <= 0 || u.Text == "" || (u.Role != "candidate" && u.Role != "avatar") {
			continue
		}
		if len(u.Text) > 8000 {
			u.Text = u.Text[:8000]
		}
		out = append(out, u)
	}
	return out
}
