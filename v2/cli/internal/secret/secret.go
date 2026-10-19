// Package secret stores the session token: in the OS keychain where one is
// usable, otherwise in a file only the user can read. The token is never
// printed or logged.
package secret

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/mindfries/v2/cli/internal/state"
)

// Service is the keychain service name.
const Service = "mindfries-cli"

// ErrNotFound means no token is stored.
var ErrNotFound = errors.New("no session token stored")

// Store keeps one token per session ID.
type Store interface {
	Name() string // "keychain" or "file"
	Set(id, token string) error
	Get(id string) (string, error)
	Delete(id string) error
}

// Keychain is the platform keychain, or nil where there is none. Tests
// replace it.
var Keychain Store = platformKeychain()

// Choose returns where a new token goes. $MINDFRIES_TOKEN_STORE=file forces
// the file; otherwise the keychain is tried first by Save.
func Choose(home string, getenv func(string) string) []Store {
	file := File{Dir: state.Dir(home)}
	if strings.EqualFold(getenv("MINDFRIES_TOKEN_STORE"), "file") || Keychain == nil {
		return []Store{file}
	}
	return []Store{Keychain, file}
}

// Save stores the token in the first store that works and returns its name.
func Save(stores []Store, id, token string) (string, error) {
	var errs []error
	for _, s := range stores {
		if err := s.Set(id, token); err == nil {
			return s.Name(), nil
		} else {
			errs = append(errs, err)
		}
	}
	return "", errors.Join(errs...)
}

// Open returns the store a token was saved in, by name.
func Open(home, name string) Store {
	if name == "keychain" && Keychain != nil {
		return Keychain
	}
	return File{Dir: state.Dir(home)}
}

// File stores the token in Dir/token with owner-only permissions. On
// Windows the file inherits the user profile's ACL, which already limits it
// to the user.
type File struct {
	Dir string
	// Base is the file's name in Dir; default "token".
	Base string
}

func (File) Name() string { return "file" }

func (f File) path() string {
	if f.Base != "" {
		return filepath.Join(f.Dir, f.Base)
	}
	return filepath.Join(f.Dir, "token")
}

// GitHubID is the store key for the candidate's GitHub token in a session.
func GitHubID(sessionID string) string { return "github:" + sessionID }

// ChooseGitHub is Choose for the GitHub token: the keychain, else its own
// owner-only file next to the session token.
func ChooseGitHub(home string, getenv func(string) string) []Store {
	file := File{Dir: state.Dir(home), Base: "github-token"}
	if strings.EqualFold(getenv("MINDFRIES_TOKEN_STORE"), "file") || Keychain == nil {
		return []Store{file}
	}
	return []Store{Keychain, file}
}

// OpenGitHub is Open for the GitHub token.
func OpenGitHub(home, name string) Store {
	if name == "keychain" && Keychain != nil {
		return Keychain
	}
	return File{Dir: state.Dir(home), Base: "github-token"}
}

func (f File) Set(id, token string) error {
	return state.WriteFileAtomic(f.path(), []byte(id+"\n"+token+"\n"), 0o600)
}

func (f File) Get(id string) (string, error) {
	b, err := os.ReadFile(f.path())
	if errors.Is(err, os.ErrNotExist) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", err
	}
	got, tok, _ := strings.Cut(strings.TrimSpace(string(b)), "\n")
	if got != id || tok == "" {
		return "", ErrNotFound
	}
	return strings.TrimSpace(tok), nil
}

func (f File) Delete(string) error {
	err := os.Remove(f.path())
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

// Memory is an in-memory store for tests.
type Memory struct{ M map[string]string }

func (*Memory) Name() string { return "keychain" }
func (m *Memory) Set(id, t string) error {
	if m.M == nil {
		m.M = map[string]string{}
	}
	m.M[id] = t
	return nil
}
func (m *Memory) Get(id string) (string, error) {
	if t, ok := m.M[id]; ok {
		return t, nil
	}
	return "", ErrNotFound
}
func (m *Memory) Delete(id string) error { delete(m.M, id); return nil }
