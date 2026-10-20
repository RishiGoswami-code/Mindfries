package httpapi

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mindfries/v2/api/internal/taskgen"
)

func TestDistServesInstallerAndBinaries(t *testing.T) {
	wd, _ := os.Getwd() // v2/api/internal/httpapi
	dist := filepath.Clean(filepath.Join(wd, "../../dist"))
	if _, err := os.Stat(filepath.Join(dist, "mindfries_linux_amd64")); err != nil {
		t.Skip("no CLI builds in dist/ — run render-build.sh first")
	}
	t.Setenv("V2_DIST_DIR", dist)
	t.Setenv("V2_PUBLIC_URL", "https://engine.test")

	root := productsRoot(t)
	reg := loadReg(t, root, "p1-support-saas")
	srv := httptest.NewServer(New(reg, root, taskgen.NewMemCache(), nil, 3, nil, "", nil))
	defer srv.Close()

	// install.sh is served and points its download URL back at the engine.
	res, _ := http.Get(srv.URL + "/install.sh")
	b, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode != 200 || !strings.Contains(string(b), "https://engine.test/cli/latest") {
		t.Fatalf("install.sh status %d, pointed at engine: %v", res.StatusCode, strings.Contains(string(b), "https://engine.test/cli/latest"))
	}

	// checksums lists a build, and the build downloads.
	res, _ = http.Get(srv.URL + "/cli/latest/checksums.txt")
	b, _ = io.ReadAll(res.Body)
	res.Body.Close()
	if !strings.Contains(string(b), "mindfries_linux_amd64") {
		t.Fatalf("checksums missing the linux build:\n%s", b)
	}

	res, _ = http.Get(srv.URL + "/cli/latest/mindfries_linux_amd64")
	res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatalf("binary download status = %d", res.StatusCode)
	}

	// A bogus name is rejected (no path traversal / arbitrary files).
	res, _ = http.Get(srv.URL + "/cli/latest/..%2f..%2fgo.mod")
	res.Body.Close()
	if res.StatusCode == 200 {
		t.Fatal("non-build filename should not be served")
	}
}
