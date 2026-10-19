// Package watcher is the session watcher: the spec's local assessment agent
// (SPEC §9). It runs in the foreground of `mindfries start`, never as a
// service, and records only what v2/PLAN.md task 1.7 and 1.8 list: Git
// state, file changes and diff snapshots inside the workspace, and the
// coding-agent signals. Events wait in an offline queue until the server has
// them.
package watcher

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"github.com/mindfries/v2/cli/internal/api"
	"github.com/mindfries/v2/cli/internal/gitx"
	"github.com/mindfries/v2/cli/internal/signals"
	"github.com/mindfries/v2/cli/internal/state"
	"github.com/mindfries/v2/cli/internal/sys"
	"github.com/mindfries/v2/cli/internal/workspace"
)

// Why the watcher stopped.
const (
	ReasonStop        = "stop"        // mindfries stop
	ReasonSubmitted   = "submitted"   // mindfries submit
	ReasonExpired     = "expired"     // time ran out
	ReasonRevoked     = "revoked"     // ended from the portal, or token refused
	ReasonInterrupted = "interrupted" // Ctrl+C: the session carries on
)

// Config is everything the watcher needs.
type Config struct {
	Client   *api.Client
	StateDir string
	Local    *state.Local
	Queue    *Queue
	Out      io.Writer
	// Live redraws a one-line status (a terminal); otherwise only changes
	// are printed.
	Live bool

	// Intervals; zero means the session's, or the defaults.
	Scan, Send, Heartbeat, Snapshot, AgentScan time.Duration
	MaxBackoff                                 time.Duration
	Now                                        func() time.Time
	// FlushTimeout bounds the final send when stopping.
	FlushTimeout time.Duration
	// PauseAfter, for tests, ends the watch as Ctrl+C would.
	PauseAfter time.Duration

	// Sys is the machine, for the coding-agent checks; nil skips the
	// process and extension checks.
	Sys *sys.Sys
	// VSCode is the VS Code launcher, for listing its extensions.
	VSCode string
}

// Watcher runs one session's watch.
type Watcher struct {
	cfg  Config
	repo *gitx.Repo
	scan *scanner

	mu           sync.Mutex
	lastSynced   *time.Time
	offlineSince *time.Time
	endsAt       time.Time
	snapSeq      int64
	snapHash     string
	lastLine     string
	agents       *agentState

	stopOnce sync.Once
	stopCh   chan string
	wake     chan struct{}
}

