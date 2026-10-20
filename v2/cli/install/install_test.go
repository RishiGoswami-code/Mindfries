// These tests run the installers against a local "release" and check that
// a tampered binary is refused.
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
	"time"
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

// box keeps an installer away from the real user's PATH: install.sh gets
// its own HOME (so its own shell startup file) and install.ps1 its own
// registry key instead of HKCU\Environment.
type box struct {
	home, key string
}

func newBox(t *testing.T) *box {
	b := &box{home: t.TempDir(), key: fmt.Sprintf(`HKCU:\Software\MindfriesInstallTest\%d`, time.Now().UnixNano())}
	if runtime.GOOS == "windows" {
		t.Cleanup(func() {
			parent := b.key[:strings.LastIndex(b.key, `\`)]
			_ = exec.Command("powershell", "-NoProfile", "-Command",
				"Remove-Item -Recurse -Force -ErrorAction SilentlyContinue '"+b.key+"'; "+
					"if (-not (Get-ChildItem '"+parent+"' -ErrorAction SilentlyContinue)) { Remove-Item -Force -ErrorAction SilentlyContinue '"+parent+"' }").Run()
		})
	}
	return b
}

// userPath is the PATH install.ps1 wrote to the box's registry key.
func (b *box) userPath(t *testing.T) string {
	out, err := exec.Command("powershell", "-NoProfile", "-Command", "(Get-Item '"+b.key+"').GetValue('Path', '', 'DoNotExpandEnvironmentNames')").Output()
	if err != nil {
		t.Fatalf("reading the test PATH key: %v", err)
	}
	return strings.TrimSpace(string(out))
}

func run(t *testing.T, inst installer, base, dir, code string, b *box) (string, error) {
	cmd := inst.cmd(code)
	cmd.Env = append(os.Environ(), "MINDFRIES_DOWNLOAD="+base, "MINDFRIES_INSTALL_DIR="+dir,
		"HOME="+b.home, "SHELL=/bin/bash", "MINDFRIES_PATH_KEY="+b.key)
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
			out, err := run(t, inst, base, dir, "K7QM-2XRP-9WDA", newBox(t))
			if err != nil || !strings.Contains(out, "Installed") || !strings.Contains(out, "fakebin setup K7QM-2XRP-9WDA") {
				t.Fatalf("install: %v\n%s", err, out)
			}
			out, err = run(t, inst, base, dir, "K7QM-2XRP-9WDA", newBox(t))
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
			out, err := run(t, inst, base, dir, "K7QM-2XRP-9WDA", newBox(t))
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

func TestPutsMindfriesOnPath(t *testing.T) {
	for _, inst := range installers(t) {
		t.Run(inst.name, func(t *testing.T) {
			base := release(t, false)
			dir := t.TempDir()
			b := newBox(t)
			for i := 0; i < 2; i++ {
				out, err := run(t, inst, base, dir, "", b)
				if err != nil || !strings.Contains(out, "mindfries start") || strings.Contains(out, "fakebin") {
					t.Fatalf("run %d: %v\n%s", i+1, err, out)
				}
				if i == 0 && !strings.Contains(out, "Added "+dir+" to your PATH") {
					t.Errorf("first run didn't say it changed PATH:\n%s", out)
				}
			}
			if inst.name == "install.ps1" {
				if got := b.userPath(t); strings.Count(got, dir) != 1 {
					t.Fatalf("user PATH = %q, want %s once", got, dir)
				}
				return
			}
			// With SHELL=/bin/bash, install.sh follows the bash convention:
			// .bash_profile on macOS (login shells), .bashrc on Linux.
			rcName := ".bashrc"
			if runtime.GOOS == "darwin" {
				rcName = ".bash_profile"
			}
			rc, err := os.ReadFile(filepath.Join(b.home, rcName))
			if err != nil {
				t.Fatal(err)
			}
			if strings.Count(string(rc), "export PATH=\""+dir+":$PATH\"") != 1 {
				t.Fatalf("%s:\n%s", rcName, rc)
			}
		})
	}
}

func TestEmbedsTheInstallers(t *testing.T) {
	for name, body := range map[string]string{"install.sh": Shell, "install.ps1": PowerShell} {
		if !strings.Contains(body, DefaultDownload) {
			t.Errorf("%s doesn't use DefaultDownload, so a self-hosting server can't point it at itself", name)
		}
	}
}

func TestDevInstallBakesInTheAPI(t *testing.T) {
	const api = "https://fake.example.test/v2/cli"
	type script struct {
		name string
		cmd  *exec.Cmd
	}
	var scripts []script
	if sh, err := exec.LookPath("sh"); err == nil {
		scripts = append(scripts, script{"dev-install.sh", exec.Command(sh, "dev-install.sh", api)})
	}
	if runtime.GOOS == "windows" {
		if ps, err := exec.LookPath("powershell"); err == nil {
			scripts = append(scripts, script{"dev-install.ps1", exec.Command(ps, "-NoProfile", "-ExecutionPolicy", "Bypass", "-File", "dev-install.ps1", "-Api", api)})
		}
	}
	if len(scripts) == 0 {
		t.Skip("no shell to run an installer")
	}
	for _, s := range scripts {
		t.Run(s.name, func(t *testing.T) {
			dir, b := t.TempDir(), newBox(t)
			s.cmd.Env = append(os.Environ(), "MINDFRIES_INSTALL_DIR="+dir, "HOME="+b.home, "SHELL=/bin/bash", "MINDFRIES_PATH_KEY="+b.key)
			out, err := s.cmd.CombinedOutput()
			if err != nil || !strings.Contains(string(out), "Installed") || !strings.Contains(string(out), "mindfries start") {
				t.Fatalf("%v\n%s", err, out)
			}
			ver, err := exec.Command(installed(dir), "--version").CombinedOutput()
			if err != nil || !strings.Contains(string(ver), "dev build for "+api+", trusting the fake API's development key") {
				t.Fatalf("installed build: %v\n%s", err, ver)
			}
		})
	}
}
