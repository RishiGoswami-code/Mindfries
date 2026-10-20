// Command server is the real V2 engine API: it connects to the v2 schema in
// Postgres (as the v2_api role), reads the product registry, and resolves
// assessments from role + job description.
//
// Deploy (e.g. Render, root directory v2/api):
//
//	build: go build -o bin/server ./cmd/server
//	start: ./bin/server
//
// Environment:
//
//	V2_DATABASE_URL   Postgres URL for the v2_api role (required)
//	V2_PRODUCTS_ROOT  directory holding the product sources (default ../products)
//	V2_POOL_SIZE      task variants generated per JD (default 3)
//	PORT              listen port (default 8080)
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"strconv"

	"github.com/mindfries/v2/api/internal/httpapi"
	"github.com/mindfries/v2/api/internal/store"
)

func main() {
	url := os.Getenv("V2_DATABASE_URL")
	if url == "" {
		log.Fatal("V2_DATABASE_URL is required (Postgres URL for the v2_api role)")
	}

	ctx := context.Background()
	db, err := store.Open(ctx, url)
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	defer db.Close()

	productsRoot := os.Getenv("V2_PRODUCTS_ROOT")
	if productsRoot == "" {
		productsRoot = "../products"
	}
	poolSize := 3
	if v := os.Getenv("V2_POOL_SIZE"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			poolSize = n
		}
	}

	handler := httpapi.New(db, productsRoot, store.NewTaskCache(db), nil, poolSize)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("V2 engine API listening on :%s (products from %s)", port, productsRoot)
	if err := http.ListenAndServe(":"+port, handler); err != nil {
		log.Fatal(err)
	}
}
