# Tasks: Local-Model PR Title/Description Suggestion (`ai-pr-summary`)

## Review Workload Forecast

| Field | Value |
|---|---|
| Estimated changed lines | ~850-1050 (new pkg ~350 incl. tests; config ~40; prereq ~90; app 5 files ~180; main.go ~40; integration/e2e ~250) |
| 400-line budget risk | High |
| 800-line budget risk | Medium-High |
| Chained PRs recommended | Yes |
| Suggested split | PR1 `internal/ai`+config → PR2 prereq `CheckAI` → PR3 app wiring+main.go+e2e |
| Delivery strategy | single-pr |
| Chain strategy | feature-branch-chain |

Decision needed before apply: Yes
Chained PRs recommended: Yes
Chain strategy: feature-branch-chain
400-line budget risk: High

`single-pr` requires `size:exception` before apply, or the orchestrator/user
picks feature-branch-chain to split PR1/2/3 below.

### Suggested Work Units

| Unit | Goal | PR (base) | Focused test | Runtime harness | Rollback boundary |
|---|---|---|---|---|---|
| 1 | `internal/ai` client/prompt/parse + `AIConfig` | PR1 (feature branch) | `go test ./internal/ai/... ./internal/config/...` | `httptest.Server` canned-JSON e2e, `-race -short` | revert `internal/ai/`, `AIConfig` block — no callers yet |
| 2 | `prereq.Checker.AI` + `CheckAI` | PR2 (base=PR1) | `go test ./internal/prereq/...` | N/A — hermetic unit/e2e covers 3 states | revert `checker_ai.go`, `Checker.AI`, registration |
| 3 | `internal/app` wiring + `main.go` + TUI integration + threat tests | PR3 (base=PR2) | `go test ./internal/app/... ./cmd/deploydeck/...` | opt-in real-model e2e (`DEPLOYDECK_E2E_AI_ENDPOINT/MODEL`, local-only) | revert `Deps.GenerateSummary`, Model AI fields, key/view, `main.go` — flow reverts to pre-existing HU-014 |

## Phase 1: `internal/ai` Package
- [x] 1.1 RED `parse_test.go`: fixtures — labeled, fenced, partial (title-only), newline-title, `--`-prefixed, oversized, garbled, empty
- [x] 1.2 GREEN `parse.go`: `ParseSummary(raw)(title,desc string,ok bool)` — strip fences, `TITLE:`/`DESCRIPTION:` labels, title first-line+ctrl-strip+cap 120, empty title→`ok=false`
- [x] 1.3 RED `prompt_test.go`: table-driven truncation caps (max subjects, per-subject len, total char budget)
- [x] 1.4 GREEN `prompt.go`: pure `Build(ticket, commitSubjects, componentSummary)(system,user string)`
- [x] 1.5 RED `client_test.go`: nil-`HTTPClient` guard; `GenerateSummary`/`Doctor` request shape via `httptest.Server`
- [x] 1.6 GREEN `client.go`: `Client` iface (`GenerateSummary`,`Doctor`), `client{Endpoint,Model,HTTPClient}`, `New(...)`; `NewRequestWithContext`+timeout+`LimitReader(1<<20)`; POST `/v1/chat/completions`→`ParseSummary`; GET `/v1/models`→3-state `DoctorState`
- [x] 1.7 RED+GREEN `client_e2e_test.go` (`-race -short`, CI): success/unreachable/timeout/malformed/oversized-body for `GenerateSummary`; listed/missing/unreachable for `Doctor`

## Phase 2: `internal/config` — `AIConfig`
- [x] 2.1 RED `validate_test.go`: absent `ai`→off/valid; `enabled:true`+empty endpoint or model→error; both set→valid
- [x] 2.2 GREEN `config.go`: `AIConfig{Endpoint,Model string;Enabled bool}` (mirrors `QuickDeployConfig`), `Config.AI yaml:"ai"`
- [x] 2.3 GREEN `validate.go`: `AI.Enabled`⇒non-empty `Endpoint`+`Model`

## Phase 3: `internal/prereq` — `CheckAI`
- [x] 3.1 RED `checker_ai_test.go` (mirrors `checker_gh_test.go`): nil `AI`→OK-skip; `Ready`→OK; `ModelMissing`/`Unreachable`→Warning; never Blocking
- [x] 3.2 GREEN `checker.go`: add `AI ai.Client` field
- [x] 3.3 GREEN `checker_ai.go`: implement `CheckAI` per 3.1
- [x] 3.4 RED+GREEN `checker_check_test.go`/`checker_check.go`: `Check()` includes `CheckAI`

