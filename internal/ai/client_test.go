package ai_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/malavolta/DeployDeck/internal/ai"
)

// TestClient_GenerateSummary_NilHTTPClient is task 1.5 (RED)'s nil-guard
// (mirrors internal/update.Checker's TestChecker_Latest_NilHTTPClient): an
// injectable Client constructed without an HTTPClient degrades to an error,
// never a panic.
func TestClient_GenerateSummary_NilHTTPClient(t *testing.T) {
	c := ai.New("http://example.invalid", "qwen2.5-coder:3b", nil)
	_, err := c.GenerateSummary(context.Background(), ai.SummaryRequest{Ticket: "PROJ-1"})
	if err == nil {
		t.Fatal("expected an error with a nil HTTP client, got nil")
	}
}

// TestClient_Doctor_NilHTTPClient mirrors the guard for Doctor, which is
// TOTAL (no error return, ADR-4) — a nil HTTPClient must classify as
// Unreachable rather than panicking.
func TestClient_Doctor_NilHTTPClient(t *testing.T) {
	c := ai.New("http://example.invalid", "qwen2.5-coder:3b", nil)
	if state := c.Doctor(context.Background()); state != ai.Unreachable {
		t.Fatalf("Doctor() with a nil HTTP client = %v, want Unreachable", state)
	}
}

// TestClient_GenerateSummary_RequestShapeAndParsing proves GenerateSummary
// POSTs {endpoint}/v1/chat/completions with the configured model and the
// Build-composed messages, and hands choices[0].message.content to
// ParseSummary.
func TestClient_GenerateSummary_RequestShapeAndParsing(t *testing.T) {
	var gotPath string
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"TITLE: PROJ-1 - Add discount validation\nDESCRIPTION: Adds server-side validation."}}]}`))
	}))
	defer srv.Close()

	c := ai.New(srv.URL, "qwen2.5-coder:3b", srv.Client())
	result, err := c.GenerateSummary(context.Background(), ai.SummaryRequest{
		Ticket:           "PROJ-1",
		CommitSubjects:   []string{"feat: add discount validation"},
		ComponentSummary: "ApexClass(1)",
	})
	if err != nil {
		t.Fatalf("GenerateSummary() unexpected error: %v", err)
	}
	if gotPath != "/v1/chat/completions" {
		t.Fatalf("request path = %q, want /v1/chat/completions", gotPath)
	}
	if gotBody["model"] != "qwen2.5-coder:3b" {
		t.Fatalf("request body model = %v, want qwen2.5-coder:3b", gotBody["model"])
	}
	if gotBody["stream"] != false {
		t.Fatalf("request body stream = %v, want false", gotBody["stream"])
	}
	if result.Title != "PROJ-1 - Add discount validation" {
		t.Fatalf("result.Title = %q, want %q", result.Title, "PROJ-1 - Add discount validation")
	}
	if result.Description == "" {
		t.Fatal("expected a non-empty description")
	}
}

// TestClient_Doctor_RequestShapeReadyAndMissing proves Doctor GETs
// {endpoint}/v1/models and classifies membership of the configured Model.
func TestClient_Doctor_RequestShapeReadyAndMissing(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			t.Errorf("Doctor request path = %q, want /v1/models", r.URL.Path)
		}
		if r.Method != http.MethodGet {
			t.Errorf("Doctor request method = %q, want GET", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":[{"id":"qwen2.5-coder:3b"},{"id":"llama3"}]}`))
	}))
	defer srv.Close()

	listed := ai.New(srv.URL, "qwen2.5-coder:3b", srv.Client())
	if state := listed.Doctor(context.Background()); state != ai.Ready {
		t.Fatalf("Doctor() listed model = %v, want Ready", state)
	}

	missing := ai.New(srv.URL, "not-listed-model", srv.Client())
	if state := missing.Doctor(context.Background()); state != ai.ModelMissing {
		t.Fatalf("Doctor() missing model = %v, want ModelMissing", state)
	}
}
