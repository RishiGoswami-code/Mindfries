// Command server runs the Helpdesk API with seeded sample data. It is the
// entry point a candidate starts to see the product running locally.
//
//	go run ./cmd/server         # listens on :8080
//	PORT=9000 go run ./cmd/server
package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"github.com/mindfries/products/p1-support-saas/internal/httpapi"
	"github.com/mindfries/products/p1-support-saas/internal/seed"
	"github.com/mindfries/products/p1-support-saas/internal/support"
)

func main() {
	store := support.NewMemStore()
	agents, teams, policies, kb := seed.Catalog()
	store.Seed(agents, teams, policies, kb)

	svc := support.NewService(store, time.Now)
	seed.Load(svc, time.Now())

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	addr := ":" + port
	log.Printf("Helpdesk API listening on %s", addr)
	if err := http.ListenAndServe(addr, httpapi.New(svc)); err != nil {
		log.Fatal(err)
	}
}
