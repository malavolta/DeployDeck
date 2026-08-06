# Apply Progress: Deploy Gate

Strict TDD Mode. Test runner: `PATH=/usr/local/go/bin:$PATH go test ./... -race`.

Execution order (hard sequential deps): Phase 1 config → Phase 2 provenance BestResult →
Phase 3 pr.go seam → Phase 4 github methods → Phase 5 gate (pure) → Phase 6 app wiring →
Phase 7 final gates.

TDD granularity note: RED/GREEN cycles are batched per phase's test file(s) (matching the
6 suggested work units in tasks.md) rather than per individual sub-task — each phase's full
RED test set is written first, run to confirm a real compile/assertion failure, then the
phase's GREEN implementation is added and re-run to confirm pass. This is the natural grain
for this codebase (one _test.go file per phase, table-driven subtests covering each spec
scenario) and every phase's evidence below shows the actual RED and GREEN command output.

## Phase 1: `internal/config` — gate schema, resolution, validation — DONE

- Added `GateConfig` (Enabled/Approvers/MinApprovals *int/Require* *bool) + `Config.Gates
  map[string]GateConfig` in `internal/config/config.go`.
- Added `internal/config/gate_for.go`: `GateFor(target)(GateConfig,bool)`, exact→path.Match,
  mirrors `sandbox_for.go`.
- Extended `internal/config/validate.go`: enabled gate rejects explicit `MinApprovals<1` and
  empty `Approvers` (unconditional).
- Tests: `internal/config/config_test.go` (+2 tests), `internal/config/gate_for_test.go`
  (new, 4 subtests), `internal/config/validate_test.go` (+1 test, 7 subtests).

### TDD Cycle Evidence
| Task | Test File | Layer | Safety Net | RED | GREEN | TRIANGULATE | REFACTOR |
|------|-----------|-------|------------|-----|-------|-------------|----------|
| 1.1 | `internal/config/config_test.go` | Unit | ✅ pre-existing suite green | ✅ Written | ✅ Passed | ✅ 2 tests (nil vs explicit false; nil vs explicit int) | ➖ None needed |
| 1.3-1.5 | `internal/config/gate_for_test.go` | Unit | N/A (new file) | ✅ Written | ✅ Passed | ✅ 4 subtests (exact/glob/no-match/disabled) | ➖ None needed |
| 1.7-1.9 | `internal/config/validate_test.go` | Unit | ✅ pre-existing suite green | ✅ Written | ✅ Passed | ✅ 7 subtests | ➖ None needed |

RED command: `PATH=/usr/local/go/bin:$PATH go test ./internal/config/... -race` →
`cfg.Gates undefined`, `undefined: config.GateConfig`, `cfg.GateFor undefined` (build failed).
GREEN command: same command → `ok github.com/malavolta/DeployDeck/internal/config`.
Phase verify (1.11): `PATH=/usr/local/go/bin:$PATH go test ./internal/config/... -race` → PASS.

## Phase 2: `internal/provenance` — extract `BestResult` — DONE

- Added `BestResult(ownerRepo,headBranch,markers)(Result,string)` + private `resultRank` to
  `internal/provenance/provenance.go`, extracted from `cmd/deploydeck/pr.go`'s former
  `bestVerifyResult`/`verifyResultRank`. Composes the real `Verify` internally — package stays
  exec-free (boundary_test.go unchanged, still green).
- New test: `TestBestResult_AnyOfClassification` in `provenance_test.go`, using a REAL
  (non-dev) injected secret via the existing `withSecret` helper — proves genuine-wins-over-
  bogus/dev-prepended, dev-beats-mismatch, and the nil/empty-markers zero-value path.

| Task | Test File | Layer | Safety Net | RED | GREEN | TRIANGULATE | REFACTOR |
|------|-----------|-------|------------|-----|-------|-------------|----------|
| 2.1-2.2 | `internal/provenance/provenance_test.go` | Unit | ✅ pre-existing suite green | ✅ Written | ✅ Passed | ✅ 6 subtests | ➖ None needed |

RED: `go test ./internal/provenance/...` → `undefined: BestResult` (build failed).
GREEN: same command → `ok`. Phase verify (2.3): `-race` → PASS, boundary test still green.

## Phase 3: `cmd/deploydeck/pr.go` — `bestResult` seam — DONE

