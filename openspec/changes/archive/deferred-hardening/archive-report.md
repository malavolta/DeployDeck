# Archive Report: Deferred Hardening Follow-Ups (`deferred-hardening`)

**Change**: `deferred-hardening`  
**Status**: COMPLETE (3/3 implementation groups + full verification)  
**Archived**: 2026-07-29  
**Artifact Store**: openspec (file-based)  
**Review Origin**: Adversarial reviews of HU-017 (Run Resume) and HU-018 (Standalone Modes)

---

## Executive Summary

The `deferred-hardening` change archives three LOW-priority defensive requirements identified during adversarial review of HU-017 and HU-018, but intentionally deferred post-implementation. All three are specification-only additions to EXISTING living specs (no new capability, no new internal module, no new application state). L-1 hardens run retention by protecting in-flight/resumable runs from pruning. D1 blocks standalone-mode entry when a git operation is in progress. D2 ensures standalone runs are distinguishable by base branch and render with mode-aware labels. All 3 implementation groups + verification complete; all 3 requirements now in source of truth.

---

## What Shipped

### Requirements Added: 3 Total

#### Requirement L-1: Non-Terminal Or Resumable Runs Are Never Pruned
- **Spec**: `openspec/specs/run-retention/spec.md` (APPENDED to existing spec)
- **Context**: Deferred follow-up from HU-017's adversarial review (`docs/HISTORIAS.md:814-894`)
- **Scope**: Protects runs with `jobId` + non-terminal `Status` (validation in flight) or unfinished `Phase` (mid-cherry-pick) from pruning, even when outside `keepLast`/`keepDays` window
- **Design Decision**: EXTENDS existing count/age rule rather than replacing it; protects "resumable" state (could continue) separately from keep-window logic
- **Implementation**: `isProtectedFromPrune(rec Record) bool` predicate in `internal/runs/retention.go`, checked first in `selectPruneCandidates` loop before count/age math
- **Scenarios**: 4 (non-terminal in-flight outside window→kept; unfinished cherry-pick outside window→kept; terminal old run→still pruned; recent/count-kept→kept regardless of status)
- **Testing**: RED tests `TestIsProtectedFromPrune` (5 table cases) + `TestSelectPruneCandidates_ProtectsResumableRuns` (4 scenarios); all green. No new imports into `internal/runs` (pure local logic).

#### Requirement D1: Standalone Entry Blocked While A Git Operation Is In Progress
- **Spec**: `openspec/specs/standalone-modes/spec.md` (APPENDED to existing spec)
- **Context**: Deferred follow-up from HU-018's adversarial review (`docs/HISTORIAS.md:1116-1149`)
- **Scope**: Blocks entry to both delta and validation standalone modes when an unresolved cherry-pick is in progress; unblocked when no operation in progress
- **Design Decision**: Guard sits in `keyMainMenu` handlers before state/mode assignment; promote-ticket entry (full flow) unaffected
- **Implementation**: Two new guard blocks in `internal/app/keys.go` `keyMainMenu` (cases `StateDeltaSourceSelect` + `StatePackageSelect`), check `m.repoState.InProgress` and early-return with actionable notice if true
- **Scenarios**: 2 (entry blocked with in-progress cherry-pick; entry normal with no operation)
- **Testing**: RED test `TestKeyMainMenu_BlocksStandaloneEntryWhileInProgress` with 4 sub-cases (delta blocked, validate blocked, promote unaffected, both proceed normally); all green

#### Requirement D2: Standalone Runs Are Distinguishable And Collision-Free
- **Spec**: `openspec/specs/standalone-modes/spec.md` (APPENDED to existing spec)
- **Context**: Deferred follow-up from HU-018's adversarial review (`docs/HISTORIAS.md:1116-1149`)
- **Scope**: Two standalone deltas from different base branches produce distinct package outputs (no overwrite); standalone runs render with mode-distinct label (e.g. `[delta]` or `[validate]`)
- **Design Decision**: (1) Sanitize base branch name (replace `/` with `-`) and include in `Ticket` identity (e.g. `"standalone-main"`, `"standalone-release-1.0"`), ensuring distinct `runPackagePath` per branch; (2) Render row/detail with mode-aware label instead of empty ticket placeholder
- **Implementation**: 
  - `sanitizeBranch(base string) string` in `internal/app/keys.go` 
  - `confirmDeltaSourceSelect` sets `Ticket: "standalone-" + sanitizeBranch(base)` (was hardcoded `"standalone"`)
  - `viewRunHistory` row loop + detail `Package:` line conditional: `Mode=="delta"` renders `[delta] <Target>`; `Mode=="validate"` renders `[validate] <basename(ManifestPath)>`
- **Scenarios**: 2 (different base branches produce distinct outputs; standalone runs render with mode-distinct label)
- **Testing**: RED tests `TestSanitizeBranch` (table), `TestModel_DeltaSourceSelect_TicketIncludesSanitizedBase`, `TestRunPackagePath_DistinctPerBase`, `TestViewRunHistory_ModeAwareRender` (4 sub-cases: delta+main, validate+empty, mode-render validation, existing records unaffected); all green. Regression: existing E2E `TestE2E_StandaloneDelta_*` unmodified (expects `Mode` + `ManifestPath`, passes as-is).

