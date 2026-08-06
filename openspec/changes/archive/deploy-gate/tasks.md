# Tasks: Deploy Gate (per-environment governance check before quick-deploy)

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | ~1400-1800 (3 config files + new `internal/gate` pure package + 2 github files + 1 provenance file + `pr.go` seam churn + 5 app files, plus RED tests mapping all 29 delta-spec scenarios) |
| 400-line budget risk | High |
| Chained PRs recommended | No |
| Suggested split | Single PR, 6 sequential work units (config → provenance → pr.go seam → github → gate → app) |
| Delivery strategy | exception-ok |
| Chain strategy | size-exception |

Decision needed before apply: No
Chained PRs recommended: No
Chain strategy: size-exception
400-line budget risk: High

Rationale: `size:exception` pre-approved (session delivery note). Phase 3 (`pr.go` seam)
hard-depends on Phase 2's `provenance.BestResult` existing; Phase 5 (`internal/gate`)
hard-depends on Phase 1's `GateConfig`/`MinApprovals *int` and Phase 2's `provenance.Result`
compiling; Phase 6 (`internal/app`) hard-depends on Phases 1, 4, and 5 all compiling
(`GateFor`, `github.Client` methods, `gate.Evaluate`). Splitting across capability
boundaries (config/gate/github/provenance/app) would add review-order friction — each
slice is dead code until the next lands — without shrinking the reviewer's real surface
(one additive, cooperative-gate arc). No real `sf`/`gh` runtime harness applies anywhere
in this change; every unit stays FakeRunner/Ascii-`TestMain`-backed.

### Suggested Work Units (sequential commits inside the single PR)

| Unit | Goal | Focused test command | Runtime harness | Rollback boundary |
|------|------|----------------------|-----------------|-------------------|
| 1 | `internal/config` gate schema, `GateFor`, `Validate` | `go test ./internal/config/... -race` | N/A — table-driven parse/validate unit tests | Revert 3 file diffs; `Gates` unused, zero-value-safe |
| 2 | `internal/provenance.BestResult` extraction | `go test ./internal/provenance/... -race` | N/A — pure ranking unit tests, real test secret | Revert `provenance.go` diff; `pr.go` still has its own copy until Unit 3 |
| 3 | `cmd/deploydeck/pr.go` `bestResult` seam | `go test ./cmd/deploydeck/... -race -run PRVerify` | N/A — FakeRunner-backed CLI test | Revert seam swap; Units 1-2 unaffected |
| 4 | `internal/github` new `Client` methods | `go test ./internal/github/... -race` | N/A — FakeRunner arg-slice assertions | Revert 2 file diffs; methods added but uncalled |
| 5 | `internal/gate` pure evaluator | `go test ./internal/gate/... -race` | N/A — pure unit tests, no gh/exec | Revert new package; not imported yet |
| 6 | `internal/app` wiring + gate-block screen | `go test ./internal/app/... -race -run "Gate\|Deploy\|Report"` | N/A — FakeRunner/Ascii `TestMain`, no real gh/sf | Revert `keys.go` branch + `onReportDone` call site; Units 1-5 stay dead code until this lands |

## Phase 1: `internal/config` — gate schema, resolution, validation

