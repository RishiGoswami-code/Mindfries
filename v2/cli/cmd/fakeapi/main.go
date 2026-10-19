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

	s := fakeapi.New(fakeapi.Config{MinVersion: *min, AdminToken: token, PublicURL: publicURL, DistDir: *dist})
	s.Invite(fakeapi.SessionOptions{Title: "Demo assessment: rate limiter"})
	explain := fakeapi.SessionOptions{Title: "Demo assessment: explain-only assistant", Duration: 60 * time.Minute}
	explain.Policy.AssistantMode = api.ModeExplainOnly
	s.Invite(explain)

	log.Printf("fake V2 API on %s%s", base, fakeapi.Prefix)
	log.Printf("sign-in page: %s/device (any email; two demo assessments)", base)
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
