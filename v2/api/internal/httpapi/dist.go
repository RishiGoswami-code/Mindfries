package httpapi

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/mindfries/v2/cli/install"
)

// CLI distribution: with V2_DIST_DIR set to a folder of CLI builds
// (mindfries_<os>_<arch>), the engine hosts the CLI itself, so a candidate on a
// fresh machine runs one command:
//
//	curl -fsSL https://<engine>/install.sh | sh
//	irm https://<engine>/install.ps1 | iex
//
// The installers are served with their download address pointed back here, and
// checksums.txt is computed from the builds. (Ported from v2/cli/fakeapi.)

var buildName = regexp.MustCompile(`^mindfries_[a-z]+_[a-z0-9]+(\.exe)?$`)

func (s *Server) registerDist(mux *http.ServeMux) {
	dir := os.Getenv("V2_DIST_DIR")
	if dir == "" {
		return
	}
	script := func(body, contentType string) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", contentType)
			w.Header().Set("Cache-Control", "no-store")
			_, _ = io.WriteString(w, strings.ReplaceAll(body, install.DefaultDownload, publicBase(r)+"/cli/latest"))
		}
	}
	mux.HandleFunc("GET /install.sh", script(install.Shell, "text/x-shellscript; charset=utf-8"))
	mux.HandleFunc("GET /install.ps1", script(install.PowerShell, "text/plain; charset=utf-8"))
	mux.HandleFunc("GET /cli/latest/checksums.txt", func(w http.ResponseWriter, r *http.Request) {
		sums, err := checksums(dir)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		_, _ = io.WriteString(w, sums)
	})
	mux.HandleFunc("GET /cli/latest/{name}", func(w http.ResponseWriter, r *http.Request) {
		name := r.PathValue("name")
		if !buildName.MatchString(name) {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/octet-stream")
		http.ServeFile(w, r, filepath.Join(dir, name))
	})
}

// publicBase is the engine's address as the candidate's shell sees it, for the
// installer's download URL: V2_PUBLIC_URL, else Render's external URL, else the
// request's own host.
func publicBase(r *http.Request) string {
	if u := os.Getenv("V2_PUBLIC_URL"); u != "" {
		return strings.TrimRight(u, "/")
	}
	if u := os.Getenv("RENDER_EXTERNAL_URL"); u != "" {
		return strings.TrimRight(u, "/")
	}
	scheme := "https"
	if r.TLS == nil && r.Header.Get("X-Forwarded-Proto") != "https" {
		scheme = "http"
	}
	return scheme + "://" + r.Host
}

func checksums(dir string) (string, error) {
	ents, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}
	var names []string
	for _, e := range ents {
		if !e.IsDir() && buildName.MatchString(e.Name()) {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	var sb strings.Builder
	for _, n := range names {
		f, err := os.Open(filepath.Join(dir, n))
		if err != nil {
			return "", err
		}
		h := sha256.New()
		_, err = io.Copy(h, f)
		f.Close()
		if err != nil {
			return "", err
		}
		fmt.Fprintf(&sb, "%s  %s\n", hex.EncodeToString(h.Sum(nil)), n)
	}
	return sb.String(), nil
}
