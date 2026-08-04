# Tasks: Incremental Promotion (reuse-on-collision)

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | ~1100-1200 (5 new files + 5 modified files + RED tests + E2E; design risk: Medium-High) |
| 400-line budget risk | High |
| Chained PRs recommended | No |
| Suggested split | Single PR, 2 sequential internal commits (design's own "Slice 1 / Slice 2" framing) |
| Delivery strategy | single-pr |
| Chain strategy | size-exception |

Decision needed before apply: No
Chained PRs recommended: No
Chain strategy: size-exception
400-line budget risk: High

Rationale: `size:exception` is pre-accepted (1.0.0, auto-forecast-only). Slice 2 (app
wiring) hard-depends on Slice 1's primitives (`FastForwardBranch`, `FilterNotOnBranch`,
`PRForBranch`, `FindRunForBranch`) compiling first, so splitting into separate PRs would
only add review-order friction without reducing the reviewer's real surface (one flow,
`onBranchCreated`→`onReuseReady`→`onPrepDone`). The 2 slices stay the rollback unit
inside the one PR.

### Suggested Work Units (sequential commits inside the single PR)

| Unit | Goal | Focused test command | Runtime harness | Rollback boundary |
|------|------|----------------------|-----------------|-------------------|
| 1 | git/github/runs primitives, isolated | `go test ./internal/git/... ./internal/github/... ./internal/runs/... -run "Trailer\|FastForward\|FilterNotOnBranch\|PRForBranch\|FindRunForBranch"` | Integration tests run against a real temp git repo (`internal/git` existing pattern); `PRForBranch`/`FindRunForBranch` are pure/fake-Runner, N/A exec harness | Revert `trailers.go`, `service_reuse.go`, `finder.go`, and the `PRForBranch` addition to `github/client.go` — zero app-layer callers yet |
| 2 | App wiring (collision UX, reuse flow, PR gate) | `go test ./internal/app/... -run "BranchCollision\|ReuseReady\|IncrementalPromotion"` | `go test ./internal/app/... -race` (existing e2e suites double as regression harness) | Revert `StateBranchCollision`/`reusing` (`app.go`), the routing in `update.go`/`commands.go`/`keys.go`/`view.go` — `onBranchCreated`'s other branches (`err==nil`, unrelated errors) stay untouched |

## Phase 1: git/github/runs Primitives (Slice 1)

