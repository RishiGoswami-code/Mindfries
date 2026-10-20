package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/mindfries/products/p1-support-saas/internal/seed"
	"github.com/mindfries/products/p1-support-saas/internal/support"
)

func testServer(t *testing.T) *httptest.Server {
	t.Helper()
	store := support.NewMemStore()
	a, tm, p, kb := seed.Catalog()
	store.Seed(a, tm, p, kb)
	svc := support.NewService(store, time.Now)
	seed.Load(svc, time.Now())
	return httptest.NewServer(New(svc))
}

func TestCreateGetAndReplyOverHTTP(t *testing.T) {
	srv := testServer(t)
	defer srv.Close()

	// Create.
	body := `{"subject":"New issue","requester_email":"q@t.test","priority":"high","body":"help"}`
	res, err := http.Post(srv.URL+"/tickets", "application/json", bytes.NewBufferString(body))
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("create status = %d", res.StatusCode)
	}
	var tk support.Ticket
	json.NewDecoder(res.Body).Decode(&tk)
	res.Body.Close()
	if tk.ID == "" || tk.Status != support.StatusOpen {
		t.Fatalf("unexpected created ticket: %+v", tk)
	}

	// Get it back.
	res, _ = http.Get(srv.URL + "/tickets/" + tk.ID)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("get status = %d", res.StatusCode)
	}
	res.Body.Close()

	// Agent reply.
	rb := `{"author_type":"agent","author_id":"agent-ada","body":"on it"}`
	res, _ = http.Post(srv.URL+"/tickets/"+tk.ID+"/reply", "application/json", bytes.NewBufferString(rb))
	json.NewDecoder(res.Body).Decode(&tk)
	res.Body.Close()
	if tk.FirstResponse == nil {
		t.Fatal("agent reply should set first response")
	}
}

func TestGetMissingTicketIs404(t *testing.T) {
	srv := testServer(t)
	defer srv.Close()
	res, _ := http.Get(srv.URL + "/tickets/nope")
	if res.StatusCode != http.StatusNotFound {
		t.Fatalf("missing ticket should be 404, got %d", res.StatusCode)
	}
	res.Body.Close()
}

func TestSuggestionsAndMetricsEndpoints(t *testing.T) {
	srv := testServer(t)
	defer srv.Close()

	// The seeded login ticket should surface the password KB article.
	res, _ := http.Get(srv.URL + "/tickets/TCK-1/suggestions")
	var sug []support.Suggestion
	json.NewDecoder(res.Body).Decode(&sug)
	res.Body.Close()
	if len(sug) == 0 {
		t.Fatal("expected at least one suggestion for the login ticket")
	}

	// Metrics over the seeded data.
	res, _ = http.Get(srv.URL + "/metrics")
	var m support.Metrics
	json.NewDecoder(res.Body).Decode(&m)
	res.Body.Close()
	if m.Total == 0 {
		t.Fatal("metrics should report the seeded tickets")
	}
}
