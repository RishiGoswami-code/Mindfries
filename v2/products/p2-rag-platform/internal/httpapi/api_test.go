package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mindfries/products/p2-rag-platform/internal/rag"
	"github.com/mindfries/products/p2-rag-platform/internal/seed"
)

func testServer() *httptest.Server {
	engine := rag.NewEngine(40, 10)
	seed.Load(engine)
	agent := rag.NewAgent(engine, 3)
	return httptest.NewServer(New(engine, agent, rag.NewEval(engine, agent)))
}

func TestQueryAndAnswer(t *testing.T) {
	srv := testServer()
	defer srv.Close()

	res, _ := http.Get(srv.URL + "/query?q=reset+my+password&k=2")
	var hits []rag.Hit
	json.NewDecoder(res.Body).Decode(&hits)
	res.Body.Close()
	if len(hits) == 0 || hits[0].Chunk.DocID != "login" {
		t.Fatalf("password query should retrieve the login doc, got %+v", hits)
	}

	res, _ = http.Post(srv.URL+"/answer", "application/json", bytes.NewBufferString(`{"q":"7 * 6"}`))
	var ans rag.AnswerResult
	json.NewDecoder(res.Body).Decode(&ans)
	res.Body.Close()
	if ans.Tool != "calc" || ans.Answer != "42" {
		t.Fatalf("arithmetic answer should be 42 via calc, got %+v", ans)
	}
}

func TestEvalOnSeededCorpus(t *testing.T) {
	srv := testServer()
	defer srv.Close()

	// On the clean seeded corpus every case's relevant doc should rank first.
	body, _ := json.Marshal(map[string]any{"cases": seed.Cases(), "k": 1})
	res, _ := http.Post(srv.URL+"/eval", "application/json", bytes.NewReader(body))
	var m map[string]float64
	json.NewDecoder(res.Body).Decode(&m)
	res.Body.Close()
	if m["hit_at_k"] < 1.0 {
		t.Fatalf("seeded eval should hit@1 = 1.0, got %v", m["hit_at_k"])
	}
}

func TestIngestMakesDocRetrievable(t *testing.T) {
	srv := testServer()
	defer srv.Close()

	res, _ := http.Post(srv.URL+"/documents", "application/json",
		bytes.NewBufferString(`{"id":"sso","title":"SSO","text":"Configure single sign-on with SAML from the security settings for enterprise login."}`))
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("ingest status = %d", res.StatusCode)
	}
	res.Body.Close()

	res, _ = http.Get(srv.URL + "/query?q=single+sign-on+SAML&k=1")
	var hits []rag.Hit
	json.NewDecoder(res.Body).Decode(&hits)
	res.Body.Close()
	if len(hits) == 0 || hits[0].Chunk.DocID != "sso" {
		t.Fatalf("the newly ingested SSO doc should be retrievable, got %+v", hits)
	}
}
