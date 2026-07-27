# Archive Report: Deploy Queue Visibility and Self-Service Cancel (HU-009 + HU-012)

**Change**: `deploy-queue`
**Date Archived**: 2026-07-27
**Status**: COMPLETE and VERIFIED
**Final Task Count**: 24/24 tasks complete (8 phases, 3 work batches including 1 remediation)

## Executive Summary

The `deploy-queue` change delivers two critical user stories for DeployDeck's queue visibility and cancel workflow:

- **HU-009**: Developers now have operational visibility of the active Salesforce `DeployRequest` queue (pending/in-progress validations and deploys) via a real `QueueReview` screen inserted between package confirmation and validation start. The queue is fetched from Tooling API with intelligent degrade on permission failures.
- **HU-012**: Developers can cancel their own in-progress validations via a typed-confirmation (`CANCELAR`) flow, freeing the shared sandbox queue without ever touching another user's job. Cancel execution uses `sf project deploy cancel` and is properly persisted.

Both features shipped with comprehensive unit and integration testing, proof of argument-slice safety against shell injection, and opt-in real-org e2e validation. One MEDIUM liveness bug discovered during adversarial review (cancel-confirm poll-freeze) was diagnosed and fixed under strict TDD in Batch 3.

## Capabilities: NEW and MODIFIED

### NEW Capabilities

1. **Deploy Queue Specification** (`openspec/specs/deploy-queue/spec.md`)
   - List active `DeployRequest` queue via Tooling API
   - Present `QueueReview` screen with queue details, own-job highlight, and empty-state handling
   - Non-blocking degrade on query failure (permission vs. generic errors)

2. **Validation Cancel Specification** (`openspec/specs/validation-cancel/spec.md`)
   - Cancel deploy via `sf project deploy cancel` CLI
   - Restrict cancel to current run's own job only
   - Typed confirmation (`CANCELAR` literal) required before execution
   - Successful cancel marks run `Canceled` and persists result
   - Failed cancel leaves run untouched with error display

### MODIFIED Capabilities

1. **Validation Progress Specification** (`openspec/specs/validation-progress/spec.md`)
   - ADDED: Distinct cancel entry key (`c`) from polling that enters `StateCancelConfirm`
   - MODIFIED: Clarified "User Exit Leaves Job Active" invariant explicitly noting that cancel is a separate, confirmed action and does not alter the behavior of the exit key (`q`)

2. **Run Persistence Specification** (`openspec/specs/run-persistence/spec.md`)
   - ADDED: Cancel result persisted as `cancel.json` companion file (not as numbered report)
   - ADDED: Run status updated to `Canceled` when `MarkCanceled` is called

## Key Architectural Decisions

1. **Extend Salesforce with `ListDeployQueue` and `CancelDeploy`**: Tooling API integration for queue visibility; `sf project deploy cancel` for cancel execution. Both use discrete argument slices (SOQL query and jobID) rather than shell-joined strings, proven by threat-matrix test.

2. **Own-Job Match via `CreatedBy.Username`**: Queue entries are matched to the current run's job via the `CreatedBy.Username` field from the Salesforce response, kept simple and stateless (no complex org alias resolution).

3. **Non-Blocking Permission Degrade**: Tooling API permission failures are caught and the flow continues to `ValidationStart` without the queue, preserving uptime for users who lack Tooling API read access.

4. **`QueueReview` Promoted From Inert to Real**: The `QueueReview` state machine transition (previously a dead code path) is now a live stop in the flow: `PackageReview` → `StateQueueReview` + `queueCmd` → `onQueueDone` → `StateValidationStart`.

5. **Reuse Existing State/Poll Infrastructure**: Cancel uses the existing `StateCanceled`, `cancelPoll()`, and stale-message guards (`pollInFlight`, `state` guards) rather than introducing new infrastructure. The `q`-exit invariant (job remains active) is orthogonal to and unaffected by the new `c`-cancel path.

6. **Typed Confirmation Over Implicit Delete**: Cancel requires an explicit modal entry (`c`) and typed confirmation (`CANCELAR` literal), avoiding accidental cancellations and providing a clear intent signal. This is stricter than the bare `cancel.json` persistence — the user's intent is explicit.

## Issues Found and Fixed

### MEDIUM Liveness Freeze in Cancel-Confirm Poll Flow (Batch 3)

