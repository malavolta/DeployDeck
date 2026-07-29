# Tasks: Standalone Modes — Delta & Validation Without Full Promotion (HU-018)

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | ~900-1400 (prod ~300-400 across 6 files; tests ~600-1000 across 5 files, 2 new) |
| 400-line budget risk | High |
| Chained PRs recommended | Yes |
| Suggested split | PR1 run-persistence → PR2 menu+migration → PR3 standalone delta → PR4 standalone validation → PR5 e2e+docs |
| Delivery strategy | ask-on-risk (default; not overridden by preflight) |
| Chain strategy | pending — ask user |

Decision needed before apply: Yes
Chained PRs recommended: Yes
Chain strategy: pending
400-line budget risk: High

### Suggested Work Units

| Unit | Goal | Likely PR | Focused test command | Runtime harness | Rollback boundary |
|------|------|-----------|----------------------|-----------------|-------------------|
| 1 | `Record.Mode`/`ManifestPath` additive growth | PR 1 | `go test ./internal/runs/... -run TestRecord_Mode` | N/A — pure unit, no subprocess | Revert 2 struct fields; zero-value-safe, nothing else depends on it yet |
| 2 | `StateMainMenu` + entry-point migration (highest risk) | PR 2 | `go test ./internal/app/... -run 'MainMenu\|OnResumeDetect\|PrereqCheck\|RunHistory'` | `go test ./internal/app/...` (full suite must stay green) | Revert the 5 landing/guard edits + drop menu view/keys/consts — additive, no persisted-state coupling |
| 3 | Standalone delta | PR 3 | `go test ./internal/app/... -run StandaloneDelta` | temp-git + real `sf sgd` (`internal/delta/generate_e2e_test.go` pattern), `-short`-skippable | Revert `StateDeltaSourceSelect` wiring + delta branch in `keyPackageReview`/`onDeltaDone`; independent of validate |
| 4 | Standalone validation | PR 4 | `go test ./internal/app/... -run StandaloneValidate` | fake `sf` (`salesforce.New(fakeRunner)`), no real org | Revert `StatePackageSelect`/`StateSandboxSelect` wiring; independent of delta |
| 5 | Consolidated e2e + docs | PR 5 | `go test ./internal/app/... -run E2E` | temp-git+sgd (delta) + fake sf (validate) + real-git resume regression, `-short`-skippable | Revert the new e2e test file + docs edits only |

For `feature-branch-chain` (if chosen): PR2 base = tracker branch (`standalone-modes`); PR3/PR4 base = PR2 branch; PR5 base = PR3+PR4 merge point.

## Group 1 — `run-persistence` (pure-unit, foundational)

- [x] 1.1 RED `internal/runs/writer_test.go`: add `TestRecord_ModeAndManifestPath_RoundTripThroughWriteReload` (Save `Record{Mode:"delta",ManifestPath:"pkg/package.xml"}`, Load, both round-trip) and `TestRecord_BackwardCompat_PriorRunJSONLoadsWithZeroModeAndManifestPath` (raw pre-HU-018 `run.json` with no `mode`/`manifestPath` keys loads clean, both `""`) — compile-fails (fields undefined). AC: run-persistence "Mode Recorded On The Run" (delta/validate/round-trip/backward-compat scenarios).
- [x] 1.2 GREEN `internal/runs/writer.go`: append `Mode string \`json:"mode,omitempty"\`` + `ManifestPath string \`json:"manifestPath,omitempty"\`` to `Record` (after `QuickDeployedAt`, additive, no `SchemaVersion` bump). Gate: `go test ./internal/runs/...` + `go vet ./internal/runs/...` green.

## Group 2 — `StateMainMenu` + entry-point migration (HIGHEST RISK — biggest review budget)

