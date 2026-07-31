# Archive Report: Local-Model PR Title/Description Suggestion (`ai-pr-summary`)

**Change**: `ai-pr-summary`  
**Branch**: `feat/ai-pr-summary`  
**Archived**: 2026-07-31  
**SDD Phase**: Verification → Archive (lifecycle complete)

## Executive Summary

Change `ai-pr-summary` successfully closes HU-014's deferred "Idea Futura"
by adding optional local-model PR title/description generation at
`pushReady`. The feature is additive, opt-in, on-demand, and never
auto-submitted. Implementation is committed, verified (23/23 scenarios,
9/9 requirements passing), and all delta specs have been merged into
living specs.

## Capabilities Delivered

### 1. NEW: `ai-pr-summary` Capability

Enables optional local HTTP-reachable model to draft conventional-commit
PR titles and descriptions from ticket commit subjects and PackageSummary
metadata counts. Requires explicit keypress at `pushReady`; accepted title
overrides only the `gh pr create --title` source.

**Scenarios**: 6/6 passing
- AI configuration is optional and zero-value-safe
- No config leaves push/PR flow unchanged
- On-demand suggestion generation (explicit keypress, never auto-fires)
- Explicit accept overrides title source only
- Never auto-submitted (PR still requires g→y confirmation)
- Silent graceful degradation (unreachable/timeout/malformed → no error)

**Living Spec**: `openspec/specs/ai-pr-summary/spec.md` (NEW — created)

### 2. MODIFIED: `prereq-check` Capability

Added informative, non-blocking `CheckAI` doctor check for AI endpoint
reachability and model availability (`GET <endpoint>/v1/models`).

**ADDED Requirement**: "AI Endpoint And Model Availability Check (Informative)"
- 5 scenarios: nil client skip, reachable with model listed, model missing, unreachable, never blocking

**Living Spec**: `openspec/specs/prereq-check/spec.md` (updated)

### 3. MODIFIED: `push-pr-preparation` Capability

Added on-demand AI affordance at `pushReady` and enhanced PR-creation
title source logic to support AI-suggested titles alongside the existing
formula-based source.

**ADDED Requirement**: "On-Demand AI Suggestion Affordance At pushReady"
- 4 scenarios: no config unchanged, explicit request produces suggestion, ignore keeps formula, degradation silent

**MODIFIED Requirement**: "PR Creation Requires Explicit Confirmation And Records The URL"
- New scenario: explicitly accepted AI title is used as title source
- All existing scenarios preserved
- Notes: AI title used only when explicitly accepted; title source previously always `github.SuggestedTitle`

**Living Spec**: `openspec/specs/push-pr-preparation/spec.md` (updated)

## Merged Artifacts

| Domain | Action | Details |
|--------|--------|---------|
| `ai-pr-summary` | Created | New capability spec with 6 scenarios, 3 requirements |
| `prereq-check` | Updated | Added 5 scenarios for CheckAI (non-blocking informative) |
| `push-pr-preparation` | Updated | Added 4 scenarios for AI affordance + 1 new scenario for title source |

## Verification Summary

**Verdict**: PASS (23/23 scenarios, 9/9 requirements)

- Build: `go build ./...` ✅
- Vet: `go vet ./...` ✅
- Format: `gofmt -l .` (0 files flagged) ✅
- Tests (full): `go test ./... -race` ✅ (13 packages)
- Tests (short): `go test ./... -race -short -count=1` ✅
- Real-model e2e: Self-skips when `DEPLOYDECK_E2E_AI_ENDPOINT`/`_MODEL` unset ✅
- Spec conformance: All 23 scenarios across 3 delta specs map to passing tests ✅
- Architecture boundary: `internal/ai` forbidden in `internal/app`, verified by `boundary_test.go` ✅
- Tasks: 34/34 complete ✅

## Review Outcome

**Lens**: `review-risk`  
**Result**: Sound with warnings (no blockers, implementation mitigates threats)

### Issues

**SUGGESTION-1 (coverage)**: The "unreachable at pushReady degrades
silently" scenario is proven across 3 tests but not by a single end-to-end
TUI round-trip test. Acceptable as threat matrix is fully covered.

**SUGGESTION-2 (TDD transparency)**: A few non-behavior tests were written
GREEN-direct rather than RED-first (hermetic e2e, full-flow integration).
Acceptable as behavior is RED-tested at unit level.

**SUGGESTION-3 (design note)**: `composeAIClient` is called from two sites,
vs design's "compose once" wording. Mirrors existing `github.New`
precedent; no functional impact.

### Deferred Items

**Deferred: AI description body-injection**  
Accepted AI description is display-only; `gh pr create` receives
`--body ""`. Future slice may add full-body capability and description
override UI. Ticket: (future slice)

**Residual-Low: Endpoint scheme validation**  
Config schema does not validate HTTP vs HTTPS scheme prefix. Nil-safe;
endpoint validation deferred to connection-time error handling. Ticket:
(future slice, if required by ops guideline)

## SDD Cycle Completion

All phases complete:
- ✅ **Proposal**: HU-014 Idea Futura closure; scope, decisions, testing strategy
- ✅ **Spec**: 3 capabilities (1 new, 2 modified) with full scenario coverage
- ✅ **Design**: 7 ADRs covering architecture, scalar wiring, parser strategy, degradation
- ✅ **Tasks**: 8 phases (ai pkg, config, prereq, app wiring, threat matrix, main.go, e2e, verify)
- ✅ **Apply**: All 34 tasks implemented and committed on `feat/ai-pr-summary`
- ✅ **Verify**: 23/23 scenarios passing, full test suite green, spec compliance confirmed
- ✅ **Archive**: Delta specs merged into living specs, archive report written

## Next Steps

The `ai-pr-summary` change is complete and ready for orchestrator to:
1. Move `openspec/changes/ai-pr-summary/` → `openspec/changes/archive/2026-07-31-ai-pr-summary/`
2. Merge changes into `main` branch
3. Proceed to next change or release planning

No follow-up SDD phase required.

## Artifact Locations

### Living Specs (updated source of truth)
- `openspec/specs/ai-pr-summary/spec.md` — NEW
- `openspec/specs/prereq-check/spec.md` — UPDATED
- `openspec/specs/push-pr-preparation/spec.md` — UPDATED

### Change Artifacts (to be archived)
- `openspec/changes/ai-pr-summary/proposal.md`
- `openspec/changes/ai-pr-summary/design.md`
- `openspec/changes/ai-pr-summary/specs/` (delta specs, source for merge above)
- `openspec/changes/ai-pr-summary/tasks.md` (34/34 complete)
- `openspec/changes/ai-pr-summary/verify-report.md` (verdict: pass)
- `openspec/changes/ai-pr-summary/apply-progress.md`
- `openspec/changes/ai-pr-summary/archive-report.md` ← this file
