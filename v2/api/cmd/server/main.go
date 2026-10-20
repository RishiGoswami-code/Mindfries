// Command server is the real V2 API: the engine (products, tasks, bundles),
// the CLI session contract, the candidate's GitHub, the monitoring panel and
// the live tracker. It connects to the v2 schema in Postgres as the v2_api
// role, which can write v2 and only read V1.
//
// Deploy (e.g. Render, root directory v2/api):
//
//	build: ./render-build.sh   (or: go build -o bin/server ./cmd/server)
//	start: ./bin/server
//
// Every setting is read once by internal/config; the table in api/README.md
// lists them. Without the optional ones the matching routes answer
// "not_configured" instead of failing later.
package main

import (
	"context"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/mindfries/v2/api/internal/activity"
	"github.com/mindfries/v2/api/internal/blob"
	"github.com/mindfries/v2/api/internal/config"
	"github.com/mindfries/v2/api/internal/httpapi"
	"github.com/mindfries/v2/api/internal/jobs"
	"github.com/mindfries/v2/api/internal/llm"
	"github.com/mindfries/v2/api/internal/session"
	"github.com/mindfries/v2/api/internal/store"
	"github.com/mindfries/v2/api/internal/taskgen"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	cfg, err := config.Load(os.Getenv, true)
	if err != nil {
		log.Fatalf("configuration: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	db, err := store.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	defer db.Close()

	if cfg.SigningKey != nil {
		logger.Info("signing bundles with the production key", "key_id", cfg.SigningKeyID)
	} else {
		logger.Warn("signing bundles with the DEV key (set V2_SIGNING_KEY_ID and V2_SIGNING_PRIVATE_KEY for production)")
	}

	// Optional brief adaptation: with a Gemini key the engine rewords each
	// authored brief to the company's JD (wording only). Without one, briefs
	// ship verbatim and the engine makes no model calls.
	var provider taskgen.Provider
	if cfg.HasGemini() {
		g := llm.NewGemini(cfg.GeminiKey, cfg.GeminiModel)
		provider = g
		logger.Info("brief adaptation on", "model", g.Model())
	} else {
		logger.Info("brief adaptation off (set V2_GEMINI_API_KEY to enable)")
	}

	// Evidence goes to a private Storage bucket; without one, uploads answer
	// "not_configured".
	var blobs blob.Store
	if cfg.HasStorage() {
		blobs = blob.Supabase{BaseURL: cfg.StorageURL, Key: cfg.StorageKey, Bucket: cfg.StorageBucket}
	} else {
		logger.Warn("no evidence storage (set V2_STORAGE_URL and V2_STORAGE_KEY): panel uploads are off")
	}
	if !cfg.HasGitHub() {
		logger.Warn("GitHub is off (set V2_GITHUB_CLIENT_ID, V2_GITHUB_CLIENT_SECRET and V2_ENCRYPTION_KEY)")
	}
	if cfg.PortalSecret == "" {
		logger.Warn("V2_PORTAL_SECRET is not set: the portal routes are off")
	}

	handler := httpapi.NewServer(httpapi.Options{
		Config: cfg, Registry: db, Cache: store.NewTaskCache(db), Provider: provider,
		Sessions: db, Devices: db, Configs: db, GitHub: db, Monitor: db, Activity: db, Panel: db, Assistant: db, Avatar: db, Staff: db, Privacy: db, Compliance: db, Blobs: blobs,
		Log: logger,
	})

	// Housekeeping: end sessions whose time ran out, delete evidence past its
	// retention, clear spent codes.
	go jobs.Runner{
		Store: db, Blobs: blobs, Log: logger, RetentionDays: cfg.RetentionDays,
		OnExpire: func(ctx context.Context, s session.Session) {
			_, _, _ = db.AddActivity(ctx, s.ID, activity.Item{At: time.Now().UTC(), Severity: activity.Info, Source: activity.Server,
				Category: activity.Verified, Kind: "session.expired", Text: "Time ran out. The assessment ended.", DedupeKey: "expired"})
		},
	}.Loop(ctx, 5*time.Minute)

	srv := &http.Server{Addr: ":" + cfg.Port, Handler: handler, ReadHeaderTimeout: 10 * time.Second}
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdown)
	}()
	logger.Info("V2 API listening", "port", cfg.Port, "products", cfg.ProductsRoot)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}
