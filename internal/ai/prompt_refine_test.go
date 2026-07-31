package ai_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/malavolta/DeployDeck/internal/ai"
)

// The refined system prompt must steer a small model toward a real
// conventional-commit type and away from inventing changes not present in the
// inputs — empirically the biggest quality lever for a 3B model.
func TestBuild_SystemPromptSteersConventionalCommitAndNoHallucination(t *testing.T) {
	system, _ := ai.Build("PROJ-1", []string{"fix: bug"}, "Types: ApexClass(1)")
	for _, want := range []string{"feat, fix, chore", "Do NOT invent"} {
		if !strings.Contains(system, want) {
			t.Errorf("system prompt should contain %q to steer the model; got:\n%s", want, system)
		}
	}
}

// The generation request must pin a low temperature so titles are deterministic
// and the model rambles/hallucinates less.
func TestGenerateSummary_SendsLowTemperature(t *testing.T) {
	var hasTemp bool
	var gotTemp float64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]any
		_ = json.NewDecoder(r.Body).Decode(&payload)
		if v, ok := payload["temperature"]; ok {
			hasTemp = true
			gotTemp, _ = v.(float64)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{{"message": map[string]string{"content": "TITLE: fix: x\nDESCRIPTION: y"}}},
		})
	}))
	defer srv.Close()

	c := ai.New(srv.URL, "m", &http.Client{})
	_, _ = c.GenerateSummary(context.Background(), ai.SummaryRequest{Ticket: "T-1"})

	if !hasTemp {
		t.Fatal("GenerateSummary request must set a temperature for deterministic output")
	}
	if gotTemp > 0.5 {
		t.Errorf("temperature should be low (<=0.5) for deterministic titles, got %v", gotTemp)
	}
}