- `pr.go`: replaced `verifyFn = provenance.Verify` + private `bestVerifyResult`/
  `verifyResultRank` with `var bestResult = provenance.BestResult` seam; `runPRVerify` now
  calls `bestResult(ownerRepo, headBranch, markers)` directly.
- `pr_verify_test.go`: `withVerifyFn` → `withBestResult(t, result, runID)` (coarse seam,
  returns a canned `(Result, runID)` pair instead of faking per-marker `Verify`).
  `TestRunPRVerify_AnyOfClassification` RESTRUCTURED (not renamed): the sig-discriminating
  any-of ranking proof now lives in `provenance_test.go` (2.1, real secret); this test proves
  ONLY that `runPRVerify` passes the FULL parsed-markers slice through to `bestResult` in
  document order and maps its Result to the correct exit code.
  `TestRunPRVerify_Verified/Mismatch/CrossRepoBranchCopiedMarker_Mismatch/
  ExhaustiveResultMapping` all migrated to `withBestResult`; `DevSignedMarker`/
  `DevBuiltVerifier_GenuineMarker`/`Degraded`/cobra-wiring tests needed NO change (they never
  used the seam — real `bestResult` composing real `Verify` still classifies "dev"/empty-secret
  paths naturally).

| Task | Test File | Layer | Safety Net | RED | GREEN | TRIANGULATE | REFACTOR |
|------|-----------|-------|------------|-----|-------|-------------|----------|
| 3.1-3.2 | `cmd/deploydeck/pr_verify_test.go` | Unit | ✅ pre-existing suite green (17 tests) | ✅ Written (restructured) | ✅ Passed | ➖ existing table coverage preserved | ➖ None needed |

RED: `go test ./cmd/deploydeck/... -run PRVerify` → `undefined: bestResult` (build failed).
GREEN: same command → `ok`, all 17 pr-verify tests pass, none weakened.
Phase verify (3.3): `-race -run PRVerify` → PASS. `go build ./...` → clean.

## Phase 4: `internal/github` — new `Client` methods — DONE

- `compare_url.go`: added `ParsePRURLParts(prURL)(host,owner,repo,number,ok)`; `ParsePRURL`
  now delegates to it (host+owner+repo join) — existing callers/tests unchanged.
- `client.go`: added `Review{Login,State}`, `Comment{Login,Body}` structs and 4 new `Client`
  interface methods + impls: `PRReviews` (`gh pr view --json latestReviews`),
  `UnresolvedThreadCount` (`gh api graphql` querying `reviewThreads(first:100){isResolved}`,
  discrete `-f query=`/`-F owner=`/`-F repo=`/`-F number=` args — never shell-joined; a Runner
  error, non-zero exit, GraphQL `errors` envelope, or malformed JSON all fail closed; an
  unparseable PR URL never calls gh at all), `PRComments` (`gh pr view --json comments`),
  `PostComment` (`gh pr comment --body`, discrete args, multi-line-safe).
- New test file `internal/github/client_gate_test.go` covers all 4 methods' success + every
  degrade path (runner error / non-zero exit / malformed JSON / graphql-errors), using a
  "probe-then-reprime" FakeRunner technique for `UnresolvedThreadCount` so the test never
  hardcodes the client's private graphql query text.
- `compare_url_test.go` extended with `TestParsePRURLParts` + a delegation regression guard.

| Task | Test File | Layer | Safety Net | RED | GREEN | TRIANGULATE | REFACTOR |
|------|-----------|-------|------------|-----|-------|-------------|----------|
| 4.1-4.2 | `internal/github/compare_url_test.go` | Unit | ✅ pre-existing suite green | ✅ Written | ✅ Passed | ✅ 6+1 cases | ➖ None needed |
| 4.3-4.10 | `internal/github/client_gate_test.go` | Unit (FakeRunner) | N/A (new file) | ✅ Written | ✅ Passed | ✅ 4 methods × success+degrade subtests | ➖ None needed |

RED: `go test ./internal/github/...` → `c.PRReviews undefined` etc. (build failed).
GREEN: same command → `ok`, all new + pre-existing tests pass.
Phase verify (4.11): `-race` → PASS. `go build ./...` → clean.

## Phase 5: `internal/gate` (pure) — DONE

