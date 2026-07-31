# Archive Report: Source Resolution — Dedupe Candidates + Current-Branch Confirm

**Date Archived**: 2026-08-01  
**Change**: source-resolution  
**Status**: ✅ **ARCHIVED AND CLOSED**

## Executive Summary

Completed implementation of source-resolution (HU-002/HU-004 feature branch selection enhancement). The change ships two coordinated fixes:
- **D1 (Dedupe)**: Local and remote-tracking branches of the same logical name (e.g. `X` and `origin/X`) are now collapsed into one candidate, fixing the bug where pushed feature branches were counted twice and incorrectly triggering "no candidates" on first-environment promotion.
- **D2 (Current-Branch Confirm)**: When discovery is ambiguous (2+ distinct candidates) and no pipeline default applies, the user is explicitly prompted to confirm using their currently-checked-out branch as the source, with clear accept/decline controls and pipeline-priority guardrails.

Both fixes land inside the existing single-source funnel, preserving all invariants. Verification: **PASS** (7/7 scenarios, 22/22 tasks, all green). Review: **CLEAN** (0 findings). Spec merged. Ready to close.

## Capability Change

**Capability Modified**: `commit-discovery`

### Requirements Updated

| Requirement | Action | Change |
|-------------|--------|--------|
| Candidate Branch Search By Name | **MODIFIED** | Added local/remote-tracking dedupe logic; 2 new scenarios (pushed-feature-single-candidate, distinct-branches-not-collapsed) |
| Current-Branch Source Confirmation | **ADDED** | New requirement with 4 scenarios covering confirmation flow, decline path, ineligibility guards, pipeline-priority precedence |

### Total: 1 Modified + 1 Added → 11 total requirements in living spec (was 9)

## Living Spec Updated

**File**: `openspec/specs/commit-discovery/spec.md`  
**Actions**:
- Replaced "Candidate Branch Search By Name" with dedupe-aware version (3 scenarios total)
- Inserted "Current-Branch Source Confirmation" as new requirement (4 scenarios) between "Candidate Branch Search By Name" and "Single Source Branch Enforcement"
- All other 9 requirements preserved unchanged

## Implementation & Verification

### Files Changed
- **Git layer**: `internal/git/service_branches.go` (dedupe helper), `internal/git/service_branches_test.go` (unit coverage), `internal/git/discovery_e2e_test.go` (repro e2e)
- **App layer**: `internal/app/flow.go` (tri-state `resolveSource`), `internal/app/flow_test.go` (8-case table), `internal/app/commands.go` (split `discoverCmd` + `confirmSourceCmd`), `internal/app/app.go` (new `StateSourceConfirm`), `internal/app/keys.go` (key routing), `internal/app/update.go` (confirm routing), `internal/app/view.go` (confirm render), `internal/app/source_confirm_test.go` (5 new unit tests), `internal/app/flow_e2e_test.go` (repro + confirm e2e)
- **Total**: 11 files modified/created

### Test Coverage

**TDD Compliance**: Strict TDD (RED-first for all behaviors). All 5 phases complete.

| Layer | Test Count | Coverage |
|-------|-----------|----------|
| Unit (Git) | 2 new + 1 guard | Dedupe logic, over-aggression guard, `ListBranches` regression guard |
| Unit (App) | 8 subtests | `resolveSource` tri-state (pipeline priority, current-branch match, ineligibility cases) |
| Unit (App/TUI) | 5 tests | `StateSourceConfirm`, `keySourceConfirm` (s/n/N/enter/esc), `viewSourceConfirm` |
| E2E (Git) | 2 subtests | Dedupe repro (1 candidate, non-empty range), distinct-candidate scenario |
| E2E (App) | 3 subtests | Dedupe path (no confirm), confirm accept, confirm decline |
| Regression | Named suites | `TestStandaloneBranchesCmd_*`, `standalone_modes_e2e_*`, `re_promote_e2e_*`, `original_branch_*`, `boundary_test` — all green, unmodified |

