# Apply Progress: `ai-pr-summary`

**Status**: All 34 tasks complete (Phases 1-8). `go build ./...`, `go vet ./...`,
`gofmt -l .`, and `go test ./... -race` (and `-race -short`) all green.
Nothing committed — all changes left in the working tree per instructions.

## Test Commands + Results (exact)

```
$ go build ./...
(clean, exit 0)

$ gofmt -l .
(no output — clean)

$ go vet ./...
(clean, exit 0)

$ go test ./... -race
ok  	github.com/malavolta/DeployDeck/cmd/deploydeck	6.593s
ok  	github.com/malavolta/DeployDeck/internal/ai	1.750s
ok  	github.com/malavolta/DeployDeck/internal/app	26.489s
ok  	github.com/malavolta/DeployDeck/internal/config	1.298s
ok  	github.com/malavolta/DeployDeck/internal/delta	7.779s
ok  	github.com/malavolta/DeployDeck/internal/exec	3.122s
ok  	github.com/malavolta/DeployDeck/internal/git	51.772s
ok  	github.com/malavolta/DeployDeck/internal/github	3.245s
ok  	github.com/malavolta/DeployDeck/internal/prereq	4.257s
ok  	github.com/malavolta/DeployDeck/internal/runs	2.075s
ok  	github.com/malavolta/DeployDeck/internal/salesforce	2.666s
ok  	github.com/malavolta/DeployDeck/internal/update	2.729s
ok  	github.com/malavolta/DeployDeck/internal/version	1.917s

$ go test ./... -race -short
(all 13 packages ok, hermetic internal/ai suite included, real-org/real-model
e2e self-skip on missing DEPLOYDECK_E2E_ORG / DEPLOYDECK_E2E_AI_ENDPOINT+MODEL)

$ go test ./internal/ai/... -run TestE2ERealModel -v
--- SKIP: TestE2ERealModel_GenerateSummary_NonEmptyTitle (env unset)
--- SKIP: TestE2ERealModel_Doctor_Ready (env unset)
```

## TDD Cycle Evidence (Strict TDD Mode)

| Task | RED (test written first, confirmed failing) | GREEN (implementation passes) | REFACTOR |
|---|---|---|---|
| 1.1/1.2 `ParseSummary` | `parse_test.go` written first; `go test ./internal/ai/...` failed with "no non-test Go files" | `parse.go` implemented; all 12 cases pass | shared `capRunes` extracted to `text.go` for prompt.go reuse |
| 1.3/1.4 `Build` | `prompt_test.go` added; failed with `undefined: ai.Build` (5 errors) | `prompt.go` implemented; all 5 tests pass | caps as named consts, no further refactor needed |
| 1.5/1.6 `Client` | `client_test.go` added; failed with `undefined: ai.New`/`SummaryRequest`/etc (10+ errors) | `client.go` implemented; all 4 tests pass | none |
| 1.7 hermetic e2e | `client_e2e_test.go` written directly against the already-passing `client.go` (mirrors `internal/update/checker_test.go`'s pattern — RED was implicit in 1.5/1.6's guards); ran green immediately, all 7 sub-cases | same | none |
| 2.1/2.2/2.3 `AIConfig` | Added mutation cases to `validate_test.go` + new cases in `config_test.go`; failed with `cfg.AI undefined`/`undefined: config.AIConfig` (8 errors) | `AIConfig` struct + `Config.AI` field + `Validate()` rule added; all cases pass | none |
| 3.1 `CheckAI` | `checker_ai_test.go` written first; failed with `checker.CheckAI undefined` / `unknown field AI` (7 errors) | `Checker.AI` field + `checker_ai.go` implemented; all 4 tests pass | none |
| 3.4 `Check()` wiring | Added `TestChecker_Check_NilAI_IncludesSkippedAICheck`; failed (assertion: "AI model" check missing) | wired `CheckAI` into `Check()`; passes | none |
| 4.1 boundary forbid | Added `internal/ai` to `forbidden` list; test still passes (guard, not new-behavior RED) — confirmed no `internal/ai` import exists yet | n/a (guard only) | none |
| 4.2 `Deps.GenerateSummary` + Model fields | Structural addition (no isolated failing test — enabling task for 4.3+) | `Deps.GenerateSummary` scalar + 5 Model AI fields added; `go build` clean | none |
| 4.3 `effectiveTitle()` | `effective_title_test.go` written first; failed with `m.effectiveTitle undefined` (3 errors) | implemented; both tests pass | none |
| 4.4/4.5 `aiSuggestCmd` | `commands_test.go` written first; failed with `m.aiSuggestCmd undefined` (3 errors) | `aiSuggestCmd`, `aiSuggestDoneMsg`, `aiSuggestTimeout` added; `createPRCmd` switched to `effectiveTitle()`; all 3 tests pass | pure helpers `commitSubjects`/`renderComponentSummary` extracted to `flow.go` first (needed by the RED test itself) |
| 4.6/4.7 `onAISuggestDone` | `update_test.go` written first; ran RED — assertions failed (aiPending stayed true, aiTitle stayed empty) since `Update()` had no case for the msg | `onAISuggestDone` reducer + `Update()` case added; both tests pass | none |
| 4.8/4.9 `a` key | `keys_test.go` written first; 2 of 5 tests failed RED (request + accept assertions) | `a` case added to `keyPushPreparation`'s `pushReady` branch; all 5 pass | none |
| 4.10/4.11 view | `view_test.go` written first; 4 of 5 tests failed RED (no AI block rendered at all) | `viewAIBlock()` + `viewPRData` switched to `effectiveTitle()`; all 5 pass | none |
| 4.12/5.2 full-flow integration | `push_preparation_ai_test.go` written directly (implementation from 4.2-4.11 already in place, so this is the composition proof, not a fresh RED) — ran green immediately, all 3 tests pass, including the 2-subtest accept-gating table | same | none |
| 5.1 threat-matrix parser | Covered by the same `parse_test.go` fixtures written in 1.1 (newline/`--`/oversized/garbled cases already exhaustive) | same | none |
| 6.1/6.2 `main.go` composition | `main_ai_test.go` written first; failed with `undefined: composeGenerateSummary` (2 errors) | `composeAIClient`/`composeGenerateSummary` helpers + wiring into `defaultChecker`/`defaultRunTUI`; all 4 tests pass | none |
| 7.1 real-model e2e | Written directly (self-skip is the only testable behavior without a live server); confirmed SKIP output | same | none |

