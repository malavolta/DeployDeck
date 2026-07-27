# Exploration: Cola de Deploys — HU-009 (queue) + HU-012 (cancel)

Builds on archived `foundation-mvp-git` (HU-001..006) + `delta-validation` (HU-007/008/010/011). 11 living specs in `openspec/specs/`. This slice is an EXTENSION — `QueueReview` and `StateCanceled` already exist as forward-compat scaffolding from `delta-validation`.

## 1. Scope IN/OUT
IN: HU-009 Consultar cola de despliegues (`docs/HISTORIAS.md:571-627`), HU-012 Cancelar validación propia (`:763-812`).
OUT: HU-013 (historial/resume), HU-014 (push/PR), HU-015 (quick deploy), HU-016 (re-promo), HU-017 (cleanup).
Order note (non-blocking): `docs/HISTORIAS.md:1176-1197` sequences HU-013/HU-016 before these; we skip ahead. Safe — HU-009/012 need nothing from HU-013 (existing `internal/runs.Writer` suffices) — call it out in the proposal.

## 2. QueueReview becomes real (verified in code)
Today inert: `internal/app/app.go:70-73` (`StateQueueReview` "inert pass-through"), `keys.go:294-306` `confirmPackageReview()` jumps `PackageReview → StateValidationStart` directly (no case for `StateQueueReview` in the dispatcher), `view.go:40-41` renders it as a dead fallback.
HU-009 inserts a REAL stop: `confirmPackageReview()` → `StateQueueReview` + new `queueCmd()`; new `onQueueDone`/`keyQueueReview`/`viewQueueReview`; then the existing `→ StateValidationStart` transition. Mockup `docs/MOCKUPS_TUI.md:274-292` ("Cola De Deploys") defines the screen + keys `r`/`Enter continuar validacion`/`Esc` — same idiom as `keyTicket`/`keyTarget`.

## 3. HU-012 cancel — composes with the existing poll loop
`q` in ValidationPolling is DELIBERATELY not a cancel (`keys.go:325-347` + spec `openspec/specs/validation-progress/spec.md:66-73` "User Exit Leaves The Job Active And Resumable" — a protected invariant). So HU-012 needs a DISTINCT new key/action, NOT repurposing `q`.
Mockup `docs/MOCKUPS_TUI.md:346-367` ("Confirmacion De Cancelacion") = a TYPED-TEXT confirm: user types `CANCELAR`. → new `StateCancelConfirm` (entered only from ValidationPolling via a new key, e.g. `c`), reusing the `keyTicket` text-input idiom (`keys.go:62-86`) for `m.cancelInput`, gated on `== "CANCELAR"` before firing cancel.
`StateCanceled` ALREADY exists as a terminal screen (`app.go:288-301` `terminalState("Canceled")→StateCanceled`, `report.go` terminalStatuses includes Canceled) — HU-012 needs a new PATH into it (explicit `CancelDeploy`), not a new state. Reuse `cancelPoll()` (`app.go:255-259`) to tear down the report subprocess, and the stale-message guard (`update.go:249-253` `if m.state != StateValidationPolling`) symmetrically on the new cancel handler so a late report doesn't clobber the Canceled status.

## 4. Per-HU acceptance criteria + E2E

**HU-009** (`:571-627`): `ListDeployQueue` in `internal/salesforce`; Tooling API `DeployRequest` query; include `CheckOnly`; **no-Tooling-API-permission → warn + continue WITHOUT queue (non-blocking)**; show `Pending`+`InProgress`; user/status/date/progress/elapsed; own job highlighted + approx position; generic query failure → actionable error (does NOT necessarily abort). Command (`:606-614` = `EPICA.md:371-375`):
`sf data query --target-org <alias> --use-tooling-api --json --query "SELECT Id, Status, CheckOnly, CreatedDate, StartDate, CompletedDate, CreatedBy.Name, NumberComponentsTotal, NumberComponentsDeployed, NumberComponentErrors, NumberTestsTotal, NumberTestsCompleted, NumberTestErrors FROM DeployRequest WHERE Status IN ('Pending','InProgress') ORDER BY CreatedDate ASC"`.
E2E `sf-fake`/`alias` (`:620-626`): canned JSON multi-user incl. own + permission-error JSON; assert ordering, fields, own-job highlight+position, CheckOnly distinguishes validate vs deploy, permission → warn+continue; variants empty/generic-failure/own-absent. CI: parcial.

**HU-012** (`:763-812`): `CancelDeploy`; cancel enabled ONLY for the current-run job; require confirmation; execute cancel; update run status; show result. ACs (`:786-790`): own in-progress + confirm → cancels; another user's job never offers cancel; cancel failure → error + run NOT marked canceled. Command (`:794-800` = `EPICA.md:395-398`): `sf project deploy cancel --job-id <id> --target-org <alias>` (verified real: `-i/--job-id`, `-r/--use-most-recent`). E2E `sf-fake`/`alias` (`:806-812`): own jobId in progress + a foreign job in queue; assert only current-run offers cancel, confirm runs CLI cancel → run Canceled, foreign shows no cancel, failed cancel → error + unmarked; variants failed-cancel / declined. CI: parcial.

