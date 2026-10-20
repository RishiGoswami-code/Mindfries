// Command keygen generates a production Ed25519 signing key for bundle
// manifests, and prints the server env vars plus the CLI ldflags needed to
// trust it.
//
//	go run ./cmd/keygen [key-id]
//
// Set the two V2_SIGNING_* vars on the server, and build release CLIs with the
// printed -ldflags so they trust this key's signature.
package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"os"
)

func main() {
	keyID := "mindfries-v2-prod"
	if len(os.Args) > 1 {
		keyID = os.Args[1]
	}

	seed := make([]byte, ed25519.SeedSize)
	if _, err := rand.Read(seed); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	key := ed25519.NewKeyFromSeed(seed)
	pub := key.Public().(ed25519.PublicKey)

	b64 := base64.StdEncoding.EncodeToString
	fmt.Println("# Server (set these as environment variables):")
	fmt.Printf("V2_SIGNING_KEY_ID=%s\n", keyID)
	fmt.Printf("V2_SIGNING_PRIVATE_KEY=%s\n", b64(seed))
	fmt.Println()
	fmt.Println("# CLI (build release binaries with this ldflag so they trust the key):")
	fmt.Printf("go build -ldflags \"-X github.com/mindfries/v2/cli/internal/trust.ManifestKeys=%s:%s\" ./cmd/mindfries\n",
		keyID, b64(pub))
}
