// Package cli is the mindfries command line: argument parsing and the six
// commands. main only calls Run, so every command can be driven from tests.
package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/mindfries/v2/cli/internal/api"
	"github.com/mindfries/v2/cli/internal/buildinfo"
	"github.com/mindfries/v2/cli/internal/sys"
	"github.com/mindfries/v2/cli/internal/trust"
	"github.com/mindfries/v2/cli/internal/ui"
	"github.com/mindfries/v2/cli/internal/watcher"
)

// App is one invocation's environment.
type App struct {
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
	Getenv func(string) string
	// Sys is the machine; nil means the real one.
	Sys *sys.Sys

	prompt *ui.UI
	setup  setupResult
	// tuneWatcher lets tests shorten the watcher's intervals.
	tuneWatcher func(*watcher.Config)
	// tunePoll lets tests shorten the browser sign-in's polling interval.
	tunePoll func(time.Duration) time.Duration
	// selfPath is this program, for git's credential helper; "" means
	// os.Executable. Tests set it to a built mindfries.
	selfPath string
	// monitoring is the running session's, for the watcher.
	monitoring *api.Monitoring
}

// Default is the real process environment.
func Default() *App {
	return &App{Stdin: os.Stdin, Stdout: os.Stdout, Stderr: os.Stderr, Getenv: os.Getenv, Sys: sys.Real()}
}

func (a *App) sys() *sys.Sys {
	if a.Sys == nil {
		a.Sys = sys.Real()
	}
	return a.Sys
}

// apiURL picks the API: the flag, then $MINDFRIES_API, then the built-in.
func (a *App) apiURL(flagValue string) string {
	if flagValue != "" {
		return flagValue
	}
	if v := a.Getenv("MINDFRIES_API"); v != "" {
		return v
	}
	return buildinfo.DefaultAPI
}

func (a *App) client(flagValue string) *api.Client {
	return api.New(a.apiURL(flagValue), buildinfo.Version)
}

type command struct {
	summary string
	run     func(a *App, args []string) error
}

var commands = map[string]command{
	"setup":  {"Set the assessment up: task, codebase, GitHub (the clock doesn't start)", (*App).setupCmd},
	"start":  {"Begin: pick your editor, check this computer, turn on monitoring, start the clock", (*App).start},
	"agent":  {"Open mindfries agent, the coding and research assistant", (*App).agent},
	"status": {"Show the session, sync and Git state", (*App).status},
	"check":  {"Run the task's checks and record the result", (*App).check},
	"submit": {"Review the diff and hand in", (*App).submit},
	"stop":   {"End the session and clean up (your code stays)", (*App).stop},
	"doctor": {"Run the preflight checks only", (*App).doctor},
	// Hidden: git calls it, not the candidate.
	"git-credential": {"", (*App).gitCredential},
}

// order is how help lists the commands.
var order = []string{"setup", "start", "agent", "status", "check", "submit", "stop", "doctor"}

// ExitError carries a specific exit code.
type ExitError struct {
	Code int
	Err  error
}

func (e *ExitError) Error() string {
	if e.Err == nil {
		return fmt.Sprintf("exit %d", e.Code)
	}
	return e.Err.Error()
}
func (e *ExitError) Unwrap() error { return e.Err }

// Run executes args (without the program name) and returns the exit code.
func (a *App) Run(args []string) int {
	a.devKeyOptIn()
	if len(args) == 0 {
		a.usage(a.Stderr)
		return 2
	}
	switch args[0] {
	case "--version", "-version", "version", "-v":
		fmt.Fprintf(a.Stdout, "mindfries %s (%s)\n", buildinfo.Version, buildinfo.Commit)
		if buildinfo.IsDev() {
			fmt.Fprintf(a.Stdout, "dev build for %s", buildinfo.DefaultAPI)
			if trust.AllowDevKeyAnywhere {
				fmt.Fprint(a.Stdout, ", trusting the fake API's development key")
			}
			fmt.Fprintln(a.Stdout)
		}
		return 0
	case "help", "--help", "-h", "-help":
		a.usage(a.Stdout)
		return 0
	}
	cmd, ok := commands[args[0]]
	if !ok {
		fmt.Fprintf(a.Stderr, "mindfries: unknown command %q\n\n", args[0])
		a.usage(a.Stderr)
		return 2
	}
	err := cmd.run(a, args[1:])
	if err == nil {
		return 0
	}
	if errors.Is(err, flag.ErrHelp) {
		return 0
	}
	var ee *ExitError
	if errors.As(err, &ee) {
		if ee.Err != nil {
			a.printError(ee.Err)
		}
		return ee.Code
	}
	a.printError(err)
	return 1
}

func (a *App) printError(err error) {
	fmt.Fprintln(a.Stderr, renderError(err))
}

func (a *App) usage(w io.Writer) {
	fmt.Fprintf(w, "mindfries %s — the Mindfries assessment CLI\n\nUsage:\n  mindfries <command> [flags]\n\nCommands:\n", buildinfo.Version)
	for _, n := range order {
		fmt.Fprintf(w, "  %-8s %s\n", n, commands[n].summary)
	}
	fmt.Fprintln(w, "\nRun 'mindfries <command> --help' for a command's flags.")
}

// newFlags makes a flag set for a command that writes help to stdout.
func (a *App) newFlags(name, usage string) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(a.Stdout)
	fs.Usage = func() {
		fmt.Fprintf(a.Stdout, "Usage: mindfries %s\n\n%s\n\nFlags:\n", name, usage)
		fs.PrintDefaults()
	}
	return fs
}

// parse parses flags and positional arguments in any order, so
// "start CODE --dir x" works as well as "start --dir x CODE".
func parse(fs *flag.FlagSet, args []string) ([]string, error) {
	var pos []string
	for {
		if err := fs.Parse(args); err != nil {
			return nil, err
		}
		rest := fs.Args()
		if len(rest) == 0 {
			return pos, nil
		}
		if rest[0] == "--" {
			return append(pos, rest[1:]...), nil
		}
		pos = append(pos, rest[0])
		args = rest[1:]
	}
}

// renderError turns an error into what the candidate reads.
func renderError(err error) string {
	msg := err.Error()
	if !strings.HasPrefix(msg, "mindfries:") {
		msg = "mindfries: " + msg
	}
	return msg
}

// devKeyOptIn lets a dev build trust the fake API's signing key from a
// remote server (a throwaway deployment of cmd/fakeapi): either baked in at
// build time (buildinfo.TrustDevKey) or, as a developer override,
// MINDFRIES_TRUST_DEV_KEY=1. Release builds ignore both.
func (a *App) devKeyOptIn() {
	trust.AllowDevKeyAnywhere = false
	env := a.Getenv("MINDFRIES_TRUST_DEV_KEY") == "1"
	if !env && buildinfo.TrustDevKey != "1" {
		return
	}
	if !buildinfo.IsDev() {
		if env {
			fmt.Fprintln(a.Stderr, "mindfries: MINDFRIES_TRUST_DEV_KEY is ignored by release builds.")
		}
		return
	}
	trust.AllowDevKeyAnywhere = true
	if env {
		fmt.Fprintln(a.Stderr, "mindfries: dev build trusting the fake API's development key from any server (MINDFRIES_TRUST_DEV_KEY=1). Only for testing.")
	}
}
