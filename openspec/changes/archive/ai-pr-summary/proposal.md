# Proposal: Local-Model PR Title/Description Suggestion (`ai-pr-summary`)

## Intent

Close HU-014's deferred "Idea Futura" (`docs/HISTORIAS.md:966-985`). At
`StatePushPreparation` (`pushReady`), an OPTIONAL local model drafts a
conventional-commit PR **title + description** from the ticket's commit subjects
and the metadata delta (`PackageSummary` counts). It is an **additive,
non-blocking, never-auto-submitted** suggestion beside the existing
`github.SuggestedTitle` formula. Why now: Fase 5 (Productización) polish; the
push/PR flow is stable, context is already on `Model` (no new I/O), and
degradation precedents (`CheckGH`, `checkUpdateCmd`) exist to copy.

**Delivery phase**: Fase 5 — Productización. **User story**: HU-014 (Idea Futura).

## Decisions (with rationale)

- **Approach 1 — one OpenAI-compatible client**: `POST <endpoint>/v1/chat/completions`;
  doctor `GET <endpoint>/v1/models`. NOT Ollama-native. One client/test surface
  covers Ollama/LM Studio/llama.cpp. Config `ai: {endpoint, model, enabled}`, no
  `provider` field.
- **On-demand trigger**: explicit keypress at `pushReady`, not auto-fired after
  push — avoids a local inference call on every push; matches explicit-confirm discipline.
- **Architecture**: new `internal/ai` is the sole `net/http` boundary (mirrors
  `internal/update`). Exposed to `internal/app` ONLY via a plain-typed scalar
  `Deps.GenerateSummary` (no ai/http types leak). `internal/app/boundary_test.go`
  forbidden list gains `internal/ai`. `prereq.Checker.AI` holds a typed client
  directly (like `GH`). `internal/app` does no HTTP/exec.
- **Doctor `CheckAI`**: informative, `StatusWarning`-only, never blocking;
  copies `CheckGH`'s nil-skip 3-state shape.
- **`AIConfig{Endpoint, Model string; Enabled bool}`**: zero-value-safe (omitted
  block → off, like `QuickDeployConfig`); `Enabled=true` requires non-empty endpoint+model.
- **Graceful degradation**: no config / unreachable / timeout / malformed →
  no suggestion offered; push/PR flow unchanged.

## Scope

**In scope**: `internal/ai` (client + prompt + parser); `AIConfig` + validation;
`CheckAI`; `Deps.GenerateSummary` wiring in `main.go`; push-prep TUI affordance
(request + accept, overriding `gh pr create --title`); full test suite below.

**Out of scope**: Ollama-native protocol; streaming; bundling/installing a model;
rich-editor editing of generated text (accept-or-ignore only); any auto-submission.

## Capabilities

### New Capabilities
- `ai-pr-summary`: local-model, opt-in, non-blocking PR title/description suggestion.

### Modified Capabilities
- `prereq-check`: add informative, non-blocking `CheckAI` doctor check.
- `push-pr-preparation`: add on-demand AI-suggestion affordance at `pushReady`;
  accepted title overrides the `gh pr create --title` source only via explicit action.

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `internal/ai/` | New | `client.go`, `prompt.go`, `parse.go` — sole `net/http` boundary |
| `internal/config/{config,load,validate}.go` | Modified | `AIConfig` block + validation |
| `internal/prereq/{checker,checker_ai,checker_check}.go` | New/Modified | `CheckAI` |
| `internal/app/{app,commands,update,keys,view}.go` | Modified | scalar dep, `aiSuggestCmd`, reducer, key, render |
| `internal/app/boundary_test.go` | Modified | add `internal/ai` to forbidden imports |
| `cmd/deploydeck/main.go` | Modified | compose `ai.New`; wire typed + scalar |

## Testing (e2e required)

- **Unit**: prompt build + truncation; response parse (malformed/partial degrade);
  config validation; degradation branches (nil scalar, nil `Checker.AI`).
- **Integration/e2e (hermetic, RUNS IN CI, `-race -short`)**: `httptest.Server`
  with canned OpenAI-compatible JSON — success/unreachable/timeout/malformed/oversized-body
  + the 3 `CheckAI` states.
- **TUI integration**: `keyPushPreparation` request/accept; AI title reaches
  `gh pr create --title` ONLY after explicit acceptance.
- **Real-model e2e (opt-in, local-only, self-skips in CI)**: gated by
  `DEPLOYDECK_E2E_AI_ENDPOINT`/`DEPLOYDECK_E2E_AI_MODEL` (mirrors `DEPLOYDECK_E2E_ORG`).

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| Small model ignores output format | High | Strict testable parser; degrade to "no suggestion", never surface garbled text |
| Accidental `net/http`/`internal/ai` import in `internal/app` | Med | Fix scalar signature up front; `boundary_test` gate |
| Doctor check treated as blocking | Low | `StatusWarning`-only; unreachable local model is normal |
| Inference cost per push | Low | On-demand keypress, never auto-fired |

## Rollback Plan

Purely additive and default-off (`Enabled=false` when `ai` block absent). Rollback =
revert the change. No shared Git state, no Salesforce, no persisted-schema migration.
The additive `AIConfig` block and the additive `boundary_test` forbidden-list entry
have no effect when the feature is unused.

## Dependencies

External local inference server (Ollama/LM Studio/llama.cpp) — user-run, never bundled.

## Success Criteria

- [x] With no `ai` config, push/PR flow is byte-for-byte unchanged; `CheckAI` skips/warns only.
- [x] With a reachable endpoint, explicit keypress yields a suggestion; accepting it
      overrides only the `gh pr create --title` source; ignoring keeps the formula title.
- [x] Unreachable/timeout/malformed responses degrade silently, no error surfaced.
- [x] Hermetic e2e passes in CI under `-race -short`; real-model e2e self-skips when env unset.
- [x] `internal/app/boundary_test.go` still passes with `internal/ai` forbidden.

## Open Questions (deferred to design)

1. Exact scalar `Deps.GenerateSummary` signature (proposed:
   `func(ctx, ticket string, commitSubjects []string, componentSummary string) (title, description string, err error)`).
2. Model response contract: two-line `TITLE:`/`DESCRIPTION:` vs JSON (former parses
   more leniently for small models).
3. Whether the accepted title also replaces the compare-URL manual-copy display
   (likely yes — same slot, different string source).
