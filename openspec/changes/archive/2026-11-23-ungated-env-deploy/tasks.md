# Tasks: Ungated Environment Deploy

Strict TDD: every code task is RED (failing test first) then GREEN (minimal code). Test command: `/usr/local/go/bin/go test ./...` (run after each RED and GREEN step; full suite green before moving on).

Reconciliation note (spec vs design): when the offer is "unavailable" (ineligible run or `allowExecution=false`), the design renders a disabled `x` footer hint plus a reason line, while the spec says "no executable hint". The disabled hint is NON-executable (`hint(keys.Quick,"x","desplegar",available=false)` style), so it is consistent with the spec. Apply follows the design. In unavailable state `x` is a strict no-op. In hidden state there is no hint, line or binding.

## Phase 1: Pure predicate (spec: quick-deploy "Offered From Result Screen", deploy-gate "Never Applies To Gated Targets")

- [x] 1.1 RED: `internal/app/result_offer_test.go`: table test for `resultDeployOffer(cfg, rec, now)`. Cases: gated enabled -> hidden; gate `enabled:false` -> not hidden; production target -> hidden; unmapped branch -> hidden; empty Target (standalone validate) -> hidden; each ineligible reason (no TestLevel, expired, already deployed) -> unavailable with `quickDeployReasonES` text; `allowExecution=false` -> unavailable with "la ejecución está deshabilitada (quickDeploy.allowExecution)"; all good -> available. Run: fails to compile.
- [x] 1.2 GREEN: create `internal/app/result_offer.go` with `deployOffer` (`offerHidden`, `offerUnavailable`, `offerAvailable`) and `resultDeployOffer`. Reuse `IsProductionTarget`, `QuickDeployEligible`, `quickDeployExecAllowed(cfg,false)`, the existing gate lookup. No new imports.
- [x] 1.3 REFACTOR: confirm the predicate is pure (no IO, no model access); suite green.

## Phase 2: Load the finished run into m.runs (spec: quick-deploy "Reuses The Typed Confirmation", "Incomplete run record hides the offer")

Depends on 1.

- [x] 2.1 RED: tests for `m.upsertResultRun` and `m.resultOffer()` using a real `runs.NewWriter(t.TempDir())`: (a) replaces the row with the same RunID in place; (b) otherwise prepends (newest-first), `m.runsCursor` untouched; (c) live `msg.report.Status` overlays a disk `Queued` status; (d) nil `Deps.Runs` or `Load` error leaves the offer hidden; (e) `resultOffer()` is hidden unless `m.state == StateSucceeded` and a row with `RunID == m.runID` exists; (f) record missing JobID/alias/TestLevel -> hidden or unavailable, never available.
- [x] 2.2 RED: `onReportDone` test: terminal success triggers the upsert; failure or other terminal states do not touch `m.runs`.
- [x] 2.3 GREEN: add `upsertResultRun` and `resultOffer()` in `result_offer.go`; call `upsertResultRun` from `onReportDone` in `internal/app/update.go` when `terminalState == StateSucceeded` (~line 1238, after AppendReport).
- [x] 2.4 RED+GREEN: test that a history reload (`update.go:407-408`) replaces `m.runs` and resets the cursor, so the upsert never leaks into history. Likely passes with no code change; keep it as a regression guard.

## Phase 3: `x` key, origin flag, Esc/q return (spec: quick-deploy "Returns To Its Origin Screen", tui-presentation esc/q rule)

Depends on 2. Sequential with 4 only for the shared `keys.go` edits; 4 can start once 3.3 lands.

- [x] 3.1 RED: `internal/app/keys_test.go` (or `result_offer_keys_test.go`): `keySucceeded` `x` with available offer -> `StateQuickDeploy`, `m.runsCursor` points at the run's row, `quickFromResult == true`, nothing executes. `x` when unavailable, hidden, gated or production -> strict no-op (same state, no cmd, no notice). `allowExecution=false` never dispatches regardless of input.
- [x] 3.2 RED: Esc and `q` (empty buffer) from `StateQuickDeploy` opened via the result screen -> `StateSucceeded`, no quit cmd, flag reset to false. Run-history `x` still opens `StateQuickDeploy` and Esc returns to `StateRunHistory`; the history `x` path resets the flag to false.
- [x] 3.3 GREEN: add `quickFromResult bool` to the model in `internal/app/app.go`. Add the `x` case in `keySucceeded` (`keys.go`). Reset the flag in the run-history `x` (`keys.go:~1433`). Esc/q in `keyQuickDeploy` returns to the origin and clears the flag (`keys.go:~1621-1631`). Enter path, in-flight guards (`quickDeployingRunID`/`gateCheckingRunID`) and the gated check at `keys.go:~1597` stay untouched.
- [x] 3.4 RED+GREEN: extend `internal/app/esc_q_matrix_test.go:103` with a "DESPLEGAR from result" row, `back: StateSucceeded`; keep the existing history row.
- [x] 3.5 RED+GREEN: e2e-style test following `quick_deploy_e2e_test.go` (real writer, fake SF): poll to Succeeded, `x`, type `DESPLEGAR`, Enter -> `QuickDeploy` called with the run's JobID/Alias, run marked `QuickDeployedAt` (H-2 mirror by RunID), offer becomes "ya se desplegó…"; Esc returns to the result screen.

