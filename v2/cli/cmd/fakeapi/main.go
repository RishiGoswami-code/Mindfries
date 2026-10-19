// Command fakeapi runs the fake V2 API for trying the CLI by hand:
//
//	go run ./cmd/fakeapi
//	go run ./cmd/mindfries start <code printed above> --api http://localhost:8787/v2/cli
package main

import (
	"flag"
	"log"
	"net/http"

	"github.com/mindfries/v2/cli/fakeapi"
)

func main() {
	addr := flag.String("addr", "localhost:8787", "listen address")
	min := flag.String("min-version", "0.0.0", "minimum CLI version")
	flag.Parse()

	s := fakeapi.New(fakeapi.Config{MinVersion: *min})
	setup(s)
	log.Printf("fake V2 API on http://%s%s", *addr, fakeapi.Prefix)
	log.Fatal(http.ListenAndServe(*addr, s.Handler()))
}

// setup is extended as the fake gains sessions and tasks.
var setup = func(*fakeapi.Server) {}
