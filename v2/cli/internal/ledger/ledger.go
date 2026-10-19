// Package ledger records every edit `mindfries agent` applies, so the
// session watcher can tell its changes apart from the candidate's (v2/PLAN.md
// task 1.17). It lives in the session folder, outside the workspace.
package ledger

import (
	"bufio"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"

	"github.com/mindfries/v2/cli/internal/api"
)

var mu sync.Mutex

func path(stateDir string) string { return filepath.Join(stateDir, "ledger.jsonl") }

// Append records an applied edit. The agent calls it before writing the
// file, so the watcher never sees the change without its record.
func Append(stateDir string, e api.AppliedEdit) error {
	mu.Lock()
	defer mu.Unlock()
	b, err := json.Marshal(e)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(stateDir, 0o700); err != nil {
		return err
	}
	f, err := os.OpenFile(path(stateDir), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	if _, err := f.Write(append(b, '\n')); err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

// Load reads every recorded edit.
func Load(stateDir string) ([]api.AppliedEdit, error) {
	f, err := os.Open(path(stateDir))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out []api.AppliedEdit
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64*1024), 4<<20)
	for sc.Scan() {
		var e api.AppliedEdit
		if json.Unmarshal(sc.Bytes(), &e) == nil && e.EditID != "" {
			out = append(out, e)
		}
	}
	return out, sc.Err()
}
