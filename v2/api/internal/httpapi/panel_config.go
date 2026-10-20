package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"path/filepath"
	"time"

	"github.com/mindfries/v2/api/internal/activity"
	"github.com/mindfries/v2/api/internal/core"
	"github.com/mindfries/v2/api/internal/panel"
	"github.com/mindfries/v2/api/internal/session"
	"github.com/mindfries/v2/api/internal/store"
	"github.com/mindfries/v2/api/internal/taskgen"
)

// B6: the panel's configuration and what the candidate makes in it. One
// request gives the page everything it needs to draw itself for this
// candidate: which tabs (from the role's modes and the task's kind), the task
// material those tabs show, and whatever the candidate already saved, so a
// reload or a second device picks up where they were.

// PanelStore is the artifacts and check runs (implemented by store.DB).
type PanelStore interface {
	SaveArtifact(ctx context.Context, sessionID, kind string, body []byte, expect *int, at time.Time) (int, error)
	Artifacts(ctx context.Context, sessionID string) (map[string]store.Artifact, error)
	ArtifactVersions(ctx context.Context, sessionID, kind string) ([]store.Artifact, error)
	SaveCheckRun(ctx context.Context, c store.CheckRun) (bool, error)
	LatestCheckRun(ctx context.Context, sessionID string) (store.CheckRun, bool, error)
}

func (s *Server) registerPanelConfig(mux *http.ServeMux) {
	mux.HandleFunc("GET /v2/panel/config", s.panelAuth(s.panelConfig))
	mux.HandleFunc("PUT /v2/panel/artifacts/{kind}", s.panelAuth(s.panelSaveArtifact))
}

// taskOf finds the session's task in its product's bank.
func (s *Server) taskOf(sess session.Session) (taskgen.BankTask, bool) {
	dir := filepath.Join(s.productsRoot, sess.ProductSlug)
	bank, err := s.bankFor(sess.ProductSlug, dir)
	if err != nil {
		return taskgen.BankTask{}, false
	}
	for _, t := range bank {
		if t.ID == sess.BankTaskID {
			return t, true
		}
	}
	return taskgen.BankTask{}, false
}

// modesFor are the modes the manifest lists for the session's role.
func (s *Server) modesFor(ctx context.Context, sess session.Session) (productName string, modes []string) {
	products, err := s.reg.Products(ctx)
	if err != nil {
		return sess.ProductSlug, nil
	}
	for _, p := range products {
		if p.Slug != sess.ProductSlug {
			continue
		}
		for _, m := range p.Manifest.Roles[core.Role(sess.Role)].Modes {
			modes = append(modes, string(m))
		}
		return p.Name, modes
	}
	return sess.ProductSlug, nil
}