- [x] 1.1 RED: `internal/git/trailers_test.go` — `ParseCherryPickTrailers` cases: single trailer / no trailer / multiline log with mixed trailers / malformed trailer line (ignored, no panic). Compile fails (`ParseCherryPickTrailers` undefined).
- [x] 1.2 GREEN: `internal/git/trailers.go` — pure `ParseCherryPickTrailers(gitLog []byte) map[string]bool` parsing `(cherry picked from commit <sha>)` lines.
- [x] 1.3 RED: `internal/git/service_reuse_test.go` — `FastForwardBranch` in a real temp repo: behind→`FFFastForwarded`; ahead→`FFLocalAhead` (no-op); remote absent→`FFRemoteAbsent`; diverged→`FFDiverged` returned as DATA (assert `merge --ff-only` never force-pushes/rewrites).
- [x] 1.4 GREEN: `internal/git/service_reuse.go` — `FFResult` enum (`FFUpToDate|FFFastForwarded|FFLocalAhead|FFRemoteAbsent|FFDiverged`) + `Service.FastForwardBranch(ctx, dir, branch)` via `merge --ff-only`, exit-code-as-data.
- [x] 1.5 RED: same file — `FilterNotOnBranch` layered cases: ancestor match (`IsAncestor`) → excluded; trailer match (`ParseCherryPickTrailers` over `git log <deployBranch>`) → excluded; cherry-equivalence fallback (`Cherry`) → excluded; none match → kept; empty `sourceRef` degrades (skips `Cherry`, no error).
- [x] 1.6 GREEN: `Service.FilterNotOnBranch(ctx, dir, deployBranch, sourceRef, commits) (remaining, err)` composing `IsAncestor` → `ParseCherryPickTrailers` → `Cherry` in order.
- [x] 1.7 RED: `internal/github/pr_for_branch_test.go` (mirrors `client_test.go`'s `exec.FakeRunner` pattern) — `PRForBranch`: open PR (`state=OPEN`)→`open=true`; closed/merged→`open=false,err=nil`; no PR (`gh pr view` non-zero exit)→`open=false,err=nil`; Runner error→`err!=nil`.
- [x] 1.8 GREEN: `internal/github/client.go` — add `PRForBranch(ctx, branch) (url string, open bool, err error)` to `Client` interface + `client` impl via `gh pr view <branch> --json url,state`.
- [x] 1.9 RED: `internal/runs/finder_test.go` — `FindRunForBranch`: match / no-match / newest-of-many (records not already sorted assumption) re-rendering `RenderBranchName`.
- [x] 1.10 GREEN: `internal/runs/finder.go` — pure `FindRunForBranch(records []Record, format, branchName string) (Record, bool)`.
- [x] 1.11 Verification: `go build ./...` && `PATH=/usr/local/go/bin:$PATH go test ./... -race` — GREEN.

## Phase 2: App Wiring (Slice 2)

- [x] 2.1 RED: `internal/app/branch_collision_test.go` — `TestOnBranchCreated_ErrPromotionBranchExists_RoutesToStateBranchCollision` (fake `git.CreatePromotionBranch` returns `git.ErrPromotionBranchExists`) and `TestOnBranchCreated_OtherError_StaysStateError` (unrelated error still lands `StateError`, unchanged). Compile fails (`StateBranchCollision` undefined).
- [x] 2.2 GREEN: `internal/app/app.go` — add `StateBranchCollision` const + `reusing bool` field on `Model`; `internal/app/update.go`'s `onBranchCreated` special-cases `errors.Is(msg.err, git.ErrPromotionBranchExists)` before the existing generic-error branch.
- [x] 2.3 RED: same file — `TestKeyBranchCollision_R_D_C`: `r`→fires `reuseBranchCmd` (state stays `StateBranchCollision` until `reuseReadyMsg`); `d`→`DeleteLocalBranch`+re-fires `branchCreateCmd`; `c`/`esc`→`StatePlanPreview`, branch/PR untouched.
- [x] 2.4 GREEN: `internal/app/keys.go` — `keyBranchCollision`, routed from `Update`'s key switch on `StateBranchCollision`.
- [x] 2.5 RED: `TestReuseBranchCmd_ChecksOutFastForwardsFilters` — fake `git.Service` asserts call order `Checkout`→`FastForwardBranch`→`FilterNotOnBranch(deployBranch, sourceRef=m.source.Name or "" , m.plan.SelectedCommits)`.
- [x] 2.6 GREEN: `internal/app/commands.go` — `reuseBranchCmd` returning `reuseReadyMsg{remaining, ffResult, err}`.
- [x] 2.7 RED: `TestOnReuseReady` table: `FFDiverged`/`err!=nil`→`StateError` (clear message, no push/overwrite); `len(remaining)==0`→notice ("nada nuevo para aplicar") + `StatePushPreparation`; remainder→`runs.FindRunForBranch` match→mutate in place (extend `Commits`, bump `PickTotal`/`UpdatedAt`, keep `CreatedAt`/`RunID`, `SourceRunID` untouched)+`Save`→`StateCherryPicking`; no match→proceeds as a fresh increment target (no error, per spec).
- [x] 2.8 GREEN: `update.go` — `onReuseReady`, setting `m.reusing=true`, `m.plan.SelectedCommits=remaining`, `m.runID`/`m.plan` accordingly.
- [x] 2.9 RED: extend the app-test `exec.FakeRunner` canning with a `cannPRForBranch` helper (`gh pr view <branch> --json url,state`); `TestPreparePRCmd_Reusing_SkipsCreateWhenPROpen` (open→no `gh pr create` call, `prURL` set) and `TestPreparePRCmd_Reusing_ClosedFallsThroughToCreateWithNotice` (closed/merged→normal create path + notice).
- [x] 2.10 GREEN: `commands.go`'s `preparePRCmd` calls `PRForBranch` only when `m.reusing && auth==github.AuthAuthenticated`; `update.go`'s `onPrepDone` gates PR creation on the result (skip-create/open, fallthrough/closed, degrade unchanged when `gh` absent/unauth).
- [x] 2.11 RED: `internal/app/view_test.go` — `TestViewBranchCollision_ShowsThreeChoices` asserts reuse/delete/cancel are all rendered.
- [x] 2.12 GREEN: `internal/app/view.go` — `StateBranchCollision` screen.
- [x] 2.13 RED/E2E: `internal/app/incremental_promotion_e2e_test.go` — full reuse flow (fake git+gh): reused branch appends only the not-yet-present commits, mutates the SAME `RunID`'s record (not a new one), and skips `gh pr create` when a PR is already open.
- [x] 2.14 GREEN: close any remaining wiring gaps surfaced by 2.13 (none — the E2E passed on first run, confirming the unit-level TDD wiring from 2.1-2.12 was already correct).
- [x] 2.15 Verification: `go build ./...` && `PATH=/usr/local/go/bin:$PATH go test ./... -race` — GREEN, all packages pass.