- [x] 1.1 RED `config_test.go`: `Gates map[string]GateConfig` parses `Require{ResolvedThreads,ValidationComment,Signature} *bool` nil (omitted) vs explicit `false`, and `MinApprovals *int` nil vs explicit values.
- [x] 1.2 GREEN `internal/config/config.go`: add `GateConfig` struct (`Enabled`, `Approvers`, `MinApprovals *int`, `Require* *bool`) + `Config.Gates map[string]GateConfig` (mirror `QuickDeployConfig`/`Sandboxes`).
- [x] 1.3 RED `gate_for_test.go`: an exact-match `gates` entry wins over a separate glob entry that would also match (spec: exact-match resolution).
- [x] 1.4 RED `gate_for_test.go`: a target with no exact or glob match → `GateFor` returns `(_, false)` — ungated (spec: no matching entry is ungated).
- [x] 1.5 RED `gate_for_test.go`: a matched entry with `enabled:false` → `GateFor` returns `(_, false)`.
- [x] 1.6 GREEN `internal/config/gate_for.go`: `GateFor(target)(GateConfig,bool)` exact→`path.Match`, mirror `sandbox_for.go`; `bool` true only when matched AND `Enabled`.
- [x] 1.7 RED `validate_test.go`: an enabled gate with explicit `minApprovals` less than 1 is rejected (spec: invalid approval settings).
- [x] 1.8 RED `validate_test.go`: an enabled gate with empty `approvers` is rejected, unconditionally, even when `minApprovals` is omitted (spec: invalid approval settings — empty list).
- [x] 1.9 RED `validate_test.go`: an enabled gate with `minApprovals` omitted (nil) and a non-empty `approvers` list passes validation — nil is not treated as `<1`.
- [x] 1.10 GREEN `internal/config/validate.go`: per enabled gate, reject explicit `MinApprovals < 1` and reject empty `Approvers`.
- [x] 1.11 Verify: `PATH=/usr/local/go/bin:$PATH go test ./internal/config/... -race` GREEN.

## Phase 2: `internal/provenance` — extract `BestResult`