**Verdict**: ✅ **PASS**  
- Requirements: 2/2 (1 MODIFIED + 1 ADDED)
- Scenarios: 7/7 compliant
- Test exit code: 0
- Build: clean (`go build ./...`, `gofmt`, `go vet`)
- Changed lines: 798 (within budget)

### Review & Quality

**Review Outcome**: ✅ **CLEAN**  
- Review lens: `review-reliability`
- Findings: 0 (no blockers, no critical issues)
- 4 non-blocking suggestions (all documentation/clarification, none blocking archive)

**Design Integrity**:
- ADR-1: Dedupe scoped to `CandidateBranches` only (not `ListBranches`/`branchesByPattern`); guards regression via pre-existing standalone test suite
- ADR-2: Current-branch match computed inside pure `resolveSource` (no new exec/git call in app layer); `internal/app` stays off exec seam (verified by `boundary_test`)
- ADR-3: Confirm fires on message boundary (pass-1 discovery → `discoverDoneMsg.confirm:true` → `StateSourceConfirm` state → pass-2 ranged discover on accept)
- ADR-4: Reuses existing `m.source`, `m.discovery`, `m.originalBranch` fields; single-source invariant preserved via unmodified `git.SelectSingleSource` funnel

**Deviations**: None in behavior. One clarifying interpretation (design-time `len(candidates) > 1` gate on needs-confirm to reconcile explicit task 4.2 e2e requirement with implicit task 2.2 prose) documented in apply-progress and verified by all tests passing.

## Artifacts

### Change Artifacts (source-resolution)
- `openspec/changes/source-resolution/proposal.md` — scope, decisions (D1/D2), rollback plan
- `openspec/changes/source-resolution/specs/commit-discovery/spec.md` — delta (1 MODIFIED + 1 ADDED requirements with all scenarios)
- `openspec/changes/source-resolution/design.md` — ADRs 1-4, data flow, interfaces, file manifest, testing strategy, threat matrix
- `openspec/changes/source-resolution/tasks.md` — 5 phases (git dedupe, app tri-state, confirm state + split, e2e, regression guard), all 22 tasks checked `[x]`
- `openspec/changes/source-resolution/apply-progress.md` — TDD cycle evidence (RED/GREEN/REFACTOR per task), files changed, design interpretation note, regression verification
- `openspec/changes/source-resolution/verify-report.md` — verdict PASS, 7/7 scenarios, 2/2 requirements, all green tests, 4 non-blocking suggestions, 0 critical findings

### Living Spec Updated
- `openspec/specs/commit-discovery/spec.md` — merged delta; now 11 requirements total (was 9)

## Deferred Follow-Up

**Out of Scope** (residual known gap, deferred to next change):
- Full source-branch PICKER for genuinely-distinct-multiple candidates with no current-branch match. Currently: user who is not on a candidate branch in a 2+ candidate ambiguous scenario will decline the confirm (unavailable) and land in a no-result state, same as before this change. The confirm-only fix is a necessary but insufficient step toward full source selection UX.

## Gates & Checklist

- [x] **Review Gate**: Verify verdict = PASS; review outcome = CLEAN (0 critical findings)
- [x] **Task Completion**: All 22 tasks marked complete in tasks.md
- [x] **Spec Merge**: Delta merged into living spec; format preserved; all other requirements unchanged
- [x] **Archive Integrity**: All artifacts (proposal, specs, design, tasks, verify-report, apply-progress) accounted for; change folder ready for orchestrator to move to archive
- [x] **No Critical Issues**: Blockers = 0; ready to close

## Closure Statement

The source-resolution change is **complete, verified, and archived**. The commit-discovery capability has been updated with dedupe logic and current-branch confirmation, shipped as a single coherent atomic change (D1 + D2 in one PR, no intermediate states). All requirements met. All tests green. All gates cleared. The change is closed.

Next recommended phase: none (this change is fully complete and archived).
