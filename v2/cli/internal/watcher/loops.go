package watcher

import (
	"context"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/mindfries/v2/cli/internal/signals"
	"github.com/mindfries/v2/cli/internal/state"
)

type extraLoop struct {
	every time.Duration
	run   func(context.Context)
}

// extraLoops are periodic jobs beyond the core ones.
func (w *Watcher) extraLoops() []extraLoop {
	return []extraLoop{{every: w.cfg.AgentScan, run: w.scanAgents}}
}

// agentState is what the coding-agent checks have already reported.
type agentState struct {
	agents    map[string]bool
	ext       string
	leftovers map[string]bool
	commits   map[string]bool
	bursts    *signals.BurstDetector
}

func newAgentState() *agentState {
	return &agentState{agents: map[string]bool{}, leftovers: map[string]bool{}, commits: map[string]bool{}, bursts: signals.NewBurstDetector()}
}

// scanAgents runs signals 1 to 3 (v2/PLAN.md task 1.8).
func (w *Watcher) scanAgents(ctx context.Context) {
	w.checkProcesses(ctx)
	w.checkExtensions(ctx)
	w.checkLeftovers()
	w.checkCommits(ctx)
}

// checkProcesses reports coding agents that start running mid-session.
func (w *Watcher) checkProcesses(ctx context.Context) {
	if w.cfg.Sys == nil {
		return
	}
	found, err := signals.Processes(ctx, w.cfg.Sys)
	if err != nil {
		return
	}
	var fresh []signals.ProcessMatch
	running := map[string]bool{}
	w.mu.Lock()
	for _, m := range found {
		running[m.Agent] = true
		if !w.agents.agents[m.Agent] {
			fresh = append(fresh, m)
		}
	}
	// An agent that was closed and reopened is reported again.
	w.agents.agents = running
	w.mu.Unlock()
	if len(fresh) > 0 {
		w.emit("agent_signal.process", map[string]any{"matches": fresh, "phase": "during"})
		var names []string
		for _, m := range fresh {
			names = append(names, m.ProcessName)
		}
		w.say("Noticed %s running. Outside coding agents aren't allowed in this assessment; this has been noted for the reviewer.", strings.Join(names, ", "))
	}
}

// checkExtensions reports the editors' AI extensions when they change.
func (w *Watcher) checkExtensions(ctx context.Context) {
	if w.cfg.Sys == nil {
		return
	}
	found := signals.Extensions(ctx, w.cfg.Sys, w.cfg.VSCode)
	b, _ := json.Marshal(found)
	w.mu.Lock()
	changed := string(b) != w.agents.ext
	w.agents.ext = string(b)
	w.mu.Unlock()
	if !changed {
		return
	}
	for _, e := range found {
		w.emit("agent_signal.extension", e)
	}
}

// skipDirs are never walked for agent files.
var skipDirs = map[string]bool{".git": true, "node_modules": true, ".venv": true, "venv": true, "__pycache__": true, "target": true}

// checkLeftovers reports agent instruction files in the workspace (names
// only), including ignored ones.
func (w *Watcher) checkLeftovers() {
	root := w.cfg.Local.Workspace
	var hits []string
	_ = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		rel, _ := filepath.Rel(root, p)
		rel = filepath.ToSlash(rel)
		if d.IsDir() && skipDirs[d.Name()] {
			return filepath.SkipDir
		}
		if rel != "." && signals.IsLeftover(rel) {
			hits = append(hits, rel)
			if d.IsDir() {
				return filepath.SkipDir
			}
		}
		return nil
	})
	var fresh []string
	w.mu.Lock()
	for _, r := range signals.LeftoverRoots(hits) {
		if !w.agents.leftovers[r] {
			w.agents.leftovers[r] = true
			fresh = append(fresh, r)
		}
	}
	w.mu.Unlock()
	if len(fresh) > 0 {
		w.emit("agent_signal.leftover", map[string]any{"paths": fresh})
	}
}

func (w *Watcher) commitsPath() string { return filepath.Join(w.cfg.StateDir, "commits-seen") }

func (w *Watcher) loadCommits() {
	b, err := os.ReadFile(w.commitsPath())
	if err != nil {
		return
	}
	for _, c := range strings.Fields(string(b)) {
		w.agents.commits[c] = true
	}
}

// checkCommits records each new commit on the assessment branch, with any
// coding-agent trailer in its message.
func (w *Watcher) checkCommits(ctx context.Context) {
	out, err := w.repo.Run(ctx, "rev-list", "--reverse", w.cfg.Local.BaseCommit+"..HEAD")
	if err != nil {
		return
	}
	var fresh []string
	w.mu.Lock()
	for _, c := range strings.Fields(out) {
		if !w.agents.commits[c] {
			w.agents.commits[c] = true
			fresh = append(fresh, c)
		}
	}
	all := make([]string, 0, len(w.agents.commits))
	for c := range w.agents.commits {
		all = append(all, c)
	}
	w.mu.Unlock()
	if len(fresh) == 0 {
		return
	}
	sort.Strings(all)
	_ = state.WriteFileAtomic(w.commitsPath(), []byte(strings.Join(all, "\n")), 0o600)
	for _, c := range fresh {
		msg, _ := w.repo.Run(ctx, "log", "-1", "--format=%B", c)
		files, _ := w.repo.Run(ctx, "diff-tree", "--no-commit-id", "--name-only", "-r", c)
		data := map[string]any{"commit": c, "files_changed": len(strings.Fields(files))}
		if t := signals.AgentTrailer(msg); t != "" {
			data["agent_trailer"] = t
		}
		w.emit("git.commit", data)
	}
}

// noteArrival feeds signal 4: code arriving in bursts.
func (w *Watcher) noteArrival(c Change) {
	if c.LinesAdded == 0 {
		return
	}
	w.mu.Lock()
	b := w.agents.bursts.Observe(signals.Arrival{Path: c.Path, Lines: c.LinesAdded, At: c.At})
	w.mu.Unlock()
	if b != nil {
		w.emit("file.burst", b)
	}
}