func New(cfg Config) *Watcher {
	sec := func(n int, def time.Duration) time.Duration {
		if n > 0 {
			return time.Duration(n) * time.Second
		}
		return def
	}
	iv := cfg.Local.Intervals
	if cfg.Scan == 0 {
		cfg.Scan = time.Second
	}
	if cfg.Send == 0 {
		cfg.Send = 2 * time.Second
	}
	if cfg.Heartbeat == 0 {
		cfg.Heartbeat = sec(iv.HeartbeatSeconds, 30*time.Second)
	}
	if cfg.Snapshot == 0 {
		cfg.Snapshot = sec(iv.SnapshotSeconds, 120*time.Second)
	}
	if cfg.AgentScan == 0 {
		cfg.AgentScan = sec(iv.AgentScanSeconds, 180*time.Second)
	}
	if cfg.MaxBackoff == 0 {
		cfg.MaxBackoff = time.Minute
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	if cfg.FlushTimeout == 0 {
		cfg.FlushTimeout = 10 * time.Second
	}
	if cfg.Out == nil {
		cfg.Out = io.Discard
	}
	var skip []string
	for _, lf := range cfg.Local.Locked {
		skip = append(skip, lf.Path)
	}
	w := &Watcher{
		cfg:    cfg,
		repo:   &gitx.Repo{Dir: cfg.Local.Workspace},
		scan:   newScanner(cfg.Local.Workspace, skip),
		endsAt: cfg.Local.EndsAt,
		stopCh: make(chan string, 1),
		wake:   make(chan struct{}, 1),
		agents: newAgentState(),
	}
	w.loadSnapMeta()
	w.loadCommits()
	return w
}

func (w *Watcher) now() time.Time { return w.cfg.Now().UTC() }

func (w *Watcher) emit(typ string, data any) {
	if _, err := w.cfg.Queue.Append(typ, data); err != nil {
		w.say("Couldn't record an event (%v).", err)
		return
	}
	select {
	case w.wake <- struct{}{}:
	default:
	}
}

// stop asks the watcher to end, once.
func (w *Watcher) stop(reason string) {
	w.stopOnce.Do(func() { w.stopCh <- reason })
}

// Run watches until the session ends, a stop is requested, or ctx is
// cancelled (Ctrl+C), and returns why.
func (w *Watcher) Run(ctx context.Context) (string, error) {
	dir := w.cfg.StateDir
	if err := acquire(dir); err != nil {
		return "", err
	}
	defer release(dir)
	_ = os.Remove(stopPath(dir))

	started := w.now()
	w.noteGap(started)
	if err := w.scan.prime(ctx); err != nil {
		return "", fmt.Errorf("can't read the workspace: %w", err)
	}
	w.checkLocked()
	w.scanAgents(ctx)

	wctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	loop := func(every time.Duration, f func(context.Context)) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			t := time.NewTicker(every)
			defer t.Stop()
			for {
				select {
				case <-wctx.Done():
					return
				case <-t.C:
					f(wctx)
				}
			}
		}()
	}
	wg.Add(1)
	go func() { defer wg.Done(); w.sendLoop(wctx) }()
	loop(w.cfg.Scan, w.tick)
	loop(w.cfg.Heartbeat, w.heartbeat)
	loop(w.cfg.Snapshot, w.snapshot)
	for _, extra := range w.extraLoops() {
		loop(extra.every, extra.run)
	}
	loop(250*time.Millisecond, func(context.Context) { w.publish(started, false, "") })
	wg.Add(1)
	go func() { defer wg.Done(); w.heartbeat(wctx) }()

	var pauseAfter <-chan time.Time
	if w.cfg.PauseAfter > 0 {
		pauseAfter = time.After(w.cfg.PauseAfter)
	}
	var reason string
	select {
	case reason = <-w.stopCh:
	case <-ctx.Done():
		reason = ReasonInterrupted
	case <-pauseAfter:
		reason = ReasonInterrupted
	}
	cancel()
	wg.Wait()

	// Last look at the workspace, then flush.
	w.tick(context.Background())
	w.checkCommits(context.Background())
	if reason != ReasonInterrupted {
		w.emit("watcher.stopped", map[string]string{"reason": reason})
		if reason == ReasonStop || reason == ReasonSubmitted {
			w.snapshot(context.Background())
		}
	}
	if reason != ReasonRevoked {
		w.flush()
	}
	w.publish(started, true, reason)
	w.clearLine()
	return reason, nil
}

// noteGap records time the watcher wasn't running, judged from the status
// file the last run left.
func (w *Watcher) noteGap(now time.Time) {
	prev, err := ReadStatus(w.cfg.StateDir)
	if err != nil {
		return
	}
	why := "crash"
	if prev.Stopped {
		why = "restart"
	}
	if now.Sub(prev.UpdatedAt) < 2*time.Second {
		return
	}
	w.emit("watcher.gap", map[string]any{"from": prev.UpdatedAt.UTC(), "to": now, "reason": why})
}

// tick is one scan of the workspace.
func (w *Watcher) tick(ctx context.Context) {
	now := w.now()
	w.mu.Lock()
	endsAt := w.endsAt
	w.mu.Unlock()
	if !endsAt.IsZero() && now.After(endsAt) {
		w.stop(ReasonExpired)
	}
	if r, ok := takeStopRequest(w.cfg.StateDir); ok {
		w.stop(r)
	}
	w.checkLocked()
	changes, err := w.scan.scan(ctx, now, true)
	if err != nil {
		return
	}
	w.handleChanges(changes)
}

// handleChanges records file changes and looks for bursts and agent
// files among them.
func (w *Watcher) handleChanges(changes []Change) {
	leftover := false
	for _, c := range changes {
		w.emit("file.changed", map[string]any{
			"path": c.Path, "change": c.Kind,
			"lines_added": c.LinesAdded, "lines_removed": c.LinesRemoved, "bytes_after": c.BytesAfter,
		})
		w.noteArrival(c)
		leftover = leftover || (c.Kind == "created" && signals.IsLeftover(c.Path))
	}
	if leftover {
		w.checkLeftovers()
	}
}

