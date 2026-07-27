# Proposal: Deploy Queue Visibility and Self-Service Cancel (HU-009 + HU-012)

**Delivery phase**: Fase 4 - Cola De Deploys (`EPICA.md:803`). Builds on archived `foundation-mvp-git` + `delta-validation`. Store: openspec.

## Intent

Developers share one Salesforce sandbox. Today they start a validation blind: no view of concurrent deploys/validations, so a job silently queues behind others with no ETA, and a job started against a wrong package can only be abandoned (left running, wasting the shared queue). This slice adds operational visibility (show the active `DeployRequest` queue before validating) and a safe escape hatch (cancel your own in-progress validation) — never touching other users' jobs.

## Scope

### In Scope
- **HU-009**: `ListDeployQueue` (Tooling API `DeployRequest`) + a real `QueueReview` stop between package confirm and validation start; non-blocking degrade on permission/query error.
- **HU-012**: `CancelDeploy` + a typed-confirm `StateCancelConfirm` screen reachable from validation polling; persist `Canceled` via `MarkCanceled`.

### Out of Scope
- HU-013 (history/resume), HU-014 (push/PR), HU-015 (quick deploy), HU-016 (re-promote), HU-017 (cleanup).
- Cancelling foreign jobs; queue auto-refresh/polling; any change to the `q` "exit leaves job active/resumable" invariant.

## Capabilities

### New Capabilities
- `deploy-queue`: query + display the active DeployRequest queue (Pending/InProgress) ordered by CreatedDate; own-job highlight + approx position; `CheckOnly` distinguishes validate vs deploy; clear empty state; non-blocking degrade (permission error → warn + skip straight to validation; generic error → actionable error, stay).
- `validation-cancel`: cancel the current run's in-progress validation only, via exact-literal `CANCELAR` typed confirm; success → `Canceled` terminal; failure → error, run untouched.

### Modified Capabilities
- `validation-progress`: add a distinct cancel entry key (`c`) from the polling screen; `q` and the "exit leaves job active/resumable" invariant stay unchanged.
- `run-persistence`: persist a cancel record (`cancel.json` companion) and the `Canceled` status via `MarkCanceled`, alongside the existing validate/report writer.

## Approach

Extend, do not rewrite:
- **`internal/salesforce`**: add `ListDeployQueue` + `CancelDeploy` reusing `Client` / `New(runner)` (unchanged) / `decodeEnvelope`. Queue target = `{TotalSize, Done, Records[]}` struct (empirically confirmed 2026-07-27 vs `AM-DEV-EDITION`), filter `Status IN ('Pending','InProgress') ORDER BY CreatedDate ASC`. Permission error detected via the `validateErrorMessage` oclif-envelope precedent; unmatched → generic actionable branch (never swallowed).
- **`internal/app`**: `confirmPackageReview()` → `StateQueueReview` + `queueCmd()`; new `onQueueDone` / `keyQueueReview` / `viewQueueReview` (keys `r` refresh / `Enter` continuar / `Esc` volver, mockup-faithful); then the existing `→ StateValidationStart`. Cancel: new `c` key → `StateCancelConfirm` (reuse the `keyTicket` text-input idiom), `CANCELAR` gate → `cancelCmd()` → reuse `cancelPoll()` + persist + existing `StateCanceled`. Apply the stale-message guard symmetrically to the cancel handler.
- **`internal/runs`**: add `Writer.MarkCanceled(runID, cancelRaw)` writing `cancel.json` (mirrors `Create`'s `validate.json`) — not `AppendReport`.

**Build order**: HU-009 queue first (independent), then HU-012 cancel.

**Testing**: unit + FakeRunner primary/required (envelope+records parse, CheckOnly, empty state, permission vs generic branch; cancel success/failure; `Model.Update` transitions PackageReview→QueueReview→ValidationStart, permission auto-skip, ValidationPolling→CancelConfirm, typed-gate, cancel success→Canceled+persist / failure→unmarked, stale guard). Opt-in real-org queue e2e vs `AM-DEV-EDITION` (read-only, feasible). Cancel real-org e2e best-effort only (timing-hard) — fake is primary.

**Order deviation**: docs sequence HU-013/016 before these; skipping ahead is safe (needs nothing from HU-013; existing `runs.Writer` suffices) — called out here.

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `internal/salesforce` | Modified | `ListDeployQueue` + `CancelDeploy`; reuse `Client`/`New`/`decodeEnvelope`, `New` unchanged |
| `internal/app` | Modified | Real `QueueReview` stop; new `StateCancelConfirm`; cmds/keys/views; symmetric stale guard |
| `internal/runs` | Modified | `Writer.MarkCanceled` + `cancel.json` companion |
| `openspec/specs/deploy-queue` | New | HU-009 capability spec |
| `openspec/specs/validation-cancel` | New | HU-012 capability spec |
| `openspec/specs/validation-progress` | Modified | Cancel entry key `c` (delta; invariant preserved) |
| `openspec/specs/run-persistence` | Modified | Cancel record persistence (delta) |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| Permission-error text unverified (no restricted profile) | Med | Heuristic via `validateErrorMessage` precedent; unmatched → generic actionable branch (never swallowed); documented fallback |
| Cancel timing — validation may finish before cancel lands | Med | Fake is primary/required; real-org cancel e2e best-effort only (assert CLI call runs, not terminal status) |
| Two writers into run status (poll + cancel) | Med | Symmetric stale-message guard; dedicated `cancel.json` companion, not `AppendReport` |
| Order deviation from documented HU sequence | Low | Verified independent of HU-013; existing `runs.Writer` suffices; noted in rationale |

## Rollback Plan

Feature is additive and revertible without shared-state cleanup:
- Revert wiring: `QueueReview` returns to its inert pass-through; the `c` key + `StateCancelConfirm` are removed → flow returns to PackageReview → ValidationStart directly. `New(runner)` is unchanged, so existing `internal/salesforce` callers are unaffected.
- **No shared Git state touched.** Salesforce impact is limited to `CancelDeploy`, which only cancels the current run's own job on explicit typed confirm; a failed or aborted cancel leaves the Salesforce job running (server-side no-op) and the run NOT marked canceled — matching HU-012 AC. `cancel.json` is local under `.deploydeck/` (gitignored), so rollback is a code revert only.

## Dependencies

- Salesforce CLI `sf` with Tooling API access (`sf data query --use-tooling-api`, `sf project deploy cancel`).
- Archived `foundation-mvp-git` + `delta-validation` (forward-compat `StateQueueReview`/`StateCanceled` scaffolding, `cancelPoll()`, stale-message guard, `internal/runs.Writer`).

## Success Criteria

- [ ] Package confirm shows the active queue ordered by `CreatedDate` with own-job highlight + approx position; empty state clear; `CheckOnly` distinguishes validate vs deploy.
- [ ] Permission error → warn + continue to validation without the queue view; generic query error → actionable error without aborting the flow.
- [ ] From polling, `c` → typed `CANCELAR` confirm cancels only the current run's job; run persists as `Canceled` with cancel raw; cancel failure → error + run untouched.
- [ ] Foreign jobs never offer cancel; `q` exit-leaves-job-resumable invariant unchanged.
- [ ] `go test ./...` green (unit + fake); opt-in queue e2e passes vs `AM-DEV-EDITION`.