- New package `internal/gate/gate.go`: `Facts`/`Condition`/`Result`/`Evaluate` (pr-resolution
  short-circuit; approvals — never toggleable, `normalizeLogin`, `effectiveMinApprovals`
  default 1, changes-requested negates; threads/validation-comment/signature — each
  `toggleOn`-gated, default-on when `Require*` nil, skipped entirely from `Result.Conditions`
  on explicit `false`; every `*Err` fails ITS OWN condition fail-closed), `AggregateCoverage`,
  `ValidationComment`+`MarkerPrefix`, `HasValidationComment`.
- `internal/gate/boundary_test.go` mirrors `internal/provenance/boundary_test.go`: asserts the
  package imports ONLY `config`+`provenance` (non-vacuous) and never `exec`/`net/http`/`github`.
- `gate_test.go` is an INTERNAL test file (`package gate`, not `gate_test`) so it can exercise
  the unexported `normalizeLogin` directly per task 5.2's explicit RED item.

| Task | Test File | Layer | Safety Net | RED | GREEN | TRIANGULATE | REFACTOR |
|------|-----------|-------|------------|-----|-------|-------------|----------|
| 5.1-5.11 | `internal/gate/gate_test.go` | Unit | N/A (new package) | ✅ Written | ✅ Passed | ✅ multi-subtest per condition | ➖ None needed |
| 5.12-5.15 | `internal/gate/gate_test.go` | Unit | N/A (new package) | ✅ Written | ✅ Passed | ✅ multi-case | ➖ None needed |
| boundary | `internal/gate/boundary_test.go` | Unit | N/A (new file) | ✅ Written | ✅ Passed | ➖ single invariant | ➖ None needed |

RED: `go test ./internal/gate/...` → `undefined: Evaluate` etc. (build failed).
GREEN: same command → `ok`, 12 top-level tests / ~30 subtests all pass, boundary test green.
Phase verify (5.16): `-race` → PASS.

## Phase 6: `internal/app` — wiring, dispatch seam, gate-block screen — DONE

- `app.go`: `StateDeployGateBlocked` (appended last, never renumbers prior States); Model
  fields `gateCheckingRunID string`, `gateConditions []gate.Condition`; imports `internal/gate`.
- `commands.go`: `resolvePRURL(ctx,gh,rec,branchFormat)(string,bool)` (rec.PRUrl → PRForBranch
  fallback → fail-closed, nil-gh-safe); `gateCheckDoneMsg{result gate.Result}`; `gateCheckCmd()`
  (resolvePRURL → PRReviews/UnresolvedThreadCount/PRComments/verifyProvenance, each gh error
  independently mapped into its own `Facts.*Err`, never a blanket failure, never a panic;
  `!ok` from resolvePRURL skips all 4 reads); `verifyProvenance` (PRDetails→ParseMarkers→
  `provenance.BestResult`, D11 — no-marker classifies as `Mismatch`, never the zero-valued
  `Verified`); `mapReviews`/`commentBodies` DTO mappers; `postCommentDoneMsg{err}`;
  `requireValidationCommentOn(cfg)`; `postValidationCommentCmd()` (resolvePRURL → PRComments →
  skip-if-marker-present → `gate.ValidationComment`+`PostComment`); `coverageTotals`/
  `coverageNotCovered` report→gate projections.
- `keys.go`: gate branch inserted between the word-check and dispatch in `keyQuickDeploy`
  (`GateFor(rec.Target)` → gated: capture `gateCheckingRunID` + fire `gateCheckCmd`; ungated:
  unchanged `quickDeployCmd` dispatch); `keyDeployGateBlocked` (q/esc → StateRunHistory, every
  other key inert — no override); wired into `handleKey`.
- `update.go`: `onGateCheckDone` (Passed → capture `quickDeployingRunID` + dispatch
  `quickDeployCmd`; blocked → `gateConditions` + `StateDeployGateBlocked`, no dispatch);
  `maybeTriggerValidationCommentCmd` + gated call site inside `onReportDone`'s terminal branch
  (SUCCESS + gate enabled + requireValidationComment on → `postValidationCommentCmd`);
  `onPostCommentDone` (best-effort no-op); both new msg types wired into `Update`'s switch.
- `view.go`: `viewDeployGateBlocked` (lists every UNMET condition + reason via `mark("XX")`/
  styleErr, closing `styleDim` "no override" note); wired into `viewBody`.
