package fakeapi

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"slices"
	"sort"
	"strconv"
	"time"

	"github.com/mindfries/v2/cli/internal/api"
	"github.com/mindfries/v2/cli/internal/trust"
)

func init() {
	register(func(s *Server, route func(string, http.HandlerFunc)) {
		route("GET /session/bundle", s.authed(s.handleManifest))
		route("GET /session/bundle/archive", s.authed(s.handleArchive))
	})
}

// built is a task packed for serving.
type built struct {
	manifest api.BundleManifest
	archive  []byte
}

func sha(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

// build packs the task deterministically and signs the manifest with the
// dev key.
func (t *Task) build() (*built, error) {
	paths := make([]string, 0, len(t.Files))
	for p := range t.Files {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	m := api.BundleManifest{
		TaskVersion:      t.Version,
		Git:              api.BundleGit{Branch: t.Branch, AuthorName: t.AuthorName, AuthorEmail: t.AuthorEmail},
		RequiredRuntimes: t.Runtimes,
		SetupCommands:    t.SetupCommands,
		CheckCommands:    t.CheckCommands,
	}
	epoch := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for _, p := range paths {
		body := []byte(t.Files[p])
		mode := int64(0o644)
		exec := slices.Contains(t.Executable, p)
		if exec {
			mode = 0o755
		}
		if err := tw.WriteHeader(&tar.Header{Name: p, Mode: mode, Size: int64(len(body)), ModTime: epoch, Typeflag: tar.TypeReg, Format: tar.FormatPAX}); err != nil {
			return nil, err
		}
		if _, err := tw.Write(body); err != nil {
			return nil, err
		}
		m.Files = append(m.Files, api.BundleFile{Path: p, SHA256: sha(body), Bytes: int64(len(body)), Executable: exec, Locked: slices.Contains(t.Locked, p)})
	}
	if err := tw.Close(); err != nil {
		return nil, err
	}
	if err := gz.Close(); err != nil {
		return nil, err
	}
	m.ArchiveSHA256 = sha(buf.Bytes())
	m.ArchiveBytes = int64(buf.Len())
	if err := trust.SignManifest(&m, trust.DevKeyID, trust.DevPrivateKey()); err != nil {
		return nil, err
	}
	return &built{manifest: m, archive: buf.Bytes()}, nil
}

func (ss *session) bundle() (*built, error) {
	if ss.ext.built == nil {
		b, err := ss.task.build()
		if err != nil {
			return nil, err
		}
		ss.ext.built = b
	}
	return ss.ext.built, nil
}

func (s *Server) handleManifest(w http.ResponseWriter, r *http.Request, ss *session) {
	if !requireActive(w, ss) {
		return
	}
	b, err := ss.bundle()
	if err != nil {
		problem(w, http.StatusInternalServerError, api.CodeInternal, "Couldn't build the task", err.Error())
		return
	}
	m := b.manifest
	if s.tamper.manifest {
		m.Files = append([]api.BundleFile(nil), m.Files...)
		m.Files[0].SHA256 = sha([]byte("tampered"))
	}
	writeJSON(w, http.StatusOK, m)
}

func (s *Server) handleArchive(w http.ResponseWriter, r *http.Request, ss *session) {
	if !requireActive(w, ss) {
		return
	}
	b, err := ss.bundle()
	if err != nil {
		problem(w, http.StatusInternalServerError, api.CodeInternal, "Couldn't build the task", err.Error())
		return
	}
	data := b.archive
	if s.tamper.archive {
		data = tamperArchive(ss.task)
	}
	if s.tamper.truncateArchiveOnce {
		s.tamper.truncateArchiveOnce = false
		// Send the headers for the full archive, then drop the connection
		// halfway, as a flaky network would.
		w.Header().Set("Content-Type", "application/gzip")
		w.Header().Set("Content-Length", strconv.Itoa(len(data)))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(data[:len(data)/2])
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		hangUp(w)
		return
	}
	w.Header().Set("Content-Type", "application/gzip")
	http.ServeContent(w, r, "task.tar.gz", time.Time{}, bytes.NewReader(data))
}

// tamperArchive packs the task with one file changed after signing.
func tamperArchive(t *Task) []byte {
	c := *t
	c.Files = map[string]string{}
	for k, v := range t.Files {
		c.Files[k] = v
	}
	for k := range c.Files {
		c.Files[k] += "\n# changed in transit\n"
		break
	}
	b, _ := c.build()
	return b.archive
}

// tampering switches, for tests.
type tampering struct {
	manifest            bool
	archive             bool
	truncateArchiveOnce bool
}

// TamperManifest makes the manifest's first file hash wrong (after signing).
func (s *Server) TamperManifest(on bool) { s.mu.Lock(); s.tamper.manifest = on; s.mu.Unlock() }

// TamperArchive serves an archive whose contents don't match the manifest.
func (s *Server) TamperArchive(on bool) { s.mu.Lock(); s.tamper.archive = on; s.mu.Unlock() }

// TruncateArchiveOnce cuts the next archive download off halfway.
func (s *Server) TruncateArchiveOnce() {
	s.mu.Lock()
	s.tamper.truncateArchiveOnce = true
	s.mu.Unlock()
}