- [ ] 2.1 RED `internal/app/main_menu_test.go` (new): `TestVisibleMenuEntries_FiltersUnimplemented` — table entries mixing `implemented:true/false`; filtered slice keeps only `true`, preserves order. Compile-fails. AC: "Unimplemented Modes Hidden From The Menu".
- [ ] 2.2 GREEN `internal/app/app.go`: append (AFTER `StateQuickDeploy`, never renumber prior) `StateMainMenu`, `StateDeltaSourceSelect`, `StatePackageSelect`, `StateSandboxSelect`; add `Model` fields `menuCursor int`, `standaloneMode string`, `branchList []git.Branch`, `branchCursor int`, `packagePath string`, `sandboxList []string`, `sandboxCursor int`; add `menuEntry{label string; target State; implemented bool}`, package var `menuEntries` (3 entries → `StateTicketInput`/`StateDeltaSourceSelect`/`StatePackageSelect`, all `true`), `visibleMenuEntries(entries []menuEntry) []menuEntry`.
- [ ] 2.3 RED `internal/app/main_menu_test.go`: `TestModel_MainMenu_CursorAndEnterRouting` — up/down clamp `menuCursor` in `[0,len-1]`; `enter` on cursor 0/1/2 routes `StateTicketInput` / `StateDeltaSourceSelect`+`standaloneMode="delta"` / `StatePackageSelect`+`standaloneMode="validate"`. AC: "Main Menu As Post-Prereq Landing", "Promote entry enters the unchanged full flow".
- [ ] 2.4 GREEN `internal/app/keys.go`: add `keyMainMenu`; wire `case StateMainMenu:` in `handleKey`.
- [ ] 2.5 RED `internal/app/main_menu_test.go`: `TestModel_MainMenu_QEscQuits` — `q`/`esc` cmd yields `tea.QuitMsg`.
- [ ] 2.6 GREEN `internal/app/keys.go`: `keyMainMenu`'s `q`/`esc` → `m.quitCmd()`.
- [ ] 2.7 RED `internal/app/main_menu_test.go`: `TestViewMainMenu_RendersEntriesAndCursor` — `View()` on `StateMainMenu` shows 3 labels + cursor marker + footer `q salir`.
- [ ] 2.8 GREEN `internal/app/view.go`: add `viewMainMenu()`; wire `case StateMainMenu:` in `viewBody`.
- [ ] 2.9 RED migrate: `internal/app/prereq_test.go:27` (`TestModel_PrereqCheck_To_TicketInput` "all OK" case) `wantState` → `StateMainMenu`; `internal/app/original_branch_test.go:104-105` assertion → `StateMainMenu`; `internal/app/resume_test.go:385` (`TestResume_RealInProgressCherryPick_RoutesToConflict`, real-git, `-short`-skip) → `StateMainMenu`. All 3 FAIL against current `onPrereqDone`. AC: "No resumable run shows the menu".
- [ ] 2.10 GREEN `internal/app/update.go:123`: `onPrereqDone` landing `m.state = StateTicketInput` → `StateMainMenu`.
- [ ] 2.11 RED migrate `internal/app/prereq_test.go:82` (`TestModel_PrereqScreen_ContinueBlockedByBlocker`, "warnings-only, c continues") assertion → `StateMainMenu`. FAILS against current `keyPrereq`. AC: same requirement — **the 4th landing site the design missed**.
- [ ] 2.12 GREEN `internal/app/keys.go:392`: `keyPrereq`'s `c` branch `m.state = StateTicketInput` → `StateMainMenu`.
- [ ] 2.13 RED (BLOCKER-2 batch A, "FAIL loudly"): new `internal/app/main_menu_test.go` `TestOnResumeDetect_GuardUsesMainMenu` (seed `m.state=StateMainMenu` + resumable record → routes `StateRunHistory` pre-selected). PLUS migrate `internal/app/resume_test.go`: `TestOnResumeDetect_RoutesToRunHistoryWhenResumable` subtests setup `:107`,`:128` → `StateMainMenu`; `TestOnResumeDetect_ResyncsStaleConflictRecord` setup `:183` + assertion `:188` → `StateMainMenu`; `TestResume_Resync_ExternallyResolved` (real-git) assertion `:435` → `StateMainMenu`. All FAIL against the current guard. AC: "Resume-Detection Preserved After Menu Landing".
- [ ] 2.14 GREEN `internal/app/update.go:269,287`: guard `!= StateTicketInput` → `!= StateMainMenu`; no-resume fallthrough → `m.state = StateMainMenu`.
- [ ] 2.15 RED→GREEN test-only (BLOCKER-2 batch B, silent-pass repair — own task, no production change): `internal/app/resume_test.go` same outer test — "nothing resumable" subtest (`:144` setup, `:150` assertion) and "detection error" subtest (`:157` setup, `:159` assertion): migrate `StateTicketInput`→`StateMainMenu`; STRENGTHEN "nothing resumable" with `len(nm.runs)==1` (only true if `reconcileStaleRuns` actually ran, not a guard no-op coincidence); "detection error" is inherently outcome-identical gated-vs-rejected (early-returns before touching any field) — add a code comment noting its guard-correctness is covered by the sibling subtests + 2.13's regression test, not fabricate a false assertion. Verify by locally reverting 2.14: only the strengthened assertion must now fail loudly.
- [ ] 2.16 RED migrate `internal/app/run_history_test.go:176-177` + `internal/app/run_history_e2e_test.go:89-90` (q/esc decline) assertions → `StateMainMenu`. FAILS against current `keyRunHistory`. AC: ADR-1 resume-decline refinement.
- [ ] 2.17 GREEN `internal/app/keys.go:808`: `keyRunHistory`'s `q`/`esc` `m.state = StateTicketInput` → `StateMainMenu`.
- [ ] 2.18 RED migrate `internal/app/flow_e2e_test.go:136` + `internal/app/promotion_real_metadata_e2e_test.go:43`: assertion → `StateMainMenu`; insert one `m = advance(t, m, keyPress("enter"))` (cursor defaults to entry 0) before re-asserting `StateTicketInput`; rest of each e2e unchanged.
- [ ] 2.19 GREEN — no new production code expected (2.4+2.10 already cover it); confirms wiring, fix only if a gap surfaces.
- [ ] 2.20 Sweep: grep `StateTicketInput` across `internal/app/*_test.go`; confirm every remaining hit is a direct `state:`/`m.state =` literal setup (ADR-5-exempt), not a post-prereq/post-decline assertion.
- [ ] 2.21 Gate: `go test ./internal/app/... ./internal/runs/...` + `go vet ./...` — FULL suite green before Group 3 starts.