// checkLocked restores any locked file that was changed.
func (w *Watcher) checkLocked() {
	for _, lf := range w.cfg.Local.Locked {
		if workspace.LockedIntact(w.cfg.Local.Workspace, lf) {
			continue
		}
		err := workspace.RestoreLocked(w.cfg.Local.Workspace, w.cfg.StateDir, lf)
		w.emit("task_file.modified", map[string]any{"path": lf.Path, "restored": err == nil})
		if err == nil {
			w.say("%s was changed. It's the task and is locked, so it has been put back.", lf.Path)
		} else {
			w.say("%s was changed and couldn't be put back: %v", lf.Path, err)
		}
	}
}

// sendLoop sends queued events, backing off while the network is down.
func (w *Watcher) sendLoop(ctx context.Context) {
	backoff := time.Duration(0)
	for {
		wait := w.cfg.Send
		if backoff > 0 {
			wait = backoff
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(wait):
		case <-w.wake:
			if backoff > 0 {
				continue
			}
		}
		err := w.sendOnce(ctx)
		switch {
		case err == nil:
			backoff = 0
		case api.IsRetryable(err):
			if backoff == 0 {
				backoff = w.cfg.Send
			} else {
				backoff = min(backoff*2, w.cfg.MaxBackoff)
			}
		default:
			backoff = 0
		}
	}
}

// sendOnce sends everything pending, batch by batch.
func (w *Watcher) sendOnce(ctx context.Context) error {
	lim := w.cfg.Local.Limits
	maxEv, maxBytes := lim.MaxEventsPerBatch, lim.MaxBatchBytes
	if maxEv <= 0 {
		maxEv = 500
	}
	if maxBytes <= 0 {
		maxBytes = 1 << 20
	}
	for {
		batch, err := w.cfg.Queue.Pending(maxEv, maxBytes)
		if err != nil || len(batch) == 0 {
			if err == nil {
				w.markSynced()
			}
			return err
		}
		reply, err := w.cfg.Client.SendEvents(ctx, api.EventBatch{Events: batch})
		if err != nil {
			return w.sendFailed(err, batch)
		}
		w.markSynced()
		if err := w.cfg.Queue.Ack(reply.AckedThroughSeq); err != nil {
			return err
		}
		if reply.AckedThroughSeq < batch[len(batch)-1].Seq {
			// The server stored less than was sent; try again later.
			return nil
		}
	}
}

func (w *Watcher) sendFailed(err error, batch []api.Event) error {
	switch {
	case api.HasCode(err, api.CodeTokenExpired):
		w.stop(ReasonExpired)
	case api.IsAuthFailure(err):
		w.stop(ReasonRevoked)
	case api.HasCode(err, api.CodeSessionNotActive):
		w.stop(reasonFromStatus(err))
	case api.HasCode(err, api.CodeMalformed, api.CodeTooLarge):
		// Lesson from V1's telemetry backlog: a batch the server refuses
		// is dropped, not retried for ever.
		_ = w.cfg.Queue.Ack(batch[len(batch)-1].Seq)
		w.say("The server refused %d events as malformed; they were dropped.", len(batch))
	case api.IsRetryable(err):
		w.markOffline()
	}
	return err
}

func reasonFromStatus(err error) string {
	if p, ok := api.AsProblem(err); ok {
		switch p.SessionStatus {
		case api.StatusExpired:
			return ReasonExpired
		case api.StatusSubmitted:
			return ReasonSubmitted
		}
	}
	return ReasonRevoked
}

func (w *Watcher) markSynced() {
	w.mu.Lock()
	defer w.mu.Unlock()
	now := w.now()
	w.lastSynced = &now
	if w.offlineSince != nil {
		w.offlineSince = nil
		w.sayLocked("Back online; catching up.")
	}
}

func (w *Watcher) markOffline() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.offlineSince == nil {
		now := w.now()
		w.offlineSince = &now
		w.sayLocked("Can't reach Mindfries. Keep working: everything is saved on this computer and will be sent when the connection is back.")
	}
}

// flush sends what's left, for at most FlushTimeout.
func (w *Watcher) flush() {
	ctx, cancel := context.WithTimeout(context.Background(), w.cfg.FlushTimeout)
	defer cancel()
	for w.cfg.Queue.Depth() > 0 {
		if err := w.sendOnce(ctx); err != nil && !api.IsRetryable(err) {
			return
		}
		if w.cfg.Queue.Depth() == 0 {
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(200 * time.Millisecond):
		}
	}
}