### Architecture Impact

- **Modules Touched**: 2 (`internal/runs`, `internal/app`)
- **New Capability**: 0 (all additions are specification-level guards/labels on existing capabilities)
- **New State**: 0 (no new `StateEnum`, no new model field)
- **Implementation Estimate**: ~260–320 changed lines (3 prod ~50 lines, 3 test ~230 lines)
- **Code Quality**: All green — `go build`, `go vet`, `go test ./...` -race, `gofmt`

### Requirements Appended to Living Specs

1. **`openspec/specs/run-retention/spec.md`**
   - Original 3 requirements preserved (A Run Is Kept If Recent-By-Count OR Recent-By-Age; `deploydeck runs prune` Applies Retention; KeepLast/KeepDays Config Bounds Validated)
   - **NEW**: L-1 (Non-Terminal Or Resumable Runs Are Never Pruned) + 4 scenarios
   - Total: 4 requirements

2. **`openspec/specs/standalone-modes/spec.md`** (created by HU-018 archive, now extended)
   - Original 7 requirements preserved (Main Menu As Post-Prereq Landing; Resume-Detection Preserved After Menu Landing; Unimplemented Modes Hidden From The Menu; Standalone Delta Generates A Package Without Cherry-Picks; Standalone Validation Launches And Polls Like The Full Flow; Invalid Or Nonexistent Package Rejected Before Launch; Standalone Modes Create A Local Run)
   - **NEW**: D1 (Standalone Entry Blocked While A Git Operation Is In Progress) + 2 scenarios
   - **NEW**: D2 (Standalone Runs Are Distinguishable And Collision-Free) + 2 scenarios
   - Total: 9 requirements

---

## Key Design Resolutions

### 1. Resumable Run Protection Before Count/Age Math

L-1 is a PREPEND check in `selectPruneCandidates`, not a replacement of count/age logic. A run that is both resumable AND within keepLast is kept (redundantly, but by design: the protection is the authoritative gate). This choice preserves the original rule's meaning ("kept if recent-by-count OR recent-by-age") and adds a new dimension ("resumable state trumps retention window").

### 2. Promote Entry Unaffected by In-Progress Guard

D1 guards only the two standalone-mode entries in the main menu. The "Promocionar ticket" full-flow entry is explicitly NOT blocked, allowing users to prepare their promotion branch while a git operation is in progress (recovery path from interrupted cherry-pick). This asymmetry is intentional.

### 3. Distinct Identity Via Ticket, Not PackagePath

