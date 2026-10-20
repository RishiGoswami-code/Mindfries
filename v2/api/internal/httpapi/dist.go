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

// CLI distribution. With V2_DIST_DIR set to a folder of CLI builds
// (mindfries_<os>_<arch>) this server hosts the CLI itself, so a candidate on a
// fresh machine runs one command:
//
//	curl -fsSL https://<engine>/install.sh | sh
//	irm https://<engine>/install.ps1 | iex
//
// With V2_CLI_DOWNLOAD_URL set instead, the installers point there (GitHub
// Releases, say) and this server hosts only the scripts. The scripts are
// served with their download address filled in, and checksums.txt is worked
// out from the builds.

func (s *Server) registerDist(mux *http.ServeMux) {
	mux.HandleFunc("GET /install.sh", s.installer(install.Shell, "text/x-shellscript; charset=utf-8"))
	mux.HandleFunc("GET /install.ps1", s.installer(install.PowerShell, "text/plain; charset=utf-8"))
	mux.HandleFunc("GET /cli/latest/checksums.txt", s.cliChecksums)
	mux.HandleFunc("GET /cli/latest/{name}", s.cliBuild)
}

// installer serves install.sh / install.ps1 with the download address filled
// in. It needs somewhere to download from, or it says so.
func (s *Server) installer(body, contentType string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !s.canInstall() {
			problem(w, http.StatusServiceUnavailable, "not_configured", "no CLI downloads are set up (V2_CLI_DOWNLOAD_URL or V2_CLI_DIST_DIR)")
			return
		}
		w.Header().Set("Content-Type", contentType)
		w.Header().Set("Cache-Control", "no-store")
		_, _ = io.WriteString(w, strings.ReplaceAll(body, install.DefaultDownload, s.downloadBase(r)))
	}
}

var buildName = regexp.MustCompile(`^mindfries_[a-z]+_[a-z0-9]+(\.exe)?$`)

func (s *Server) cliBuild(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if !s.hostsBuilds() || !buildName.MatchString(name) {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	http.ServeFile(w, r, filepath.Join(s.cfg.CLIDistDir, name))
}

func (s *Server) cliChecksums(w http.ResponseWriter, r *http.Request) {
	if !s.hostsBuilds() {
		http.NotFound(w, r)
		return
	}
	ents, err := os.ReadDir(s.cfg.CLIDistDir)
	if err != nil {
		problem(w, http.StatusInternalServerError, "internal", err.Error())
		return
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
		f, err := os.Open(filepath.Join(s.cfg.CLIDistDir, n))
		if err != nil {
			problem(w, http.StatusInternalServerError, "internal", err.Error())
			return
		}
		h := sha256.New()
		_, err = io.Copy(h, f)
		f.Close()
		if err != nil {
			problem(w, http.StatusInternalServerError, "internal", err.Error())
			return
		}
		fmt.Fprintf(&sb, "%s  %s\n", hex.EncodeToString(h.Sum(nil)), n)
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = io.WriteString(w, sb.String())
}
