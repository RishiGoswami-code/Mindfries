package store

import (
	"context"
	"os"
	"testing"
	"time"
)

// Integration test: runs only when V2_DATABASE_URL is set (a v2_api connection
// string). It reads the registry — read-only, so it is safe against a real DB.
func TestProductsAgainstRealDB(t *testing.T) {
	url := os.Getenv("V2_DATABASE_URL")
	if url == "" {
		t.Skip("set V2_DATABASE_URL to run the store integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	db, err := Open(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	products, err := db.Products(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(products) == 0 {
		t.Fatal("expected at least one seeded product (run migrations 0005-0007)")
	}
	for _, p := range products {
		if len(p.Manifest.Roles) == 0 {
			t.Fatalf("product %q has an empty manifest", p.Slug)
		}
	}
	t.Logf("registry has %d verified products", len(products))
}