- New test files: `internal/app/app_test.go` (State/Model field wiring), `deploy_gate_test.go`
  (resolvePRURL, gateCheckCmd error-independence + no-PR-skip, postValidationCommentCmd
  skip/post). Extended `keys_test.go` (gated-vs-ungated dispatch, keyDeployGateBlocked),
  `update_test.go` (onGateCheckDone pass/block, onReportDone trigger/non-trigger x4,
  onPostCommentDone best-effort), `view_test.go` (every-unmet-condition, no-override-key).
- `internal/app/boundary_test.go` (`TestApp_NeverImportsExecSeam`) reverified green: `gate`/
  `provenance`/`config` are not in its forbidden list — every gh call still flows through
  `internal/github.Client` via `commands.go`.

| Task | Test File | Layer | Safety Net | RED | GREEN | TRIANGULATE | REFACTOR |
|------|-----------|-------|------------|-----|-------|-------------|----------|
| 6.1-6.4 | `internal/app/deploy_gate_test.go` | Unit (FakeRunner) | N/A (new file) | ✅ Written | ✅ Passed | ✅ 3+5+1 subtests | ➖ None needed |
| 6.5-6.6 | `internal/app/app_test.go` | Unit | N/A (new file) | ✅ Written | ✅ Passed | ➖ structural | ➖ None needed |
| 6.7-6.8 | `internal/app/keys_test.go` | Unit | ✅ pre-existing suite green | ✅ Written | ✅ Passed | ✅ gated+ungated | ➖ None needed |
| 6.9-6.11 | `internal/app/update_test.go` | Unit | ✅ pre-existing suite green | ✅ Written | ✅ Passed | ✅ pass+block | ➖ None needed |
| 6.12-6.13 | `internal/app/keys_test.go` | Unit | ✅ pre-existing suite green | ✅ Written | ✅ Passed | ✅ q/esc + 5 non-override keys | ➖ None needed |
| 6.14-6.15 | `internal/app/view_test.go` (Ascii TestMain) | Unit | ✅ pre-existing suite green | ✅ Written | ✅ Passed | ✅ 2 tests | ➖ None needed |
| 6.16, 6.18 | `internal/app/update_test.go` | Unit | ✅ pre-existing suite green | ✅ Written | ✅ Passed | ✅ 4 subtests (trigger/ungated/toggle-off/failed+canceled) | ➖ None needed |
| 6.17 | `internal/app/deploy_gate_test.go` | Unit (FakeRunner) | N/A (new file) | ✅ Written | ✅ Passed | ✅ skip+post companion | ➖ None needed |
| 6.19-6.20 | `internal/app/update_test.go` | Unit | ✅ pre-existing suite green | ✅ Written | ✅ Passed | ➖ single invariant | ➖ None needed |

RED: `go test ./internal/app/... -race` → `undefined: resolvePRURL`, `m.gateCheckCmd undefined`,
`undefined: gateCheckDoneMsg`, etc. (build failed).
GREEN: same command → `ok` (all ~450 pre-existing + ~30 new tests pass, ~28s).
One self-caught bug during GREEN: `TestViewDeployGateBlocked_NoOverrideMentioned`'s first draft
banned the word "omitir" anywhere in the screen, which false-failed against the view's own
correct "no hay ninguna opción para omitir este bloqueo" explanatory text — narrowed to
`TestViewDeployGateBlocked_NoOverrideKeyOffered`, asserting the FOOTER (the actionable key
list) never advertises a bypass key, while leaving the body free to explain the block honestly.
Phase verify (6.21): `-race -run "Gate|Deploy|Report"` → PASS; `TestApp_NeverImportsExecSeam`
reverified green.

## Phase 7: Final Gates — DONE

- `go build ./...` — clean, exit 0.
- `go vet ./...` — clean, exit 0.
- `gofmt -l internal cmd` — empty (4 files needed `gofmt -w`: `deploy_gate_test.go`,
  `gate/gate_test.go`, `provenance_test.go`, `pr.go` — formatting only, no behavior change).
- `go test ./... -race -count=1` — full suite GREEN across all 15 packages (~92s), including
  the new `internal/gate` package and every pre-existing package unchanged.

### Status
69/69 tasks complete. All 7 phases done. Ready for verify.
