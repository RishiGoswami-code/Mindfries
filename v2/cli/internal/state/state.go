// Package state is the CLI's local record of the active session: what
// `start` set up, kept outside the workspace so the candidate's code and
// `mindfries agent` never see it. There is at most one active session.
//
// Layout under Home():
//
//	session/session.json   this file's Local
//	session/token          the token, only when the keychain isn't usable
//	session/...            the watcher's queue, status and ledger (later tasks)
package state

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/mindfries/v2/cli/internal/api"
)

// Home is where the CLI keeps its state: $MINDFRIES_HOME, or the user's
// config directory.
func Home(getenv func(string) string) (string, error) {
	if h := getenv("MINDFRIES_HOME"); h != "" {
		return h, nil
	}
	d, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("can't find a folder for mindfries' state: %w", err)
	}
	return filepath.Join(d, "mindfries"), nil
}

// Dir is the active session's folder.
func Dir(home string) string { return filepath.Join(home, "session") }

// LockedFile is a file the candidate may not change, with its hash.
type LockedFile struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

// Local is everything the CLI remembers about the active session.
type Local struct {
	APIURL       string        `json:"api_url"`
	SessionID    string        `json:"session_id"`
	AssessmentID string        `json:"assessment_id"`
	Title        string        `json:"title,omitempty"`
	EndsAt       time.Time     `json:"ends_at"`
	TokenStore   string        `json:"token_store"`
	Policy       api.Policy    `json:"policy"`
	Intervals    api.Intervals `json:"intervals"`
	Limits       api.Limits    `json:"limits"`
	ConnectedAt  time.Time     `json:"connected_at"`

	// Set once the workspace exists.
	Workspace     string        `json:"workspace,omitempty"`
	TaskVersion   string        `json:"task_version,omitempty"`
	Branch        string        `json:"branch,omitempty"`
	BaseCommit    string        `json:"base_commit,omitempty"`
	Locked        []LockedFile  `json:"locked,omitempty"`
	Runtimes      []api.Runtime `json:"runtimes,omitempty"`
	CheckCommands []string      `json:"check_commands,omitempty"`
	IDE           string        `json:"ide,omitempty"`
}

// ErrNone means there is no active session on this computer.
var ErrNone = errors.New("no active session on this computer")

// Load reads the active session.
func Load(home string) (*Local, error) {
	b, err := os.ReadFile(filepath.Join(Dir(home), "session.json"))
	if errors.Is(err, os.ErrNotExist) {
		return nil, ErrNone
	}
	if err != nil {
		return nil, err
	}
	var l Local
	if err := json.Unmarshal(b, &l); err != nil {
		return nil, fmt.Errorf("the local session record is damaged (%v); run 'mindfries stop' and the setup command again", err)
	}
	return &l, nil
}

// Save writes the active session.
func Save(home string, l *Local) error {
	b, err := json.MarshalIndent(l, "", "  ")
	if err != nil {
		return err
	}
	return WriteFileAtomic(filepath.Join(Dir(home), "session.json"), b, 0o600)
}

// Remove deletes the session folder and everything in it.
func Remove(home string) error {
	return os.RemoveAll(Dir(home))
}

// WriteFileAtomic writes via a temporary file and a rename, creating the
// parent directory (readable only by the user) if needed.
func WriteFileAtomic(path string, data []byte, perm os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".tmp-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(name)
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		os.Remove(name)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(name)
		return err
	}
	if err := os.Chmod(name, perm); err != nil {
		os.Remove(name)
		return err
	}
	return renameReplace(name, path)
}
