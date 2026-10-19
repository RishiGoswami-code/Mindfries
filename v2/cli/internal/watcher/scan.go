package watcher

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mindfries/v2/cli/internal/gitx"
	"github.com/mindfries/v2/cli/internal/workspace"
)

// Change is one file changing between two scans.
type Change struct {
	Path         string
	Kind         string // created, modified, deleted
	LinesAdded   int
	LinesRemoved int
	BytesAfter   int64
	SHA256       string // after the change; "" when deleted
	At           time.Time
}

type fileState struct {
	size    int64
	modTime time.Time
	sha     string
	lines   map[string]int // nil for binary or large files
}

// maxTextBytes is the largest file whose lines are counted.
const maxTextBytes = 2 << 20

// scanner polls the workspace. It looks only at files Git would track
// (tracked, or untracked but not ignored), and only inside the workspace.
type scanner struct {
	repo  *gitx.Repo
	dir   string
	files map[string]*fileState
	skip  map[string]bool // locked files, reported separately
}

func newScanner(dir string, skip []string) *scanner {
	s := &scanner{repo: &gitx.Repo{Dir: dir}, dir: dir, files: map[string]*fileState{}, skip: map[string]bool{}}
	for _, p := range skip {
		s.skip[p] = true
	}
	return s
}

func (s *scanner) list(ctx context.Context) ([]string, error) {
	out, err := s.repo.Run(ctx, "ls-files", "-z", "--cached", "--others", "--exclude-standard")
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	var paths []string
	for _, p := range strings.Split(out, "\x00") {
		if p != "" && !seen[p] && !s.skip[p] {
			seen[p] = true
			paths = append(paths, p)
		}
	}
	return paths, nil
}

func readState(full string, fi os.FileInfo) *fileState {
	st := &fileState{size: fi.Size(), modTime: fi.ModTime()}
	b, err := os.ReadFile(full)
	if err != nil {
		return st
	}
	st.sha = workspace.HashBytes(b)
	if len(b) <= maxTextBytes && !bytes.Contains(b[:min(len(b), 8000)], []byte{0}) {
		st.lines = countLines(b)
	}
	return st
}

func countLines(b []byte) map[string]int {
	m := map[string]int{}
	for _, l := range strings.Split(strings.ReplaceAll(string(b), "\r\n", "\n"), "\n") {
		m[l]++
	}
	return m
}

// lineDelta counts lines added and removed, ignoring order: a line moved
// within a file counts as neither.
func lineDelta(old, cur map[string]int) (added, removed int) {
	for l, n := range cur {
		if d := n - old[l]; d > 0 {
			added += d
		}
	}
	for l, n := range old {
		if d := n - cur[l]; d > 0 {
			removed += d
		}
	}
	return
}

// prime records the current state without reporting changes.
func (s *scanner) prime(ctx context.Context) error {
	_, err := s.scan(ctx, time.Now(), false)
	return err
}

// scan compares the workspace with the last scan.
func (s *scanner) scan(ctx context.Context, now time.Time, report bool) ([]Change, error) {
	paths, err := s.list(ctx)
	if err != nil {
		return nil, err
	}
	var changes []Change
	present := map[string]bool{}
	for _, p := range paths {
		full := filepath.Join(s.dir, filepath.FromSlash(p))
		fi, err := os.Stat(full)
		if err != nil || fi.IsDir() {
			continue
		}
		present[p] = true
		old := s.files[p]
		if old != nil && old.size == fi.Size() && old.modTime.Equal(fi.ModTime()) {
			continue
		}
		cur := readState(full, fi)
		s.files[p] = cur
		if old != nil && old.sha == cur.sha {
			continue
		}
		if !report {
			continue
		}
		c := Change{Path: p, Kind: "modified", BytesAfter: cur.size, SHA256: cur.sha, At: now}
		var oldLines map[string]int
		if old == nil {
			c.Kind = "created"
		} else {
			oldLines = old.lines
		}
		if cur.lines != nil {
			c.LinesAdded, c.LinesRemoved = lineDelta(oldLines, cur.lines)
		}
		changes = append(changes, c)
	}
	for p, old := range s.files {
		if present[p] {
			continue
		}
		delete(s.files, p)
		if report {
			c := Change{Path: p, Kind: "deleted", At: now}
			if old.lines != nil {
				_, c.LinesRemoved = lineDelta(old.lines, nil)
			}
			changes = append(changes, c)
		}
	}
	return changes, nil
}
