// Command fakeapi runs the fake V2 API for trying the CLI by hand:
//
//	go run ./cmd/fakeapi
//	go run ./cmd/mindfries start --api http://localhost:8787/v2/cli
//
// start opens the fake's sign-in page (/device) in the browser, where any
// email works and the seeded demo assessments are on offer.
//
// It also runs as a throwaway deployment (see v2/cli/DEPLOY-FAKEAPI.md):
// $PORT sets the port (listening on all interfaces), $RENDER_EXTERNAL_URL or
// -public sets its public address, $FAKEAPI_ADMIN_TOKEN protects the admin
// endpoints, and $FAKEAPI_DIST (or -dist) is a folder of CLI builds to
// host with the installers. Everything else is in memory and lost on
// restart; the demo invitations are seeded again on every start.
package main

import (
	"flag"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/mindfries/v2/cli/fakeapi"
	"github.com/mindfries/v2/cli/internal/api"
)

func main() {
	addr := flag.String("addr", "localhost:8787", "listen address (ignored when $PORT is set)")
	public := flag.String("public", "", "public base URL, e.g. https://x.onrender.com (default: $RENDER_EXTERNAL_URL or http://<addr>)")
	dist := flag.String("dist", os.Getenv("FAKEAPI_DIST"), "folder of CLI builds (mindfries_<os>_<arch>) to serve with the installers")
	min := flag.String("min-version", "0.0.0", "minimum CLI version")
	flag.Parse()

	if p := os.Getenv("PORT"); p != "" {
		*addr = "0.0.0.0:" + p
	}
	base := *public
	if base == "" {
		base = os.Getenv("RENDER_EXTERNAL_URL")
	}
	publicURL := base
	if base == "" {
		base = "http://" + *addr
	}
	base = strings.TrimRight(base, "/")
	token := os.Getenv("FAKEAPI_ADMIN_TOKEN")

	cfg := fakeapi.Config{MinVersion: *min, AdminToken: token, PublicURL: publicURL, DistDir: *dist}
	// The candidate's GitHub: the Mindfries OAuth app (its callback URL is
	// <public>/github/callback). Without it the work stays local and is
	// handed in as a bundle. Reviewers belong to each assessment: set them
	// with the admin API (/__admin/invitations), never here.
	if id := os.Getenv("FAKEAPI_GITHUB_CLIENT_ID"); id != "" {
		cfg.GitHubApp = fakeapi.GitHubApp{ClientID: id, ClientSecret: os.Getenv("FAKEAPI_GITHUB_CLIENT_SECRET")}
		cfg.GitHub = &api.GitHub{Required: os.Getenv("FAKEAPI_GITHUB_OPTIONAL") != "1", Deployment: true}
	}
	s := fakeapi.New(cfg)
	// The demos ask for camera, screen and microphone in the browser panel.
	watched := &api.Monitoring{Camera: true, Screen: true, Audio: true}
	s.Invite(fakeapi.SessionOptions{Title: "Demo assessment: rate limiter", Monitoring: watched})
	explain := fakeapi.SessionOptions{Title: "Demo assessment: explain-only assistant", Duration: 60 * time.Minute, Monitoring: watched}
	explain.Policy.AssistantMode = api.ModeExplainOnly
	s.Invite(explain)

	log.Printf("fake V2 API on %s%s", base, fakeapi.Prefix)
	log.Printf("dashboard: %s/dashboard (Start assessment gives the setup command; two demo assessments)", base)
	log.Printf("sign-in page: %s/device (for 'mindfries setup' without a code)", base)
	if cfg.GitHub != nil {
		log.Printf("GitHub: the work goes to the candidate's own GitHub (OAuth app %s, callback %s/github/callback); set each assessment's reviewers with PUT /__admin/invitations/<id>/reviewers", cfg.GitHubApp.ClientID, base)
		if cfg.GitHubApp.ClientSecret == "" {
			log.Printf("GitHub: FAKEAPI_GITHUB_CLIENT_SECRET is missing, so connecting GitHub will fail")
		}
	} else {
		log.Printf("GitHub: off (set FAKEAPI_GITHUB_CLIENT_ID to turn it on)")
	}
	if *dist != "" {
		log.Printf("install: irm %s/install.ps1 | iex      (Windows PowerShell)", base)
		log.Printf("install: curl -fsSL %s/install.sh | sh   (macOS, Linux)", base)
	}
	log.Printf("then: mindfries start")
	auth := ""
	if token != "" {
		auth = "-H \"Authorization: Bearer <FAKEAPI_ADMIN_TOKEN>\" "
	}
	log.Printf("more invitations: curl -X POST %s%s/__admin/invitations -d '{\"title\":\"...\"}'", auth, base)
	srv := &http.Server{Addr: *addr, Handler: s.Handler(), ReadHeaderTimeout: 10 * time.Second}
	log.Fatal(srv.ListenAndServe())
}
