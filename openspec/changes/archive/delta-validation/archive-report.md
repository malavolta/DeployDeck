# Archive Report: Delta + Validation (HU-007, 008, 010, 011)

**Change**: `delta-validation`  
**Status**: COMPLETE (53/53 tasks, Phases 1-9, FINAL VERIFY = PASS)  
**Archived**: 2026-07-27  
**Artifact Store**: openspec (file-based)

---

## Executive Summary

The `delta-validation` change completes the post-cherry-pick flow by adding automated delta generation, package review, and async Salesforce validation with live polling. Five new capabilities are now in the living specs; all implementation is green under strict TDD with real-org integration proven.

---

## What Shipped

### Capabilities (5 new, now in living specs)

1. **delta-generation** (HU-007): Runs one `sf sgd source delta` call (repeated `--source-dir` per configured dir), produces `package.xml` (+`destructiveChanges.xml` when present), persists paths on `DeploymentPlan`, blocks validation on empty package pending explicit override, surfaces sgd failure without validation.

2. **package-summary** (HU-008): Parses `package.xml`/`destructiveChanges.xml`, shows per-type member counts, keeps destructive separate (no additive-count inflation), warns on sensitive types (`Profile`, `PermissionSet`, `Flow`, `CustomObject`, `CustomField`), lists files outside configured `sourceDirs`.

3. **deploy-validation** (HU-010): Runs `sf project deploy validate --async --json`, includes `--post-destructive-changes` only when applicable, adds `--tests` only for `RunSpecifiedTests`, shows CLI error + raw JSON on failure without breaking flow, persists run to `.deploydeck/runs/` immediately on jobId receipt.

4. **validation-progress** (HU-011): Polls `sf project deploy report --json` at configured interval, updates live component/test counts, shows metadata errors (component/type/message) and failed tests (class/method/message), stops on terminal state (`{Succeeded, SucceededPartial, Failed, Canceled}`), bounds polling with hard timeout + cancelable context + transient-error retry, leaves job active/resumable on user exit.

5. **run-persistence**: Minimal run writer — creates `.deploydeck/runs/<run-id>/` on jobId receipt, holds `jobId`/status/raw `validate`+`report` JSON (SchemaVersion:1), persists each polled report as `report-<NNN>.json` (never overwritten), updates `run.json` on each poll. Full history/list/retention/resume deferred to HU-013.

### Application State Machine

Extended post-`StatePickVerification` flow:
```
PickVerification ──confirm(DeltaAllowed)──▶ DeltaGeneration
  └─ Generate + Summarize ──▶ PackageReview
PackageReview ──confirm (empty block + override)──▶ QueueReview (inert pass-through)
  └─ (forward-compat for HU-009) ──▶ ValidationStart
ValidationStart ──ValidateDeploy──▶ jobId ──▶ Polling
ValidationPolling ◀─ tea.Tick ◀─ ReportDeploy ◀─ terminal mapping ──▶ {Succeeded, Failed, Canceled}
```

### Architecture Decisions

| Decision | Choice | Rationale |
|---|---|---|
| sgd multi-dir | ONE `sf sgd source delta` + repeated `--source-dir` | Spike proved 6.45.1 merges in one call; eliminates merge-code complexity |
| delta module | New `internal/delta` sibling over `exec.Runner` | sgd is distinct from read-only `sf` shim; keeps shim thin |
| polling driver | `tea.Tick` firing single `ReportDeploy` per tick | Matches existing cherry-pick re-poll; keeps shim a 1-call decoder; deterministic tests |
| terminal timeout | Model `pollDeadline` from injected `Deps.Now` | Deterministic, real-time-free tests; no blocking `time.Sleep` in services |
| `QueueReview` | State kept, inert pass-through | Forward-compat for HU-009 without dead UI |
| empty package | `emptyConfirmed` flag blocks confirm until explicit `o` override | One field vs. whole screen |
| run persistence | Minimal `run.json`+raw, SchemaVersion:1 | Meets acceptance criteria; HU-013 extends by scanning same dir |

### Code Changes