## Work Unit Evidence

| Evidence | Value |
|---|---|
| Focused test command and exact result | `go test ./internal/ai/... ./internal/config/... ./internal/prereq/... ./internal/app/... ./cmd/deploydeck/...` — all green, 0 failures |
| Runtime harness command/scenario and exact result | `go test ./internal/ai/... -race -short -v` — hermetic `httptest.Server` e2e (success/unreachable/timeout/malformed/oversized-body/doctor-3-state) all pass; `go test ./internal/ai/... -run TestE2ERealModel -v` — self-skips cleanly (no `DEPLOYDECK_E2E_AI_ENDPOINT`/`_MODEL` in this environment) |
| Rollback boundary | Purely additive, default-off. Revert `internal/ai/` (no callers without config), the `AIConfig`/`Checker.AI`/`checker_ai.go` additions (inert when unused), and the `internal/app`/`cmd/deploydeck/main.go` diffs (flow reverts byte-for-byte to pre-existing HU-014 push/PR behavior, proven by `TestView_AI_NoConfig_NoAffordanceShown` and the full pre-existing `push_preparation_test.go` suite staying green unmodified) |

## Files Changed

| File | Action | What Was Done |
|---|---|---|
| `internal/ai/client.go` | Created | Sole `net/http` boundary: `Client` iface, `client` impl, `New`, `GenerateSummary` (POST `/v1/chat/completions`), `Doctor` (GET `/v1/models`, 3-state `DoctorState`) |
| `internal/ai/parse.go` | Created | `ParseSummary(raw) (title, description string, ok bool)` — lenient `TITLE:`/`DESCRIPTION:` label parser, fence-stripping, control-char stripping, length/line caps |
| `internal/ai/prompt.go` | Created | Pure `Build(ticket, commitSubjects, componentSummary) (system, user string)` with truncation caps |
| `internal/ai/text.go` | Created | Shared `capRunes` helper |
| `internal/ai/parse_test.go` | Created | RED fixtures: labeled/fenced/partial/newline/`--`-prefixed/oversized/garbled/empty + threat-matrix extensions |
| `internal/ai/prompt_test.go` | Created | Table-driven truncation-cap tests |
| `internal/ai/client_test.go` | Created | Nil-guard + request-shape unit tests via `httptest.Server` |
| `internal/ai/client_e2e_test.go` | Created | Hermetic `-race -short` e2e: success/unreachable/timeout/malformed/oversized-body/doctor-3-state |
| `internal/ai/real_model_e2e_test.go` | Created | Opt-in real-model e2e, gated by `DEPLOYDECK_E2E_AI_ENDPOINT`/`_MODEL`, self-skips |
| `internal/config/config.go` | Modified | `AIConfig{Endpoint,Model string;Enabled bool}` + `Config.AI` field |
| `internal/config/validate.go` | Modified | `AI.Enabled` ⇒ non-empty `Endpoint`+`Model` |
| `internal/config/config_test.go` | Modified | `AISection` Load round-trip + zero-value-safe tests |
| `internal/config/validate_test.go` | Modified | `AIConfig` validation mutation cases |
| `internal/prereq/checker.go` | Modified | `AI ai.Client` field |
| `internal/prereq/checker_ai.go` | Created | `CheckAI` (nil-skip, 3-state, warning-only, never blocking) |
| `internal/prereq/checker_check.go` | Modified | Registers `CheckAI` in `Check()` |
| `internal/prereq/checker_ai_test.go` | Created | Nil-guard + Ready/ModelMissing/Unreachable + `Check()` regression test |
| `internal/app/app.go` | Modified | `Deps.GenerateSummary` scalar; Model AI fields (`aiTitle`,`aiDescription`,`aiAccepted`,`aiPending`,`aiErr`); `effectiveTitle()` |
| `internal/app/commands.go` | Modified | `aiSuggestCmd`, `aiSuggestDoneMsg`, `aiSuggestTimeout`; `createPRCmd` title → `effectiveTitle()` |
| `internal/app/update.go` | Modified | `onAISuggestDone` reducer wired into `Update()` |
| `internal/app/keys.go` | Modified | `a` case in `keyPushPreparation`'s `pushReady` branch (request/accept/inert) |
| `internal/app/view.go` | Modified | `viewAIBlock()`; `viewPRData` title → `effectiveTitle()` |
| `internal/app/flow.go` | Modified | Pure helpers `commitSubjects`, `renderComponentSummary`, `joinTypeCounts` |
| `internal/app/boundary_test.go` | Modified | `internal/ai` added to `forbidden` imports |
| `internal/app/effective_title_test.go` | Created | `effectiveTitle()` unit tests |
| `internal/app/commands_test.go` | Created | `aiSuggestCmd` unit tests (fake dep) |
| `internal/app/update_test.go` | Created | `onAISuggestDone` unit tests |
| `internal/app/keys_test.go` | Created | `a` key state-machine tests |
| `internal/app/view_test.go` | Created | AI-block rendering tests |
| `internal/app/push_preparation_ai_test.go` | Created | Full `Model.Update` integration: request→accept→`gh pr create --title` gating, never-auto-fires-after-push |
| `cmd/deploydeck/main.go` | Modified | `composeAIClient`/`composeGenerateSummary`; wired into `defaultChecker`/`defaultRunTUI` |
| `cmd/deploydeck/main_ai_test.go` | Created | Composition wiring tests (enabled/disabled) |
| `openspec/changes/ai-pr-summary/tasks.md` | Modified | All 34 tasks marked `[x]` |
| `openspec/changes/ai-pr-summary/proposal.md` | Modified | Success Criteria checked off |

