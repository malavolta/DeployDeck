package prereq

import (
	"context"

	"github.com/malavolta/DeployDeck/internal/ai"
)

// CheckAI reports the local-model AI endpoint's reachability and configured-
// model availability as an informative, NON-BLOCKING PrereqCheck
// (ai-pr-summary, closing the remaining part of HU-014's deferred "Idea
// Futura" — "Chequeo INFORMATIVO/no bloqueante en el doctor"): reachable
// with the model listed, reachable but missing the model, and unreachable
// are all reported without ever using StatusBlocking — the AI suggestion is
// an optional, on-demand affordance, never a hard requirement. A nil AI
// (every Checker built before this slice, and every existing checker test
// that never sets AI) skips this check entirely, reporting OK — mirrors
// CheckGH's nil-skip 3-state shape.
func (c *Checker) CheckAI(ctx context.Context) (PrereqCheck, error) {
	if c.AI == nil {
		return PrereqCheck{Name: "AI model", Status: StatusOK, Detail: "AI check skipped (no AI client configured)"}, nil
	}

	switch c.AI.Doctor(ctx) {
	case ai.Ready:
		return PrereqCheck{Name: "AI model", Status: StatusOK, Detail: "AI endpoint is reachable and the configured model is available"}, nil
	case ai.ModelMissing:
		return PrereqCheck{
			Name:   "AI model",
			Status: StatusWarning,
			Detail: "AI endpoint is reachable but the configured model was not found; the AI-suggestion affordance will be unavailable",
		}, nil
	default: // ai.Unreachable
		return PrereqCheck{
			Name:   "AI model",
			Status: StatusWarning,
			Detail: "AI endpoint is unreachable; the AI-suggestion affordance will be unavailable",
		}, nil
	}
}