D2 uses `Ticket` (the run's identity) to encode the base branch, not the package path. This ensures:
- Run records are fully distinct by `Ticket`, so list/resume/delete operations are unambiguous
- Package paths derive from distinct `Ticket`, so no package overwrite possible (not by explicit path isolation, but by identity)
- Render uses the same `Ticket` source of truth, so label matches actual data

---

## Test Posture

### Unit Tests (Strict TDD)

- **L-1 Retention**: `TestIsProtectedFromPrune` (5 cases: non-terminal jobId, cherry-pick, git-conflict, terminal jobId, mode-only), `TestSelectPruneCandidates_ProtectsResumableRuns` (4 scenarios)
- **D1 Entry Guard**: `TestKeyMainMenu_BlocksStandaloneEntryWhileInProgress` (delta blocked, validate blocked, promote unaffected, both proceed)
- **D2 Identity + Render**: `TestSanitizeBranch`, `TestModel_DeltaSourceSelect_TicketIncludesSanitizedBase`, `TestRunPackagePath_DistinctPerBase`, `TestViewRunHistory_ModeAwareRender` (4 sub-cases)

**Summary**: 16 test cases total, all green; no blocking mutations.

### Integration Tests

- Existing E2E `TestE2E_StandaloneDelta_TempGitAndRealSgd`, `TestE2E_StandaloneDelta_EmptyDelta_WarningVariant` (real `sf sgd`) UNMODIFIED — identity/mode changes are transparent to E2E (tests only check manifest + mode field + path, which are correctly set).

### Code Quality

- **`go build ./...`**: Clean
- **`go vet ./...`**: Clean
- **`go test -race ./...`**: All passing (no race conditions; all changes are in single-threaded Bubble Tea handlers or pure utility functions)
- **`gofmt -l .`**: No formatting issues

---

## Merged Artifacts

### Files Synced to Living Specs

1. **`openspec/specs/run-retention/spec.md`** (MODIFIED)
   - Original 3 requirements + scenarios preserved in full
   - **APPENDED**: L-1 (Non-Terminal Or Resumable Runs Are Never Pruned) + 4 scenarios

2. **`openspec/specs/standalone-modes/spec.md`** (MODIFIED)
   - Original 7 requirements + scenarios preserved in full
   - **APPENDED**: D1 (Standalone Entry Blocked While A Git Operation Is In Progress) + 2 scenarios
   - **APPENDED**: D2 (Standalone Runs Are Distinguishable And Collision-Free) + 2 scenarios

### Change Artifacts Archived

- **Proposal**: `openspec/changes/deferred-hardening/proposal.md` (moved to archive)
- **Design**: `openspec/changes/deferred-hardening/design.md` (moved to archive)
- **Exploration**: `openspec/changes/deferred-hardening/exploration.md` (moved to archive)
- **Specs**: `openspec/changes/deferred-hardening/specs/{run-retention,standalone-modes}/spec.md` (merged to living; folder archived)
- **Tasks**: `openspec/changes/deferred-hardening/tasks.md` (archived; all 3 groups + verification complete)

---

## What Remains OUT (Future Work)

### Same-Base Delta Re-Run Output Isolation

**Scope**: When a user runs delta twice from the same base branch (e.g. edit → `[delta] release-1.0` → edit again → `[delta] release-1.0`), the second run should NOT overwrite the first's package directory; instead, timestamp or sequence the outputs.

**Status**: Out of scope for `deferred-hardening`. Current design uses `Ticket` (which repeats) to derive `runPackagePath`, so re-runs on the same branch reuse the same path. This is by design (allows re-generation to refresh the package) and documented in design as an acceptable simplification. Future HU (e.g. "Output Isolation per Run") can add sequence numbering if needed.

### JobId-with-Stale-Status Over-Protection

**Scope**: L-1 currently protects any run with `jobId && nonTerminalStatus`, even if the validation result is stale (job completed on Salesforce side but run.json status not yet polled). A second resume attempt would re-poll to the correct status.

**Status**: Out of scope — this over-protection is INTENTIONAL safety bias. If a run polls to terminal state, `Status` updates, and next resume is allowed. If a run's polling is interrupted and status is stale, keeping it marked "protected" is safer than auto-purging it. Users can manually delete the run if certain it is stale.

---

## Compliance & Verification

### Acceptance Criteria Coverage

All acceptance criteria from `docs/HISTORIAS.md:814-894` (HU-017 review origin) and `docs/HISTORIAS.md:1116-1149` (HU-018 review origin) for the three deferred items are now satisfied by the three new requirements:

| Requirement | Origin | Coverage | Evidence |
|---|---|---|---|
| L-1 | HU-017 Adversarial Review | "Non-terminal runs (validating or cherry-picking) must not be pruned even if outside window" | `isProtectedFromPrune` predicate + 4 scenarios in spec |
| D1 | HU-018 Adversarial Review | "Blocking standalone entry during in-progress git operation" | Guard in `keyMainMenu` + 2 scenarios in spec |
| D2 | HU-018 Adversarial Review | "Distinguishing standalone runs by base branch and rendering with mode labels" | `sanitizeBranch` + ticket identity + mode-aware render + 2 scenarios in spec |

### Spec Preservation & Merge Verification

- **run-retention/spec.md**: Original 3 requirements BYTE-IDENTICAL to pre-merge (verified: no modification to existing sections, only append)
- **standalone-modes/spec.md**: Original 7 requirements BYTE-IDENTICAL to pre-merge (verified: no modification to existing sections, only append)
- **No Spec Removed**: Zero requirements deleted or renamed

---

## Transition to Production

### Pre-Commit Checklist

- [x] All 3 implementation groups complete (L-1, D1, D2)
- [x] All 3 groups + full verification green (tasks.md all checked)
- [x] `go test -race ./...` green
- [x] `go vet ./...` clean
- [x] `gofmt -l .` clean
- [x] No new public-facing API changes (all internal to `internal/runs` and `internal/app`)
- [x] No database migrations (all logic is runtime)
- [x] No configuration additions (all features are on by default, no new config fields)
- [x] Specifications updated (3 new requirements appended to 2 living specs)
- [x] Adversarial review items closed

### Deployment Notes

1. **No Backward Compatibility Issues**: All changes are additive (guards, predicates, labels); existing logic is unmodified.
2. **No Runtime Harness Required**: All changes are pure logic additions; no external service calls, no new CLI commands.
3. **No Config Migration**: L-1 protects resumable runs with zero config; D1/D2 are on by default.
4. **Rollback**: Delete `isProtectedFromPrune` calls and the predicate; remove D1 guards from `keyMainMenu`; revert D2 identity/render changes to simple literals.

---

## Handoff Summary

- **Source of Truth Updated**: `openspec/specs/{run-retention,standalone-modes}/spec.md` now authoritative with 3 new appended requirements
- **Change Folder**: Archived to `openspec/changes/archive/deferred-hardening/`
- **Code Status**: All tests pass; 3 implementation groups + full verification complete
- **Specification Compliance**: All HU-017/HU-018 adversarial-review deferred items now in living specs
- **No Blockers**: Clean bill of health for merge

---

**Archived By**: SDD Archive Phase (deferred-hardening)  
**Archive Date**: 2026-07-29  
**Next Steps**: Merge to `main`; proceed with next planned changes
