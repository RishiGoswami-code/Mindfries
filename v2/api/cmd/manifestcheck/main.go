// Command manifestcheck validates a product's role-coverage manifest
// (v2/codebase-engine PLAN task 0.3). It is the CI gate that stops a product
// shipping with a malformed or dishonest manifest.
//
//	go run ./cmd/manifestcheck path/to/manifest.json [more.json ...]
//
// Exits non-zero if any manifest is invalid, printing every problem found.
package main

import (
	"fmt"
	"os"

	"github.com/mindfries/v2/api/internal/manifest"
)

func main() {
	paths := os.Args[1:]
	if len(paths) == 0 {
		fmt.Fprintln(os.Stderr, "usage: manifestcheck <manifest.json> [...]")
		os.Exit(2)
	}
	bad := false
	for _, p := range paths {
		raw, err := os.ReadFile(p)
		if err != nil {
			fmt.Printf("FAIL %s: %v\n", p, err)
			bad = true
			continue
		}
		m, err := manifest.Parse(raw)
		if err != nil {
			fmt.Printf("FAIL %s: %v\n", p, err)
			bad = true
			continue
		}
		fmt.Printf("OK   %s: %d role(s) covered\n", p, len(m.Roles))
	}
	if bad {
		os.Exit(1)
	}
}
