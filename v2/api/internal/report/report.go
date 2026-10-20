// Package report turns a session's captured evidence into the review the
// hiring team reads (PRD: "collect evidence about how the candidate worked …
// help the hiring team make a better decision"). It is computed on-read from
// the stored events — nothing here runs code or re-derives the grade; the
// objective pass/fail comes from package grade and is passed in.
package report

import (
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/mindfries/v2/api/internal/manifest"
	"github.com/mindfries/v2/api/internal/session"
)

type Report struct {
	Outcome   Outcome      `json:"outcome"`
	Submitted bool         `json:"submitted"` // false: nothing handed in yet, so submission facts below are unknown
	Effort    Effort       `json:"effort"`
	AI        AISummary    `json:"ai"`
	Changes   []FileChange `json:"changes"`
	Coverage  Coverage     `json:"coverage"`
	Flags     []string     `json:"flags"` // things a reviewer should look at
}

// Outcome is the objective result: did the submission pass the hidden tests.
type Outcome struct {
	Graded bool     `json:"graded"`
	Passed bool     `json:"passed"`
	Check  []string `json:"check,omitempty"`
}

type Effort struct {
	WindowMinutes   int  `json:"window_minutes"` // the allotted window
	ActiveMinutes   int  `json:"active_minutes"` // first edit → last commit
	FilesChanged    int  `json:"files_changed"`
	LinesAdded      int  `json:"lines_added"`
	LinesRemoved    int  `json:"lines_removed"`
	Commits         int  `json:"commits"`
	TaskFilesIntact bool `json:"task_files_intact"`
}

type AISummary struct {
	AgentsDetected           []string `json:"agents_detected"`     // e.g. ["claude-code"]
	AgentsLeftRunning        bool     `json:"agents_left_running"` // a match with closed_by_candidate=false
	AIExtensions             []string `json:"ai_extensions"`       // e.g. ["github.copilot-chat"]
	CandidateAttributedLines int      `json:"candidate_attributed_lines"`
	AgentAttributedLines     int      `json:"agent_attributed_lines"`   // edits the candidate accepted from the coding assistant
	UnknownAttributedLines   int      `json:"unknown_attributed_lines"` // arrived neither gradually nor through the assistant
}

type FileChange struct {
	Path         string `json:"path"`
	LinesAdded   int    `json:"lines_added"`
	LinesRemoved int    `json:"lines_removed"`
	Origin       string `json:"origin,omitempty"` // candidate | agent (from attribution)
}

type Coverage struct {
	Competencies []string `json:"competencies"`
	CannotAssess []string `json:"cannot_assess"`
}

// Build assembles the report. grade may be nil (not yet graded).
func Build(sess session.Session, sub session.Submission, events []session.Event, cov manifest.RoleCoverage, grade *session.Grade) Report {
	r := Report{
		Submitted: sub.ID != "",
		Effort:    Effort{WindowMinutes: sess.DurationMinutes, TaskFilesIntact: sub.TaskFilesIntact},
		Coverage:  Coverage{Competencies: cov.Competencies, CannotAssess: cov.CannotAssess},
	}
	if grade != nil {
		r.Outcome = Outcome{Graded: true, Passed: grade.Passed, Check: grade.Check}
	}

	agents := map[string]bool{}
	exts := map[string]bool{}
	changes := map[string]*FileChange{}
	var order []string
	var firstEdit, lastCommit time.Time

	for _, e := range events {
		switch e.Type {
		case "agent_signal.process":
			var d struct {
				Matches []struct {
					Agent       string `json:"agent"`
					ProcessName string `json:"process_name"`
				} `json:"matches"`
				ClosedByCandidate bool `json:"closed_by_candidate"`
			}
			_ = json.Unmarshal(e.Data, &d)
			for _, m := range d.Matches {
				agents[m.Agent] = true
			}
			if len(d.Matches) > 0 && !d.ClosedByCandidate {
				r.AI.AgentsLeftRunning = true
			}
		case "agent_signal.extension":
			var d struct {
				Extensions []string `json:"extensions"`
			}
			_ = json.Unmarshal(e.Data, &d)
			for _, x := range d.Extensions {
				exts[x] = true
			}
		case "file.changed":
			var d struct {
				Path         string `json:"path"`
				LinesAdded   int    `json:"lines_added"`
				LinesRemoved int    `json:"lines_removed"`
			}
			_ = json.Unmarshal(e.Data, &d)
			fc := changes[d.Path]
			if fc == nil {
				fc = &FileChange{Path: d.Path}
				changes[d.Path] = fc
				order = append(order, d.Path)
			}
			fc.LinesAdded += d.LinesAdded
			fc.LinesRemoved += d.LinesRemoved
			if firstEdit.IsZero() {
				firstEdit = e.OccurredAt
			}
		case "change.attributed":
			var d struct {
				Path   string `json:"path"`
				Lines  int    `json:"lines"`
				Origin string `json:"origin"`
			}
			_ = json.Unmarshal(e.Data, &d)
			if fc := changes[d.Path]; fc != nil {
				fc.Origin = d.Origin
			}
			// The CLI says where a change came from: the assistant, the
			// candidate, or neither (unknown). "agent" is the older spelling.
			switch d.Origin {
			case "assistant", "agent":
				r.AI.AgentAttributedLines += d.Lines
			case "unknown":
				r.AI.UnknownAttributedLines += d.Lines
			default:
				r.AI.CandidateAttributedLines += d.Lines
			}
		case "git.commit":
			r.Effort.Commits++
			lastCommit = e.OccurredAt
		}
	}

	for _, p := range order {
		fc := changes[p]
		r.Changes = append(r.Changes, *fc)
		r.Effort.FilesChanged++
		r.Effort.LinesAdded += fc.LinesAdded
		r.Effort.LinesRemoved += fc.LinesRemoved
	}
	if !firstEdit.IsZero() && !lastCommit.IsZero() && lastCommit.After(firstEdit) {
		r.Effort.ActiveMinutes = int(lastCommit.Sub(firstEdit).Minutes())
	}

	r.AI.AgentsDetected = sortedKeys(agents)
	r.AI.AIExtensions = sortedKeys(exts)
	r.Flags = flags(r)
	return r
}

func flags(r Report) []string {
	var f []string
	if len(r.AI.AgentsDetected) > 0 {
		f = append(f, fmt.Sprintf("A coding agent was detected running during the assessment: %v", r.AI.AgentsDetected))
	}
	if r.AI.AgentsLeftRunning {
		f = append(f, "A coding agent was still running (not closed when asked).")
	}
	if r.AI.AgentAttributedLines > 0 {
		f = append(f, fmt.Sprintf("%d changed line(s) were attributed to an AI agent, not the candidate.", r.AI.AgentAttributedLines))
	}
	if len(r.AI.AIExtensions) > 0 {
		f = append(f, fmt.Sprintf("AI editor extensions were installed: %v", r.AI.AIExtensions))
	}
	// Only a submission can say whether locked files were touched; with none,
	// there is nothing to flag.
	if r.Submitted && !r.Effort.TaskFilesIntact {
		f = append(f, "Locked task files were modified during the assessment.")
	}
	return f
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