- [x] 2.1 RED `provenance_test.go`: `BestResult(ownerRepo, headBranch, markers)` selects the correct `Verified` result among any-of candidate markers using a real (non-dev) test secret (moves the sig-discriminating proof here per design's Churn row).
- [x] 2.2 GREEN `internal/provenance/provenance.go`: add `BestResult` (+`resultRank`), moved from `pr.go`.
- [x] 2.3 Verify: `PATH=/usr/local/go/bin:$PATH go test ./internal/provenance/... -race` GREEN — package stays exec-free (pure computation over already-fetched marker data).

## Phase 3: `cmd/deploydeck/pr.go` — `bestResult` seam

- [x] 3.1 RED `pr_verify_test.go`: RESTRUCTURE `TestRunPRVerify_AnyOfClassification` — remove the per-marker sig-discriminating fake `verifyFn`; keep only exit-code mapping via a coarse `bestResult` seam (the sig-based any-of proof now lives in `provenance_test.go` from 2.1 — this is a restructure, not a rename).
- [x] 3.2 GREEN `cmd/deploydeck/pr.go`: replace `verifyFn`/`bestVerifyResult`/`verifyResultRank` with `var bestResult = provenance.BestResult` test seam.
- [x] 3.3 Verify: `PATH=/usr/local/go/bin:$PATH go test ./cmd/deploydeck/... -race -run PRVerify` GREEN.

## Phase 4: `internal/github` — new `Client` methods

- [x] 4.1 RED `compare_url_test.go`: `ParsePRURLParts` returns `(host,owner,repo,number,ok)`; `ParsePRURL` delegates and existing callers' behavior is unchanged.
- [x] 4.2 GREEN `internal/github/compare_url.go`: add `ParsePRURLParts`; `ParsePRURL` delegates.
- [x] 4.3 RED `client_test.go` (FakeRunner): `PRReviews` maps `gh pr view <url> --json latestReviews` into `[]Review{Login,State}`; asserts the exact arg slice.
- [x] 4.4 GREEN `internal/github/client.go`: `PRReviews`.
- [x] 4.5 RED `client_test.go`: `UnresolvedThreadCount` via `gh api graphql` counts `isResolved:false` entries (first GraphQL call, discrete args asserted, never shell-joined); a GraphQL error / non-zero / auth failure returns an error (threads-unavailable fail-closed source).
- [x] 4.6 GREEN `internal/github/client.go`: `UnresolvedThreadCount`.
- [x] 4.7 RED `client_test.go`: `PRComments` maps `gh pr view <url> --json comments` into `[]Comment{Login,Body}`; asserts exact arg slice.
- [x] 4.8 GREEN `internal/github/client.go`: `PRComments`.
- [x] 4.9 RED `client_test.go`: `PostComment` builds `gh pr comment <url> --body <body>` as discrete, non-shell-joined args and returns the raw output.
- [x] 4.10 GREEN `internal/github/client.go`: `PostComment`.
- [x] 4.11 Verify: `PATH=/usr/local/go/bin:$PATH go test ./internal/github/... -race` GREEN.

## Phase 5: `internal/gate` (pure) — `Facts`/`Result`/`Evaluate` + helpers

- [x] 5.1 RED `gate_test.go`: `Evaluate` — `!Facts.PRResolved` yields exactly ONE failed `pr-resolution` condition; no other condition is evaluated.
- [x] 5.2 RED `gate_test.go`: `normalizeLogin` strips a leading `@` and lowercases; approvals — `effectiveMin` defaults to 1 when `Config.MinApprovals` is nil, passes once that many distinct normalized approver-list logins have latest state `APPROVED`; non-listed logins never count; a listed login's later `CHANGES_REQUESTED` negates an earlier `APPROVED`; `Facts.ReviewsErr != nil` fails the condition (fail-closed).
- [x] 5.3 GREEN `internal/gate/gate.go`: `Evaluate` approvals branch + `normalizeLogin`.
- [x] 5.4 RED `gate_test.go`: threads — `UnresolvedCount==0` passes, `>0` fails; `Facts.ThreadErr != nil` fails the condition (fail-closed).
- [x] 5.5 GREEN `internal/gate/gate.go`: `Evaluate` threads branch.
- [x] 5.6 RED `gate_test.go`: validation-comment — `Facts.CommentPresent` true passes / false fails, regardless of the comment's author (cooperative, presence-only trust — condition-4's signature is the anti-forgery anchor); `Facts.CommentErr != nil` fails the condition (fail-closed).
- [x] 5.7 GREEN `internal/gate/gate.go`: `Evaluate` validation-comment branch.
- [x] 5.8 RED `gate_test.go`: signature — `provenance.Verified` passes; no-marker/`Mismatch` fails; `DevMarker`/`DevVerifier` fails (unverifiable, not valid); `Facts.ProvenanceErr != nil` fails the condition (fail-closed).
- [x] 5.9 GREEN `internal/gate/gate.go`: `Evaluate` signature branch — passes ONLY on `provenance.Verified`.
- [x] 5.10 RED `gate_test.go`: a `Require*` explicit `false` skips that condition from `Result.Passed` even when its underlying fact would fail; `Require*` nil (omitted) IS evaluated — default-on toggle wiring; approval is never toggleable.
- [x] 5.11 GREEN `internal/gate/gate.go`: `Evaluate` toggle-skip wiring per condition.
- [x] 5.12 RED `gate_test.go`: `AggregateCoverage(totals, notCovered)` sums `Σ(total-notCovered)/Σtotal`; an all-zero `totals` slice returns `known=false`.
- [x] 5.13 GREEN `internal/gate/gate.go`: `AggregateCoverage`.
- [x] 5.14 RED `gate_test.go`: `ValidationComment(jobID, runID, componentErrs, testErrs, coveragePct, coverageKnown)` returns a `MarkerPrefix`-carrying marker plus a human-readable body with the error counts and coverage figure, and embeds NO provenance secret; `HasValidationComment(bodies)` is true iff any body contains `MarkerPrefix`.
- [x] 5.15 GREEN `internal/gate/gate.go`: `ValidationComment`, `MarkerPrefix` const, `HasValidationComment`.
- [x] 5.16 Verify: `PATH=/usr/local/go/bin:$PATH go test ./internal/gate/... -race` GREEN — package stays exec-free (deps only `config`+`provenance`).

## Phase 6: `internal/app` — wiring, dispatch seam, gate-block screen

