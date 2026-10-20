package activity

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func ev(seq int64, typ string, data any) Event {
	b, _ := json.Marshal(data)
	return Event{Seq: seq, At: time.Unix(1_800_000_000+seq, 0).UTC(), Type: typ, Data: b}
}

func one(t *testing.T, e Event) Item {
	t.Helper()
	items := FromEvent(e)
	if len(items) != 1 {
		t.Fatalf("%s: want one item, got %d", e.Type, len(items))
	}
	it := items[0]
	if !it.Valid() {
		t.Fatalf("%s: invalid item %+v", e.Type, it)
	}
	if it.EventSeq == nil || *it.EventSeq != e.Seq || it.DedupeKey == "" || it.Source != Watcher {
		t.Fatalf("%s: an item must say which event it came from: %+v", e.Type, it)
	}
	return it
}

func TestEachRuleSaysWhatWasRecordedAtTheRightSeverity(t *testing.T) {
	cases := []struct {
		name  string
		e     Event
		sev   string
		cat   string
		parts []string
	}{
		{"checks ok", ev(1, "preflight.result", map[string]any{"checks": []map[string]any{{"name": "git", "ok": true}, {"name": "ide", "ok": true}}}), Info, Verified, []string{"checks passed (2)"}},
		{"checks failed", ev(2, "preflight.result", map[string]any{"checks": []map[string]any{{"name": "disk_space", "ok": false, "detail": "1 GB free"}}}), Notice, Observed, []string{"disk_space 1 GB free"}},
		{"workspace", ev(3, "workspace.ready", map[string]any{"task_version": "v1", "resumed": false}), Info, Observed, []string{"set up on your computer"}},
		{"workspace resumed", ev(4, "workspace.ready", map[string]any{"resumed": true}), Info, Observed, []string{"reopened"}},
		{"ide", ev(5, "ide.launched", map[string]any{"ide": "vscode", "ok": true}), Info, Observed, []string{"VS Code"}},
		{"ide failed", ev(6, "ide.launched", map[string]any{"ide": "zed", "ok": false}), Notice, Observed, []string{"Couldn't open Zed"}},
		{"small burst", ev(7, "file.burst", map[string]any{"paths": []string{"a.go"}, "lines_added": 90, "window_ms": 4000}), Notice, Signal, []string{"90 lines", "1 file", "4s"}},
		{"big burst", ev(8, "file.burst", map[string]any{"paths": []string{"a.go", "b.go"}, "lines_added": 400, "window_ms": 9000}), Flag, Signal, []string{"400 lines", "2 files", "reviewer may want to look"}},
		{"assistant change", ev(9, "change.attributed", map[string]any{"path": "internal/x/y.go", "origin": "assistant", "lines": 12}), Info, Observed, []string{"12 lines in y.go", "coding assistant"}},
		{"unknown change", ev(10, "change.attributed", map[string]any{"path": "z.go", "origin": "unknown", "lines": 200}), Notice, Signal, []string{"200 lines appeared in z.go"}},
		{"task file", ev(11, "task_file.modified", map[string]any{"path": "TASK.md", "restored": true}), Flag, Observed, []string{"TASK.md was changed", "restored"}},
		{"commit", ev(12, "git.commit", map[string]any{"commit": "3f2a91c4d5e6", "files_changed": 3}), Info, Observed, []string{"Commit 3f2a91c: 3 files"}},
		{"agent commit", ev(13, "git.commit", map[string]any{"commit": "aaaaaaaaaa", "files_changed": 1, "agent_trailer": "Co-Authored-By: Some Agent"}), Flag, Signal, []string{"coding-agent line"}},
		{"agent running", ev(14, "agent_signal.process", map[string]any{"phase": "during", "matches": []map[string]any{{"agent": "claude-code", "process_name": "claude.exe"}}}), Flag, Observed, []string{"claude.exe is running", "noted for the reviewer"}},
		{"agent at start, closed", ev(15, "agent_signal.process", map[string]any{"phase": "start", "closed_by_candidate": true, "matches": []map[string]any{{"agent": "aider", "process_name": "aider"}}}), Info, Observed, []string{"aider was running before the start and was closed"}},
		{"agent at start, left on", ev(16, "agent_signal.process", map[string]any{"phase": "start", "matches": []map[string]any{{"agent": "aider", "process_name": "aider"}}}), Notice, Observed, []string{"when the assessment started"}},
		{"extension", ev(17, "agent_signal.extension", map[string]any{"ide": "vscode", "extensions": []string{"github.copilot"}}), Notice, Signal, []string{"github.copilot"}},
		{"leftover", ev(18, "agent_signal.leftover", map[string]any{"paths": []string{"CLAUDE.md", ".cursorrules"}}), Notice, Signal, []string{"CLAUDE.md, .cursorrules"}},
		{"watcher gap", ev(19, "watcher.gap", map[string]any{"from": "2026-10-20T10:00:00Z", "to": "2026-10-20T10:07:00Z", "reason": "sleep"}), Notice, Observed, []string{"7m0s", "the computer slept", "evidence gap"}},
		{"stopped", ev(20, "watcher.stopped", map[string]any{"reason": "submitted"}), Info, Observed, []string{"stopped (submitted)"}},
	}
	for _, c := range cases {
		it := one(t, c.e)
		if it.Severity != c.sev || it.Category != c.cat {
			t.Errorf("%s: %s/%s, want %s/%s (%q)", c.name, it.Severity, it.Category, c.sev, c.cat, it.Text)
		}
		for _, p := range c.parts {
			if !strings.Contains(it.Text, p) {
				t.Errorf("%s: %q should contain %q", c.name, it.Text, p)
			}
		}
	}
}

