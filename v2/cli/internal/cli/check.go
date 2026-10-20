package cli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/mindfries/v2/cli/internal/api"
	"github.com/mindfries/v2/cli/internal/shell"
	"github.com/mindfries/v2/cli/internal/state"
	"github.com/mindfries/v2/cli/internal/watcher"
)

// checkEvent is the data of a check.run event (contracts/cli-api.yaml).
type checkEvent struct {
	Command    string   `json:"command"`
	Passed     bool     `json:"passed"`
	Failing    []string `json:"failing"`
	Passing    []string `json:"passing"`
	OutputTail string   `json:"output_tail"`
	DurationMS int64    `json:"duration_ms"`
}

// checkRun is one of the task's check commands, run.
type checkRun struct {
	Result api.CheckResult
	Event  checkEvent
	Tests  testNames
	// Err is why it couldn't start; TimedOut that checkTimeout ended it.
	Err      error
	TimedOut bool
}

func (c checkRun) failed() bool { return !c.Event.Passed }

func (a *App) check(args []string) error {
	fs := a.newFlags("check", "Runs the task's checks, as 'mindfries submit' does, and records the result for the session.\nNothing is handed in.")
	if _, err := parse(fs, args); err != nil {
		return err
	}
	l, _, err := a.active()
	if l == nil {
		return err
	}
	// The checks run locally; a missing token only means the watcher can't
	// have been sending anything.
	if l.Workspace == "" {
		return errors.New("the workspace isn't set up yet: run 'mindfries start' first")
	}
	if len(l.CheckCommands) == 0 {
		fmt.Fprintln(a.Stdout, "This task has no checks to run.")
		return nil
	}
	home, err := a.home()
	if err != nil {
		return err
	}
	dir := state.Dir(home)
	ctx := context.Background()

	var runs []checkRun
	recorded := true
	for _, cmd := range l.CheckCommands {
		run := a.runCheck(ctx, l, cmd)
		runs = append(runs, run)
		recorded = a.recordCheck(dir, run, false) && recorded
	}

	fmt.Fprintln(a.Stdout, "\nChecks:")
	failed := 0
	for _, run := range runs {
		verdict := "passed"
		switch {
		case run.Err != nil:
			verdict = "couldn't run"
		case run.TimedOut:
			verdict = "timed out"
		case run.failed():
			verdict = fmt.Sprintf("failed (exit code %d)", run.Result.ExitCode)
		}
		if run.failed() {
			failed++
		}
		fmt.Fprintf(a.Stdout, "  %-22s %s%s, %s\n", verdict, run.Result.Command, testCounts(run.Tests), roundDuration(time.Duration(run.Event.DurationMS)*time.Millisecond))
	}
	switch {
	case recorded:
		fmt.Fprintln(a.Stdout, "The result is recorded for your session.")
	default:
		fmt.Fprintln(a.Stdout, "Not recorded: the session watcher isn't running. Run 'mindfries start' to carry on with the session.")
	}
	if failed > 0 {
		return &ExitError{Code: 1}
	}
	return nil
}

// testCounts says how many tests passed and failed, when the output told.
func testCounts(t testNames) string {
	switch {
	case t.Passed+t.Failed == 0:
		return ""
	case t.Failed == 0:
		return fmt.Sprintf(" (%d passed)", t.Passed)
	default:
		return fmt.Sprintf(" (%d passed, %d failed)", t.Passed, t.Failed)
	}
}

func roundDuration(d time.Duration) time.Duration {
	if d >= time.Second {
		return d.Round(100 * time.Millisecond)
	}
	return d.Round(time.Millisecond)
}

// runCheck runs one of the task's check commands in the workspace, showing
// its output as it comes.
func (a *App) runCheck(ctx context.Context, l *state.Local, cmd string) checkRun {
	fmt.Fprintf(a.Stdout, "\nRunning the task's check: %s\n", cmd)
	var buf bytes.Buffer
	res, err := shell.Run(ctx, l.Workspace, cmd, io.MultiWriter(a.Stdout, &buf), checkTimeout)
	run := checkRun{Err: err, TimedOut: res.TimedOut}
	run.Result = api.CheckResult{Command: cmd, ExitCode: res.ExitCode, DurationMS: res.Duration.Milliseconds(), Summary: tail(buf.String(), 20, 2000)}
	out := buf.String()
	switch {
	case err != nil:
		run.Result.ExitCode = -1
		run.Result.Summary = "couldn't run: " + err.Error()
		out = run.Result.Summary
	case res.TimedOut:
		run.Result.Summary = "timed out\n" + run.Result.Summary
	default:
		run.Tests = parseTests(out)
	}
	run.Event = checkEvent{
		Command: cmd, Passed: err == nil && !res.TimedOut && res.ExitCode == 0,
		Failing: orEmpty(run.Tests.Failing), Passing: orEmpty(run.Tests.Passing),
		OutputTail: outputTail(out), DurationMS: res.Duration.Milliseconds(),
	}
	return run
}

func orEmpty(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

// recordCheck gives the session record a check.run event. The running
// watcher takes it; with none running it is written to the queue directly
// if direct is set (submit sends the queue itself), and otherwise not
// recorded at all. It never touches the network.
func (a *App) recordCheck(dir string, run checkRun, direct bool) bool {
	if watcher.Post(dir, "check.run", run.Event) {
		return true
	}
	if !direct {
		return false
	}
	q, err := watcher.OpenQueue(dir)
	if err != nil {
		return false
	}
	_, err = q.Append("check.run", run.Event)
	return err == nil
}
