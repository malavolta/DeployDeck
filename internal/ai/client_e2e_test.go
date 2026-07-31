package ai_test

// TestClientE2E_* — hermetic end-to-end coverage of internal/ai's client
// against an httptest.Server standing in for an OpenAI-compatible local
// model server (Ollama/LM Studio/llama.cpp). Unlike internal/*'s real-org
// e2e suites, these run in CI under `-race -short` (design's Testing
// Strategy: "hermetic, CI, -race -short") — no real network, no external
// process, mirroring internal/update/checker_test.go's httptest coverage.

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/malavolta/DeployDeck/internal/ai"
)

// TestClientE2E_GenerateSummary_Success proves the full generate -> POST ->
// parse round trip against a canned OpenAI-compatible response.
func TestClientE2E_GenerateSummary_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"TITLE: PROJ-1 - Add discount validation\nDESCRIPTION: Adds validation."}}]}`))
	}))
	defer srv.Close()

	c := ai.New(srv.URL, "qwen2.5-coder:3b", srv.Client())
	result, err := c.GenerateSummary(context.Background(), ai.SummaryRequest{Ticket: "PROJ-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Title == "" {
		t.Fatal("expected a non-empty title")
	}
}

// TestClientE2E_GenerateSummary_Unreachable proves an unreachable endpoint
// degrades to an error, never a panic or a hang.
func TestClientE2E_GenerateSummary_Unreachable(t *testing.T) {
	c := ai.New("http://127.0.0.1:1", "qwen2.5-coder:3b", &http.Client{Timeout: 2 * time.Second})
	_, err := c.GenerateSummary(context.Background(), ai.SummaryRequest{Ticket: "PROJ-1"})
	if err == nil {
		t.Fatal("expected an error for an unreachable endpoint")
	}
}

// TestClientE2E_GenerateSummary_Timeout proves GenerateSummary respects
// context cancellation, mirroring internal/update's TestChecker_Latest_Timeout.
func TestClientE2E_GenerateSummary_Timeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	c := ai.New(srv.URL, "qwen2.5-coder:3b", srv.Client())
	_, err := c.GenerateSummary(ctx, ai.SummaryRequest{Ticket: "PROJ-1"})
	if err == nil {
		t.Fatal("expected an error when the context times out")
	}
}

// TestClientE2E_GenerateSummary_MalformedJSON proves a non-JSON response
// body degrades to an error rather than a panic.
func TestClientE2E_GenerateSummary_MalformedJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`not json`))
	}))
	defer srv.Close()

	c := ai.New(srv.URL, "qwen2.5-coder:3b", srv.Client())
	_, err := c.GenerateSummary(context.Background(), ai.SummaryRequest{Ticket: "PROJ-1"})
	if err == nil {
		t.Fatal("expected an error for a malformed response")
	}
}

// TestClientE2E_GenerateSummary_NoUsableTitle proves a syntactically valid
// response whose content has no TITLE: label degrades to an error (ADR-2:
// ParseSummary's ok=false propagates as a GenerateSummary error).
func TestClientE2E_GenerateSummary_NoUsableTitle(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"no recognizable labels here at all"}}]}`))
	}))
	defer srv.Close()

	c := ai.New(srv.URL, "qwen2.5-coder:3b", srv.Client())
	_, err := c.GenerateSummary(context.Background(), ai.SummaryRequest{Ticket: "PROJ-1"})
	if err == nil {
		t.Fatal("expected an error when the model response has no usable title")
	}
}

// TestClientE2E_GenerateSummary_OversizedBodyCapped mirrors
// internal/update's TestChecker_Latest_LargeBody: a response body exceeding
// maxResponseBodyBytes is truncated mid-decode and must error, never
// exhaust memory.
func TestClientE2E_GenerateSummary_OversizedBodyCapped(t *testing.T) {
	huge := strings.Repeat("A", 2<<20) // 2 MiB, twice the 1 MiB cap
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"TITLE: x","padding":"`))
		_, _ = w.Write([]byte(huge))
		_, _ = w.Write([]byte(`"}}]}`))
	}))
	defer srv.Close()

	c := ai.New(srv.URL, "qwen2.5-coder:3b", srv.Client())
	_, err := c.GenerateSummary(context.Background(), ai.SummaryRequest{Ticket: "PROJ-1"})
	if err == nil {
		t.Fatal("expected an error when the response body exceeds the size cap")
	}
}

// TestClientE2E_Doctor_ThreeStates proves Doctor's full 3-state
// classification: reachable+listed, reachable+missing, unreachable.
func TestClientE2E_Doctor_ThreeStates(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":[{"id":"qwen2.5-coder:3b"}]}`))
	}))
	defer srv.Close()

	listed := ai.New(srv.URL, "qwen2.5-coder:3b", srv.Client())
	if state := listed.Doctor(context.Background()); state != ai.Ready {
		t.Fatalf("Doctor() listed = %v, want Ready", state)
	}

	missing := ai.New(srv.URL, "not-listed", srv.Client())
	if state := missing.Doctor(context.Background()); state != ai.ModelMissing {
		t.Fatalf("Doctor() missing = %v, want ModelMissing", state)
	}

	unreachable := ai.New("http://127.0.0.1:1", "qwen2.5-coder:3b", &http.Client{Timeout: 2 * time.Second})
	if state := unreachable.Doctor(context.Background()); state != ai.Unreachable {
		t.Fatalf("Doctor() unreachable = %v, want Unreachable", state)
	}
}