func (s *Server) panelConfig(w http.ResponseWriter, r *http.Request, sess session.Session) {
	ctx := r.Context()
	cfg := s.configFor(ctx, sess)
	productName, modes := s.modesFor(ctx, sess)
	task, haveTask := s.taskOf(sess)
	kind := ""
	if haveTask {
		kind = string(task.Kind)
	}

	tabs := panel.TabsFor(modes, kind)
	if len(cfg.TabsOverride) > 0 {
		tabs = tabs[:0]
		for _, id := range cfg.TabsOverride {
			tabs = append(tabs, panel.TabsFor([]string{id}, "")...)
		}
		tabs = dedupeTabs(append(tabs, panel.TabsFor(nil, "")...))
	}

	var saved map[string]store.Artifact
	if s.panelStore != nil {
		saved, _ = s.panelStore.Artifacts(ctx, sess.ID)
	}
	var counts map[string]int
	if s.acts != nil {
		counts, _ = s.acts.ActivityCounts(ctx, sess.ID)
	}
	for i := range tabs {
		if tabs[i].ID == "activity" {
			tabs[i].Badge = counts["flag"]
		}
	}

	body := func(kind string, into any) int {
		a, ok := saved[kind]
		if !ok {
			return 0
		}
		_ = json.Unmarshal(a.Body, into)
		return a.Version
	}
	diagram := map[string]any{"title": "Untitled Diagram", "nodes": []any{}, "edges": []any{}}
	diagramV := body(panel.Diagram, &diagram)
	notes := struct {
		Text string `json:"text"`
	}{}
	notesV := body(panel.Notes, &notes)
	review := struct {
		Comments []any `json:"comments"`
	}{Comments: []any{}}
	reviewV := body(panel.Review, &review)
	hyp := struct {
		Items []any `json:"items"`
	}{Items: []any{}}
	hypV := body(panel.Hypotheses, &hyp)

	productDir := filepath.Join(s.productsRoot, sess.ProductSlug)
	reviewFiles := []panel.ReviewFile{}
	debugLog := []string{}
	if haveTask {
		if task.Kind == core.Review || task.Kind == core.Design {
			reviewFiles = panel.ReviewFromOverlay(filepath.Join(task.Dir, "overlay"))
		}
		debugLog = panel.DebugLog(task.Dir)
	}

	test := map[string]any{"title": "No checks run yet", "failing": []string{}, "passing": []string{}}
	if s.panelStore != nil {
		if run, ok, _ := s.panelStore.LatestCheckRun(ctx, sess.ID); ok {
			test = map[string]any{"title": run.Command, "failing": nonNilStrings(run.Failing), "passing": nonNilStrings(run.Passing),
				"passed": run.Passed, "ran_at": run.RanAt, "output_tail": run.OutputTail}
		}
	}

	var feed []activity.Item
	if s.acts != nil {
		feed, _ = s.acts.ActivityAfter(ctx, sess.ID, 0, 200)
	}
	if feed == nil {
		feed = []activity.Item{}
	}
	title := sess.AssessmentTitle
	if haveTask && task.Title != "" {
		title = task.Title
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"sessionId":       sess.ID,
		"assessmentTitle": sess.AssessmentTitle,
		"taskTitle":       title,
		"taskKind":        kind,
		"role":            sess.Role,
		"product":         productName,
		"status":          sess.Status,
		"startedAt":       sess.StartedAt,
		"endsAt":          sess.EndsAt,
		"tabs":            tabs,
		"firstTab":        panel.FirstTab(tabs, kind),
		"monitoring":      map[string]bool{"camera": cfg.RequireCamera, "screen": cfg.RequireScreen, "audio": cfg.RequireAudio},
		"avatar":          map[string]any{"name": "Aria", "enabled": cfg.AvatarEnabled && s.cfg.HasGemini()},
		"diagram":         diagram,
		"notes":           notes.Text,
		"resources":       panel.ResourcesFor(sess.Brief, productDir),
		"review":          map[string]any{"title": "Review: " + title, "files": reviewFiles, "comments": review.Comments},
		"debug":           map[string]any{"title": "Logs: " + title, "log": debugLog, "hypotheses": hyp.Items},
		"test":            test,
		"activity":        feed,
		"versions":        map[string]int{panel.Diagram: diagramV, panel.Notes: notesV, panel.Review: reviewV, panel.Hypotheses: hypV},
	})
}

func nonNilStrings(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

func dedupeTabs(in []panel.Tab) []panel.Tab {
	seen := map[string]bool{}
	var out []panel.Tab
	order := []string{"design", "notes", "resources", "review", "debug", "test", "activity"}
	for _, t := range in {
		seen[t.ID] = true
		_ = t
	}
	for _, id := range order {
		if seen[id] {
			for _, t := range in {
				if t.ID == id {
					out = append(out, t)
					break
				}
			}
		}
	}
	return out
}

// panelSaveArtifact stores a new version of the diagram, notes, review
// comments or hypotheses. The page autosaves; every version is kept, so the
// reviewer can replay how the candidate worked.
func (s *Server) panelSaveArtifact(w http.ResponseWriter, r *http.Request, sess session.Session) {
	if s.panelStore == nil {
		problem(w, http.StatusServiceUnavailable, "not_configured", "saving isn't set up on this server")
		return
	}
	kind := r.PathValue("kind")
	if !panel.ValidKind(kind) {
		problem(w, http.StatusNotFound, "not_found", "no such artifact")
		return
	}
	if !sess.Live() {
		problemWith(w, http.StatusConflict, "session_not_active", "the session has ended", map[string]any{"session_status": sess.Status})
		return
	}
	var req struct {
		Version *int            `json:"version"`
		Body    json.RawMessage `json:"body"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, panel.MaxArtifactBytes+4096)).Decode(&req); err != nil {
		problem(w, http.StatusBadRequest, "malformed", "expected {version?, body}")
		return
	}
	clean, err := panel.Validate(kind, req.Body)
	if err != nil {
		problem(w, http.StatusBadRequest, "malformed", err.Error())
		return
	}
	now := s.now()
	v, err := s.panelStore.SaveArtifact(r.Context(), sess.ID, kind, clean, req.Version, now)
	if errors.Is(err, store.ErrVersionConflict) {
		problemWith(w, http.StatusConflict, "conflict", "this was changed in another window; reload to see the latest",
			map[string]any{"current_version": v})
		return
	}
	if err != nil {
		problem(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	if v == 1 {
		s.track(r.Context(), sess.ID, activity.Item{
			At: now, Severity: activity.Info, Source: activity.Panel, Category: activity.Observed, Kind: "artifact." + kind,
			Text: artifactStarted[kind], DedupeKey: "artifact:" + kind,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"version": v})
}

var artifactStarted = map[string]string{
	panel.Diagram:    "Started the system design diagram.",
	panel.Notes:      "Started taking notes.",
	panel.Review:     "Added the first code review comment.",
	panel.Hypotheses: "Wrote the first debugging hypothesis.",
}