func (w *Watcher) heartbeat(ctx context.Context) {
	hb := api.Heartbeat{SentAt: w.now(), Watcher: api.HeartbeatWatch{QueueDepth: w.cfg.Queue.Depth(), LastSentSeq: w.cfg.Queue.Acked()}}
	w.mu.Lock()
	hb.Watcher.OfflineSince = w.offlineSince
	w.mu.Unlock()
	gctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	if b, err := w.repo.Branch(gctx); err == nil {
		head, _ := w.repo.Head(gctx)
		n, _ := w.repo.ChangedFiles(gctx)
		hb.Git = &api.HeartbeatGit{Branch: b, Head: head, ChangedFiles: n}
	}
	cancel()
	reply, err := w.cfg.Client.Heartbeat(ctx, hb)
	switch {
	case err == nil:
		w.mu.Lock()
		w.endsAt = reply.EndsAt
		w.mu.Unlock()
		if reply.Stop || reply.SessionStatus != api.StatusActive {
			switch reply.SessionStatus {
			case api.StatusExpired:
				w.stop(ReasonExpired)
			case api.StatusSubmitted:
				w.stop(ReasonSubmitted)
			default:
				w.stop(ReasonRevoked)
			}
		}
	case api.HasCode(err, api.CodeTokenExpired):
		w.stop(ReasonExpired)
	case api.IsAuthFailure(err):
		w.stop(ReasonRevoked)
	case api.IsRetryable(err):
		w.markOffline()
	}
}

// publish writes the status file and redraws the status line.
func (w *Watcher) publish(started time.Time, stopped bool, reason string) {
	w.mu.Lock()
	s := &Status{
		PID: os.Getpid(), StartedAt: started, UpdatedAt: w.now(),
		LastSyncedAt: w.lastSynced, OfflineSince: w.offlineSince,
		QueueDepth: w.cfg.Queue.Depth(), AckedSeq: w.cfg.Queue.Acked(),
		Stopped: stopped, StopReason: reason,
	}
	endsAt := w.endsAt
	w.mu.Unlock()
	_ = writeStatus(w.cfg.StateDir, s)
	if !w.cfg.Live || stopped {
		return
	}
	synced := "not synced yet"
	if s.OfflineSince != nil {
		synced = "offline since " + s.OfflineSince.Local().Format("15:04:05")
	} else if s.LastSyncedAt != nil {
		synced = "last synced " + s.LastSyncedAt.Local().Format("15:04:05")
	}
	left := time.Until(endsAt).Round(time.Minute)
	if left < 0 {
		left = 0
	}
	line := fmt.Sprintf("Watching · %s · %d waiting · %s left · Ctrl+C pauses", synced, s.QueueDepth, fmtDur(left))
	w.mu.Lock()
	if line != w.lastLine {
		fmt.Fprintf(w.cfg.Out, "\r\033[K%s", line)
		w.lastLine = line
	}
	w.mu.Unlock()
}

func fmtDur(d time.Duration) string {
	h, m := int(d.Hours()), int(d.Minutes())%60
	if h > 0 {
		return fmt.Sprintf("%dh%02dm", h, m)
	}
	return fmt.Sprintf("%dm", m)
}

func (w *Watcher) clearLine() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.cfg.Live && w.lastLine != "" {
		fmt.Fprint(w.cfg.Out, "\r\033[K")
		w.lastLine = ""
	}
}

func (w *Watcher) say(format string, args ...any) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.sayLocked(format, args...)
}

func (w *Watcher) sayLocked(format string, args ...any) {
	if w.cfg.Live && w.lastLine != "" {
		fmt.Fprint(w.cfg.Out, "\r\033[K")
		w.lastLine = ""
	}
	fmt.Fprintf(w.cfg.Out, format+"\n", args...)
}

// ErrNotSetUp means the session has no workspace yet.
var ErrNotSetUp = errors.New("the workspace isn't set up yet")

// Flush sends whatever is left in the queue when no watcher is running,
// for up to timeout. It reports how many events are still waiting.
func Flush(c *api.Client, q *Queue, limits api.Limits, timeout time.Duration) int {
	w := &Watcher{
		cfg:    Config{Client: c, Queue: q, Local: &state.Local{Limits: limits}, Out: io.Discard, Now: time.Now, FlushTimeout: timeout},
		stopCh: make(chan string, 1),
		wake:   make(chan struct{}, 1),
		agents: newAgentState(),
	}
	w.flush()
	return q.Depth()
}
