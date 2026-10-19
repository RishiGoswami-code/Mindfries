package fakeapi

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mindfries/v2/cli/internal/api"
	"github.com/mindfries/v2/cli/internal/gitx"
)

func init() {
	register(func(s *Server, route func(string, http.HandlerFunc)) {
		route("POST /session/submission", s.authed(s.handleCreateSubmission))
		route("GET /session/submission", s.authed(s.handleGetSubmission))
		route("PUT /session/submission/{id}/bundle", s.authed(s.handleUploadBundle))
	})
}

type submission struct {
	api.Submission
	manifest api.SubmissionManifest
	raw      []byte
}

func (s *Server) handleCreateSubmission(w http.ResponseWriter, r *http.Request, ss *session) {
	var m api.SubmissionManifest
	if !decode(w, r, &m) {
		return
	}
	if m.HeadCommit == "" || m.BaseCommit == "" || m.BundleSHA256 == "" || m.TaskVersion == "" {
		problem(w, http.StatusBadRequest, api.CodeMalformed, "Incomplete submission manifest", "")
		return
	}
	if cur := ss.ext.submission; cur != nil {
		if cur.Status == api.SubmissionVerified {
			problem(w, http.StatusConflict, api.CodeAlreadySubmitted, "This assessment has already been submitted", "")
			return
		}
		raw, _ := json.Marshal(m)
		if cur.Status == api.SubmissionAwaitingBundle && bytes.Equal(raw, cur.raw) {
			writeJSON(w, http.StatusCreated, cur.Submission)
			return
		}
	}
	if !requireActive(w, ss) {
		return
	}
	if m.BundleBytes > ss.Limits.MaxBundleBytes {
		problem(w, http.StatusRequestEntityTooLarge, api.CodeTooLarge, "The submission is too large", "")
		return
	}
	raw, _ := json.Marshal(m)
	sub := &submission{Submission: api.Submission{ID: randomID("sub_", 6), Status: api.SubmissionAwaitingBundle, CreatedAt: s.now()}, manifest: m, raw: raw}
	ss.ext.submission = sub
	ss.SubmissionID = sub.ID
	writeJSON(w, http.StatusCreated, sub.Submission)
}

func (s *Server) handleGetSubmission(w http.ResponseWriter, r *http.Request, ss *session) {
	if ss.ext.submission == nil {
		problem(w, http.StatusNotFound, api.CodeNotFound, "Nothing submitted yet", "")
		return
	}
	writeJSON(w, http.StatusOK, ss.ext.submission.Submission)
}

func (s *Server) handleUploadBundle(w http.ResponseWriter, r *http.Request, ss *session) {
	sub := ss.ext.submission
	if sub == nil || sub.ID != r.PathValue("id") {
		problem(w, http.StatusNotFound, api.CodeNotFound, "No such submission", "")
		return
	}
	if sub.Status == api.SubmissionVerified {
		writeJSON(w, http.StatusOK, sub.Submission)
		return
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, ss.Limits.MaxBundleBytes+1))
	if err != nil || int64(len(body)) > ss.Limits.MaxBundleBytes {
		problem(w, http.StatusRequestEntityTooLarge, api.CodeTooLarge, "The bundle is too large", "")
		return
	}
	mismatches := verifyBundle(r.Context(), body, sub.manifest)
	if len(mismatches) > 0 {
		sub.Status = api.SubmissionRejected
		writeProblem(w, &api.Problem{Code: api.CodeBundleMismatch, Title: "The uploaded work doesn't match the submission manifest", Status: http.StatusUnprocessableEntity, Mismatches: mismatches})
		return
	}
	now := s.now()
	sub.Status = api.SubmissionVerified
	sub.VerifiedAt = &now
	sub.HeadCommit = sub.manifest.HeadCommit
	ss.ext.bundle = body
	ss.Status = api.StatusSubmitted
	writeJSON(w, http.StatusOK, sub.Submission)
}

// verifyBundle checks a Git bundle against its manifest: the hash, the
// head and base commits, and every file at the head.
func verifyBundle(ctx context.Context, body []byte, m api.SubmissionManifest) []api.Mismatch {
	sum := sha256.Sum256(body)
	if got := hex.EncodeToString(sum[:]); got != m.BundleSHA256 {
		return []api.Mismatch{{What: "bundle_sha256", Expected: m.BundleSHA256, Actual: got}}
	}
	dir, err := os.MkdirTemp("", "fakeapi-bundle-*")
	if err != nil {
		return []api.Mismatch{{What: "bundle_sha256", Actual: err.Error()}}
	}
	defer os.RemoveAll(dir)
	bundle := filepath.Join(dir, "work.bundle")
	_ = os.WriteFile(bundle, body, 0o600)
	repo := &gitx.Repo{Dir: filepath.Join(dir, "repo")}
	_ = os.Mkdir(repo.Dir, 0o700)
	if _, err := repo.Run(ctx, "init", "-q"); err != nil {
		return []api.Mismatch{{What: "head_commit", Actual: "unreadable bundle"}}
	}
	if _, err := repo.Run(ctx, "fetch", "-q", bundle, "+refs/heads/*:refs/heads/*"); err != nil {
		return []api.Mismatch{{What: "head_commit", Actual: "unreadable bundle"}}
	}
	head, err := repo.Line(ctx, "rev-parse", "--verify", "refs/heads/"+m.Branch)
	if err != nil || head != m.HeadCommit {
		return []api.Mismatch{{What: "head_commit", Expected: m.HeadCommit, Actual: head}}
	}
	if _, err := repo.Run(ctx, "merge-base", "--is-ancestor", m.BaseCommit, head); err != nil {
		return []api.Mismatch{{What: "base_commit", Expected: m.BaseCommit}}
	}
	var out []api.Mismatch
	changed, _ := repo.Run(ctx, "diff", "--name-only", "--no-renames", m.BaseCommit, head)
	actual := map[string]bool{}
	for _, p := range strings.Split(strings.TrimSpace(changed), "\n") {
		if p != "" {
			actual[p] = true
		}
	}
	declared := map[string]bool{}
	for _, f := range m.Files {
		declared[f.Path] = true
		blob, err := repo.Bytes(ctx, "cat-file", "blob", head+":"+f.Path)
		switch {
		case f.Change == "deleted":
			if err == nil {
				out = append(out, api.Mismatch{What: "file_extra", Path: f.Path})
			}
		case err != nil:
			out = append(out, api.Mismatch{What: "file_missing", Path: f.Path})
		default:
			if got := sha(blob); got != f.SHA256 {
				out = append(out, api.Mismatch{What: "file_hash", Path: f.Path, Expected: f.SHA256, Actual: got})
			}
		}
	}
	var extra []string
	for p := range actual {
		if !declared[p] {
			extra = append(extra, p)
		}
	}
	sort.Strings(extra)
	for _, p := range extra {
		out = append(out, api.Mismatch{What: "file_extra", Path: p})
	}
	return out
}

// Submission returns a session's submission and bundle, for tests.
func (s *Server) Submission(id string) (*api.Submission, []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ss := s.sessions[id]
	if ss == nil || ss.ext.submission == nil {
		return nil, nil
	}
	sub := ss.ext.submission.Submission
	return &sub, ss.ext.bundle
}
