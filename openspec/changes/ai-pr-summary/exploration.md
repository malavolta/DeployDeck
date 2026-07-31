# Exploration: Local-Model PR Title/Description Generation (`ai-pr-summary`)

Closes the deferred "Idea Futura" from HU-014 (`docs/HISTORIAS.md:966-985`): at
`StatePushPreparation`, use an HTTP-reachable **local** small model to draft a
conventional-commit PR title + description from the ticket's commits and the
metadata delta, as an **additional, non-blocking, never-auto-submitted**
suggestion alongside the existing `github.SuggestedTitle` formula. The archived
`push-pr/exploration.md:14` explicitly scoped this OUT ("the 'Idea Futura
(Opcional)' local-model PR title/description ... CONFIRMED out"). Fase 5
(Productización), maps to HU-014.

## Current State (verified, file:line)

**Push/PR flow today** — `internal/app/app.go:119` (`StatePushPreparation`),
sub-state machine `pushPhase` (`app.go:218-221`:
`pushConfirm → pushPushing → pushReady → pushPRConfirm → pushPRCreating`).

- `internal/app/keys.go:356-402` `keyPushPreparation` — `p` fires `pushCmd`; on
  `pushReady`, `g` (only when `authState==AuthAuthenticated`) reveals
  `pushPRConfirm`; `y` there fires `createPRCmd`. No PR is ever created without
  this two-step explicit gate.
- `internal/app/commands.go:1032-1040` `pushCmd` → `git.Push`.
- `internal/app/commands.go:1049-1077` `preparePRCmd` — fired from `onPushDone`
  (`update.go:987-999`) right after a successful push: one `gh.AuthStatus`,
  `git.RemoteURL(origin)`, then `github.CompareURL`. Lands on `onPrepDone`
  (`update.go:1006-1014`), which sets `pushPhase = pushReady`. **This is the
  natural place to add a parallel AI-suggestion command.**
- `internal/app/commands.go:1201-1214` `createPRCmd` — builds
  `title := github.SuggestedTitle(m.plan.Ticket, m.plan.TargetBranch)`
  (`internal/github/client.go:106-108`, `"<ticket> - Promote changes to
  <target>"`) and calls `gh.CreatePR(ctx, base, head, title)`. This is the ONE
  place the final title is chosen — an AI-suggested title must be able to
  override it, but only via explicit user action, never implicitly.
- `internal/app/view.go:705-733` `viewPushPreparation` and `view.go:740-794`
  `viewPRData` — render base/compare/title; `github.SuggestedTitle` is called
  again at `view.go:742` purely for display.

**Data already on `Model` at push-prep time (no new I/O needed)**:

- `m.plan.SelectedCommits []git.DiscoveredCommit`
  (`internal/git/deployment_plan.go:11-18`, populated at
  `GenerateDeploymentPlan`, never cleared) — each `DiscoveredCommit.Commit.Subject`/`.SHA`
  (`internal/git/commit.go:14-21`) gives the ticket's commit subjects for the
  prompt, with zero new git calls.
- `m.summary delta.PackageSummary` (`internal/app/app.go:432`, set at
  `StatePackageReview` from `internal/delta.Summarize`,
  `internal/delta/package.go:97-109`) — per-type additive/destructive member
  **counts** (not full diffs), already bounded/summarized. This is the
  metadata-delta context, also with zero new I/O.

Both context sources are structurally pre-summarized (subjects, not diffs;
counts, not content), which materially reduces the "large diff" truncation
problem the HISTORIAS note worries about — the raw diff text is never even
available to `internal/app`.

**Degradation precedents to mirror**:

- `internal/prereq/checker_gh.go:9-40` `CheckGH` — nil-client skip
  (`if c.GH == nil { return StatusOK, "skipped" }`), 3-state informative (never
  `StatusBlocking`). Exact shape a new `CheckAI` should copy.
- `internal/app/commands.go:398-409` `checkUpdateCmd` — nil-func guard on
  `m.deps.CheckUpdate`, bounded `context.WithTimeout`, single scalar closure
  composed in `main.go`.
- HU-009 queue-permission degrade and gh-absent degrade — same discipline: no
  `ai` config or unreachable endpoint → the AI suggestion affordance simply
  isn't offered; nothing else in the push/PR flow changes.

**HTTP-client pattern to copy** — `internal/update/checker.go`:
`Checker{BaseURL, HTTPClient *http.Client}`, nil-client guard (`:46-48`),
`io.LimitReader(resp.Body, maxResponseBodyBytes)` (`:68`), context-bound request
(`http.NewRequestWithContext`, `:52`). `internal/update/checker_test.go` shows
the exact `httptest.Server` test shape (success, non-2xx, timeout via short
context, nil-client, oversized-body-cap) to replicate for `internal/ai`.

**Scalar Deps composition** — `cmd/deploydeck/main.go:252-261`
`defaultCheckUpdate(ctx) (bool, string, error)` composes `update.Checker` and is
wired into `app.Deps.CheckUpdate` (`main.go:243`); `internal/app` never imports
`internal/update` or `net/http` directly — enforced by
`internal/app/boundary_test.go:34-58` (`TestApp_NeverImportsExecSeam`), which
forbids `"os/exec"`, `internal/exec`, `net/http`, `internal/version`,
`internal/update` as direct imports of internal/app's own files. **This list
must grow one more entry: `github.com/malavolta/DeployDeck/internal/ai`.** A new
`Deps.GenerateSummary` scalar func must use only plain Go types in its signature
(strings/slices), never `ai.SummaryRequest`/`ai.Client`, mirroring
`CheckUpdate`'s `func(context.Context) (bool, string, error)` shape.

**Config** — `internal/config/config.go:82-120` `Config` struct;
`internal/config/load.go:36-58` `applyDefaults`; `internal/config/validate.go:19-55`
`Validate()`. `QuickDeployConfig` (`config.go:72-79`) is the closest precedent
for a small, all-optional, zero-value-safe sub-block (both fields default
`false`, no defaulting entry). A new `AIConfig{Endpoint, Model string; Enabled
bool}` should follow the same "zero value is safe" discipline: omitted block →
`Enabled=false` → feature off, no defaulting needed.

**Doctor-check typed field** — `internal/prereq.Checker` (unlike `internal/app`)
already holds typed clients directly (`Git *git.Service`, `SF
salesforce.Client`, `GH github.Client` — `checker.go:31-49`), composed via the
scalar `NewChecker` factory in `main.go`. So `prereq.Checker.AI` can hold a typed
`ai.Client` interface directly (matching the `GH` field precedent) — the
scalar-func indirection is only required for the code path that runs **inside
`internal/app`** (the push-prep suggestion itself), not for the doctor check.

## Affected Areas

- `internal/ai/` (NEW) — HTTP client (`client.go`), prompt construction
  (`prompt.go`), response parsing (`parse.go`); the sole `net/http` boundary for
  this feature, mirroring `internal/update`.
- `internal/app/boundary_test.go:46-52` — add
  `github.com/malavolta/DeployDeck/internal/ai` to the `forbidden` slice.
- `internal/config/{config,load,validate}.go` — new `AIConfig` block +
  defaulting/validation.
- `internal/prereq/{checker.go,checker_ai.go (NEW),checker_check.go}` —
  informative, non-blocking `CheckAI`.
- `internal/app/app.go` — `Deps.GenerateSummary` scalar field; new `Model`
  fields (`aiTitle`, `aiDescription`, `aiErr`, `aiPending`).
- `internal/app/commands.go` — new `aiSuggestCmd`; reuses `m.plan.SelectedCommits`
  + `m.summary`, no new git/delta calls.
- `internal/app/update.go` — new `onAISuggestDone` reducer; `createPRCmd`'s title
  source becomes conditional on an explicit "use AI title" acceptance.
- `internal/app/keys.go:356-402` `keyPushPreparation` — new key (e.g. `a`) on
  `pushReady` to request/accept the AI suggestion.
- `internal/app/view.go:740-794` `viewPRData` — render the AI-suggested block,
  never replacing the formula title silently.
- `cmd/deploydeck/main.go:196-261` — compose `ai.New(...)` once; wire into both
  `prereq.Checker.AI` (typed) and `app.Deps.GenerateSummary` (scalar closure).
- Living specs: `openspec/specs/prereq-check/` and the push-pr living spec —
  delta additions.

## Approaches

1. **OpenAI-compatible `/v1/chat/completions` only (RECOMMENDED)** — single
   request/response shape (`{model, messages, stream:false}` →
   `choices[0].message.content`), reachable on Ollama (ships this compat
   surface), LM Studio, and llama.cpp's `llama-server` (all OpenAI-compatible).
   The ONLY shape common to all three runtimes HISTORIAS names. Doctor check via
   `GET <endpoint>/v1/models`. Pros: satisfies multi-runtime requirement with ONE
   client/test surface; config stays exactly `{endpoint, model, enabled}` (no
   `provider` discriminator). Cons: response parsing depends on the model
   following an instructed format → needs a strict, testable parser with a safe
   "give up → no suggestion" fallback. Effort: Medium.
2. **Ollama-native `/api/generate` only** — simpler envelope but NOT implemented
   by LM Studio/llama.cpp; contradicts the multi-runtime principle; forces a
   second client path later. Effort: Low now, High later.
3. **Support both (provider field)** — doubles request/response shapes and test
   matrix; HISTORIAS' example config has no provider field; unjustified
   complexity for an explicitly optional/deferred slice. Effort: High.

**Recommendation**: Approach 1. Trigger the suggestion on an **explicit user
keypress** at `pushReady` (not auto-fired alongside `preparePRCmd`) to avoid an
automatic inference call (local CPU/GPU cost) on every push, consistent with
this codebase's explicit-confirmation discipline.

## Testing Strategy (e2e is a mandatory requirement)

| Layer | Scope | Approach |
|---|---|---|
| Unit | Prompt construction (ticket + commit subjects + `PackageSummary` counts → prompt) + truncation (caps on commit count/subject length/type-member list, char budget) | table-driven, pure functions |
| Unit | Response parsing (title/description extraction, malformed/partial degrade) | table-driven, raw model-text fixtures |
| Unit | Config validation (`Enabled=true` requires non-empty `Endpoint`+`Model`; omitted block valid/off) | mirrors `config/validate_test.go` |
| Unit | Degradation branches: nil `Deps.GenerateSummary`, nil `prereq.Checker.AI` | mirrors `checker_gh_test.go` nil-GH-skip + `checkUpdateCmd` nil-func test |
| Integration/e2e (hermetic, **runs in CI**) | Full generate→parse→suggest + `CheckAI` states, against a fake endpoint | `net/http/httptest.Server` with canned OpenAI-compatible JSON (mirrors `internal/update/checker_test.go`): success, unreachable, timeout, malformed/oversized-body cap; doctor states: reachable+model-listed / reachable+model-missing / unreachable |
| Integration | TUI: `keyPushPreparation` request/accept flow via `Model.Update` with a fake `Deps.GenerateSummary`, asserting the AI title reaches `createPRCmd`'s `gh pr create --title` ONLY after explicit acceptance | mirrors `push_preparation_test.go` fake-composition style |
| Real-model e2e (opt-in, local-only, **never CI**) | Hits a real local Ollama/LM Studio/llama.cpp, asserts a non-empty conventional-commit-shaped title | new env vars `DEPLOYDECK_E2E_AI_ENDPOINT`/`DEPLOYDECK_E2E_AI_MODEL`, gated like `internal/salesforce/real_org_e2e_test.go:40-49` (env-var `t.Skip` when unset) |

CI (`.github/workflows/ci.yml`) runs `go test ./... -race -short`. The existing
convention has two independent gates: git/sgd integration tests skip on
`testing.Short()`; real-org tests skip on the env var alone. Recommendation:
follow the **real-org precedent** (env-var-only gate) for the real-model AI e2e,
for consistency.

## Risks

1. Small local models may not reliably follow an instructed output format — the
   parser must degrade to "no suggestion", not surface garbled text; needs strong
   malformed-output coverage.
2. `internal/app/boundary_test.go` is a hard gate — any accidental direct
   `net/http`/`internal/ai` import in internal/app fails CI; the scalar-func
   signature must be right up front.
3. The `AIConfig` doctor check must stay `StatusWarning`-only, never blocking
   (unreachable local model is normal for most users).
4. Auto-fire-vs-on-demand trigger UX has no exact existing precedent — needs an
   explicit decision in the proposal.
5. Real-model e2e env-var naming/gating (single vs double gate) — confirm in
   proposal.

## Open Questions For The Proposal

1. Auto-fire the AI suggestion right after push vs on-demand keypress?
   (Recommendation: on-demand.)
2. Exact scalar `Deps.GenerateSummary` signature — proposed:
   `func(ctx, ticket string, commitSubjects []string, componentSummary string)
   (title, description string, err error)`.
3. Real-model e2e: single env-var gate (matches `real_org_e2e_test.go`) vs
   double-gate with `testing.Short()`? (Recommend single.)
4. Does the accepted AI title replace `github.SuggestedTitle` only for
   `gh pr create --title`, or also the compare-URL manual-copy display? (Likely
   both — same display slot, different string source.)
5. Model response contract: fixed two-line `TITLE:`/`DESCRIPTION:` vs JSON?
   (Former is simpler to parse leniently for small models.)

## Ready for Proposal

Yes — integration point (`preparePRCmd`/`onPushDone`/`pushReady`), degradation
pattern (`CheckGH`/`checkUpdateCmd`), HTTP-client pattern
(`internal/update/checker.go`), boundary-test extension point, and config
precedent (`QuickDeployConfig`) are all concretely identified. Resolve the open
questions in `sdd-propose`/`sdd-design` rather than assuming.