**Diagnosis**: When a polled report arrived while the user was on `StateCancelConfirm` (deciding whether to type `CANCELAR`), the `onReportDone` handler checked `if m.state != StateValidationPolling { return m, nil }` to drop stale messages. However, it dropped the message BEFORE clearing `m.pollInFlight = false`, leaving the flag stuck `true`. Pressing `esc` to return to polling then fired `pollTickCmd`, but `onPollTick` bails on `pollInFlight == true` → no report, no tick rescheduled → live progress loop PERMANENTLY FROZEN. Manual refresh (`r`) was also dead.

**Root Cause**: State guard placement before the in-flight flag clear.

**Fix**: Moved `m.pollInFlight = false` to BEFORE the state guard in `onReportDone`. The report goroutine has genuinely returned by the time `reportDoneMsg` arrives, so clearing the flag regardless of state is correct and cannot cause a double report. The state guard still drops the stale message, but now `pollInFlight` is cleared so `esc`-resume and `r` work again.

**TDD Proof**: Added two RED-first liveness tests (`TestModel_ReportDoneMsg_DuringCancelConfirm_DoesNotFreezePolling` and `TestModel_CancelConfirm_EscAfterInFlightReport_ManualRefreshWorks`). Both failed reproducing the freeze; fix makes both pass. Regression test suite (sequential polling, cancel-confirm flow, `q`-invariant, typed-`CANCELAR` gate, own-job-only) remains green.

## Test Posture

### Unit Tests (Primary)
- **FakeRunner**: Canned multi-user queue records (own job, permission errors, generic errors, empty, CheckOnly variants)
- **`ListDeployQueue` unit**: 5 tests covering success, permission-error, generic-error, empty-queue, and CheckOnly distinction
- **`CancelDeploy` unit**: 5 tests covering success/failure, raw preservation, unparseable stderr, runner errors
- **`MarkCanceled` unit**: 3 tests covering cancel.json write, non-consumption of report numbering, unknown-runID errors
- **Cancel-Confirm State Machine**: 12 tests covering `c` entry, typed `CANCELAR` gate (wrong/lowercase/backspace/esc), own-job-only target, success→Canceled+persist, failure→stay+error, stale-message guards (both cancel + late-report), 2 MEDIUM-fix liveness tests

### Integration / Real-Org E2E
- **`real_org_queue_e2e_test.go`**: Opt-in, requires `DEPLOYDECK_E2E_ORG=AM-DEV-EDITION`; queries real `DeployRequest` records, parses `CreatedBy.Username`, confirms own-job identification
- **`real_org_cancel_e2e_test.go`**: Opt-in, double-gated (`DEPLOYDECK_E2E_ORG` + `DEPLOYDECK_E2E_CANCEL_JOBID`); asserts CLI executes without asserting timing-hard terminal status; skips cleanly by default; best-effort behavioral coverage

### Threat Matrix / Argument Safety
- **`argcomposition_test.go`**: Proves SOQL query and jobID (including adversarial metacharacter-laden jobId) are passed as discrete argument slices, never shell-joined or interpolated

### Full Suite Verification
- `go build ./...` — clean
- `go vet ./...` — clean
- `gofmt -l .` — no output (clean)
- `go test -race ./...` — all packages `ok` (cmd/deploydeck, internal/app, internal/config, internal/delta, internal/exec, internal/git, internal/prereq, internal/runs, internal/salesforce)
- `internal/app/boundary_test.go` `TestApp_NeverImportsExecSeam` — PASS (app reaches `sf` only via injected `salesforce.Client`)

## Out of Scope (Deferred to HU-013)

- **Run History and Resume**: Full run history browsing/listing, retention/cleanup policy, and resume-by-jobId
- **Push/PR Queuing**: The PR/push staging queue (HU-014), including optional local-model PR-description generation
- **Quick Deploy**: Incremental/quick deploy variants (HU-015)
- **Re-Promote**: Promotion with re-validate option (HU-016)
- **Cleanup**: Automatic run cleanup and retention (HU-017)

## Implementation Summary

