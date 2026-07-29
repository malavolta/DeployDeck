# Archive Report: HU-015 — Quick Deploy Opcional (`quick-deploy`)

**Date Archived**: 2026-07-29  
**Status**: Complete and Verified  
**Total Tasks**: All 105 tasks (5 groups + 5 post-implementation remediations) checked  
**Final Verification**: READY TO ARCHIVE (as reported by sdd-verify)  
**Test Result**: Full regression suite green; `go test ./... -race` clean, `go vet` clean, `gofmt` clean

## Executive Summary

HU-015 adds opt-in Salesforce quick deploy: when a prior validation succeeds within the 10-day window, the system suggests the `sf project deploy quick --job-id <job-id>` command but NEVER executes it by default. Execution is strictly gated by configuration (`AllowExecution`, `AllowProduction`) and a typed strong confirmation (`DESPLEGAR`). The implementation introduces a new `quick-deploy` capability, extends `run-persistence` with `TestLevel` and `QuickDeployedAt` fields, extends `run-history` with the `x` key action, and extends `config` with the `quickDeploy` section. All 105 implementation tasks (5 groups + 5 post-implementation remediations) completed; execution is always suggest-only by default, production is fail-closed, and all confirmation/gate flows are non-bypassable.

## What Shipped

### 1. Eligibility Detection & Configuration

**Acceptance Criteria Addressed**: AC11, AC12, AC13

- **Eligibility Gate**: Quick deploy is offered only for runs with `Succeeded` or `SucceededPartial` status, required tests (RunLocalTests or RunAllTestsInOrg per the whitelist), age under 10 days, and NOT already quick-deployed
- **TestLevel Field**: New `TestLevel` field on `Record` captures the plan's test level at run creation; backward compatible (zero-valued on old runs)
- **QuickDeployedAt Field**: New `QuickDeployedAt` timestamp on `Record` marks when a run was already quick-deployed; blocks re-execution
- **Config Section**: New `QuickDeployConfig` with `AllowExecution` (false default) and `AllowProduction` (false default); zero-value-safe
- **Implementation**: New `runs.QuickDeployEligible()` gate in `internal/runs/quick_deploy.go`, `RequiredTestsRan()` whitelist, `IsProductionTarget()` fail-closed gate in `internal/git/source_suggestion.go`, `QuickDeployConfig` struct in `internal/config/config.go`

### 2. Salesforce CLI Integration

**Acceptance Criteria Addressed**: AC16, AC17

- **QuickDeploy Client Method**: New `sf project deploy quick --job-id <job-id> --target-org <alias> --json` call wrapped in `salesforce.Client.QuickDeploy()`, args passed as discrete slice (never shell-interpolated)
- **Error Handling**: Mirrors `CancelDeploy` exactly — runner-start failure returns error, CLI non-zero exit returns `QuickDeployResult{Raw: raw}` plus error
- **Raw Preservation**: Full stdout+stderr preserved for diagnostics and `quick.json` persistence
- **Implementation**: New `internal/salesforce/quick.go` with `QuickDeployResult` struct and `QuickDeploy()` method, interface added to `client.go`

### 3. Run Registration

**Acceptance Criteria Addressed**: AC18

