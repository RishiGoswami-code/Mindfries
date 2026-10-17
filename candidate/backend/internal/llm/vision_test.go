package llm

import (
	"context"
	"encoding/base64"
	"os"
	"strings"
	"testing"
)

// TestDescribeStillLive runs the real describer against the real Gemini API.
// It is skipped unless GEMINI_API_KEY and MF_TEST_IMAGE are set, so it never
// runs in CI without a key — it is a manual end-to-end check.
func TestDescribeStillLive(t *testing.T) {
	key := os.Getenv("GEMINI_API_KEY")
	imgPath := os.Getenv("MF_TEST_IMAGE")
	if key == "" || imgPath == "" {
		t.Skip("set GEMINI_API_KEY and MF_TEST_IMAGE to run the live describer check")
	}
	raw, err := os.ReadFile(imgPath)
	if err != nil {
		t.Fatalf("reading test image: %v", err)
	}
	agents := NewAgents(NewGeminiClient(key, ""), GeminiAgentModels())
	desc, err := agents.DescribeStill(context.Background(),
		ImagePart{MimeType: "image/png", Data: base64.StdEncoding.EncodeToString(raw)},
		"A Python bug-fix task worked entirely in the workspace. The candidate was not asked to use any outside site or AI chat tool.")
	if err != nil {
		t.Fatalf("DescribeStill: %v", err)
	}
	if strings.TrimSpace(desc) == "" {
		t.Fatal("empty description")
	}
	t.Logf("DESCRIPTION: %s", desc)
}
