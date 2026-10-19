package watcher

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/mindfries/v2/cli/internal/api"
	"github.com/mindfries/v2/cli/internal/state"
	"github.com/mindfries/v2/cli/internal/workspace"
)

type snapMeta struct {
	Seq  int64  `json:"seq"`
	Hash string `json:"hash"`
}

func (w *Watcher) snapMetaPath() string { return filepath.Join(w.cfg.StateDir, "snapshot.json") }

func (w *Watcher) loadSnapMeta() {
	b, err := os.ReadFile(w.snapMetaPath())
	if err != nil {
		return
	}
	var m snapMeta
	if json.Unmarshal(b, &m) == nil {
		w.snapSeq, w.snapHash = m.Seq, m.Hash
	}
}

// diff returns `git diff --binary <base>` of the whole working tree,
// untracked files included, using a throwaway index so the candidate's own
// staging area is never touched.
func (w *Watcher) diff(ctx context.Context) ([]byte, error) {
	index := filepath.Join(w.cfg.StateDir, "snapshot.index")
	defer os.Remove(index)
	r := *w.repo
	r.Env = append(append([]string(nil), r.Env...), "GIT_INDEX_FILE="+index)
	if _, err := r.Run(ctx, "read-tree", w.cfg.Local.BaseCommit); err != nil {
		return nil, err
	}
	if _, err := r.Run(ctx, "add", "-A"); err != nil {
		return nil, err
	}
	return r.Bytes(ctx, "diff", "--cached", "--binary", "--no-color", "--no-ext-diff", w.cfg.Local.BaseCommit)
}

// snapshot sends a diff snapshot if the workspace changed since the last.
func (w *Watcher) snapshot(ctx context.Context) {
	patch, err := w.diff(ctx)
	if err != nil {
		return
	}
	sum := workspace.HashBytes(patch)
	w.mu.Lock()
	same := sum == w.snapHash
	seq := w.snapSeq
	w.mu.Unlock()
	if same {
		return
	}
	var gz bytes.Buffer
	zw := gzip.NewWriter(&gz)
	_, _ = zw.Write(patch)
	_ = zw.Close()
	if max := w.cfg.Local.Limits.MaxSnapshotBytes; max > 0 && gz.Len()*4/3 > max {
		// Too big to send; the event stream and the final bundle still
		// carry the work.
		return
	}
	head, _ := w.repo.Head(ctx)
	snap := api.Snapshot{Seq: seq + 1, TakenAt: w.now(), BaseCommit: w.cfg.Local.BaseCommit, HeadCommit: head, Patch: gz.Bytes(), PatchSHA256: sum}
	if err := w.cfg.Client.SendSnapshot(ctx, snap); err != nil {
		if api.IsRetryable(err) {
			w.markOffline()
		}
		return
	}
	w.mu.Lock()
	w.snapSeq, w.snapHash = snap.Seq, sum
	w.mu.Unlock()
	b, _ := json.Marshal(snapMeta{Seq: snap.Seq, Hash: sum})
	_ = state.WriteFileAtomic(w.snapMetaPath(), b, 0o600)
}
