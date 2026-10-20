package watcher

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/mindfries/v2/cli/internal/state"
)

// The inbox is how another mindfries command gives the running watcher an
// event to record. The watcher owns the queue (each event is chained to the
// one before it), so a command never writes to the queue while the watcher
// runs: it leaves a file here and the watcher appends it on its next scan.

type posted struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

func inboxDir(dir string) string { return filepath.Join(dir, "inbox") }

// Post hands an event to the running watcher. It reports false, without
// writing anything, when no watcher is running to take it.
func Post(dir, typ string, data any) bool {
	if !Running(dir) {
		return false
	}
	raw, err := json.Marshal(data)
	if err != nil {
		return false
	}
	b, err := json.Marshal(posted{Type: typ, Data: raw})
	if err != nil {
		return false
	}
	if err := os.MkdirAll(inboxDir(dir), 0o700); err != nil {
		return false
	}
	name := fmt.Sprintf("%020d-%d.json", time.Now().UnixNano(), os.Getpid())
	return state.WriteFileAtomic(filepath.Join(inboxDir(dir), name), b, 0o600) == nil
}

// drainInbox appends what other commands have posted, oldest first.
func (w *Watcher) drainInbox() {
	dir := inboxDir(w.cfg.StateDir)
	ents, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	var names []string
	for _, e := range ents {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".json") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	for _, n := range names {
		p := filepath.Join(dir, n)
		b, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		_ = os.Remove(p)
		var ev posted
		if json.Unmarshal(b, &ev) == nil && ev.Type != "" {
			w.emit(ev.Type, ev.Data)
		}
	}
}