## Phase 4: Footer, keymap and view line (spec: tui-presentation "Succeeded Screen Shows The Deploy Offer And Hint Consistently")

Depends on 2. Parallel with 3 after 3.3 (different files: `keymap.go`, `view.go`).

- [x] 4.1 RED: `internal/app/footer_parity_test.go`: fixtures for succeeded + offer available (footer lists `x desplegar`, handler acts on `x`) and succeeded + offer unavailable (disabled/non-executable hint, handler no-op). Gated or production fixture: no `x desplegar` hint, `x` not bound. Pin the `x desplegar` fragment. Coverage lives in the `footer_parity_test.go` fixtures (`keymap_states_test.go` was not edited). Assert the footer grammar of other keys and the other terminal states is byte-identical.
- [x] 4.2 RED: view test for `viewValidationResult`: available shows the "Entorno sin gate: x despliega esta validación (quick deploy, requiere DESPLEGAR)" line; unavailable shows "Despliegue no disponible: <reason>" (existing Spanish reason or the execution-disabled reason); hidden shows no line. The line sits before `noticeLine`.
- [x] 4.3 GREEN: `internal/app/keymap.go`: for `StateSucceeded` with offer not hidden, append `hint(keys.Quick,"x","desplegar",available)` before `exit` (`keymap.go:~191-192`); available=false when unavailable. `internal/app/view.go` (`viewValidationResult`, ~932): add the offer line via `m.resultOffer()`, no IO in View.
- [x] 4.4 Full-suite run; check `internal/app` still has no new imports (no `os/exec`, `net/http`, `internal/version`, `internal/update`).

## Phase 5: Documentation (no tests; verify by reading)

Parallel with 3 and 4; do last for wording accuracy. Depends on spec only.

- [x] 5.1 `README.md`, "Deploying to a test environment": ungated default (governance is opt-in per environment), the `branches` mapping footgun (unlisted branch = production, fail-closed), the `allowExecution` opt-in, `x` from the result screen.
- [x] 5.2 `docs/HISTORIAS.md`: add HU-020 after HU-019 (~1151), add the e2e row (~1287), and refresh the HU-015 wording (~987) to mention the result-screen entry point.
- [x] 5.3 `docs/MOCKUPS_TUI.md`: result screen mockup with the offer line and footer near the quick-deploy mockup (~420), including the unavailable variant.

## Review Workload Forecast

| Area | Forecast lines | Notes |
|---|---|---|
| Production code (`result_offer.go`, `update.go`, `keys.go`, `app.go`, `keymap.go`, `view.go`) | ~110 | |
| Tests (phases 1-4) | ~280 | |
| Docs (README, HISTORIAS, MOCKUPS) | ~150 | |
| **Design total** | **~540** | |
| **Realistic total** | **~650-700** | Agent forecasts undercount about 1.6x on code and tests per prior SDD runs. Docs are reliable. |

- Delivery: single-pr with a size exception already approved (budget ~650 changed lines). The realistic forecast sits at or slightly above budget, so trim headroom first: keep tests table-driven, avoid duplicate fixtures between phases 3 and 4, and keep docs terse.
- Review risk: low-medium. Behavior and state change in `internal/app` only, reusing the existing quick-deploy dispatch behind unchanged guards. No new subprocess, schema, or imports.
- Parallelism: 1 -> 2 sequential; then 3 and 4 may proceed in parallel after 3.3 (disjoint files except shared fixtures); 5 is independent of code.
- Commit suggestion: (a) feat(app) phases 1-4, (b) docs(openspec)+docs phase 5.