## Deviations From Design

None — implementation matches design.md's ADRs (1-7), Data Flow sequence, File
Changes table, and Threat Matrix exactly. One clarification applied where
design left room: "compose `ai.New(...)` once" (task 6.2) was implemented as
a single shared `composeAIClient` helper function called from both
`defaultChecker` and `defaultRunTUI` (each still produces its own `ai.Client`
instance per call site, mirroring the pre-existing `github.New(runner)`
precedent in the same file — two composition call sites, one shared
construction helper).

## Issues Found

None. All existing test suites (`internal/app`, `internal/prereq`,
`internal/config`, `cmd/deploydeck`, and the rest of the module) stayed green
throughout — the AI-disabled default path is proven byte-for-byte identical
to pre-existing HU-014 behavior by `TestView_AI_NoConfig_NoAffordanceShown`
and the full pre-existing `push_preparation_test.go` suite passing unmodified.

## Workload / PR Boundary

- Mode: single PR, `size:exception` (pre-approved per session config)
- Current work unit: N/A — implemented as one coherent change per instructions
- Boundary: entire `ai-pr-summary` change, Phases 1-8, all 34 tasks
- Actual diff size: 2261 insertions / 9 deletions across 32 files (production
  code + tests + e2e; exceeds the tasks.md forecast of ~850-1050 lines —
  the exhaustive threat-matrix/degradation test coverage this domain demands,
  mirroring existing project conventions like `internal/update`/`checker_gh_test.go`,
  accounts for the difference). `size:exception` was already granted for this
  single-PR delivery regardless of exact count.

## Status

34/34 tasks complete. Ready for verify.
