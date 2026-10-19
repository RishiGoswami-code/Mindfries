// Package install holds the one-line installers. These tests run them
// against a local "release" and check that a tampered binary is refused.
package install

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func binaryName() string {
	ext := ""
	if runtime.GOOS == "windows" {
		ext = ".exe"
	}
	return fmt.Sprintf("mindfries_%s_%s%s", runtime.GOOS, runtime.GOARCH, ext)
}

// release builds the fake binary and serves it with its checksum. If
// tamper is set, the binary served differs from the checksum.
func release(t *testing.T, tamper bool) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "fakebin")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	if out, err := exec.Command("go", "build", "-o", bin, "./testdata/fakebin").CombinedOutput(); err != nil {
		t.Fatalf("build: %v %s", err, out)
	}
	good, _ := os.ReadFile(bin)
	sum := sha256.Sum256(good)
	served := good
	if tamper {
		served = append(append([]byte(nil), good...), []byte("tampered")...)
	}
	files := map[string][]byte{
		"/checksums.txt":   []byte(fmt.Sprintf("%s  %s\n%s  mindfries_other_arch\n", hex.EncodeToString(sum[:]), binaryName(), strings.Repeat("0", 64))),
		"/" + binaryName(): served,
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, ok := files[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Write(b)
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}

type installer struct {
	name string
	cmd  func(code string) *exec.Cmd
}

func installers(t *testing.T) []installer {
	var out []installer
	if sh, err := exec.LookPath("sh"); err == nil {
		out = append(out, installer{"install.sh", func(code string) *exec.Cmd { return exec.Command(sh, "install.sh", code) }})
	}
	if runtime.GOOS == "windows" {
		if ps, err := exec.LookPath("powershell"); err == nil {
			out = append(out, installer{"install.ps1", func(code string) *exec.Cmd {
				return exec.Command(ps, "-NoProfile", "-ExecutionPolicy", "Bypass", "-File", "install.ps1", code)
			}})
		}
	}
	if len(out) == 0 {
		t.Skip("no shell to run an installer")
	}
	return out
}

func run(t *testing.T, inst installer, base, dir, code string) (string, error) {
	cmd := inst.cmd(code)
	cmd.Env = append(os.Environ(), "MINDFRIES_DOWNLOAD="+base, "MINDFRIES_INSTALL_DIR="+dir)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func installed(dir string) string {
	if runtime.GOOS == "windows" {
		return filepath.Join(dir, "mindfries.exe")
	}
	return filepath.Join(dir, "mindfries")
}

func TestInstallsVerifiesAndStarts(t *testing.T) {
	for _, inst := range installers(t) {
		t.Run(inst.name, func(t *testing.T) {
			base := release(t, false)
			dir := t.TempDir()
			out, err := run(t, inst, base, dir, "K7QM-2XRP-9WDA")
			if err != nil || !strings.Contains(out, "Installed") || !strings.Contains(out, "fakebin start K7QM-2XRP-9WDA") {
				t.Fatalf("install: %v\n%s", err, out)
			}
			out, err = run(t, inst, base, dir, "K7QM-2XRP-9WDA")
			if err != nil || !strings.Contains(out, "up to date") {
				t.Fatalf("second run: %v\n%s", err, out)
			}
		})
	}
}

func TestTamperedBinaryIsRefused(t *testing.T) {
	for _, inst := range installers(t) {
		t.Run(inst.name, func(t *testing.T) {
			base := release(t, true)
			dir := t.TempDir()
			out, err := run(t, inst, base, dir, "K7QM-2XRP-9WDA")
			if err == nil || !strings.Contains(out, "doesn't match the release checksum") {
				t.Fatalf("tampered binary accepted: %v\n%s", err, out)
			}
			if strings.Contains(out, "fakebin") {
				t.Fatal("the tampered binary was run")
			}
			if _, err := os.Stat(installed(dir)); !os.IsNotExist(err) {
				t.Fatal("the tampered binary was installed")
			}
		})
	}
}