func TestRoutineAndUnknownEventsMakeNothing(t *testing.T) {
	for _, e := range []Event{
		ev(1, "file.changed", map[string]any{"path": "a.go", "change": "modified", "lines_added": 3}),
		ev(2, "change.attributed", map[string]any{"path": "a.go", "origin": "candidate", "lines": 40}),
		ev(3, "change.attributed", map[string]any{"path": "a.go", "origin": "unknown", "lines": 10}),
		ev(4, "something.from.a.newer.cli", map[string]any{"x": 1}),
		{Seq: 5, Type: "git.commit", Data: json.RawMessage(`not json`)},
		ev(6, "agent_signal.process", map[string]any{"phase": "during", "matches": []any{}}),
	} {
		if items := FromEvent(e); len(items) != 0 {
			t.Errorf("%s should make no item, got %+v", e.Type, items)
		}
	}
}

func TestWordingNeverAccuses(t *testing.T) {
	// A flag is a prompt to look. None of the wording may claim misconduct.
	banned := []string{"cheat", "cheating", "fraud", "dishonest", "violation", "guilty", "plagiar"}
	events := []Event{
		ev(1, "file.burst", map[string]any{"paths": []string{"a"}, "lines_added": 999, "window_ms": 1000}),
		ev(2, "agent_signal.process", map[string]any{"phase": "during", "matches": []map[string]any{{"agent": "x", "process_name": "x"}}}),
		ev(3, "git.commit", map[string]any{"commit": "abcdefgh", "agent_trailer": "Co-Authored-By: X"}),
		ev(4, "task_file.modified", map[string]any{"path": "TASK.md", "restored": true}),
		ev(5, "watcher.gap", map[string]any{"from": "2026-10-20T10:00:00Z", "to": "2026-10-20T10:07:00Z", "reason": "unknown"}),
	}
	for _, e := range events {
		for _, it := range FromEvent(e) {
			for _, b := range banned {
				if strings.Contains(strings.ToLower(it.Text), b) {
					t.Errorf("%q contains %q", it.Text, b)
				}
			}
		}
	}
}

func TestDedupeKeyIsStablePerEvent(t *testing.T) {
	a := FromEvent(ev(42, "git.commit", map[string]any{"commit": "abcdefgh", "files_changed": 1}))
	b := FromEvent(ev(42, "git.commit", map[string]any{"commit": "abcdefgh", "files_changed": 1}))
	if a[0].DedupeKey != b[0].DedupeKey || a[0].DedupeKey != "ev:42" {
		t.Fatal("a re-sent event must dedupe")
	}
}