- [x] 6.1 RED `deploy_gate_test.go`: `resolvePRURL(rec)` returns `rec.PRUrl` when set; empty `PRUrl` falls back to `PRForBranch(RenderBranchName(BranchFormat,Ticket,Target))`; neither resolves → fail-closed (no PR).
- [x] 6.2 GREEN `internal/app/commands.go`: `resolvePRURL`.
- [x] 6.3 RED `deploy_gate_test.go`: `gateCheckCmd` maps each of `PRReviews`/`UnresolvedThreadCount`/`PRComments`/provenance-fetch gh errors independently into the matching `Facts.*Err`, always returning a `Result` — never panics; when `resolvePRURL` resolves no PR, `gateCheckCmd` sets `Facts.PRResolved=false` and skips the 4 gh reads entirely.
- [x] 6.4 GREEN `internal/app/commands.go`: `gateCheckCmd` — `resolvePRURL`, then `PRReviews`/`UnresolvedThreadCount`/`PRComments`/`verifyProvenance` (composes `PRDetails`→`ParseMarkers`→`bestResult`, D11), maps DTOs → `gate.Facts`, calls `gate.Evaluate`.
- [x] 6.5 RED `app_test.go`: `StateDeployGateBlocked` + `Model.gateCheckingRunID`/`gateConditions` fields exist and are wired.
- [x] 6.6 GREEN `internal/app/app.go`: add `StateDeployGateBlocked` + Model fields.
- [x] 6.7 RED `keys_test.go`: the typed-`DESPLEGAR` confirmation on a gated target dispatches `gateCheckCmd`, NOT `quickDeployCmd` directly; on an ungated target it still dispatches `quickDeployCmd` unchanged (deploys as today).
- [x] 6.8 GREEN `internal/app/keys.go`: branch on `GateFor(rec.Target)` between the word-check (`keys.go:1464-1467`) and dispatch (`keys.go:1477`).
- [x] 6.9 RED `update_test.go`: `onGateCheckDone` with `Result.Passed==true` dispatches `quickDeployCmd` (point-of-no-return); the action is recorded.
- [x] 6.10 RED `update_test.go`: `onGateCheckDone` with `Result.Passed==false` sets `m.gateConditions`, transitions to `StateDeployGateBlocked`, does NOT dispatch `quickDeployCmd` — no action recorded.
- [x] 6.11 GREEN `internal/app/update.go`: `onGateCheckDone`.
- [x] 6.12 RED `keys_test.go`: `keyDeployGateBlocked` — `q`/`esc` → `StateRunHistory`; no key offers an override or bypass.
- [x] 6.13 GREEN `internal/app/keys.go`: `keyDeployGateBlocked`.
- [x] 6.14 RED `view_test.go` (Ascii `TestMain`, plain-text assertions): `viewDeployGateBlocked` lists EVERY unmet condition + its reason, not only the first; uses `style.go`'s semantic palette (err/warn/dim), asserted via plain text.
- [x] 6.15 GREEN `internal/app/view.go`: `viewDeployGateBlocked`.
- [x] 6.16 RED `update_test.go`: `onReportDone` terminal SUCCESS on a gate-enabled target with `requireValidationComment` on and no existing marker comment triggers `postValidationCommentCmd`; on an ungated target, or on a non-successful terminal state (`Failed`/`Canceled`), it does NOT trigger.
- [x] 6.17 RED `deploy_gate_test.go`: `postValidationCommentCmd` skips posting when the PR already carries the job's marker comment — no duplicate.
- [x] 6.18 GREEN `internal/app/update.go` + `commands.go`: `postValidationCommentCmd` (list `PRComments`, skip-if-marker-present, else `gate.ValidationComment`+`PostComment`) + gated `onReportDone` call site.
- [x] 6.19 RED `update_test.go`: `onPostCommentDone` failure is best-effort — does not alter the already-terminal-success run state.
- [x] 6.20 GREEN `internal/app/update.go`: `onPostCommentDone`.
- [x] 6.21 Verify: `PATH=/usr/local/go/bin:$PATH go test ./internal/app/... -race -run "Gate|Deploy|Report"` GREEN; `internal/app` remains the only layer that decides/renders — every gh call flows through `internal/github.Client` via `commands.go`.

## Phase 7: Final Gates

- [x] 7.1 `PATH=/usr/local/go/bin:$PATH go build ./...` — clean, exit 0.
- [x] 7.2 `PATH=/usr/local/go/bin:$PATH go vet ./...` — clean, exit 0.
- [x] 7.3 `gofmt -l internal cmd` — empty output.
- [x] 7.4 `PATH=/usr/local/go/bin:$PATH go test ./... -race -count=1` — full suite GREEN; all 29 delta-spec scenarios covered by RED tests above.