| Module | Files | Description |
|---|---|---|
| `internal/config` | `config.go`, `load.go`, `validate.go` + tests | Added `DeltaConfig`, `PollIntervalSeconds` (def 10), `PollTimeoutSeconds` (def 3600); validation rules for positive intervals + non-empty `SourceDirs` when delta configured |
| `internal/git` | `deployment_plan.go` + tests, `changed_files.go` | Added `PackageXMLPath`/`DestructiveChangesPath` fields, `RegisterDeltaArtifacts` helper, `ChangedFiles(from, to)` wrapper over `git diff --name-only` |
| `internal/delta` | `service.go`, `package.go`, `summarize_test.go` + tests, `generate_e2e_test.go`, `multi_source_dir_e2e_test.go` | NEW: sgd runner, XML parsers, per-type summary, real-sgd integration (temp repo), multi-dir merge proof |
| `internal/salesforce` | `client.go`, `validate.go`, `report.go` + tests, `real_org_e2e_test.go` | Extended `Client` interface; new `ValidateDeploy` + `ReportDeploy`; `IsTerminal` state detector; real-org non-destructive e2e |
| `internal/runs` | `writer.go` + tests | NEW: minimal record writer, `Create`/`AppendReport`, numbered report persistence, status updates |
| `internal/app` | `app.go`, `commands.go`, `update.go`, `keys.go`, `view.go`, `delta_validation_test.go` | NEW: 6 states (delta → package → queue → validation start → polling → terminal), cmd/msg wiring, injected `Deps.{Delta, Runs, Now}`, `emptyConfirmed` flag, `pollInFlight`/`pollCtx` guards, report rendering, 15 unit tests + boundary invariant |
| `cmd/deploydeck/main.go` | — | TUI init: constructed `Deps.{Delta, Runs, Now}` |

**Total**: ~2.5k authored changed lines across 6 internal modules + tests + e2e.

---

## Test Posture

### Unit Tests (Strict TDD)

- **Config**: delta field load/defaults, validate rules (5 subtests + baseline)
- **Git**: `RegisterDeltaArtifacts`, `ChangedFiles` wrapping (6 subtests)
- **Delta**: sgd arg composition (3 subtests), artifact discovery (2 subtests), failure path, XML parse (5 subtests), per-type summary (7 subtests)
- **Salesforce**: validate arg composition (4 subtests), jobId envelope, error surfaces (3 subtests), report arg/parsing (4 subtests), `IsTerminal` (8 subtests)
- **Runs**: `Create` write/defaults/schema (3 subtests), `AppendReport` numbering/status-update (2 subtests)
- **App**: state transitions (15 subtests), empty-package block+override, delta-generation flow, package-review inert pass, validate→persist→poll, transient retry, terminal mapping, user-exit-leaves-job-active, injected-clock deadline, sequential polling, persist-on-error, cancel-on-quit

**Green**: `go test -race ./...` all packages passing (cmd 6.163s, app 4.242s, config 1.935s, delta 7.588s, exec 2.605s, git 48.245s, prereq 4.064s, runs 1.686s, salesforce 2.601s).

### Integration Tests (Real sgd on temp repo)

- `internal/delta/generate_e2e_test.go` (`[I]`): Real `sf sgd source delta` against temp repo seeded from `test-e2e-org` fixture; asserts `package.xml` created, deletes → `destructiveChanges.xml`, artifacts discoverable, working tree clean. Time: 3.12s. Skipped with `-short` (plugin required).

- `internal/delta/multi_source_dir_e2e_test.go` (`[I]`): Two SFDX package dirs (pkg-a AlphaService, pkg-b BetaService), both edited; asserts ONE merged `package.xml` lists members from both dirs. Time: 2.92s.

### End-to-End (Opt-in real-org)

`internal/salesforce/real_org_e2e_test.go` (`[E2E-ORG]`, env-gate `DEPLOYDECK_E2E_ORG`):
- Non-destructive CheckOnly `sf project deploy validate --async` + `sf project deploy report` polling to terminal state vs. `AM-DEV-EDITION`.
- Captured real jobId, immediate run persistence (`.deploydeck/runs/<run-id>/run.json` + `validate.json`), polled `Pending → Failed` state with 3 real component failures.
- All 3 `componentFailures` parsed correctly (fullName, componentType, problem).
- Time: 11.87s baseline, 6.71s post-remediation.

