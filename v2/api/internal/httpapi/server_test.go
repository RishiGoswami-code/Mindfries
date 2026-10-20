package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/mindfries/v2/api/internal/engine"
	"github.com/mindfries/v2/api/internal/manifest"
	"github.com/mindfries/v2/api/internal/taskgen"
)

// fakeRegistry loads the real product manifests from disk, so Select behaves as
// in production without needing a database.
type fakeRegistry struct{ products []engine.Product }

func (f fakeRegistry) Products(context.Context) ([]engine.Product, error) { return f.products, nil }

func productsRoot(t *testing.T) string {
	t.Helper()
	wd, _ := os.Getwd() // .../v2/api/internal/httpapi
	return filepath.Clean(filepath.Join(wd, "../../../products"))
}

func loadReg(t *testing.T, root string, slugs ...string) fakeRegistry {
	t.Helper()
	var ps []engine.Product
	for _, slug := range slugs {
		raw, err := os.ReadFile(filepath.Join(root, slug, "manifest.json"))
		if err != nil {
			t.Fatalf("read manifest %s: %v", slug, err)
		}
		m, err := manifest.Parse(raw)
		if err != nil {
			t.Fatalf("parse manifest %s: %v", slug, err)
		}
		ps = append(ps, engine.Product{Slug: slug, Name: slug, Domain: slug, GoldenCommit: "test", Manifest: m})
	}
	return fakeRegistry{products: ps}
}

func testServer(t *testing.T) *httptest.Server {
	root := productsRoot(t)
	reg := loadReg(t, root, "p1-support-saas", "p2-rag-platform", "p3-quant-platform")
	return httptest.NewServer(New(reg, root, taskgen.NewMemCache(), nil, 3))
}

func TestHealthAndProducts(t *testing.T) {
	srv := testServer(t)
	defer srv.Close()

	res, _ := http.Get(srv.URL + "/healthz")
	if res.StatusCode != http.StatusOK {
		t.Fatalf("health status = %d", res.StatusCode)
	}
	res.Body.Close()

	res, _ = http.Get(srv.URL + "/v2/products")
	var list []map[string]any
	json.NewDecoder(res.Body).Decode(&list)
	res.Body.Close()
	if len(list) != 3 {
		t.Fatalf("expected 3 products, got %d", len(list))
	}
}

func TestResolveSelectsProductAndDeliversWorkspace(t *testing.T) {
	srv := testServer(t)
	defer srv.Close()

	// A support-flavoured backend JD -> P1, with a real workspace.
	body := `{"role":"backend","jd":"Backend engineer for our customer support helpdesk SaaS, tickets and SLA"}`
	res, err := http.Post(srv.URL+"/v2/assessments/resolve", "application/json", bytes.NewBufferString(body))
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusOK {
		t.Fatalf("resolve status = %d", res.StatusCode)
	}
	var out resolveResp
	json.NewDecoder(res.Body).Decode(&out)
	res.Body.Close()

	if out.Product.Slug != "p1-support-saas" {
		t.Fatalf("support JD should select P1, got %q (%s)", out.Product.Slug, out.Product.Why)
	}
	if out.Task.TaskVersion == "" || out.Task.Title == "" {
		t.Fatalf("task should be resolved with a version and title, got %+v", out.Task)
	}
	if out.Workspace.Files["TASK.md"] == "" {
		t.Fatal("workspace should include TASK.md")
	}
	if _, ok := out.Workspace.Files["internal/support/tickets.go"]; !ok {
		t.Fatal("workspace should include the P1 product source")
	}
	if len(out.Workspace.Check) == 0 {
		t.Fatal("workspace should carry a check command")
	}
}

func TestResolveQuantJDPicksPython(t *testing.T) {
	srv := testServer(t)
	defer srv.Close()

	body := `{"role":"quant","jd":"Quant researcher: backtesting, portfolio, sharpe, alpha signals"}`
	res, _ := http.Post(srv.URL+"/v2/assessments/resolve", "application/json", bytes.NewBufferString(body))
	var out resolveResp
	json.NewDecoder(res.Body).Decode(&out)
	res.Body.Close()
	if out.Product.Slug != "p3-quant-platform" {
		t.Fatalf("quant JD should select P3, got %q", out.Product.Slug)
	}
	if len(out.Workspace.Runtimes) == 0 || out.Workspace.Runtimes[0] != "python" {
		t.Fatalf("P3 workspace should need python, got %v", out.Workspace.Runtimes)
	}
}

func TestResolveUnknownRoleRejected(t *testing.T) {
	srv := testServer(t)
	defer srv.Close()
	res, _ := http.Post(srv.URL+"/v2/assessments/resolve", "application/json", bytes.NewBufferString(`{"role":"wizard","jd":"x"}`))
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("unknown role should be 400, got %d", res.StatusCode)
	}
	res.Body.Close()
}
