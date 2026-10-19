package secret

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFileStore(t *testing.T) {
	f := File{Dir: t.TempDir()}
	if _, err := f.Get("s1"); err != ErrNotFound {
		t.Fatalf("empty store: %v", err)
	}
	if err := f.Set("s1", "tok-123"); err != nil {
		t.Fatal(err)
	}
	if got, err := f.Get("s1"); err != nil || got != "tok-123" {
		t.Fatalf("get: %q %v", got, err)
	}
	if _, err := f.Get("other"); err != ErrNotFound {
		t.Fatalf("another session's id read the token: %v", err)
	}
	if fi, err := os.Stat(filepath.Join(f.Dir, "token")); err != nil {
		t.Fatal(err)
	} else if p := fi.Mode().Perm(); p&0o077 != 0 && os.PathSeparator == '/' {
		t.Fatalf("token file is readable by others: %v", p)
	}
	if err := f.Delete("s1"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(f.Dir, "token")); !os.IsNotExist(err) {
		t.Fatal("token file still there after Delete")
	}
	if err := f.Delete("s1"); err != nil {
		t.Fatal("second Delete should be a no-op")
	}
}

func TestSaveFallsBack(t *testing.T) {
	dir := t.TempDir()
	name, err := Save([]Store{broken{}, File{Dir: dir}}, "s", "t")
	if err != nil || name != "file" {
		t.Fatalf("fallback: %q %v", name, err)
	}
}

// TestRealKeychain exercises the platform keychain. It touches the real
// keychain, so it only runs when asked.
func TestRealKeychain(t *testing.T) {
	if os.Getenv("MINDFRIES_TEST_KEYCHAIN") == "" || Keychain == nil {
		t.Skip("set MINDFRIES_TEST_KEYCHAIN=1 to test the OS keychain")
	}
	id := "test-" + filepath.Base(t.TempDir())
	if err := Keychain.Set(id, "secret-value"); err != nil {
		t.Fatal(err)
	}
	if got, err := Keychain.Get(id); err != nil || got != "secret-value" {
		t.Fatalf("get: %q %v", got, err)
	}
	if err := Keychain.Delete(id); err != nil {
		t.Fatal(err)
	}
	if _, err := Keychain.Get(id); err != ErrNotFound {
		t.Fatalf("after delete: %v", err)
	}
}

type broken struct{}

func (broken) Name() string               { return "keychain" }
func (broken) Set(string, string) error   { return os.ErrPermission }
func (broken) Get(string) (string, error) { return "", ErrNotFound }
func (broken) Delete(string) error        { return nil }