Architecture charter already lists these (`docs/ARQUITECTURA.md:84-91`), and the non-blocking degrade is an explicit decision (`ARQUITECTURA.md:429`, `EPICA.md:840`).

## 5. `internal/salesforce` extension (extend, don't rewrite)
`client.go:1-6` doc already anticipates queue/cancel. Reuse `Client` interface + `New(runner)` (unchanged) + `decodeEnvelope`. Add `ListDeployQueue` (historia's exact name, `:587`) and `CancelDeploy` (`:779`). `validate.go:132-149 validateErrorMessage` is the precedent for detecting a permission error distinctly from generic failure.
**Query-result shape CONFIRMED empirically (2026-07-27) against `AM-DEV-EDITION`:** `sf data query --use-tooling-api --json` → envelope `{status, result:{records:[], totalSize, done}, warnings}`; each record `{attributes, Id, Status, StartDate, CompletedDate, CreatedBy:{Name}, ...}`. So `ListDeployQueue`'s target type is a `{TotalSize int; Done bool; Records []deployRequestRecord}` struct (NOT a bare slice). `DeployRequest` covers deploys AND validations (CheckOnly distinguishes).

## 6. `internal/app` wiring
New tea.Cmds mirror `validateCmd`/`reportCmd` (`commands.go:408-462`) with a `queueCallTimeout` constant. `confirmPackageReview()` → `StateQueueReview` + `queueCmd()`. `onQueueDone`: permission-error → skip to `StateValidationStart` + `validateCmd()` with a `m.notice`; generic error → land on QueueReview showing error (non-blocking); success → QueueReview with parsed list. `keyQueueReview`: `enter`→ValidationStart+validateCmd, `r`→re-queue, `esc`→PackageReview. `viewQueueReview` replaces the dead fallback. Cancel: new `keyValidationPolling` case `c`→`StateCancelConfirm`; typed `CANCELAR` gate → `cancelCmd()`; success → `cancelPoll()` + persist Canceled + `StateCanceled`; failure → stay + error, run untouched.

## 7. Testability (strict TDD)
Pure unit (FakeRunner canned): `ListDeployQueue` envelope/records parse, CheckOnly, empty-state, permission-error branch vs generic; `CancelDeploy` success/failure + Raw; `Model.Update` transitions (PackageReview→QueueReview→ValidationStart happy + permission auto-skip; ValidationPolling→CancelConfirm; typed-CANCELAR gate wrong/backspace/esc; cancelDoneMsg success→Canceled+persist, failure→unmarked; stale-message guard).
Real-org e2e (`DEPLOYDECK_E2E_ORG`, `real_org_e2e_test.go` convention): **queue query FEASIBLE** (read-only Tooling API vs `AM-DEV-EDITION`, confirms record shape). **Cancel real-org e2e is TIMING-HARD** (validation may finish before cancel lands) → fake is primary/required; a real cancel test, if any, is best-effort (assert the CLI call runs without error against a real jobId, don't assert terminal status) — documented as such.

## 8. Open decisions for sdd-propose (recommendations)
1. Permission-error detection: parse the oclif error envelope (`validateErrorMessage` precedent); unmatched errors → generic actionable branch (never swallowed). Best-guess heuristic + documented fallback (no restricted profile to verify against).
2. DeployRequest fields/shape: CONFIRMED (§5) — lock the `result.records[]` struct.
3. Cancel/pollCtx: reuse existing `cancelPoll()` from the success handler (idempotent). No new primitive.
4. Cancel persistence: new `internal/runs.Writer.MarkCanceled(runID, cancelRaw)` writing a `cancel.json` companion (mirrors `Create`'s `validate.json`) — do NOT misuse `AppendReport` (would write a `report-<NNN>.json`, misrepresenting the cancel as a poll).
5. Typed confirm = exact literal `"CANCELAR"` (case-sensitive, no normalization).
6. HU-012 needs nothing from HU-013 (verified).

## Recommendation
One change `deploy-queue`, extend `internal/salesforce` (`ListDeployQueue`, `CancelDeploy`), make `QueueReview` a real stop (cmd+view+key, no new state constant), add one new state `StateCancelConfirm`, reuse `StateCanceled`/`cancelPoll`/stale-guard/`keyTicket` idiom, add `runs.Writer.MarkCanceled`. Approach A (manual-confirm queue) + C (immediate cancel transition) + F (dedicated cancel persistence).

## Risks
- Permission-error text unverified (no restricted profile) — heuristic + fallback.
- Cancel real-org e2e timing-hard → best-effort, fake primary.
- Two writers into run.json Status (poll + cancel) → apply the stale-message guard symmetrically.
- Order deviation from docs (non-blocking) — note in proposal.

## Ready for Proposal: yes.