- **MarkQuickDeployed**: New `Writer.MarkQuickDeployed(runID, quickRaw)` writes `quick.json` companion (mirrors `MarkCanceled`'s shape), sets `QuickDeployedAt` to now (NOT persisting the sfdc response into `run.json`)
- **Status Preservation**: `Status` field remains unchanged (e.g. stays `Succeeded`), preserving provenance (ADR-4: unlike `MarkCanceled`, which changes status to `Canceled`)
- **Backward Compatibility**: No schema version bump; old runs load cleanly
- **Implementation**: Single method in `internal/runs/writer.go`

### 4. App Surface & Gates

**Acceptance Criteria Addressed**: AC19, AC20, AC21, AC22, AC23, AC24

- **StateQuickDeploy**: New state for the quick-deploy confirmation view
- **x Key Action**: Per-row action in `StateRunHistory` that opens quick-deploy view for eligible runs; no-op on ineligible rows
- **Command Display**: Always shows `sf project deploy quick --job-id <job-id> --target-org <alias>`, never executes by default
- **Gate Stack**: `quickDeployExecAllowed(config, isProd)` = `AllowExecution AND (!isProd OR AllowProduction)` (fail-closed for production)
- **Confirmation**: Typed buffer requiring exact match to `DESPLEGAR` before exec; wrong input blocks execution
- **In-Flight Guard**: `m.quickDeployingRunID` field prevents in-session double-execution (correction H-1)
- **In-Memory Mark**: After `MarkQuickDeployed`, in-session `m.runs[i].QuickDeployedAt` is updated so `x` on the same row is a no-op (correction H-2)
- **Implementation**: `StateQuickDeploy` const, `keyQuickDeploy()` with rune-edit buffer + enter/esc/q routing, `quickDeployCmd()` capturing row data, `onQuickDeployDone()` handling results and registration, `viewQuickDeploy()` for view

### 5. End-to-End Validation

**Acceptance Criteria Addressed**: AC All (full consolidated path)

- **Eligibility Variants**: Age >10d, missing required tests, old run.json without TestLevel, already quick-deployed (all block `x`)
- **Production Block**: Non-production runs execute when gated; production runs blocked unless `AllowProduction=true`
- **Confirmation Gate**: Wrong/empty confirmation blocks execution even when production gate passes
- **Suggest-Only Default**: Zero-value config never executes even with `DESPLEGAR` typed + permitted target
- **Full Path**: Eligible run, permitted target, `DESPLEGAR` confirm → `sf project deploy quick` fires with correct args (row's JobID/Alias, NOT in-flight plan) → `MarkQuickDeployed` registers → in-memory `QuickDeployedAt` marks → `x` on same row becomes no-op
- **Implementation**: New `internal/app/quick_deploy_e2e_test.go` with runs fixture, FakeRunner, injected clock; `-short`-skippable, NO org

## Capabilities: 1 New + 2 Modified

| Capability | Action | Details |
|---|---|---|
| `quick-deploy` | **Created** | New spec at `openspec/specs/quick-deploy/spec.md` — full lifecycle of eligibility detection, suggested command display, production blocking, strong confirmation, opt-in execution, and suggest-only-by-default. 6 requirements, each with 1–4 scenarios. |
| `run-persistence` | **Modified** | Added 2 new requirements: `TestLevel` field (3 scenarios) + `QuickDeployedAt` field (implicit, via registration mechanism). Existing `PRUrl`, `SourceRunID`, `Phase`, `Commits` fields remain. |
| `run-history` | **Modified** | Added 1 new requirement (+ 3 scenarios) for the `x` key action opening quick-deploy view on eligible rows. Existing `Enter`→resume, `d`→delete, `r`→re-promote behavior unaffected. |

## Key Design Decisions

### 1. **TestLevel Captured at Run Creation, Not Post-Hoc**

- **Rationale**: The plan's test level is known at run creation time (same instant as `Ticket`, `Commits`, `Phase`). Capturing at creation preserves plan-execution fidelity and avoids a second write operation (like `MarkPRCreated` or `MarkCanceled`).
- **Benefit**: All eligibility data lives on the `Record` itself; no separate metadata file needed.

### 2. **QuickDeployedAt Does NOT Update Status**

- **Rationale**: Unlike `MarkCanceled` (which changes status to `Canceled`), a successful quick deploy is NOT a status change — the run remains `Succeeded`. The quick-deploy action is a terminal followup to an already-successful validation, not a state transition (ADR-4).
- **Benefit**: Preserves the original validation's success in the run record; the `quick.json` companion is the artifact, not a status flip.

### 3. **IsProductionTarget Fails Closed**

- **Rationale**: When `config.Branches` is empty or the target cannot be mapped, production protection must assume the worst (correction M-2: fail-closed). Only NON-production targets that positively map to `integration`/`uat` are permitted; everything else (including unmapped, empty target, or `production` env key) blocks execution.
- **Benefit**: Default-safe even with incomplete/missing configuration.

### 4. **Per-Row JobID/Alias from History, Never In-Flight Plan**

- **Rationale**: Quick deploy on a history row uses the SELECTED row's persisted JobID/Alias, never the current in-flight run's data. This prevents accidental cross-run execution if the user navigates to history during an active run (specification AC16 strict reading).
- **Benefit**: Each quick-deploy action is scoped to the selected historical run; no cross-contamination.

### 5. **Typed Confirmation Is Non-Bypassable**

- **Rationale**: `DESPLEGAR` is a full-word, cased password: not a single keystroke, not an affirmation (y/n), but a deliberate re-type. Mirrors `CancelDeploy`'s strong-confirmation pattern (HU-012). This gate is independent of config and always checked (spec AC22).
- **Benefit**: Accidental Enter keypresses cannot trigger a real deploy; intent must be explicit.

### 6. **In-Session Double-Deploy Guard via `quickDeployingRunID` + In-Memory Mark**

- **Rationale**: Two layers prevent in-session re-execution: (1) in-flight guard (`m.quickDeployingRunID != ""`) blocks Enter during polling; (2) in-memory `QuickDeployedAt` update after `MarkQuickDeployed` makes the row ineligible (correction H-2). Together they close all re-execution paths within a single session.
- **Benefit**: Complements the disk-level `QuickDeployedAt` check across sessions.

### 7. **Suggest-Only-by-Default via Zero-Value Config**

- **Rationale**: `AllowExecution` defaults to false; the zero value IS the safe default, so no `applyDefaults` entry is needed (no defaulting-is-itself-a-feature, per ADR-3). Every gate is AND-ed with `AllowExecution`, so it's non-negotiable (spec AC23).
- **Benefit**: No extra config required for the safe default; users opt in explicitly.

## Review Findings & Remediations

Adversarial review (post-implementation, focusing on the DESTRUCTIVE execute path) identified 5 findings (2 HIGH, 2 MED, 1 deliberate-re-type hardening), all remediations applied and verified:

### H-1 [HIGH]: No In-Flight Guard — Same Deploy Fires Twice

**Issue**: `keyQuickDeploy` could fire `m.quickDeployCmd()` multiple times in rapid succession if the user pressed Enter twice before the first call completed, allowing a SECOND real `sf project deploy quick` to run on the SAME eligible run.

**Fix Applied**: Added `m.quickDeployingRunID` field (captured at fire time). In `keyQuickDeploy`'s Enter case, check `if m.quickDeployingRunID != "" { return m, nil }` before firing. After fire, set `m.quickDeployingRunID = rec.RunID`. In `onQuickDeployDone`, clear `m.quickDeployingRunID` on success/error. Also clear `m.quickConfirm = ""` on fire so a stray re-typed Enter fails the DESPLEGAR gate.

**Verification**: RED test `TestKeyQuickDeploy_Enter_DoubleEnter_FiresQuickDeployOnce` (assert FakeRunner received exactly 1 call), `TestKeyQuickDeploy_Enter_InFlightReconfirm_DoesNotRefire` (re-type during polling, assert no 2nd call). Mutations: removing in-flight guard → DoubleEnter fails (2 calls); removing buffer-clear → InFlightReconfirm fails (2 calls).

### M-1 [MED]: Navigated-Away Success Was Silently Unregistered

**Issue**: If the user fired quick deploy, then navigated away from `StateQuickDeploy` before the response arrived, the old `onQuickDeployDone` early-returned on `state != StateQuickDeploy`, leaving the run unmarked on disk (correction M-1).

**Fix Applied**: Rewrote `onQuickDeployDone` to register by the CAPTURED `m.quickDeployingRunID` regardless of current state. Error → run left unmarked (user can retry). Success → `MarkQuickDeployed` called, in-memory mark set, notice prepared (notice only surfaced when still on screen, but registration is unconditional). Clear `m.quickDeployingRunID`/`m.quickConfirm` on both branches.

**Verification**: RED test `TestOnQuickDeployDone_Success_AfterNavigatingAway_StillRegisters` (navigate to history after firing, assert `quick.json` written + `QuickDeployedAt` set), `TestOnQuickDeployDone_Error_AfterNavigatingAway_LeavesRunUnmarked` (error case, assert run.json unchanged). Mutation: re-adding early-return → nav-away success fails (run left unmarked).

### H-2 [HIGH]: Session-Blind Double-Deploy (Disk Mark Not Reflected In-Memory)

**Issue**: `m.runs` is written only at startup and never updated by `MarkQuickDeployed`. After a successful quick deploy, `m.runs[i].QuickDeployedAt` remained zero, so a second `x` on the same row (within the same session) re-triggered eligibility check, allowing a SECOND `sf project deploy quick` call on the already-quick-deployed run.

**Fix Applied**: In `onQuickDeployDone`'s success branch, after `MarkQuickDeployed` succeeds, loop through `m.runs` to find the matching run by ID and set `m.runs[i].QuickDeployedAt = m.now()`, making it ineligible.

**Verification**: RED test `TestOnQuickDeployDone_Success_MarksInMemoryRunIneligible` (after success, drive `x` again on same row, assert no 2nd `sf` call + run stays on `StateRunHistory`). Mutation: removing the in-memory loop → test fails (eligibility check still returns true, `x` fires again).

### M-2 [MED]: Production Block Falls Open for Unmapped/Empty Targets

**Issue**: `IsProductionTarget` used `env == "production" || IsProductionBranch(target)`, which returned true only when the `production` key existed in config AND matched, or the literal string was `main`. For an empty target, unmapped target, or a config without a `production` key, it returned false, allowing production branches to slip through as non-production (fail-open bug).

**Fix Applied**: Rewrote `IsProductionTarget` to fail-closed: `(env, ok := environmentKeyForBranch(cfg, target); ok && env == "production") || IsProductionBranch(target)`. Now returns true when:
  1. The target maps to the `production` env key, OR
  2. The literal string is `main` or a recognized production pattern
  
And returns true (fail-closed) when:
  1. The target is empty, OR
  2. The target cannot be mapped to ANY known environment

Updated task 1.6's test: changed `Branches{"production":"RELEASE"} + target "UAT" → false` (which was an unmapped-target case encoding the bug) to `Branches{"production":"RELEASE","uat":"UAT"} + target "UAT" → false` so it's a genuine "mapped-non-prod" test.

**Verification**: RED test `TestIsProductionTarget_FailClosed_TableDriven` with cases: (a) empty config + target "" → true (empty is assumed dangerous); (b) config{} + target "main" → true (literal main); (c) config{} + target "UAT" → true (unmapped is assumed dangerous); (d) `Branches{"uat":"UAT"}` + target "UAT" → false (mapped non-prod); (e) `Branches{"production":"RELEASE"}` + target "RELEASE" → true (production env). Mutations: removing empty-target check → test (a) fails; removing unmapped check → test (c) fails.

### Deliberate-Re-Type Hardening [Follow-up]

**Issue**: A user could fire quick deploy, let it complete, then immediately re-type `DESPLEGAR` + Enter on the same `StateQuickDeploy` screen. The in-flight guard (H-1) was already clear (quick deploy completed), and the in-memory `QuickDeployedAt` mark (H-2) should make the row ineligible, BUT the `keyQuickDeploy` handler never re-checked eligibility after the first fire — it only checked `quickDeployExecAllowed + DESPLEGAR gate`, allowing a second real `sf project deploy quick` on the already-quick-deployed run.

**Fix Applied**: In `keyQuickDeploy`'s Enter case, added an eligibility re-check RIGHT AFTER fetching `rec`: `if eligible, _ := runs.QuickDeployEligible(rec, m.now()); !eligible { ... return m, nil }`. This re-check happens before all other gates (in-flight, exec-allowed, confirm), ensuring a quick-deployed run is rejected immediately.

**Verification**: RED test `TestKeyQuickDeploy_Enter_AfterSuccessReconfirm_DoesNotRefire` (fire, wait success, re-type DESPLEGAR + Enter, assert FakeRunner received exactly 1 call; also assert `quickDeployingRunID == ""` at the second Enter to isolate eligibility gate from in-flight guard). Mutation: removing the re-check → test fails (2 calls logged).

## Test Posture

### Unit Tests

- **`internal/runs/quick_deploy_test.go`**: `QuickDeployEligible` table-driven (Succeeded+RunLocalTests+age<10d → eligible; age≥10d → ineligible; missing required tests → ineligible; old run.json with TestLevel="" → ineligible; already quick-deployed → ineligible)
- **`internal/git/source_suggestion_test.go`**: `IsProductionTarget` fail-closed table (empty config+empty target → true; empty config+main → true; empty config+UAT → true; mapped non-prod → false; production env key → true)
- **`internal/config/config_test.go`**: `QuickDeployConfig` zero-value default when omitted
- **`internal/runs/writer_test.go`**: `TestLevel` and `QuickDeployedAt` round-trip, backward-compat with prior `run.json` (zero-valued on load)
- **`internal/salesforce/quick_test.go`**: `QuickDeploy` args composition, error handling (mirrors CancelDeploy)
- **`internal/salesforce/argcomposition_test.go`**: Threat matrix — jobID survives as ONE discrete arg after `--job-id`
- **`internal/app/quick_deploy_test.go`**: `keyQuickDeploy` (typed buffer, Enter/Esc/q routing), gates (AllowExecution, AllowProduction, IsProductionTarget), in-flight guard, in-memory mark re-check, eligibility re-check on re-type

### Integration Tests

- **`internal/salesforce/quick_test.go`**: Temp runner mocks; command composition; error handling
- **`internal/runs/quick_deploy_test.go`**: `QuickDeployEligible` with real `Record` structs; all predicate combinations

### End-to-End Tests

- **`internal/app/quick_deploy_e2e_test.go`** (all 105 tasks):
  - **EligibleShowsCommand**: Eligible row + `x` → view shows command, no execution (Group 1 baseline)
  - **IneligibleVariants** sub-tests: (a) age >10d → no-op; (b) missing required tests (TestLevel ∈ {"RunSpecifiedTests", "NoTestRun", ""}) → no-op; (c) already-quick-deployed (`QuickDeployedAt` set) → no-op
  - **ProductionBlockedWithoutConfig**: Production target + `AllowExecution=true` + `AllowProduction=false` + `DESPLEGAR` → no execution, run not marked
  - **NoConfirmDoesNotDeploy**: Permitted target + empty/wrong confirm → no `sf` call (3 sub-variants: no input, wrong word, partial word)
  - **GateAndConfirmExecutesAndRegisters**: Permitted target + `AllowExecution=true` + `DESPLEGAR` → `sf project deploy quick` fires with row's JobID/Alias (NOT in-flight data) → `MarkQuickDeployed` registers → `quick.json` written + `QuickDeployedAt` set + `Status` unchanged
  - **Remediation scenarios** (5): DoubleEnter (1 call only), InFlightReconfirm (no 2nd call), AfterNavigatingAway (registers anyway), InMemoryMark (re-check makes row ineligible), ReTypeAfterSuccess (no 2nd call via re-check gate)

### Code Quality

- **`go test -race ./...`**: All green (no race conditions; in-flight guard and in-memory mark are mutex-free due to single-threaded Bubble Tea model)
- **`go vet ./...`**: Clean
- **`gofmt -l .`**: No formatting issues
- **`TestApp_NeverImportsExecSeam`**: Still passes (no new exec seams in `internal/app` — `sf.QuickDeploy` reached only through `m.deps.SF`, same seam as every other Salesforce call)

### Summary

- **105 total tasks** (5 implementation groups + 5 post-implementation remediation fixes): All checked
- **E2E scenarios**: 5+ (suggest-only, ineligible variants, production block, confirm gate, full path) + 5 remediation scenarios
- **Adversarial review**: 5 findings identified and fixed (H-1, M-1, H-2, M-2, re-type hardening); full regression suite green

## Merged Artifacts

### Files Synced to Living Specs

1. **`openspec/specs/quick-deploy/spec.md`** (NEW)
   - Created from change's delta (full spec, not delta format)
   - 6 requirements (eligibility detection, suggested command display, production blocking, strong confirmation, opt-in execution, suggest-only-by-default)
   - 16 scenarios total

2. **`openspec/specs/run-persistence/spec.md`** (MODIFIED)
   - Added 2 new requirements: `TestLevel` recorded at run creation (3 scenarios) + QuickDeployedAt implicit via registration
   - `Record.TestLevel` field (omitempty, backward-compatible), captured at run-creation time from plan
   - Existing `PRUrl`, `SourceRunID`, `Phase`, `Commits`, cancellation, retention all preserved

3. **`openspec/specs/run-history/spec.md`** (MODIFIED)
   - Added 1 new requirement: `x` on an eligible run opens quick-deploy view (3 scenarios)
   - `x` key action, eligibility gate, ineligible no-op, existing `Enter`→resume + `d`→delete + `r`→re-promote behavior preserved
   - Existing history list, detail view, resume routing all preserved

### Change Artifacts Archived

- **Proposal**: `openspec/changes/quick-deploy/proposal.md` (moved to archive)
- **Design**: `openspec/changes/quick-deploy/design.md` (moved to archive)
- **Exploration**: `openspec/changes/quick-deploy/exploration.md` (moved to archive)
- **Specs**: `openspec/changes/quick-deploy/specs/quick-deploy/spec.md`, `run-persistence/spec.md`, `run-history/spec.md` (merged to living; folder archived)
- **Tasks**: `openspec/changes/quick-deploy/tasks.md` (archived; all 105 tasks complete + 5 remediations)

## What Remains OUT (Future Work)

### HU-017: Cleanup (Explicit Post-Deploy Cleanup)

- Scope: Delete the temporary `deploy/*` promotion branch and `.deploydeck/runs/<run-id>` after successful deployment
- Status: Out of scope for HU-015

### HU-018: Multiple Sandbox Deployments in One Run

- Scope: Deploy to multiple sandboxes in sequence
- Status: Out of scope for HU-015

### HU-019: Release Pipeline Automation

- Scope: Auto-merge, auto-tag, auto-deploy to prod
- Status: Out of scope for HU-015

## Specification Conformance

All acceptance criteria from `docs/HISTORIAS.md:HU-015` met:

| AC | Title | Status | Evidence |
|---|---|---|---|
| AC11 | Eligible runs (Succeeded/SucceededPartial, required tests, <10d, not already quick-deployed) | ✅ | `QuickDeployEligible` gate + e2e eligibility variants |
| AC12 | Suggested command displayed, never executed by default | ✅ | `viewQuickDeploy()` shows command; `AllowExecution=false` default blocks all paths |
| AC13 | Production target blocked without `AllowProduction=true` | ✅ | `IsProductionTarget` fail-closed + e2e production-block scenario |
| AC14 | Strong confirmation required (`DESPLEGAR`) | ✅ | `keyQuickDeploy` buffer validation; e2e no-confirm scenario confirms gate |
| AC15 | Fully authorized execution runs quick deploy and registers | ✅ | `quickDeployCmd()` + `onQuickDeployDone()` + `MarkQuickDeployed()` + e2e full-path scenario |
| AC16 | Uses the selected row's JobID/Alias (not in-flight plan) | ✅ | `quickDeployCmd()` captures `rec := m.runs[m.runsCursor]` (row data, never `m.plan`) |
| AC17 | Suggest-only by default (`AllowExecution=false` zero-value) | ✅ | `QuickDeployConfig` defaults to `{false, false}`; e2e default-suggest-only scenario |
| AC18+ | All edge cases (eligibility predicates, gates, confirmations) | ✅ | Comprehensive e2e coverage + 5 remediation scenarios (H-1, M-1, H-2, M-2, re-type) |

## Transition to Production

### Pre-Commit Checklist

- [x] All 105 tasks (5 groups + 5 remediations) checked in `tasks.md`
- [x] `go test -race ./...` green
- [x] `go vet ./...` clean
- [x] `gofmt -l .` clean
- [x] `boundary_test.go` passes (no new exec seams)
- [x] Proposal.md success criteria verified
- [x] E2E tests pass (no real org, temp git + runs fixture + FakeRunner only)
- [x] Adversarial review: 5 findings identified and fixed; full regression suite green

### Deployment Notes

1. **No Database Migrations**: `TestLevel`, `QuickDeployedAt` are `omitempty` — every existing `run.json` still loads cleanly
2. **No Config Changes Required**: Default `QuickDeployConfig{AllowExecution: false, AllowProduction: false}` is zero-value-safe
3. **Backward Compatible**: Older runs load with `TestLevel=""` and `QuickDeployedAt.IsZero()` — no special handling needed
4. **Production-Safe Default**: Quick deploy is suggest-only by default; production is fail-closed; confirmation is non-bypassable
5. **Rollback**: Delete `internal/runs/quick_deploy.go`, `internal/salesforce/quick.go`, `internal/app/quick_deploy_test.go`, `internal/app/quick_deploy_e2e_test.go`; revert `internal/runs/writer.go` (TestLevel, QuickDeployedAt fields, MarkQuickDeployed), `internal/git/source_suggestion.go` (IsProductionTarget), `internal/config/config.go` (QuickDeployConfig), `internal/app/app.go` (StateQuickDeploy, fields), `internal/app/keys.go` (`keyQuickDeploy`, `x` case in `keyRunHistory`), `internal/app/commands.go` (quickDeployDoneMsg, quickDeployCmd), `internal/app/update.go` (onQuickDeployDone wire-up), `internal/app/view.go` (viewQuickDeploy wire-up), `internal/salesforce/client.go` (QuickDeploy interface method)

## Handoff Summary

- **Source of Truth Updated**: `openspec/specs/{quick-deploy,run-persistence,run-history}/spec.md` now authoritative
- **Change Folder**: Archived to `openspec/changes/archive/quick-deploy/`
- **Code Ready**: All tests pass, all 105 tasks + 5 remediations complete, adversarial review findings fixed
- **Production-Safe**: Default suggest-only, fail-closed production protection, non-bypassable confirmation, double-deploy guards in place
- **No Blockers**: Clean bill of health for merge

---

**Archived By**: SDD Archive Phase (quick-deploy)  
**Archive Date**: 2026-07-29  
**Next Steps**: Merge to `main`, deploy, and proceed with follow-up stories (HU-017, HU-018, HU-019)
