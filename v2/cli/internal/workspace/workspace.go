// Package workspace sets up the candidate's assessment folder: it downloads
// the task archive (resuming a broken download), checks every file against
// the signed manifest, creates the Git repository with a baseline commit,
// and locks the files the candidate may not change, such as TASK.md.
package workspace

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/mindfries/v2/cli/internal/api"
	"github.com/mindfries/v2/cli/internal/gitx"
	"github.com/mindfries/v2/cli/internal/state"
)

// ErrTampered means what was downloaded doesn't match the signed manifest.
var ErrTampered = errors.New("the downloaded task doesn't match its signed manifest")

// Backoff is the wait before each download retry. Tests shorten it.
var Backoff = []time.Duration{time.Second, 2 * time.Second, 4 * time.Second, 8 * time.Second}

// HashFile returns a file's SHA-256.
func HashFile(p string) (string, error) {
	f, err := os.Open(p)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// HashBytes returns the SHA-256 of b.
func HashBytes(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

// Download fetches the archive into stateDir, resuming a partial download,
// retrying with backoff, and checking the whole archive's hash. It returns
// the archive's path.
func Download(ctx context.Context, c *api.Client, m *api.BundleManifest, stateDir string, progress io.Writer) (string, error) {
	if err := os.MkdirAll(stateDir, 0o700); err != nil {
		return "", err
	}
	part := filepath.Join(stateDir, "task.tar.gz.part")
	final := filepath.Join(stateDir, "task.tar.gz")
	if h, err := HashFile(final); err == nil && h == m.ArchiveSHA256 {
		return final, nil
	}
	var lastErr error
	for attempt := 0; attempt <= len(Backoff); attempt++ {
		if attempt > 0 {
			fmt.Fprintf(progress, "Download interrupted (%v); retrying…\n", lastErr)
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-time.After(Backoff[attempt-1]):
			}
		}
		lastErr = downloadOnce(ctx, c, part)
		if lastErr == nil {
			break
		}
		if p, ok := api.AsProblem(lastErr); ok && !api.IsRetryable(p) {
			return "", lastErr
		}
	}
	if lastErr != nil {
		return "", fmt.Errorf("couldn't download the task: %w", lastErr)
	}
	fi, err := os.Stat(part)
	if err != nil {
		return "", err
	}
	if fi.Size() != m.ArchiveBytes {
		os.Remove(part)
		return "", fmt.Errorf("%w (size %d, expected %d)", ErrTampered, fi.Size(), m.ArchiveBytes)
	}
	h, err := HashFile(part)
	if err != nil {
		return "", err
	}
	if h != m.ArchiveSHA256 {
		os.Remove(part)
		return "", fmt.Errorf("%w (archive hash)", ErrTampered)
	}
	if err := os.Rename(part, final); err != nil {
		return "", err
	}
	return final, nil
}

func downloadOnce(ctx context.Context, c *api.Client, part string) error {
	f, err := os.OpenFile(part, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		return err
	}
	_, err = c.BundleArchive(ctx, f, fi.Size())
	if errors.Is(err, api.ErrRangeIgnored) {
		if err := f.Truncate(0); err != nil {
			return err
		}
		_, err = c.BundleArchive(ctx, f, 0)
	}
	return err
}

// ValidPath reports whether p is a safe relative path inside the workspace.
func ValidPath(p string) bool {
	if p == "" || strings.Contains(p, "\\") || strings.HasPrefix(p, "/") || strings.Contains(p, ":") {
		return false
	}
	clean := path.Clean(p)
	if clean != p || clean == "." || clean == ".." || strings.HasPrefix(clean, "../") {
		return false
	}
	for _, seg := range strings.Split(clean, "/") {
		if seg == ".." || strings.EqualFold(seg, ".git") {
			return false
		}
	}
	return true
}

// Extract unpacks the archive into staging, checking every entry against
// the manifest: each file's hash, no missing files and nothing extra.
func Extract(archive, staging string, m *api.BundleManifest) error {
	if err := os.RemoveAll(staging); err != nil {
		return err
	}
	if err := os.MkdirAll(staging, 0o700); err != nil {
		return err
	}
	want := map[string]api.BundleFile{}
	for _, f := range m.Files {
		if !ValidPath(f.Path) {
			return fmt.Errorf("%w (bad path %q)", ErrTampered, f.Path)
		}
		want[f.Path] = f
	}
	af, err := os.Open(archive)
	if err != nil {
		return err
	}
	defer af.Close()
	gz, err := gzip.NewReader(af)
	if err != nil {
		return fmt.Errorf("%w (%v)", ErrTampered, err)
	}
	tr := tar.NewReader(gz)
	seen := map[string]bool{}
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("%w (%v)", ErrTampered, err)
		}
		if h.Typeflag == tar.TypeDir {
			continue
		}
		bf, ok := want[h.Name]
		if !ok || h.Typeflag != tar.TypeReg || seen[h.Name] {
			return fmt.Errorf("%w (unexpected entry %q)", ErrTampered, h.Name)
		}
		seen[h.Name] = true
		dst := filepath.Join(staging, filepath.FromSlash(h.Name))
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
		if err != nil {
			return err
		}
		hasher := sha256.New()
		n, err := io.Copy(io.MultiWriter(out, hasher), io.LimitReader(tr, bf.Bytes+1))
		out.Close()
		if err != nil {
			return err
		}
		if n != bf.Bytes || hex.EncodeToString(hasher.Sum(nil)) != bf.SHA256 {
			return fmt.Errorf("%w (%s)", ErrTampered, h.Name)
		}
	}
	for p := range want {
		if !seen[p] {
			return fmt.Errorf("%w (%s is missing)", ErrTampered, p)
		}
	}
	return nil
}