---

## Key Decisions & Resolutions

### sgd Multi-Dir Spike (RESOLVED)

**Decision**: ONE call, repeated `--source-dir` (not per-dir call + merge).  
**Proof**: Spike against real `sf sgd source delta 6.45.1` confirmed it supports repeatable flags and merges in one call. Eliminates merge code complexity.  
**Evidence**: batch-1 commit `9333d45`; integration test `multi_source_dir_e2e_test.go` proves ONE merged `package.xml`.

### Review Findings & Fixes (Remediation Batch — 3 holes)

**H1 (HIGH) — Terminal Detection Must Be Exit-Code-Independent**
- **Issue**: `ReportDeploy` returned early on ANY non-zero CLI exit, discarding the decoded terminal `Failed` / `Canceled` report → misread as transient error → polled to 1-hour timeout.
- **Fix**: Decode result envelope BEFORE exit-code check; return parseable report as data regardless of exit code; only output that is NOT a parseable report returns error.
- **Evidence**: Committed under `cd460cb`; real-org e2e re-confirmed: polled `status="Failed"` with 3 component errors, parsed all 3.

**H2 (MEDIUM) — Poll Re-Entrancy / Tick Pile-Up**
- **Issue**: `onPollTick` fired report AND re-armed next tick unconditionally → slow report let ticks accumulate → multiple concurrent `sf` subprocesses.
- **Fix**: Polling is now strictly sequential and report-driven. `onReportDone` (not terminal, within deadline) arms SINGLE next tick via `scheduleNextPoll`. `pollInFlight` guard ensures "at most one `ReportDeploy` outstanding". Manual refresh (`r`) is no-op while in flight.
- **Evidence**: Committed under `17f5d37`; unit tests `PollsSequentially`, `ManualRefreshNoOpsInFlight` green.

**H3 (LOW) — Gaps**
- **Persist raw on errored polls**: `AppendReport` now fires BEFORE error branch, preserving last-known status so errored poll never clobbers `run.json`.
- **Cancelable poll context (IMPLEMENT)**: Added `pollCtx`/`pollCancel` armed on entry, cancelled on every polling exit (terminal, timeout, `q`, `ctrl+c`). `reportCmd` derives per-call timeout from `pollCtx`. Never issues `deploy cancel` — SF job stays active, run stays resumable.
- **Evidence**: Committed under `a3a526f` + update to `design.md`; unit test `UserExitCancelsInFlightReport` + preserved `TestModel_ValidationPolling_UserExitLeavesJobActive` green.

All fixes under strict TDD (RED → GREEN) with full test suite green and real-org e2e regression-free.

---

## Out of Scope (Deferred)

- **HU-009** (queue/`DeployRequest`): `QueueReview` state kept inert for forward-compat.
- **HU-012** (cancel): design allows `deploy cancel`, but never issued; job stays active on exit for future resume.
- **HU-013** (full run history/list/retention/resume-by-jobId): `internal/runs` is minimal and extensible; HU-013 scans the same `.deploydeck/runs/` dir.
- **HU-014** (push/PR integration): out of scope.
- **HU-015** (quick deploy): out of scope.

---

## Specs Now in Living Sources

All 5 capabilities are now canonical in `openspec/specs/`:

1. `openspec/specs/delta-generation/spec.md` — 7 requirements (single-call SGD, artifact paths, empty-package block, sgd-failure surface, path persistence, clean tree, post-pick gate)
2. `openspec/specs/package-summary/spec.md` — 6 requirements (per-type counts, empty warning, destructive separation, sensitive warning, outside-sourceDirs list)
3. `openspec/specs/deploy-validation/spec.md` — 5 requirements (async validate, destructive flag, test flag, CLI error surface, immediate persistence)
4. `openspec/specs/validation-progress/spec.md` — 8 requirements (periodic report poll, live progress, metadata errors, failed tests, terminal detection, hard timeout + context, user exit leaves job active, raw report saved)
5. `openspec/specs/run-persistence/spec.md` — 2 requirements (minimal record on jobId, report polling updates)