## Group 3 — Standalone delta

- [ ] 3.1 RED `internal/app/standalone_delta_test.go` (new): `TestStandaloneBranchesCmd_ComposesListBranches` — fake `Git` canned for `ListBranches` → `standaloneBranchesCmd()` returns `standaloneBranchesMsg{branches:[...]}`. Compile-fails.
- [ ] 3.2 GREEN `internal/app/commands.go`: add `standaloneBranchesMsg{branches []git.Branch; err error}` + `standaloneBranchesCmd()` (mirrors `onDeployBranches`'s async pattern).
- [ ] 3.3 RED `internal/app/standalone_delta_test.go`: `TestModel_DeltaSourceSelect_CursorPickAndNormalize` — `enter` on `{Name:"origin/main",Remote:true}` strips `origin/` → `plan.TargetBranch=="main"`, `plan.Ticket=="standalone"`, state → `StateDeltaGeneration`, fires `deltaCmd`. AC: "Standalone Delta Generates A Package Without Cherry-Picks".
- [ ] 3.4 GREEN `internal/app/keys.go` (`keyDeltaSourceSelect` + menu delta-branch fires `standaloneBranchesCmd`), `internal/app/update.go` (`onStandaloneBranches` stores `m.branchList`), `internal/app/view.go` (`viewDeltaSourceSelect`); wire dispatch/switch cases.
- [ ] 3.5 RED `internal/app/standalone_delta_test.go`: `TestModel_PackageReview_DeltaModeStopsAfterSummary` — `standaloneMode="delta"`: `enter` no-ops (stays `StatePackageReview`, no `queueCmd`/`StateQueueReview`); `e` no-ops (neutralized — standalone delta never ran commit selection); footer omits "Enter validar".
- [ ] 3.6 GREEN `internal/app/keys.go`: `keyPackageReview`/`confirmPackageReview` mode-aware branch; `internal/app/view.go` `viewPackageReview` footer branch.
- [ ] 3.7 RED `internal/app/standalone_delta_test.go`: `TestOnDeltaDone_StandaloneDelta_SavesRunModeDelta` — `standaloneMode="delta"`, real `t.TempDir()` writer → after `onDeltaDone`, `writer.List()` has a run `Mode=="delta"`, `ManifestPath==result.PackageXMLPath`, `JobID==""`. AC: "Standalone Modes Create A Local Run" (delta scenario).
- [ ] 3.8 GREEN `internal/app/update.go`: `onDeltaDone`, after `RegisterDeltaArtifacts`, when `standaloneMode=="delta"`: `Writer.Save(runs.Record{RunID:"delta-"+target+"-"+m.now().Format("20060102150405"), Mode:"delta", ManifestPath:...})`.
- [ ] 3.9 RED→already-GREEN (pure-reuse pin) `internal/app/standalone_delta_test.go`: `TestModel_StandaloneDelta_EmptyDelta_ReusesWarning` — `summary.Empty=true` → existing "Package vacio" warning renders unchanged. AC: "Empty delta reuses the existing warning".
- [ ] 3.10 Gate: `go test ./internal/app/...` + `go vet ./...` green.

## Group 4 — Standalone validation

- [ ] 4.1 RED `internal/app/standalone_validate_test.go` (new): `TestModel_PackageSelect_ValidPath_AdvancesToSandboxSelect` — real `t.TempDir()` valid `package.xml`, `enter` → pre-check passes, `packagePath` set, state → `StateSandboxSelect`, `sandboxList` from `cfg.Sandboxes`.
- [ ] 4.2 GREEN `internal/app/keys.go` (`keyPackageSelect`, `enter` runs `parsePackageFile(path,false)`), `internal/app/view.go` (`viewPackageSelect`); wire dispatch/switch; menu validate-branch lands here.
- [ ] 4.3 RED `internal/app/standalone_validate_test.go`: `TestModel_PackageSelect_NonexistentOrInvalidPackage_BlocksAdvance` — table {nonexistent, malformed-XML} → stays `StatePackageSelect`, actionable `m.notice`, no state change. AC: "Invalid Or Nonexistent Package Rejected Before Launch".
- [ ] 4.4 GREEN — covered by 4.2's error branch; confirm passes.
- [ ] 4.5 RED `internal/app/standalone_validate_test.go`: `TestModel_SandboxSelect_ConfirmPreCreatesRunAndFiresValidateCmd` — real writer, `enter` → minimal plan `{PackageXMLPath,SandboxAlias,TestLevel}`; `writer.List()` shows a pre-created run `Mode=="validate"`, `ManifestPath` set, `JobID==""`; `m.runID` set; state → `StateValidationStart`; returns `validateCmd`. AC: "Standalone Validation Launches And Polls Like The Full Flow", "Standalone Modes Create A Local Run" (validate).
- [ ] 4.6 GREEN `internal/app/keys.go`: `keySandboxSelect` — nav+`enter` sets minimal plan, `Writer.Save(runs.Record{RunID:"validate-"+alias+"-"+ts, Mode:"validate", ManifestPath, Alias, TestLevel})`, sets `m.runID`, state `StateValidationStart`, returns `m.validateCmd()`; `internal/app/view.go` `viewSandboxSelect`; wire dispatch/switch.
- [ ] 4.7 RED `internal/app/standalone_validate_test.go`: `TestValidateCmd_StandaloneValidate_MergesJobIdOntoPreCreatedRecord` — pre-created `Mode="validate"` record + `m.runID` set, fake `SF.ValidateDeploy` success → `validateCmd()`'s existing `runID!=""` reuse branch (`commands.go:748-764`, UNCHANGED) merges `JobID`; `writer.Load` shows `Mode`/`ManifestPath` survive the round-trip.
- [ ] 4.8 Gate: `go test ./internal/app/...` + `go vet ./...` green.

## Group 5 — Consolidated HU-018 e2e

- [ ] 5.1 RED `internal/app/standalone_modes_e2e_test.go` (new): `TestE2E_MenuRouting_AllThreeEntriesReachable` — prereqDoneMsg → `StateMainMenu`; each cursor position → expected next state.
- [ ] 5.2 GREEN — proof task; fix only wiring gaps surfaced at full-flow level.
- [ ] 5.3 RED `internal/app/standalone_modes_e2e_test.go`: `TestE2E_StandaloneDelta_TempGitAndRealSgd` — `-short`-skip; temp git (mirrors `internal/delta/generate_e2e_test.go`), real `sf sgd` via `delta.New(execpkg.NewOSRunner())`; menu→delta→base pick→summary; assert `package.xml` under `.deploydeck/manifest/`, no `CHERRY_PICK_HEAD`, run `Mode=="delta"`. AC: "Delta mode generates and summarizes the package" (HU-018 Test E2E).
- [ ] 5.4 GREEN — proof/fix.
- [ ] 5.5 RED `internal/app/standalone_modes_e2e_test.go`: `TestE2E_StandaloneValidate_FakeSFPollsToTerminal` — real `package.xml` on disk, `salesforce.New(fakeRunner)` for `deploy validate --async`+`deploy report`; menu→validate→path→sandbox; assert `jobID` captured, poll reaches terminal, run `Mode=="validate"`. AC: "Validation mode launches and polls to terminal" (HU-018 Test E2E).
- [ ] 5.6 GREEN — proof/fix.
- [ ] 5.7 RED: `TestE2E_StandaloneValidate_InvalidPackage_ActionableErrorNoLaunch` — nonexistent path → stays `StatePackageSelect`, notice shown, `SF.ValidateDeploy` never invoked (fake fails test if called).
- [ ] 5.8 GREEN — proof/fix.
- [ ] 5.9 RED: `TestE2E_StandaloneDelta_EmptyDelta_WarningVariant` — temp git, base==current ref (no diff) → summary shows empty-delta warning, run still created `Mode=="delta"`.
- [ ] 5.10 GREEN — proof/fix.
- [ ] 5.11 RED: `TestE2E_ResumeDetection_StillOfferedAfterMenuLanding_Regression` — real temp git in-progress cherry-pick (`setupConflictRepo2`/`driveToCherryPicking` helpers) + matching `run.json` → fresh `New()`→prereq→resumeDetect → `StateRunHistory` pre-selected (full-stack proof of 2.13/2.14).
- [ ] 5.12 GREEN — proof/fix; a failure here signals a Group 2 wiring gap unit tests missed.
- [ ] 5.13 Gate: `go test ./internal/app/... -short` (fast) AND full `go test ./internal/app/...` (includes real-git/sgd) + `go vet ./...` green.

## Group 6 — Docs housekeeping

- [ ] 6.1 `docs/HISTORIAS.md`: mark HU-018's implemented AC items (menu, standalone delta, standalone validation, invalid-package/empty-delta variants, resume-regression guard); leave existing "Fase Futuro" marker as-is — no fabricated status beyond what's true.
- [ ] 6.2 Check README/`docs/ACTIVATION-CHECKLIST.md` for an HU-018-status reference; update if present, else skip-with-rationale (record "N/A — no such reference exists") per prior-slice policy.
