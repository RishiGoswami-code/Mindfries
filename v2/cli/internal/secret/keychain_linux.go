//go:build linux

package secret

import (
	"errors"
	"os/exec"
	"strings"
)

// The Secret Service (GNOME Keyring, KWallet), through secret-tool. On a
// machine without it, or without a session bus, Set fails and the token
// falls back to the file.

type secretTool struct{}

func platformKeychain() Store {
	if _, err := exec.LookPath("secret-tool"); err != nil {
		return nil
	}
	return secretTool{}
}

func (secretTool) Name() string { return "keychain" }

func (secretTool) Set(id, token string) error {
	cmd := exec.Command("secret-tool", "store", "--label=Mindfries session", "service", Service, "account", id)
	cmd.Stdin = strings.NewReader(token)
	if out, err := cmd.CombinedOutput(); err != nil {
		return errors.New("keychain: " + strings.TrimSpace(string(out)))
	}
	return nil
}

func (secretTool) Get(id string) (string, error) {
	out, err := exec.Command("secret-tool", "lookup", "service", Service, "account", id).Output()
	if err != nil || len(out) == 0 {
		return "", ErrNotFound
	}
	return strings.TrimSpace(string(out)), nil
}

func (secretTool) Delete(id string) error {
	_ = exec.Command("secret-tool", "clear", "service", Service, "account", id).Run()
	return nil
}
