//go:build darwin

package secret

import (
	"errors"
	"os/exec"
	"strings"
)

// The macOS login keychain, through /usr/bin/security.

type macKeychain struct{}

func platformKeychain() Store {
	if _, err := exec.LookPath("security"); err != nil {
		return nil
	}
	return macKeychain{}
}

func (macKeychain) Name() string { return "keychain" }

func (macKeychain) Set(id, token string) error {
	// -U updates an existing item. security has no stdin option for the
	// password, so it is an argument for the moment the command runs.
	out, err := exec.Command("security", "add-generic-password", "-U", "-s", Service, "-a", id, "-w", token).CombinedOutput()
	if err != nil {
		return errors.New("keychain: " + strings.TrimSpace(string(out)))
	}
	return nil
}

func (macKeychain) Get(id string) (string, error) {
	out, err := exec.Command("security", "find-generic-password", "-s", Service, "-a", id, "-w").Output()
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) && ee.ExitCode() == 44 {
			return "", ErrNotFound
		}
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func (macKeychain) Delete(id string) error {
	err := exec.Command("security", "delete-generic-password", "-s", Service, "-a", id).Run()
	var ee *exec.ExitError
	if errors.As(err, &ee) && ee.ExitCode() == 44 {
		return nil
	}
	return err
}
