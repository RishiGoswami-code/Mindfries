package activity

import (
	"encoding/json"
	"fmt"
	"path"
	"strings"
	"time"
)

// Event is one of the CLI's watcher events, as stored.
type Event struct {
	Seq  int64
	At   time.Time
	Type string
	Data json.RawMessage
}

// Thresholds. A burst of code is worth a flag (a prompt to look) from here up;
// below it, a notice. Neither says anyone did anything wrong.
const (
	BurstFlagLines   = 150
	UnknownNoteLines = 80
)

// FromEvent turns a watcher event into the tracker lines it deserves: none for
// routine events (file changes are far too many), one for most, and plain
// wording that says what was recorded, never what it means. Unknown event
// types make nothing, so a newer CLI never breaks an older server.
func FromEvent(e Event) []Item {
	mk := func(sev, cat, kind, text string) []Item {
		return []Item{{
			At: e.At, Severity: sev, Source: Watcher, Category: cat, Kind: kind, Text: text,
			EventSeq: &e.Seq, DedupeKey: fmt.Sprintf("ev:%d", e.Seq),
		}}
	}
	switch e.Type {
	case "preflight.result":
		var d struct {
			Checks []struct {
				Name   string `json:"name"`
				OK     bool   `json:"ok"`
				Detail string `json:"detail"`
			} `json:"checks"`
		}
		if json.Unmarshal(e.Data, &d) != nil {
			return nil
		}
		var failed []string
		for _, c := range d.Checks {
			if !c.OK {
				failed = append(failed, strings.TrimSpace(c.Name+" "+c.Detail))
			}
		}
		if len(failed) > 0 {
			return mk(Notice, Observed, "preflight.failed", "Computer checks that didn't pass: "+strings.Join(failed, "; ")+".")
		}
		return mk(Info, Verified, "preflight.ok", fmt.Sprintf("Computer checks passed (%d).", len(d.Checks)))

	case "workspace.ready":
		var d struct {
			Resumed bool `json:"resumed"`
		}
		_ = json.Unmarshal(e.Data, &d)
		if d.Resumed {
			return mk(Info, Observed, "workspace.resumed", "The workspace was reopened.")
		}
		return mk(Info, Observed, "workspace.ready", "The task and codebase are set up on your computer.")

	case "ide.launched":
		var d struct {
			IDE string `json:"ide"`
			OK  bool   `json:"ok"`
		}
		if json.Unmarshal(e.Data, &d) != nil {
			return nil
		}
		if !d.OK {
			return mk(Notice, Observed, "ide.failed", "Couldn't open "+IDEName(d.IDE)+" automatically.")
		}
		return mk(Info, Observed, "ide.launched", "Opened the workspace in "+IDEName(d.IDE)+".")

	case "file.burst":
		var d struct {
			Paths      []string `json:"paths"`
			LinesAdded int      `json:"lines_added"`
			WindowMS   int      `json:"window_ms"`
		}
		if json.Unmarshal(e.Data, &d) != nil {
			return nil
		}
		sev := Notice
		if d.LinesAdded >= BurstFlagLines {
			sev = Flag
		}
		return mk(sev, Signal, "file.burst", fmt.Sprintf("%d lines arrived across %s within %s, not through the assistant. A reviewer may want to look.",
			d.LinesAdded, plural(len(d.Paths), "file"), (time.Duration(d.WindowMS)*time.Millisecond).Round(time.Second)))

	case "change.attributed":
		var d struct {
			Path   string `json:"path"`
			Origin string `json:"origin"`
			Lines  int    `json:"lines"`
		}
		if json.Unmarshal(e.Data, &d) != nil {
			return nil
		}
		switch {
		case d.Origin == "assistant" && d.Lines > 0:
			return mk(Info, Observed, "change.assistant", fmt.Sprintf("%s in %s came from the coding assistant.", plural(d.Lines, "line"), path.Base(d.Path)))
		case d.Origin == "unknown" && d.Lines >= UnknownNoteLines:
			return mk(Notice, Signal, "change.unknown", fmt.Sprintf("%s appeared in %s that weren't built up gradually or by the assistant.", plural(d.Lines, "line"), path.Base(d.Path)))
		}
		return nil

	case "task_file.modified":
		var d struct {
			Path     string `json:"path"`
			Restored bool   `json:"restored"`
		}
		if json.Unmarshal(e.Data, &d) != nil {
			return nil
		}
		text := d.Path + " was changed."
		if d.Restored {
			text = d.Path + " was changed and Mindfries restored it."
		}
		return mk(Flag, Observed, "task_file.modified", text)

	case "git.commit":
		var d struct {
			Commit       string `json:"commit"`
			FilesChanged int    `json:"files_changed"`
			AgentTrailer string `json:"agent_trailer"`
		}
		if json.Unmarshal(e.Data, &d) != nil {
			return nil
		}
		short := d.Commit
		if len(short) > 7 {
			short = short[:7]
		}
		if d.AgentTrailer != "" {
			return mk(Flag, Signal, "git.commit.agent", fmt.Sprintf("Commit %s carries a coding-agent line: %q.", short, d.AgentTrailer))
		}
		return mk(Info, Observed, "git.commit", fmt.Sprintf("Commit %s: %s changed.", short, plural(d.FilesChanged, "file")))

	case "agent_signal.process":
		var d struct {
			Matches []struct {
				Agent       string `json:"agent"`
				ProcessName string `json:"process_name"`
			} `json:"matches"`
			Phase             string `json:"phase"`
			ClosedByCandidate bool   `json:"closed_by_candidate"`
		}
		if json.Unmarshal(e.Data, &d) != nil || len(d.Matches) == 0 {
			return nil
		}
		names := make([]string, 0, len(d.Matches))
		for _, m := range d.Matches {
			names = append(names, m.ProcessName)
		}
		list := strings.Join(names, ", ")
		if d.Phase == "start" {
			if d.ClosedByCandidate {
				return mk(Info, Observed, "agent.process.closed", list+" was running before the start and was closed.")
			}
			return mk(Notice, Observed, "agent.process.start", list+" was running when the assessment started.")
		}
		return mk(Flag, Observed, "agent.process", list+" is running. Outside coding agents aren't allowed; noted for the reviewer.")

	case "agent_signal.extension":
		var d struct {
			IDE        string   `json:"ide"`
			Extensions []string `json:"extensions"`
		}
		if json.Unmarshal(e.Data, &d) != nil || len(d.Extensions) == 0 {
			return nil
		}
		return mk(Notice, Signal, "agent.extension", "AI extensions are installed in "+IDEName(d.IDE)+": "+strings.Join(d.Extensions, ", ")+".")

	case "agent_signal.leftover":
		var d struct {
			Paths []string `json:"paths"`
		}
		if json.Unmarshal(e.Data, &d) != nil || len(d.Paths) == 0 {
			return nil
		}
		return mk(Notice, Signal, "agent.leftover", "Coding-agent instruction files are in the workspace: "+strings.Join(d.Paths, ", ")+".")

	case "watcher.gap":
		var d struct {
			From   time.Time `json:"from"`
			To     time.Time `json:"to"`
			Reason string    `json:"reason"`
		}
		if json.Unmarshal(e.Data, &d) != nil {
			return nil
		}
		return mk(Notice, Observed, "watcher.gap", fmt.Sprintf("The watcher wasn't running for %s (%s). Recorded as an evidence gap.",
			d.To.Sub(d.From).Round(time.Second), gapReason(d.Reason)))

	case "watcher.stopped":
		var d struct {
			Reason string `json:"reason"`
		}
		_ = json.Unmarshal(e.Data, &d)
		return mk(Info, Observed, "watcher.stopped", "The watcher stopped ("+d.Reason+").")

	}
	return nil
}

// IDEName is a person's name for an editor id.
func IDEName(id string) string {
	switch id {
	case "vscode":
		return "VS Code"
	case "jetbrains", "pycharm":
		return "a JetBrains editor"
	case "zed":
		return "Zed"
	case "manual":
		return "your own editor"
	}
	return "an editor"
}

func gapReason(r string) string {
	switch r {
	case "sleep":
		return "the computer slept"
	case "crash":
		return "it crashed"
	case "restart":
		return "it was restarted"
	}
	return "reason unknown"
}

func plural(n int, noun string) string {
	if n == 1 {
		return fmt.Sprintf("1 %s", noun)
	}
	return fmt.Sprintf("%d %ss", n, noun)
}
