// Command fakeapi runs the fake V2 API for trying the CLI by hand:
//
//	go run ./cmd/fakeapi
//	go run ./cmd/mindfries start <code printed above> --api http://localhost:8787/v2/cli
//
// It also runs as a throwaway deployment (see v2/cli/DEPLOY-FAKEAPI.md):
// $PORT sets the port (listening on all interfaces), $RENDER_EXTERNAL_URL or
// -public sets the address it prints, and $FAKEAPI_ADMIN_TOKEN protects the
// admin endpoints. Everything is in memory and lost on restart.
package main

import (
	"flag"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/mindfries/v2/cli/fakeapi"
)

func main() {
	addr := flag.String("addr", "localhost:8787", "listen address (ignored when $PORT is set)")
	public := flag.String("public", "", "public base URL to print, e.g. https://x.onrender.com (default: $RENDER_EXTERNAL_URL or http://<addr>)")
	min := flag.String("min-version", "0.0.0", "minimum CLI version")
	flag.Parse()

	if p := os.Getenv("PORT"); p != "" {
		*addr = "0.0.0.0:" + p
	}
	base := *public
	if base == "" {
		base = os.Getenv("RENDER_EXTERNAL_URL")
	}
	if base == "" {
		base = "http://" + *addr
	}
	base = strings.TrimRight(base, "/")
	token := os.Getenv("FAKEAPI_ADMIN_TOKEN")

	s := fakeapi.New(fakeapi.Config{MinVersion: *min, AdminToken: token})
	log.Printf("fake V2 API on %s%s", base, fakeapi.Prefix)
	log.Printf("setup code: %s", s.MintSetupCode(fakeapi.SessionOptions{}))
	if token != "" {
		log.Printf("mint more: curl -X POST -H \"Authorization: Bearer <FAKEAPI_ADMIN_TOKEN>\" %s/__admin/setup-codes", base)
	} else {
		log.Printf("mint more: curl -X POST %s/__admin/setup-codes   (no admin token set: anyone can)", base)
	}
	log.Printf("try: mindfries start <code> --api %s%s", base, fakeapi.Prefix)
	srv := &http.Server{Addr: *addr, Handler: s.Handler(), ReadHeaderTimeout: 10 * time.Second}
	log.Fatal(srv.ListenAndServe())
}
