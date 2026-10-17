package orchestrator

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/mindfries/candidate-backend/internal/db"
	"github.com/mindfries/candidate-backend/internal/llm"
	"github.com/mindfries/candidate-backend/internal/storage"
)

// TestDescribedFlagsNoteLive exercises the whole caption path — parse a flag,
// fetch its still from storage, describe it with the vision model — on real
// data. Skipped unless the four env vars are set; a manual check, never CI.
func TestDescribedFlagsNoteLive(t *testing.T) {
	gkey := os.Getenv("GEMINI_API_KEY")
	surl := strings.TrimRight(os.Getenv("SUPABASE_URL"), "/")
	skey := os.Getenv("SUPABASE_SERVICE_ROLE_KEY")
	still := os.Getenv("MF_TEST_STILL")
	if gkey == "" || surl == "" || skey == "" || still == "" {
		t.Skip("set GEMINI_API_KEY, SUPABASE_URL, SUPABASE_SERVICE_ROLE_KEY and MF_TEST_STILL to run the live caption check")
	}
	o := &Orchestrator{
		Agents: llm.NewAgents(llm.NewGeminiClient(gkey, ""), llm.GeminiAgentModels()),
		Stills: storage.New(surl, skey),
	}
	start := time.Now().Add(-5 * time.Minute)
	payload, _ := json.Marshal(map[string]any{
		"kind":       "left_workspace",
		"at":         start.Add(2 * time.Minute).Format(time.RFC3339Nano),
		"screenPath": still,
	})
	events := []db.ActivityEvent{{EventType: "activity_flag", Payload: payload, OccurredAt: start.Add(2 * time.Minute)}}
	note := o.describedFlagsNote(context.Background(), "test-session", events, start,
		"A Python bug-fix task worked entirely in the workspace; no outside site was required.")
	if note == "" {
		t.Fatal("empty note")
	}
	t.Logf("NOTE:\n%s", note)
	if !strings.Contains(note, "on screen:") {
		t.Fatal("expected a caption (\"on screen:\") in the note")
	}
}
