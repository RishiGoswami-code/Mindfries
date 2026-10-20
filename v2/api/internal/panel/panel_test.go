package panel

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func ids(tabs []Tab) []string {
	var out []string
	for _, t := range tabs {
		out = append(out, t.ID)
	}
	return out
}

func TestTabsForEveryKindOfCandidate(t *testing.T) {
	cases := []struct {
		name  string
		modes []string
		kind  string
		want  []string
	}{
		{"a backend engineer on P1", []string{"implementation", "debugging", "testing", "delivery"}, "debug", []string{"notes", "resources", "debug", "test", "activity"}},
		{"a front-end engineer", []string{"implementation", "debugging", "testing"}, "feature", []string{"notes", "resources", "debug", "test", "activity"}},
		{"an architect", []string{"system_design", "research"}, "design", []string{"design", "notes", "resources", "activity"}},
		{"a quant", []string{"comprehension", "research"}, "review", []string{"notes", "resources", "review", "activity"}},
		{"a forward-deployed engineer", []string{"implementation", "debugging", "research", "communication"}, "feature", []string{"notes", "resources", "debug", "activity"}},
		{"nothing known", nil, "", []string{"notes", "resources", "activity"}},
		{"the task's kind adds its tab", []string{"implementation"}, "review", []string{"notes", "resources", "review", "activity"}},
		{"case and spacing don't matter", []string{" System Design ", "TESTING"}, "", []string{"design", "notes", "resources", "test", "activity"}},
	}
	for _, c := range cases {
		if got := ids(TabsFor(c.modes, c.kind)); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: %v, want %v", c.name, got, c.want)
		}
	}
	for _, tb := range TabsFor([]string{"system_design"}, "") {
		if tb.Label == "" {
			t.Errorf("%s has no label", tb.ID)
		}
	}
}

func TestFirstTab(t *testing.T) {
	tabs := TabsFor([]string{"debugging", "testing"}, "debug")
	if FirstTab(tabs, "debug") != "debug" || FirstTab(tabs, "feature") != "debug" {
		t.Fatal("the task's own mode, else the first working tab")
	}
	if FirstTab(TabsFor(nil, ""), "") != "notes" {
		t.Fatal("notes when there's nothing else")
	}
	if FirstTab(TabsFor([]string{"system_design"}, ""), "design") != "design" {
		t.Fatal("design")
	}
}

func raw(v any) json.RawMessage { b, _ := json.Marshal(v); return b }

func TestValidateAcceptsWhatThePanelSends(t *testing.T) {
	good := map[string]any{
		Diagram: map[string]any{"title": "x", "nodes": []map[string]any{
			{"id": "a", "x": 1, "y": 2, "w": 100, "h": 50, "title": "A", "tone": "red", "icon": "database"},
			{"id": "b", "x": 5, "y": 5, "w": 100, "h": 50, "title": "B", "tone": "plaid", "icon": "rocket"},
		}, "edges": []map[string]any{{"id": "e1", "from": "a", "to": "b"}}, "groups": []map[string]any{{"id": "g", "nodes": []string{"a"}}}},
		Notes:      map[string]any{"text": "## Decisions\n- cache reads"},
		Review:     map[string]any{"comments": []map[string]any{{"file": "a.go", "line": 3, "text": "singleflight?"}}},
		Hypotheses: map[string]any{"items": []map[string]any{{"text": "off by one in the loop", "status": "open"}}},
	}
	for kind, body := range good {
		out, err := Validate(kind, raw(body))
		if err != nil {
			t.Errorf("%s: %v", kind, err)
			continue
		}
		if !json.Valid(out) {
			t.Errorf("%s: output isn't JSON", kind)
		}
	}
	// Unknown tones and icons fall back rather than failing a candidate's work.
	out, _ := Validate(Diagram, raw(good[Diagram]))
	if !strings.Contains(string(out), `"tone":"blue"`) || !strings.Contains(string(out), `"icon":"box"`) {
		t.Errorf("fallbacks: %s", out)
	}
}