// Install copies the verified files from staging into dir.
func Install(staging, dir string, m *api.BundleManifest) error {
	for _, f := range m.Files {
		src := filepath.Join(staging, filepath.FromSlash(f.Path))
		dst := filepath.Join(dir, filepath.FromSlash(f.Path))
		b, err := os.ReadFile(src)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		// A locked file from an earlier attempt is read-only.
		_ = os.Chmod(dst, 0o644)
		mode := os.FileMode(0o644)
		if f.Executable {
			mode = 0o755
		}
		if err := os.WriteFile(dst, b, mode); err != nil {
			return err
		}
		if f.Executable {
			_ = os.Chmod(dst, 0o755)
		}
	}
	return os.RemoveAll(staging)
}

// InitGit creates the repository, the assessment branch, a repository-local
// identity and the baseline commit, and returns the baseline's hash.
func InitGit(ctx context.Context, dir string, m *api.BundleManifest) (string, error) {
	r := &gitx.Repo{Dir: dir}
	steps := [][]string{
		{"init", "-q"},
		{"checkout", "-q", "-b", m.Git.Branch},
		{"config", "user.name", m.Git.AuthorName},
		{"config", "user.email", m.Git.AuthorEmail},
		{"config", "core.autocrlf", "false"},
		{"config", "commit.gpgsign", "false"},
		{"add", "-A"},
		{"commit", "-q", "--no-verify", "--allow-empty", "-m", "Mindfries task " + m.TaskVersion},
	}
	for _, s := range steps {
		if _, err := r.Run(ctx, s...); err != nil {
			return "", err
		}
	}
	return r.Head(ctx)
}

// Lock makes the locked files read-only and keeps a copy of each in
// stateDir so it can be restored after an edit.
func Lock(dir, stateDir string, m *api.BundleManifest) ([]state.LockedFile, error) {
	var out []state.LockedFile
	for _, f := range m.Files {
		if !f.Locked {
			continue
		}
		p := filepath.Join(dir, filepath.FromSlash(f.Path))
		b, err := os.ReadFile(p)
		if err != nil {
			return nil, err
		}
		if err := state.WriteFileAtomic(filepath.Join(stateDir, "locked", f.SHA256), b, 0o600); err != nil {
			return nil, err
		}
		if err := os.Chmod(p, 0o444); err != nil {
			return nil, err
		}
		out = append(out, state.LockedFile{Path: f.Path, SHA256: f.SHA256})
	}
	return out, nil
}

// LockedIntact reports whether a locked file still matches.
func LockedIntact(dir string, lf state.LockedFile) bool {
	h, err := HashFile(filepath.Join(dir, filepath.FromSlash(lf.Path)))
	return err == nil && h == lf.SHA256
}

// RestoreLocked puts a locked file back from the copy in stateDir and makes
// it read-only again.
func RestoreLocked(dir, stateDir string, lf state.LockedFile) error {
	b, err := os.ReadFile(filepath.Join(stateDir, "locked", lf.SHA256))
	if err != nil {
		return fmt.Errorf("no saved copy of %s: %w", lf.Path, err)
	}
	p := filepath.Join(dir, filepath.FromSlash(lf.Path))
	_ = os.Chmod(p, 0o644)
	_ = os.Remove(p)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(p, b, 0o444); err != nil {
		return err
	}
	return os.Chmod(p, 0o444)
}

// Unlock makes the locked files writable again, for when the session ends.
func Unlock(dir string, locked []state.LockedFile) {
	for _, lf := range locked {
		_ = os.Chmod(filepath.Join(dir, filepath.FromSlash(lf.Path)), 0o644)
	}
}

// Check describes a folder chosen for the workspace.
type Check int

const (
	FolderNew      Check = iota // doesn't exist, or empty
	FolderNotEmpty              // has files; needs the candidate's say-so
	FolderIsRepo                // already a Git repository; refused
	FolderIsFile                // not a folder
)

// CheckFolder looks at a folder before using it.
func CheckFolder(dir string) (Check, error) {
	fi, err := os.Stat(dir)
	if errors.Is(err, os.ErrNotExist) {
		return FolderNew, nil
	}
	if err != nil {
		return 0, err
	}
	if !fi.IsDir() {
		return FolderIsFile, nil
	}
	if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
		return FolderIsRepo, nil
	}
	ents, err := os.ReadDir(dir)
	if err != nil {
		return 0, err
	}
	if len(ents) == 0 {
		return FolderNew, nil
	}
	return FolderNotEmpty, nil
}

var slugRe = regexp.MustCompile(`[^a-z0-9]+`)

// DefaultFolder is where the workspace goes unless the candidate says.
func DefaultFolder(home, title string) string {
	slug := strings.Trim(slugRe.ReplaceAllString(strings.ToLower(title), "-"), "-")
	if slug == "" {
		slug = "assessment"
	}
	if len(slug) > 40 {
		slug = strings.Trim(slug[:40], "-")
	}
	return filepath.Join(home, "mindfries", slug)
}

// Present reports whether a set-up workspace is still there.
func Present(ctx context.Context, l *state.Local) bool {
	if l.Workspace == "" || l.BaseCommit == "" {
		return false
	}
	if _, err := os.Stat(filepath.Join(l.Workspace, ".git")); err != nil {
		return false
	}
	return (&gitx.Repo{Dir: l.Workspace}).HasCommit(ctx, l.BaseCommit)
}