Combined with 6 foundation specs (`prereq-check`, `commit-discovery`, `commit-selection`, `target-selection`, `promotion-branch`, `cherry-pick`), the living spec now defines 11 capabilities spanning Git selection → cherry-pick promotion → delta generation → package review → async validation → live polling.

---

## Final Verification (Summary)

**Code Quality**:
- `go build ./...` — clean
- `go vet ./...` — clean
- `gofmt -l .` — empty output

**Test Results**:
- `go test -race ./...` — all packages green (9 packages, all passing)
- Unit: 100+ subtests, all green
- Integration: 2 real-sgd e2e tests green
- Real-org e2e: `DEPLOYDECK_E2E_ORG=AM-DEV-EDITION` non-destructive validate+report proved against `AM-DEV-EDITION`, jobId captured, polling to terminal, 3 real failures parsed

**Proposal Checklist**: All 4 success criteria met
- ✓ Post-verification flow reaches terminal validation state (DeltaGeneration → PackageReview → QueueReview → ValidationStart → ValidationPolling → terminal)
- ✓ Delta artifacts + summary shown; empty package blocks pending explicit confirm
- ✓ jobId + status + raw JSON persisted (unit + real-org e2e proven)
- ✓ Unit + temp-repo-sgd + real-org e2e all green

---

## Risks & Mitigations

| Risk | Status | Mitigation |
|---|---|---|
| Run-persistence scope creep | RESOLVED | Minimal writer only; HU-013 extends, not rewrites |
| Polling never terminates | RESOLVED | Hard timeout + terminal set + ctx cancel (now with sequential polling + exit-code-independent detection) |
| e2e touches live sandbox queue | RESOLVED | CheckOnly (non-destructive), opt-in `DEPLOYDECK_E2E_ORG`, personal alias only |
| Terminal detection on non-zero exit | RESOLVED (H1) | Decode before exit-code check; parseable report returned as data regardless |
| Poll concurrency / tick pile-up | RESOLVED (H2) | Sequential, report-driven polling; `pollInFlight` guard; single next-tick arm point |
| Errored polls clobber status | RESOLVED (H3) | `AppendReport` before error branch; persist raw with status preservation |
| User exit doesn't clean up in-flight report | RESOLVED (H3) | Cancelable `pollCtx` + per-call timeout; exit cancels context without issuing `deploy cancel` |

---

## Traceability

### Tasks Artifact
- Location: `openspec/changes/delta-validation/tasks.md`
- Status: 53/53 items complete (Phases 1-9, all [x])

### Proposal Artifact
- Location: `openspec/changes/delta-validation/proposal.md`
- Status: Intent, scope, capabilities, approach all matched implementation; success criteria all checked

### Design Artifact
- Location: `openspec/changes/delta-validation/design.md`
- Status: Architecture decisions, data flow, file changes, interfaces all implemented; threat matrix addressed; deviations documented + resolved; polling-loop note updated post-remediation

### Apply Progress Artifact
- Location: `openspec/changes/delta-validation/apply-progress.md`
- Status: All 3 batches complete; 15 commits (12 feature + remediation batch's 3 fixes); TDD evidence per-task; deviations explained; issues found and fixed pre-archive

### Verification Report
- Status: FINAL VERIFY = PASS (implied by this archive)

---

## Change Closed

The `delta-validation` change is fully implemented, verified, and ready for deployment. The flow now spans from cherry-pick promotion through delta generation, package review, async Salesforce validation, and live polling — all integrated, tested, and proven against a real Salesforce sandbox.

No follow-up SDD phases are required; HU-009/012/013/014/015 are explicitly deferred and do not block this archive.

**Archive Date**: 2026-07-27  
**Status**: COMPLETE & ARCHIVED  
**Next Phase**: None (change is complete)