func TestValidateRefusesWhatCouldHurtTheReader(t *testing.T) {
	huge := strings.Repeat("x", MaxArtifactBytes+1)
	bad := []struct {
		name, kind string
		body       json.RawMessage
	}{
		{"unknown kind", "poster", raw(map[string]any{})},
		{"empty", Notes, nil},
		{"too big", Notes, json.RawMessage(`{"text":"` + huge + `"}`)},
		{"not json", Diagram, json.RawMessage(`<script>`)},
		{"arrow to nowhere", Diagram, raw(map[string]any{"title": "t", "nodes": []map[string]any{{"id": "a", "w": 1, "h": 1}}, "edges": []map[string]any{{"id": "e", "from": "a", "to": "zzz"}}})},
		{"duplicate node ids", Diagram, raw(map[string]any{"title": "t", "nodes": []map[string]any{{"id": "a", "w": 1, "h": 1}, {"id": "a", "w": 1, "h": 1}}})},
		{"node off the map", Diagram, raw(map[string]any{"title": "t", "nodes": []map[string]any{{"id": "a", "x": 1e9, "w": 1, "h": 1}}})},
		{"node with no size", Diagram, raw(map[string]any{"title": "t", "nodes": []map[string]any{{"id": "a"}}})},
		{"group of nobody", Diagram, raw(map[string]any{"title": "t", "nodes": []map[string]any{}, "groups": []map[string]any{{"id": "g", "nodes": []string{"q"}}}})},
		{"comment with no file", Review, raw(map[string]any{"comments": []map[string]any{{"line": 1, "text": "x"}}})},
		{"hypothesis with a made-up status", Hypotheses, raw(map[string]any{"items": []map[string]any{{"text": "x", "status": "proven"}}})},
		{"notes as a list", Notes, raw([]string{"a"})},
	}
	for _, b := range bad {
		if _, err := Validate(b.kind, b.body); err == nil {
			t.Errorf("%s should be refused", b.name)
		}
	}
}

func TestResources(t *testing.T) {
	dir := t.TempDir()
	_ = os.MkdirAll(filepath.Join(dir, "docs"), 0o755)
	_ = os.WriteFile(filepath.Join(dir, "README.md"), []byte("x"), 0o644)
	_ = os.WriteFile(filepath.Join(dir, "docs", "architecture.md"), []byte("x"), 0o644)
	res := ResourcesFor("See https://www.postgresql.org/docs/current/indexes.html, and https://redis.io/docs/. Again https://redis.io/docs/.", dir)
	var titles []string
	for _, r := range res {
		titles = append(titles, r.Title)
	}
	want := []string{"TASK.md", "README.md", "docs/architecture.md", "www.postgresql.org", "redis.io"}
	if !reflect.DeepEqual(titles, want) {
		t.Fatalf("%v", titles)
	}
	if res[3].Href != "https://www.postgresql.org/docs/current/indexes.html" || res[4].Kind != "link" {
		t.Fatalf("%+v", res)
	}
	if got := ResourcesFor("", ""); len(got) != 1 {
		t.Fatalf("just the brief: %+v", got)
	}
}

func TestReviewAndDebugMaterial(t *testing.T) {
	dir := t.TempDir()
	_ = os.MkdirAll(filepath.Join(dir, "overlay", "internal"), 0o755)
	_ = os.WriteFile(filepath.Join(dir, "overlay", "internal", "a.go"), []byte("package a\n\nfunc A() {}\n"), 0o644)
	_ = os.WriteFile(filepath.Join(dir, "overlay", "blob.bin"), []byte{0xff, 0xfe, 0x00}, 0o644)
	files := ReviewFromOverlay(filepath.Join(dir, "overlay"))
	if len(files) != 1 || files[0].Path != "internal/a.go" || len(files[0].Lines) != 3 || files[0].Lines[2].Text != "func A() {}" || files[0].Lines[2].N != 3 {
		t.Fatalf("%+v", files)
	}
	if got := DebugLog(dir); got == nil || len(got) != 0 {
		t.Fatalf("no logs shipped: %#v", got)
	}
	_ = os.WriteFile(filepath.Join(dir, "logs.txt"), []byte("12:00 INFO a\n12:01 WARN b\n"), 0o644)
	if got := DebugLog(dir); len(got) != 2 || got[1] != "12:01 WARN b" {
		t.Fatalf("%v", got)
	}
}