## Phase 4: `internal/app` — Wiring And TUI Affordance
- [x] 4.1 RED `boundary_test.go`: add `internal/ai` to `forbidden`
- [x] 4.2 GREEN `app.go`: `Deps.GenerateSummary func(ctx,ticket string,commitSubjects []string,componentSummary string)(title,desc string,err error)` (ADR-1, scalar); Model fields `aiTitle,aiDescription string; aiAccepted,aiPending bool; aiErr error`
- [x] 4.3 RED+GREEN `effective_title_test.go`+`app.go`: `effectiveTitle()` = `aiTitle` if `aiAccepted` else `github.SuggestedTitle(...)`
- [x] 4.4 RED `commands_test.go`: `aiSuggestCmd` w/ fake dep — builds ticket/subjects (`m.plan.SelectedCommits[].Commit.Subject`)/`componentSummary` (`m.summary`), emits `onAISuggestDone`
- [x] 4.5 GREEN `commands.go`: `aiSuggestCmd()` (nil-guard, `WithTimeout`, mirrors `checkUpdateCmd`); `createPRCmd` title→`m.effectiveTitle()`
- [x] 4.6 RED `update_test.go`: `onAISuggestDone` — err/empty-title→`aiPending=false`,`aiErr` set, no title change; non-empty→`aiTitle/aiDescription` set, `aiAccepted` stays false
- [x] 4.7 GREEN `update.go`: wire `aiSuggestDoneMsg`→`onAISuggestDone`
- [x] 4.8 RED `keys_test.go`: `a` at `pushReady` — no suggestion+dep!=nil→`aiPending=true`+`aiSuggestCmd`; unaccepted suggestion→accept; pending/accepted→inert; dep nil→inert
- [x] 4.9 GREEN `keys.go`: `a` case in `keyPushPreparation`'s `pushReady` branch (ADR-7)
- [x] 4.10 RED `view_test.go`: no config→AI block absent, screen byte-for-byte unchanged; pending/unaccepted/accepted states render correctly; accepted→title/PR-preview/compare-URL all show `aiTitle`
- [x] 4.11 GREEN `view.go`: render AI block at `pushReady`; `viewPRData`+compare-URL block→`m.effectiveTitle()`
- [x] 4.12 RED+GREEN `push_preparation_ai_test.go` (mirrors `push_preparation_test.go`): fake dep, full `Model.Update` sequence — title reaches `createPRCmd`'s `--title` ONLY after 2nd `a`

## Phase 5: Threat-Matrix RED Tests (Untrusted AI Title → `gh pr create --title`)
- [x] 5.1 RED `parse_test.go` (extends 1.1): newline→first-line-only; `--`-prefixed→literal content, never re-parsed as flag; oversized→truncated, `ok=true`
- [x] 5.2 RED `push_preparation_ai_test.go` (extends 4.12): accepted title arrives at `gh.CreatePR` as one discrete `Args` element (never shell-joined); present only after 2nd `a`, never before accept or auto after push

## Phase 6: `cmd/deploydeck/main.go` Composition
- [x] 6.1 RED `main_ai_test.go`: `AI.Enabled=false`/absent→`Checker.AI`+`Deps.GenerateSummary` nil; `Enabled=true`→both wired
- [x] 6.2 GREEN `main.go`: compose `ai.New(cfg.AI.Endpoint,cfg.AI.Model,&http.Client{})` once; wire `Checker.AI`+`Deps.GenerateSummary` closure only when `cfg.AI.Enabled`

## Phase 7: Real-Model Opt-In E2E
- [x] 7.1 `internal/ai/real_model_e2e_test.go` (mirrors `real_org_e2e_test.go`): gated by `DEPLOYDECK_E2E_AI_ENDPOINT`/`_MODEL`, self-skips when unset; real local server (example `qwen2.5-coder:3b`), asserts non-empty title

## Phase 8: Verification
- [x] 8.1 `go build ./...` + `go test ./... -race` all green (opt-in e2e self-skips)
- [x] 8.2 `boundary_test.go` confirms `internal/ai` stays out of `internal/app` imports
- [x] 8.3 Check off `proposal.md` Success Criteria
