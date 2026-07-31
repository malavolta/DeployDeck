package ai_test

// TestE2ERealModel_* — opt-in end-to-end test against a REAL, developer-run
// local model server (Ollama/LM Studio/llama.cpp). Mirrors
// internal/prereq/real_org_e2e_test.go's activation discipline: gated by
// DEPLOYDECK_E2E_AI_ENDPOINT + DEPLOYDECK_E2E_AI_MODEL; SKIPPED (never
// failed) when either is unset, so `go test ./...` stays green in CI and
// for any contributor without a local model running. NEVER runs in CI —
// this file has no build tag because the env-var gate alone is the
// documented activation mechanism, and unset vars must produce a visible
// SKIP, not a silently excluded file.
//
// Example local setup: `ollama pull qwen2.5-coder:3b && ollama serve`, then
// run with:
//
//	DEPLOYDECK_E2E_AI_ENDPOINT=http://localhost:11434 \
//	DEPLOYDECK_E2E_AI_MODEL=qwen2.5-coder:3b \
//	go test ./internal/ai/... -run TestE2ERealModel -v

import (
	"context"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/malavolta/DeployDeck/internal/ai"
)

// e2eAIEndpointAndModel returns the DEPLOYDECK_E2E_AI_ENDPOINT/_MODEL pair,
// skipping the calling test when either is unset.
func e2eAIEndpointAndModel(t *testing.T) (endpoint, model string) {
	t.Helper()
	endpoint = os.Getenv("DEPLOYDECK_E2E_AI_ENDPOINT")
	model = os.Getenv("DEPLOYDECK_E2E_AI_MODEL")
	if endpoint == "" || model == "" {
		t.Skip("set DEPLOYDECK_E2E_AI_ENDPOINT and DEPLOYDECK_E2E_AI_MODEL (e.g. http://localhost:11434 / qwen2.5-coder:3b) to run the real-model e2e (local only)")
	}
	return endpoint, model
}

// TestE2ERealModel_GenerateSummary_NonEmptyTitle exercises the real
// GenerateSummary round trip against a live local model, proving the full
// prompt -> HTTP -> parse pipeline against real (not canned) model output.
func TestE2ERealModel_GenerateSummary_NonEmptyTitle(t *testing.T) {
	endpoint, model := e2eAIEndpointAndModel(t)

	client := ai.New(endpoint, model, &http.Client{})

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	result, err := client.GenerateSummary(ctx, ai.SummaryRequest{
		Ticket: "PROJ-1",
		CommitSubjects: []string{
			"feat: add discount validation before checkout",
			"fix: guard against nil account lookup",
		},
		ComponentSummary: "Types: ApexClass(2), CustomField(1)",
	})
	if err != nil {
		t.Fatalf("GenerateSummary against the real local model: unexpected error: %v", err)
	}
	if result.Title == "" {
		t.Fatal("expected a non-empty title from the real local model")
	}
}

// TestE2ERealModel_Doctor_Ready proves Doctor classifies the real,
// configured local model as Ready (reachable and listed).
func TestE2ERealModel_Doctor_Ready(t *testing.T) {
	endpoint, model := e2eAIEndpointAndModel(t)

	client := ai.New(endpoint, model, &http.Client{})

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if state := client.Doctor(ctx); state != ai.Ready {
		t.Fatalf("Doctor() against the real local model = %v, want Ready (endpoint %q, model %q)", state, endpoint, model)
	}
}
