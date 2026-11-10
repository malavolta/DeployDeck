# Archive Report: TUI Self-Update

**Change**: `tui-self-update`  
**Status**: `pass with warnings`. Verify passed with warnings. CRITICAL-1 is fixed. W-1 and W-5 are resolved. W-2, W-3, W-4, W-6 and W-7 were carried forward (see below).  
**Branch**: `feat/tui-self-update`  
**Archived**: 2026-11-05  
**Archived to**: `openspec/changes/archive/2026-11-05-tui-self-update/`

---

## What Shipped

Upgrade DeployDeck from the main menu with the `u` key. The TUI quits, restoring the original branch, and then the process runs the install-method upgrade command with inherited stdio. `internal/app` never executes processes. It only exposes `UpgradeRequested()`.

Commits (work units, reported by the orchestrator after user approval):

| Unit | Commit | Scope |
|------|--------|-------|
| plan | `da140b2` | openspec plan |
| 1 | `3baa8fc` | install-method detection and upgrade argv |
| 2 | `edce664` | 24h latest-release cache |
| 3 | `720ccf7` | app `u` key, guard, Spanish banner |
| 4 | `7d66364` | `runUpgrade` with Windows print-only |
| 5 | `63fbef3` | composition: opt-out, cache wiring, post-run hook |
| 6 | cut | ADR-7 build-info fallback, not implemented |
| 7 | docs and verification | README `## Update`, activation checklist |

---

## Specs Merged

### `update-notification` (modified, existing living spec)

File: `openspec/specs/update-notification/spec.md`

- **ADDED** `Real Release Repository`: 1 scenario.
- **ADDED** `24-Hour Check Cache`: 5 scenarios.
- **ADDED** `Update Check Opt-Out`: 2 scenarios.
- **ADDED** `Spanish Update Banner With Action Hint`: 4 scenarios.
- **MODIFIED** `Semver-Based Newer-Version Detection`: replaced in place. Adds the `version.Version` source, the pre-release rule, and the scenario `Pre-release tag never triggers a notice`.
- **Preserved unchanged**: `Non-Blocking Startup Check` and `Silent Skip on Check Failure`.

Result: 5 requirements to 9 requirements.

### `tui-self-update` (new living spec)

File: `openspec/specs/tui-self-update/spec.md`

Converted from the delta using `## Requirements`, with the `## Purpose` section kept. 8 requirements:

1. Install Method Detection
2. Upgrade Command Construction
3. Update Key On Main Menu Only
4. Guard While An Operation Is In Progress
5. No-Op Without Update Or Wiring
6. Quit-Then-Run Execution
7. Failure Fallback Prints Manual Commands
8. Manual Install Is Hint Only

**Not merged**: the ADR-7 build-info version fallback. Its requirement was removed from the delta when the ADR was cut (state.yaml, resolved_issues).

---

## Open Follow-Ups (from state.yaml)

1. **ADR-7 build-info version fallback** (cut for the 800-line budget). Binaries installed with `go install` report `dev`, so they never show the banner and the `u` key never appears for them. This is W-6. `README.md:65` overstates the feature for those users. Needs its own change.
2. **In-place self-replace for manual binaries**: follow-up `tui-self-update-manual`. Out of scope for this change.

---

## Remaining Warnings (carried forward)

- **W-2**: `internal/app/self_update_test.go:36-42` checks `u` on other screens only with a negative assertion. It does not prove that the existing `u` handling (for example typing into the ticket input) is preserved.
- **W-3**: the requested branch of `postRun` (`cmd/deploydeck/selfupdate.go:96-99`) is untested. The stdio inheritance of `runInherited` is verified only by reading the source.
- **W-4**: `cmd/deploydeck/selfupdate_test.go:60-62` checks stdout and stderr concatenated. "The command is on stderr" is not strictly asserted. The same applies to the "command cannot start" scenario.
- **W-6**: the `go install` users see no banner, as described under follow-up 1.

Also recorded in verify: W-7, the size is 801 lines against the 800-line budget. This is one line over budget, from the CRITICAL-1 test row.

---

## Resolved Before Archive

- **CRITICAL-1**: scenario `Pre-release tag never triggers a notice` had no test. Fixed with the row `{name: "pre-release latest never nags", current: "1.0.0", latest: "v2.0.0-rc1", want: false}` in `internal/update/hasnewer_test.go`.
- **W-1**: design, proposal and state.yaml described ADR-7 as in scope. The cut is now recorded as an amendment.
- **W-5**: the commit tasks (1.7, 3.3, 4.6, 5.3, 6.4) were unchecked because commits were deferred. The commits are now made and the checkboxes are `[x]`. No stale-checkbox reconciliation was used.
- **ADR-5 Windows deviation**: the delta has the Windows print-only scenario.

---

## Known Gap: Archive Folder Naming

`openspec/changes/archive/` holds undated folder names (for example `ai-pr-summary`, `config-validation-wiring`), while `openspec-convention.md` requires `YYYY-MM-DD-{change-name}`. `config-validation-wiring/archive-report.md` names a dated path that does not exist on disk. This change follows the convention and uses the dated name. The existing entries are not renamed, because the archive is an audit trail. Recorded here as a known inconsistency for a future cleanup decision.

---

## Verification Evidence

From `verify-report.md`, all exited 0:

- `go vet ./...`, `gofmt -l .` (no files listed), `go build ./...`
- `go test ./... -short -race`: 15 packages ok
- `go test ./internal/app/... ./internal/update/... ./cmd/... -count=1`: ok
- `internal/app/boundary_test.go` unchanged and passing

Spec coverage: 35 scenarios across 13 requirements at verify time. 30 compliant, 4 partial (W-2, W-3, W-4 x2), 1 untested and now fixed (CRITICAL-1).

---

## Artifact Traceability

- Delta specs: `openspec/changes/archive/2026-11-05-tui-self-update/specs/update-notification/spec.md` and `.../specs/tui-self-update/spec.md`
- Verify report: `openspec/changes/archive/2026-11-05-tui-self-update/verify-report.md`
- Tasks: `openspec/changes/archive/2026-11-05-tui-self-update/tasks.md`

---

## SDD Cycle Complete

Planned, specified, designed, implemented in six commits, verified with warnings, and archived. The living specs now reflect the new behavior.

**Archived to**: `openspec/changes/archive/2026-11-05-tui-self-update/`
