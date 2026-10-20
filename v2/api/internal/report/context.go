package report

import (
	"fmt"
	"time"
)

// Context is the rest of what a session recorded, beyond the CLI's events:
// the monitoring panel's gaps, the candidate's checks and saved work, how they
// used the assistant and the avatar, and their repository. It is gathered by
// the caller from the stores and attached with WithContext. Nothing here is a
// score; it is what a reviewer would want beside the flags.
type Context struct {
	Gaps             []Gap     `json:"gaps"`
	LatestCheck      *CheckRun `json:"latest_check,omitempty"`
	SavedArtifacts   []string  `json:"saved_artifacts"` // diagram, notes, review, hypotheses
	AssistantTurns   int       `json:"assistant_turns"`
	AvatarUtterances int       `json:"avatar_utterances"`
	Repo             *Repo     `json:"repo,omitempty"`
	Notes            []string  `json:"notes"` // plain lines built from the above
}

// Gap is a stretch where a share or the panel was off.
type Gap struct {
	Source  string     `json:"source"`
	Started time.Time  `json:"started"`
	Ended   *time.Time `json:"ended,omitempty"`
}

// CheckRun is the candidate's most recent run of the task's checks.
type CheckRun struct {
	Command string `json:"command"`
	Passed  bool   `json:"passed"`
	Failing int    `json:"failing"`
	Passing int    `json:"passing"`
}

// Repo is the candidate's own repository for the work.
type Repo struct {
	FullName  string `json:"full_name"`
	URL       string `json:"url"`
	Reviewers int    `json:"reviewers_invited"`
}

// WithContext attaches the extra evidence and the plain lines that describe
// it. A gap is a gap in the record, never a finding about the candidate.
func (r Report) WithContext(c Context) Report {
	c.Notes = nil
	for _, g := range c.Gaps {
		if g.Ended == nil {
			c.Notes = append(c.Notes, fmt.Sprintf("%s wasn't being recorded at the end (a gap in the record).", label(g.Source)))
			continue
		}
		c.Notes = append(c.Notes, fmt.Sprintf("%s wasn't recorded for %s (a gap in the record).", label(g.Source), g.Ended.Sub(g.Started).Round(time.Second)))
	}
	if c.LatestCheck != nil {
		if c.LatestCheck.Failing > 0 {
			c.Notes = append(c.Notes, fmt.Sprintf("Last check run: %d passed, %d failed (%s).", c.LatestCheck.Passing, c.LatestCheck.Failing, c.LatestCheck.Command))
		} else if c.LatestCheck.Passed {
			c.Notes = append(c.Notes, "Last check run passed ("+c.LatestCheck.Command+").")
		}
	}
	if len(c.SavedArtifacts) > 0 {
		c.Notes = append(c.Notes, fmt.Sprintf("The candidate saved work in the panel: %v.", c.SavedArtifacts))
	}
	if c.AssistantTurns > 0 {
		c.Notes = append(c.Notes, fmt.Sprintf("Asked the assistant %d time(s).", c.AssistantTurns))
	}
	if c.AvatarUtterances > 0 {
		c.Notes = append(c.Notes, fmt.Sprintf("Spoke with the avatar (%d line(s) kept).", c.AvatarUtterances))
	}
	if c.Repo != nil {
		c.Notes = append(c.Notes, "Work was pushed to their own repository: "+c.Repo.FullName+".")
	}
	if c.Gaps == nil {
		c.Gaps = []Gap{}
	}
	if c.SavedArtifacts == nil {
		c.SavedArtifacts = []string{}
	}
	r.Context = &c
	return r
}

func label(src string) string {
	switch src {
	case "camera":
		return "The camera"
	case "screen":
		return "The screen"
	case "audio":
		return "The microphone"
	case "panel":
		return "The monitoring panel"
	}
	return src
}
