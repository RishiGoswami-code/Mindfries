// Command server runs the RAG platform API with a seeded knowledge base.
//
//	go run ./cmd/server         # listens on :8080
//	PORT=9000 go run ./cmd/server
package main

import (
	"log"
	"net/http"
	"os"

	"github.com/mindfries/products/p2-rag-platform/internal/httpapi"
	"github.com/mindfries/products/p2-rag-platform/internal/rag"
	"github.com/mindfries/products/p2-rag-platform/internal/seed"
)

func main() {
	engine := rag.NewEngine(40, 10)
	seed.Load(engine)
	agent := rag.NewAgent(engine, 3)
	eval := rag.NewEval(engine, agent)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	addr := ":" + port
	log.Printf("RAG platform API listening on %s", addr)
	if err := http.ListenAndServe(addr, httpapi.New(engine, agent, eval)); err != nil {
		log.Fatal(err)
	}
}
