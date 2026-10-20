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
	"crypto/ed25519"
	"encoding/base64"
	"log"
	"net/http"
	"os"
	"strconv"

	"github.com/mindfries/v2/api/internal/httpapi"
	"github.com/mindfries/v2/api/internal/llm"
	"github.com/mindfries/v2/api/internal/store"
	"github.com/mindfries/v2/api/internal/taskgen"
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

	// Bundle signing key: a production Ed25519 key from env (release CLIs trust
	// it via their baked-in ManifestKeys), or the dev key when unset.
	keyID := os.Getenv("V2_SIGNING_KEY_ID")
	var signKey ed25519.PrivateKey
	if seedB64 := os.Getenv("V2_SIGNING_PRIVATE_KEY"); seedB64 != "" {
		seed, err := base64.StdEncoding.DecodeString(seedB64)
		if err != nil || len(seed) != ed25519.SeedSize {
			log.Fatalf("V2_SIGNING_PRIVATE_KEY must be base64 of a %d-byte seed", ed25519.SeedSize)
		}
		signKey = ed25519.NewKeyFromSeed(seed)
		log.Printf("signing bundles with production key %q", keyID)
	} else {
		log.Printf("signing bundles with the DEV key (set V2_SIGNING_KEY_ID/V2_SIGNING_PRIVATE_KEY for production)")
	}

	// Optional BRIEF adaptation: when GEMINI_API_KEY is set, the engine rewords
	// each authored brief to the company's JD (wording only). Unset -> nil
	// provider -> briefs ship verbatim and the engine makes no model calls.
	var provider taskgen.Provider
	if key := os.Getenv("GEMINI_API_KEY"); key != "" {
		g := llm.NewGemini(key, os.Getenv("GEMINI_MODEL"))
		provider = g
		log.Printf("brief adaptation ON (Gemini %s)", g.Model())
	} else {
		log.Printf("brief adaptation OFF (set GEMINI_API_KEY to enable)")
	}

	// db implements both the product Registry and the session Store.
	handler := httpapi.New(db, productsRoot, store.NewTaskCache(db), provider, poolSize, db, keyID, signKey)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("V2 engine API listening on :%s (products from %s)", port, productsRoot)
	if err := http.ListenAndServe(":"+port, handler); err != nil {
		log.Fatal(err)
	}
}