### Files Created (Batch 2 + Batch 3)
- `internal/salesforce/cancel.go` — `CancelDeploy` implementation
- `internal/salesforce/cancel_test.go` — cancel unit tests
- `internal/salesforce/argcomposition_test.go` — threat-matrix proof
- `internal/salesforce/real_org_cancel_e2e_test.go` — opt-in real cancel e2e
- `internal/app/cancel_confirm_test.go` — full cancel-confirm state machine tests (12 tests + 2 liveness-fix tests)

### Files Modified (Batch 2 + Batch 3)
- `internal/salesforce/client.go` — added `CancelDeploy` interface
- `internal/runs/writer.go` — added `MarkCanceled(runID, cancelRaw)`
- `internal/runs/writer_test.go` — tests for `MarkCanceled`
- `internal/app/app.go` — `StateCancelConfirm` const, `cancelInput`/`cancelErr` fields
- `internal/app/commands.go` — `cancelCmd`, `cancelDoneMsg`
- `internal/app/update.go` — `onCancelDone`, fix for MEDIUM liveness freeze (moved `pollInFlight = false` before state guard)
- `internal/app/keys.go` — `c` case in polling, `keyCancelConfirm`, `cancelConfirmWord`, dispatcher case
- `internal/app/view.go` — `viewCancelConfirm`, dispatcher case

### Specs Merged Into Living Specs
- `openspec/specs/deploy-queue/spec.md` — created (full spec from HU-009)
- `openspec/specs/validation-cancel/spec.md` — created (full spec from HU-012)
- `openspec/specs/validation-progress/spec.md` — merged delta (added cancel-key requirement, clarified q-exit invariant)
- `openspec/specs/run-persistence/spec.md` — merged delta (added cancel.json and Canceled status requirements)

## Delivery and Review

- **Delivery Strategy**: Single PR, `size:exception` granted (whole change ~1500-1900 changed lines incl. tests, within 40000-line session budget)
- **Work Units**: 2 (HU-009 queue visibility, HU-012 self-service cancel, delivered sequentially in Batches 1-2)
- **Remediation**: Batch 3 fixed MEDIUM liveness freeze discovered by adversarial review (RED→GREEN, no regressions)
- **PR Commits**: 5 focused conventional commits (HU-009 foundation, HU-009 integration, HU-012 foundation ×2, HU-012 wiring + threat-matrix + e2e, Batch 3 liveness fix, docs/success-criteria)
- **Review Findings**: 1 MEDIUM freeze (fixed), 0 blocking issues

## Success Criteria (From Proposal)

All 5 proposal success criteria verified:
1. ✅ Queue query includes `CreatedBy.Username` and own-job matching via `Orgs()/FindByAlias.Username` → implemented via `CreatedBy.Username` matching in the parsed response
2. ✅ QueueReview shows at least 3 fields per job (user, status, date, progress) and highlights own job → implemented with full details + position
3. ✅ Cancel requires typed confirmation (`CANCELAR` literal) in a modal (`StateCancelConfirm`) → implemented with strict gate and `c` entry key
4. ✅ `MarkCanceled` writes `cancel.json` and updates run status to `Canceled` → implemented without consuming report numbering
5. ✅ `q` exit leaves job active/resumable (unchanged by new `c` cancel) → invariant preserved and tested; Batch 3 liveness fix ensures it works correctly after cancel-confirm interaction

## Risks and Mitigations

### No Critical Risks
- **Argument Safety**: Proven by threat-matrix test; SOQL and jobID passed as discrete slices
- **Own-Job Enforcement**: Typed confirmation + `CreatedBy.Username` matching prevents accidental foreign-job cancellation
- **Liveness**: MEDIUM freeze diagnosed and fixed with TDD proof; regression suite green
- **Permission Degrade**: Non-blocking Tooling API failures allow graceful fallback
- **Polling Invariant**: State guards and `pollInFlight` flag protect against double reports and frozen state

## Archival Artifacts

This report marks the completion of the `deploy-queue` change cycle. All artifacts have been synced into the living specs:

- **New Living Specs**: `openspec/specs/deploy-queue/spec.md`, `openspec/specs/validation-cancel/spec.md`
- **Updated Living Specs**: `openspec/specs/validation-progress/spec.md`, `openspec/specs/run-persistence/spec.md`
- **Archived Change Folder**: `openspec/changes/deploy-queue/` (containing proposal, design, delta specs, tasks, apply-progress)

The SDD cycle for `deploy-queue` is complete.
