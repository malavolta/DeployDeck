# Apply Progress: Delta + Validation (HU-007, 008, 010, 011)

**Batch**: 3 of 3, FINAL (Phases 7-9 this batch; Phases 1-3 in batch 1, 4-6 in batch 2)
**Mode**: Strict TDD
**Delivery**: single-pr under `size:exception` (maintainer-approved, per tasks.md's Review Workload Forecast)
**Branch**: `delta-validation`

## Scope Covered So Far

Phase 1 (Foundations: config/plan/changed-files), Phase 2 (HU-007
`internal/delta` delta generation), Phase 3 (HU-008 `Summarize`
package-summary) — batch 1. Phase 4 (HU-010 `salesforce.ValidateDeploy`),
Phase 5 (`internal/runs` minimal writer), Phase 6 (HU-011
`salesforce.ReportDeploy` + `IsTerminal`) — batch 2. Phase 7 (`internal/app`
state-machine wiring past `StatePickVerification`), Phase 8 (real e2e:
multi-`--source-dir` real sgd + opt-in `[E2E-ORG]` validate+report), Phase 9
(final verification) — batch 3 (this FINAL batch). **All 53 tasks.md items
are now complete.**

## Completed Tasks

### Phase 1: Foundations — config, plan, changed-files helper
- [x] 1.1 RED `internal/config/config_test.go`
- [x] 1.2 GREEN `config.Config` additions + `load.go` defaults
- [x] 1.3 RED `internal/config/validate_test.go` additions
- [x] 1.4 GREEN `validate.go` rules
- [x] 1.5 RED `internal/git/deployment_plan_test.go` additions
- [x] 1.6 GREEN `RegisterDeltaArtifacts` + plan fields
- [x] 1.7 RED `internal/git/changed_files_test.go`
- [x] 1.8 GREEN `ChangedFiles`

### Phase 2: HU-007 `internal/delta` (delta-generation)
- [x] 2.1 RED `internal/delta/service_test.go` (arg composition table)
- [x] 2.2 GREEN `Request`/`Result`/`Generate` arg composition
- [x] 2.3 RED artifact discovery table
- [x] 2.4 GREEN artifact discovery
- [x] 2.5 RED sgd failure path
- [x] 2.6 GREEN failure path
- [x] 2.7 `[I]` real-sgd integration on temp repo seeded from `test-e2e-org`

### Phase 3: HU-008 `Summarize` (package-summary)
- [x] 3.1 RED `internal/delta/package_test.go`
- [x] 3.2 GREEN `ParsePackage`/`ParseDestructive`
- [x] 3.3 RED `internal/delta/summarize_test.go` (table-driven)
- [x] 3.4 GREEN `Summarize` → `PackageSummary`

### Phase 4: HU-010 `salesforce.ValidateDeploy` (deploy-validation)
- [x] 4.1 RED `internal/salesforce/validate_test.go`: arg-composition table (no destructive, with destructive, `RunSpecifiedTests`, non-`RunSpecifiedTests` with `Tests` set ignored)
- [x] 4.2 GREEN `ValidateRequest`/`ValidateResult`/`ValidateDeploy` in `validate.go`, added to `Client` interface
- [x] 4.3 RED CLI error path: decoded-message case + unparseable-raw-fallback case + runner-start-failure case
- [x] 4.4 GREEN error path (`validateErrorMessage` helper + `Raw` always preserved on the returned `ValidateResult`)

### Phase 5: `internal/runs` minimal writer (run-persistence)
- [x] 5.1 RED `internal/runs/writer_test.go`: `Create` writes `run.json`+`validate.json`, forces `SchemaVersion:1`
- [x] 5.2 GREEN `Record`/`NewWriter`/`Create`
- [x] 5.3 RED `AppendReport`: numbered `report-<NNN>.json` never overwritten, `run.json` status/`UpdatedAt` updated, unknown `runID` errors
- [x] 5.4 GREEN `AppendReport` with directory-scan-based `NNN` counter (stateless across `Writer` instances)

### Phase 6: HU-011 `salesforce.ReportDeploy` (validation-progress, CLI half)
- [x] 6.1 RED `internal/salesforce/report_test.go`: arg composition, `ComponentFailures`/`TestFailures` parsing, CLI error surfacing
- [x] 6.2 GREEN `ReportDeploy` in `report.go`, added to `Client` interface
- [x] 6.3 RED `IsTerminal` table (8 cases: 4 terminal, 4 non-terminal incl. empty string)
- [x] 6.4 GREEN `IsTerminal`

### Phase 7: `internal/app` wiring (state machine)
- [x] 7.1 RED PickVerification confirm gated by `Model.DeltaAllowed()`
- [x] 7.2 GREEN `State` consts `DeltaGeneration..Canceled`, `Deps.{Delta,Runs,Now}`, gate
- [x] 7.3 RED DeltaGeneration → `deltaCmd` → PackageReview; sgd failure keeps user + Raw, no validation
- [x] 7.4 GREEN `deltaCmd` + transition
- [x] 7.5 RED PackageReview blocks confirm while `summary.Empty` until `emptyConfirmed`
- [x] 7.6 GREEN `emptyConfirmed` flag + `o` override gate
- [x] 7.7 RED PackageReview confirm → QueueReview (inert) → ValidationStart same tick
- [x] 7.8 GREEN inert pass-through
- [x] 7.9 RED ValidationStart → `ValidateDeploy` → jobId → `runs.Create` → Polling; CLI error stays alive
- [x] 7.10 GREEN `validateCmd` + immediate persistence
- [x] 7.11 RED Polling: `tea.Tick` single `ReportDeploy`/tick; injected `Now` deadline; timeout→Failed; transient retry; each report → `AppendReport`
- [x] 7.12 GREEN `reportCmd`/`onPollTick`/`onReportDone` loop
- [x] 7.13 RED terminal mapping {Succeeded,SucceededPartial}→Succeeded, Failed→Failed, Canceled→Canceled; stops
- [x] 7.14 GREEN terminal mapping
- [x] 7.15 RED user exit during Polling leaves job active/resumable — no cancel issued
- [x] 7.16 `boundary_test.go` (app never execs directly) still passes unmodified

### Phase 8: Real e2e
- [x] 8.1 `[I]` real-sgd multi-`--source-dir` merge (new) + destructive path (batch-1 `generate_e2e_test.go`)
- [x] 8.2 `[E2E-ORG]` `internal/salesforce/real_org_e2e_test.go`: non-destructive async CheckOnly validate + report poll-to-terminal vs `AM-DEV-EDITION`

### Phase 9: Final verification
- [x] 9.1 `go test -race ./...` green
- [x] 9.2 `go vet ./...` clean
- [x] 9.3 `gofmt -l .` empty
- [x] 9.4 `proposal.md` Success Criteria checked off

**53/53 tasks.md items complete** (all phases done and green).

## Files Changed

### Batch 1 (Phases 1-3)

| File | Action | What Was Done |
|---|---|---|
| `internal/config/config.go` | Modified | Added `DeltaConfig`, `Config.Delta`, `Config.PollIntervalSeconds`, `Config.PollTimeoutSeconds`, and `Default{PollIntervalSeconds,PollTimeoutSeconds,DeltaOutputDir}` constants |
| `internal/config/load.go` | Modified | `applyDefaults` now defaults poll seconds unconditionally, and `Delta.OutputDir` only once `Delta.SourceDirs` is non-empty (see Deviations) |
| `internal/config/validate.go` | Modified | `Validate()` rejects `pollIntervalSeconds<=0`, `pollTimeoutSeconds<=0`, and any `Delta` field set with empty `SourceDirs` (`deltaConfigured` helper) |
| `internal/config/config_test.go` | Created | `Load()` delta-field round-trip + poll/outputDir default-vs-override table, each asserting `Validate()` still passes |
| `internal/config/validate_test.go` | Modified | Extended `validConfig()` baseline + 5 new table cases for the new rules |
| `internal/git/deployment_plan.go` | Modified | Added `PackageXMLPath`/`DestructiveChangesPath` fields + `RegisterDeltaArtifacts` (mirrors `RegisterPromotionBranch`) |
| `internal/git/deployment_plan_test.go` | Modified | 2 new tests: field-set + preserve-other-fields, optional destructive path |
| `internal/git/changed_files.go` | Created | `Service.ChangedFiles(ctx, dir, from, to)`: resolves repo root then delegates to the existing `diffNameOnly` helper |
| `internal/git/changed_files_test.go` | Created | 3 `FakeRunner`-based tests: wraps diff, empty diff, propagates `RepoRoot` error |
| `internal/git/promotion_branch_e2e_test.go` | Modified | Fixed a pre-existing e2e fixture broken by the new `Validate()` poll-seconds rule (see Deviations) |
| `internal/delta/service.go` | Created | `Request`/`Result`/`Service`/`New`/`Generate`: composes ONE `sf sgd source delta` invocation, runs it, discovers artifacts, wraps failures with `Raw` |
| `internal/delta/service_test.go` | Created | Arg-composition table (3 cases), artifact-discovery table (2 cases), sgd-failure test |
| `internal/delta/package.go` | Created | `Package`/`PackageType`/`MetadataTypeSummary`/`PackageSummary`, `ParsePackage`/`ParseDestructive` (`encoding/xml`), `Summarize` |
| `internal/delta/package_test.go` | Created | 5 tests: types+members parse, empty package, invalid XML (both parsers) |
| `internal/delta/summarize_test.go` | Created | 7-case table: normal, empty, destructive-separation, sensitive (additive + destructive-only), outside-sourceDirs, exact-match-not-outside |
| `internal/delta/generate_e2e_test.go` | Created | `[I]` real-sgd integration: seeds a temp repo from `test-e2e-org`, edits+deletes real Apex classes, runs the real `sf sgd source delta`, asserts parsed artifacts + clean working tree |

### Batch 2 (Phases 4-6, this batch)

| File | Action | What Was Done |
|---|---|---|
| `internal/salesforce/client.go` | Modified | Added `ValidateDeploy` and `ReportDeploy` to the `Client` interface (`New`/`client`/`decodeEnvelope` unchanged) |
| `internal/salesforce/validate.go` | Created | `ValidateRequest`/`ValidateResult`/`ValidateDeploy`: composes `sf project deploy validate --async --json`, conditional `--post-destructive-changes` + repeated `--tests` (`RunSpecifiedTests` only), `validateErrorMessage` (decoded-message-or-raw-fallback), private `combineOutput` |
| `internal/salesforce/validate_test.go` | Created | Arg-composition table (4 cases), JobID-from-envelope test, decoded-CLI-error test, unparseable-CLI-error test, runner-start-failure test |
| `internal/salesforce/report.go` | Created | `ComponentFailure`/`TestFailure`/`DeployReport`/`reportResultEnvelope`/`ReportDeploy`/`IsTerminal`/`terminalStatuses` |
| `internal/salesforce/report_test.go` | Created | Arg-composition + counters test, `ComponentFailures` parse test, `TestFailures` parse test, CLI-error test, `IsTerminal` 8-case table |
| `internal/runs/writer.go` | Created | `Record`/`SchemaVersion1`/`Writer`/`NewWriter`/`Create`/`AppendReport`/`readRecord`/`nextReportNumber`/`writeJSON` |
| `internal/runs/writer_test.go` | Created | `Create` round-trip + forced-schema-version + directory-creation tests; `AppendReport` numbered-report + status/UpdatedAt-update + unknown-runID-errors tests |

### Batch 3 (Phases 7-9, this FINAL batch)

| File | Action | What Was Done |
|---|---|---|
| `internal/app/app.go` | Modified | Added `State` consts `StateDeltaGeneration..StateCanceled`; `Deps.{Delta *delta.Service, Runs *runs.Writer, Now func()time.Time}`; Model fields (`deltaResult`/`summary`/`deltaErr`/`emptyConfirmed`/`jobID`/`runID`/`runDir`/`report`/`validateErr`/`reportErr`/`pollDeadline`/`timedOut`); helpers `now`/`pollIntervalSeconds`/`pollTimeoutSeconds`/`terminalState`/`deltaBaseDir` |
| `internal/app/commands.go` | Modified | Added `deltaDoneMsg`/`validateDoneMsg`/`reportDoneMsg`/`pollTickMsg`; `deltaCmd` (Generate+parse+ChangedFiles+Summarize), `parsePackageFile`, `validateCmd` (ValidateDeploy+immediate `runs.Create`), `reportCmd` (per-call `context.WithTimeout`), `pollTickCmd` |
| `internal/app/update.go` | Modified | Wired the 4 new msg cases; `onDeltaDone`/`onValidateDone`/`onReportDone`/`onPollTick` (deadline enforcement, transient retry, `AppendReport` each poll, terminal mapping) |
| `internal/app/keys.go` | Modified | Routing for the 5 new states; `keyVerification` enter now gates on `DeltaAllowed` → DeltaGeneration; `keyDeltaGeneration`/`keyPackageReview`(+`confirmPackageReview` inert pass-through + `o` override)/`keyValidationStart`/`keyValidationPolling` (`q` leaves job active, `r` manual refresh) |
| `internal/app/view.go` | Modified | `viewDeltaGeneration`/`viewPackageReview`/`viewValidationStart`/`viewValidationPolling`/`viewValidationResult`/`validationBody` — surface per-type/destructive/sensitive/outside-dir summary, sgd raw on failure, live component/test counts, metadata errors (component/type/message), failed tests (class/method/message), timeout note |
| `internal/app/delta_validation_test.go` | Created | 15 `Model.Update`/`FakeRunner`/injected-clock tests covering tasks 7.1-7.15 + a `deltaCmd` composition test + a live/terminal view test |
| `cmd/deploydeck/main.go` | Modified | `defaultRunTUI` now constructs `Delta: delta.New(runner)` + `Runs: runs.NewWriter(dir)` (Now left nil → real clock) |
| `internal/delta/multi_source_dir_e2e_test.go` | Created | `[I]` real-sgd multi-`--source-dir` merge: two SFDX package dirs, one Apex class each, both edited; asserts ONE merged `package.xml` lists members from BOTH dirs |
| `internal/salesforce/real_org_e2e_test.go` | Created | `[E2E-ORG]` opt-in (`DEPLOYDECK_E2E_ORG`) non-destructive async CheckOnly validate + report poll-to-terminal vs `AM-DEV-EDITION`; asserts jobId, immediate persistence, terminal state, and report-shape parsing (fail-on-mismatch) |
| `internal/salesforce/report.go` | Modified | Doc-only: `reportResultEnvelope` comment now records the shape was confirmed verbatim against real `sf` CLI 2.135.7 output (resolves batch-2 deviation 7) |

## TDD Cycle Evidence

| Task | RED (failing test first) | GREEN (implementation) | REFACTOR |
|---|---|---|---|
| 1.1/1.2 | `go test ./internal/config/...` → `PollIntervalSeconds = 0, want 10` etc. (confirmed) | `load.go` defaults added; suite green | Doc comments only |
| 1.3/1.4 | `go test -run TestConfig_Validate` → 4 new subtests FAIL (`expected error, got nil`) (confirmed) | `validate.go` rules added; all 9 subtests green | `deltaConfigured` extracted as a named helper |
| 1.5/1.6 | `go test -run TestRegisterDeltaArtifacts -short` → `undefined: git.RegisterDeltaArtifacts` build failure (confirmed) | Fields + function added; green | none |
| 1.7/1.8 | `go test -run TestService_ChangedFiles -short` → `svc.ChangedFiles undefined` build failure (confirmed) | `changed_files.go` added; green | none |
| 2.1-2.6 | `go test ./internal/delta/... -short` → `no non-test Go files` build failure (confirmed) | `service.go` added incrementally (args → discovery → failure path) in one file; all subtests green | `buildArgs`/`combineOutput`/`discoverArtifacts` extracted as named helpers instead of one long `Generate` body |
| 2.7 | Test authored against the not-yet-existing `delta.ParsePackage`/`ParseDestructive` (Phase 3), confirmed failing to compile until Phase 3 landed | Passes against REAL `sf sgd source delta` (plugin installed), 3.12s | none |
| 3.1-3.4 | `go test ./internal/delta/... -short` → `undefined: delta.Package` etc. (confirmed, same run as 2.1-2.6's initial RED) | `package.go` added; all subtests green | `summarizeTypes`/`sensitiveTypesIn`/`outsideSourceDirs`/`underAnySourceDir` extracted as named helpers |
| 4.1/4.2 | `go test ./internal/salesforce/... -run ValidateDeploy` → `undefined: salesforce.ValidateRequest` build failure (confirmed) | `validate.go` added, `Client` interface extended; all 4 arg-composition subtests + JobID test green | none — kept as one `buildValidateArgs` function per design's Interfaces snippet |
| 4.3/4.4 | Same RED run also exercised the 3 error-path tests (decoded-message, unparseable-raw, runner-start-failure), all failing to compile alongside the rest | `validateErrorMessage` + `Raw`-preserving error return added; all 3 green | none |
| 5.1/5.2 | `go test ./internal/runs/...` → `no non-test Go files in .../internal/runs` build failure (confirmed) | `writer.go`'s `Record`/`NewWriter`/`Create`/`writeJSON` added; 3 `Create` tests green | none |
| 5.3/5.4 | `go test ./internal/runs/... -run AppendReport` → `w.AppendReport undefined` build failure (confirmed) | `AppendReport`/`readRecord`/`nextReportNumber` added; both `AppendReport` tests green | none |
| 6.1/6.2 | `go test ./internal/salesforce/... -run 'ReportDeploy\|IsTerminal'` → `client.ReportDeploy undefined` + `undefined: salesforce.IsTerminal` build failures (confirmed) | `report.go` added, `Client` interface extended; arg/counters/ComponentFailures/TestFailures/CLI-error tests green | `gofmt -w` re-aligned the `DeployReport{}` struct literal after adding fields (mechanical, no behavior change) |
| 6.3/6.4 | Same RED run also exercised `TestIsTerminal`'s 8 subtests, failing to compile alongside the rest | `terminalStatuses` map + `IsTerminal` added; all 8 subtests green | none |
| 7.1-7.15 | `go test ./internal/app/... -run 'DeltaValidation\|DeltaGeneration\|PackageReview\|Validation\|PickVerification_ConfirmGated\|DeltaCmd'` → compile failure `undefined: StateDeltaGeneration`, `undefined: deltaDoneMsg`, `m.summary undefined`, etc. (confirmed) | states/deps/messages/commands/handlers/views added; all 15 tests + subtests green | `pollTickMsg`/`pollTickCmd` kept distinct from cherry-pick `tickMsg`/`tickCmd` (different cadence, different command); `parsePackageFile` extracted as a shared helper |
| 7.16 | Boundary invariant — `internal/app` must import neither `os/exec` nor `internal/exec` even after importing `delta`+`runs` | `TestApp_NeverImportsExecSeam` re-run PASS unmodified: `delta`/`runs` are DIRECT imports, the seam is only transitive (allowed) | none |
| 8.1 | `go test ./internal/delta/... -run MultiSourceDir -count=1` → RED against not-yet-written seed helpers (compile) then real sgd | Passes against REAL `sf sgd source delta`, 2.92s: one merged `package.xml` carries AlphaService (pkg-a) + BetaService (pkg-b) | reused `runGit`/`memberOfType` from `generate_e2e_test.go` (same package) |
| 8.2 | Skips cleanly with no `DEPLOYDECK_E2E_ORG` (visible SKIP); RED-authored strong shape assertions | Passes vs `AM-DEV-EDITION`: jobId `0Affj...` captured, run persisted, polled `Pending → Failed` (3 component errors) to terminal, all 3 `componentFailures` parsed with component/type/message | none — the inferred shape matched real output, so no `report.go` tag change was needed |

## Work Unit Evidence

### Unit 1 — Foundations (config/plan/changed-files)
- Focused test: `go test ./internal/config/... ./internal/git/... -short` → `ok deploydeck/internal/config`, `ok deploydeck/internal/git`
- Runtime harness: N/A — no user-visible behavior yet (pure config/plan/diff-wrapper primitives), per tasks.md's own forecast
- Rollback boundary: revert `internal/config/{config,load,validate}.go` Delta/Poll additions + `internal/git/deployment_plan.go` additions + delete `internal/git/changed_files.go`(+test); no dependents yet

### Unit 2 — HU-007 `internal/delta` service
- Focused test: `go test ./internal/delta/... -short` → `ok deploydeck/internal/delta` (service tests only, real-sgd test self-skips under `-short`)
- Runtime harness: `go test ./internal/delta/... -run TestService_Generate_RealSgd_TempRepo -v` → `PASS (3.12s)`, real `sf sgd source delta` against a temp repo seeded from `test-e2e-org`
- Rollback boundary: delete `internal/delta/service.go`(+test)(+`generate_e2e_test.go`); no dependents yet

### Unit 3 — HU-008 `Summarize`
- Focused test: `go test ./internal/delta/... -run Summarize` → `ok deploydeck/internal/delta`
- Runtime harness: N/A — pure parser/summarizer, fixture-proven (matches tasks.md's own forecast)
- Rollback boundary: delete `internal/delta/package.go`(+test)(+`summarize_test.go`)

### Unit 4 — HU-010 `salesforce.ValidateDeploy`
- Focused test: `go test ./internal/salesforce/... -run ValidateDeploy -v` → `ok deploydeck/internal/salesforce` (10 subtests, all PASS)
- Runtime harness: `[E2E-ORG]` deferred to Phase 8 (opt-in, `DEPLOYDECK_E2E_ORG`) — N/A this batch, per tasks.md's own Suggested Work Units table for Unit 5
- Rollback boundary: delete `internal/salesforce/validate.go`(+test); revert the `ValidateDeploy` method from `Client` in `client.go`; no dependents yet (Phase 7 wiring not started)

### Unit 5 — `internal/runs` minimal writer
- Focused test: `go test ./internal/runs/... -v` → `ok deploydeck/internal/runs` (5 subtests, all PASS)
- Runtime harness: N/A — `t.TempDir` filesystem writer, per tasks.md's own forecast
- Rollback boundary: delete `internal/runs/` package entirely; no dependents yet (Phase 7 wiring not started)

### Unit 6 — HU-011 `salesforce.ReportDeploy`/`IsTerminal`
- Focused test: `go test ./internal/salesforce/... -run 'ReportDeploy|IsTerminal' -v` → `ok deploydeck/internal/salesforce` (13 subtests, all PASS)
- Runtime harness: `[E2E-ORG]` deferred to Phase 8 (opt-in, `DEPLOYDECK_E2E_ORG`) — N/A this batch, per tasks.md's own Suggested Work Units table for Unit 6
- Rollback boundary: delete `internal/salesforce/report.go`(+test); revert the `ReportDeploy` method from `Client` in `client.go`; no dependents yet (Phase 7 wiring not started)

### Unit 7 — `internal/app` state-machine wiring (Phase 7)
- Focused test: `go test ./internal/app/... -short` → `ok deploydeck/internal/app`; `-run '...ConfirmGated|DeltaGeneration|PackageReview|Validation|DeltaCmd|NeverImportsExecSeam' -v` → 16 top-level tests + subtests all PASS
- Runtime harness: the injected-clock + `FakeRunner` + real `runs.Writer(t.TempDir())` tests ARE the harness for the app half (design's "direct `Model.Update(msg)` + `FakeRunner`"); the real service composition is exercised by Unit 8's e2e. `boundary_test.go` re-confirms no direct exec.
- Rollback boundary: revert `internal/app/{app,commands,update,keys,view}.go` additions + delete `delta_validation_test.go` + revert `cmd/deploydeck/main.go`'s `Delta`/`Runs` deps; the flow falls back to stopping at `StatePickVerification` (foundation flow untouched)

### Unit 8 — Real e2e suite (Phase 8)
- Focused test: `go test ./internal/delta/... -run MultiSourceDir -v` → `PASS (2.92s)` real sgd; `DEPLOYDECK_E2E_ORG=AM-DEV-EDITION go test ./internal/salesforce/... -run TestE2ERealOrg_ValidateAndReport -v` → `PASS (11.87s)` real org
- Runtime harness: the tests ARE the harness — real `sf sgd source delta` (two `--source-dir`) and real non-destructive CheckOnly `sf project deploy validate --async` + `sf project deploy report` polled to a terminal `Failed` state
- Rollback boundary: delete the two e2e test files (`multi_source_dir_e2e_test.go`, `real_org_e2e_test.go`) + revert the `report.go` doc-comment note; no production code changes

## Deviations From Design

1. **`os.MkdirAll` before invoking sgd (undocumented in design.md).** A
   spike against the real CLI (`sf sgd source delta --output-dir out ...`
   with `out` not yet created) proved sgd hard-errors
   (`Error (2): Parsing --output-dir  No directory found at out`) unless
   `--output-dir` already exists. `Service.Generate` now creates
   `req.OutputDir` (`0o755`) before running sgd. This is a real
   precondition discovered during implementation, not a design choice —
   documented here since design.md's Interfaces/Contracts section didn't
   anticipate it.
2. **`Delta.OutputDir` Load-time default is conditional on `SourceDirs`,
   not unconditional as design.md's File Changes table literally states
   ("defaults (interval 10, timeout 3600, outputDir)").** Applying the
   outputDir default unconditionally made `Validate()`'s "any Delta field
   set requires non-empty SourceDirs" rule self-trigger on every config
   that never touches `delta:` at all — caught by two PRE-EXISTING
   HU-004/HU-005 e2e tests (`target_selection_e2e_test.go`,
   `promotion_branch_e2e_test.go`) that build a `Config` with no delta
   section and call `Validate()`. Fixed by making `Delta.OutputDir`
   default apply only once `Delta.SourceDirs` is already non-empty, so an
   entirely omitted delta section stays entirely empty post-`Load`, and
   `promotion_branch_e2e_test.go`'s config fixture was updated to set
   `PollIntervalSeconds`/`PollTimeoutSeconds` explicitly (it never went
   through `Load`, only a literal `config.Config{}`). No other
   `config.Config{}` literal in the codebase was affected (verified via
   `grep -rn "\.Validate()"`).
3. **`internal/delta.Service.Generate`'s error on sgd failure embeds `Raw`
   in the error message text** (via `fmt.Errorf`), rather than a typed
   error struct with a `Raw` field. This matches the codebase's existing
   convention (no typed error struct exists anywhere in `internal/`;
   `internal/salesforce` and `internal/git` both embed raw
   stdout/stderr text into `fmt.Errorf` messages the same way).
4. **`internal/delta/package.go` bundles both the parser (`ParsePackage`/
   `ParseDestructive`) and `Summarize`/`PackageSummary`** in one file,
   exactly matching design.md's File Changes table row for
   `internal/delta/package.go` — tests are still split into
   `package_test.go` (parser) and `summarize_test.go` (`Summarize`) per
   tasks.md's task-level file names.
5. **The batch instruction's Phase 6 prose mentioned "the poll loop" (fixed
   interval + hard timeout + context cancel + transient-error retry) as
   part of `internal/salesforce`'s scope; this was NOT implemented in
   `internal/salesforce` this batch.** `internal/salesforce/report.go`
   implements exactly `ReportDeploy` (one call, one decode) and
   `IsTerminal` — matching tasks.md's actual Phase 6 items (6.1-6.4, which
   list only these two) and design.md's own "polling driver" Architecture
   Decision, which explicitly REJECTS "blocking loop in `salesforce`" in
   favor of `tea.Tick` firing single `ReportDeploy` calls from
   `internal/app`'s `ValidationPolling` state — that state, `Deps.Now`,
   `pollDeadline`, and the transient-retry loop are tasks 7.11/7.12,
   explicitly out of scope per "Do NOT start Phase 7+". Nothing was
   skipped: the loop simply belongs to Phase 7, not Phase 6, by both
   authoritative sources (design.md and tasks.md) — the orchestrator
   prompt's Phase 6 bullet is read as descriptive context for HU-011's
   eventual behavior, not a scope override of tasks.md/design.md.
6. **`ReportDeploy`'s CLI-error path uses the same stderr-based message
   convention as `Orgs`/`Version`** (`fmt.Errorf("... exited %d: %s",
   ExitCode, stderr)`), not `ValidateDeploy`'s decoded-message-or-raw-
   fallback duality. tasks.md's Phase 6 (6.1-6.4) has no equivalent to
   Phase 4's explicit 4.3/4.4 "CLI error surfaces decoded message" task,
   and `validation-progress/spec.md` has no "CLI Error Surfaces Message"
   requirement (only the app-level "transient error retries" one, Phase
   7). `Raw` is still always preserved on the returned `DeployReport`
   (tested), matching design.md's field but not over-engineering an
   error-decoding contract nothing asked for.
7. **[RESOLVED in batch 3 — shape CONFIRMED, no fix needed]**
   `reportResultEnvelope`'s nested `details.componentFailures`/
   `details.runTestResult.failures` shape was not literally specified in
   design.md** (design.md only names the flat source keys:
   `fullName,componentType,problem` and `name,methodName,message`, not
   their container path). This batch had no `[E2E-ORG]` access to confirm
   the real `sf project deploy report --json` payload (that's Phase 8,
   opt-in), so the nesting was inferred from the standard Salesforce
   Metadata API `DeployResult`/`RunTestsResult` JSON shape (top-level
   counters + `details.componentFailures[]` +
   `details.runTestResult.failures[]`), which is the well-documented,
   stable shape those three sf CLI commands (`validate`/`deploy`/`report`)
   share. Flagged here as an assumption to confirm against real CLI output
   in Phase 8's `[E2E-ORG]` test — a shape mismatch there is a localized
   fix to `reportResultEnvelope`'s struct tags only, since `DeployReport`
   and `ReportDeploy`'s signature are unaffected either way.
   **Batch-3 resolution**: `TestE2ERealOrg_ValidateAndReport` against
   `AM-DEV-EDITION` returned a `Failed` validate with
   `numberComponentErrors:3` and `result.details.componentFailures[]` whose
   entries are exactly `{fullName, componentType, problem}`, plus
   `result.details.runTestResult.failures` as the test-failure container —
   the inferred nesting matches real `sf` CLI 2.135.7 output VERBATIM and
   the parser extracted all 3 failures. **No struct-tag change required**;
   `report.go`'s comment now records the confirmation.

### Batch 3 (Phase 7-9) deviations

8. **`pollTickMsg`/`pollTickCmd` are separate from the cherry-pick
   `tickMsg`/`tickCmd`, not a reuse of `onTick`.** Reusing one `tickMsg`
   would conflate two genuinely different cadences (750ms cherry-pick
   re-poll vs the configured `pollIntervalSeconds`, default 10s) firing two
   different commands (`repoStateCmd` vs `reportCmd`). A dedicated
   `pollTickMsg` + `onPollTick` keeps each loop explicit and independently
   testable — behaviorally identical to design.md's polling-loop intent.
9. **`deltaCmd`'s `git.ChangedFiles` call is best-effort** (a git-diff error
   degrades to no `OutsideSourceDirs`, never sinks a successful delta),
   matching the existing `depWarningsCmd` degrade-to-empty policy.
10. **`validateCmd` does not populate `ValidateRequest.Tests`** — HU-010's
    `RunSpecifiedTests` class list has no selection UI in this slice; the
    salesforce-layer repeated-`--tests` composition is Phase-4-tested and
    exercised by the real-org e2e (`RunSpecifiedTests`+`AccountServiceTest`).
11. **An sgd failure surfaces its raw output via the error TEXT**, not a
    separate model `Raw` field: `Generate` already embeds raw stdout/stderr
    in its error (batch-1 deviation 3), and `viewDeltaGeneration` renders
    `deltaErr.Error()`.

## Issues Found

None. Every deviation above is resolved within its batch; batch-2 deviation
7 is now CONFIRMED against real `sf` output with no code change required.

## Remaining Tasks

None — all 53 `tasks.md` items (Phases 1-9) are complete and green.

## Workload / PR Boundary

- Mode: single PR under `size:exception` (delivery=`single-pr`,
  chain-strategy=`size-exception`, per tasks.md's Review Workload
  Forecast — maintainer-approved, ~2.5k lines within the 40,000 budget)
- Current work unit (batch 3): Units 7-8 of 8 (`internal/app` state-machine
  wiring + real e2e suite)
- Boundary: starts from batch 2's clean, green Phase 1-6 state; ends with
  Phases 7-9 complete and green (full flow reaches a terminal validation
  state; real-org e2e confirmed). The single PR now covers all three batches.
- Estimated review budget impact: batch 3 added 2 commits — the `internal/app`
  wiring (5 files + 1 test + main) and the 2 e2e test files (+ a `report.go`
  doc note). `size:exception` remains the granted delivery mode; the full
  running total is still under the 40,000 session budget.

## Final Verification (this batch's scope)

```
$ export PATH="/usr/local/go/bin:$PATH" && go build ./... && go vet ./... && gofmt -l . && go test -race ./... -count=1
ok  	deploydeck/cmd/deploydeck	5.933s
ok  	deploydeck/internal/app	4.630s
ok  	deploydeck/internal/config	1.935s
ok  	deploydeck/internal/delta	4.748s
ok  	deploydeck/internal/exec	2.605s
ok  	deploydeck/internal/git	49.113s
ok  	deploydeck/internal/prereq	4.399s
ok  	deploydeck/internal/runs	1.686s
ok  	deploydeck/internal/salesforce	2.065s
```

`go build`, `go vet`, and `gofmt -l .` all produced no output (clean).
`go test -race ./...` — run WITHOUT `-short`, so it also exercises every
pre-existing real-git integration test plus the real-sgd integration test
from batch 1 — is fully green, including the new `internal/runs` package
and the extended `internal/salesforce` package. `internal/app` wiring
(Phase 7, `Deps.{Delta,Runs,Now}`, the `ValidationPolling` timeout/
transient-retry loop) and the real e2e suite (Phase 8, `[E2E-ORG]`) remain
for later batches; Phase 9's own final `go test -race ./...` / `go vet
./...` / `gofmt -l .` / proposal-checklist tasks remain for the final
batch.

Focused Phase 4-6 test commands, isolated:
```
$ go test ./internal/salesforce/... -v -count=1
ok  	deploydeck/internal/salesforce	0.187s  (23 test functions, all PASS)

$ go test ./internal/runs/... -v -count=1
ok  	deploydeck/internal/runs	0.524s  (5 test functions, all PASS)
```

## Final Verification (Phase 9 — full change)

Command 1 — full suite, no env var (real-org tests SKIP), verbatim:
```
$ export PATH="/usr/local/go/bin:$PATH" && go build ./... && go vet ./... && gofmt -l . && go test -race ./...
ok  	deploydeck/cmd/deploydeck	(cached)
ok  	deploydeck/internal/app	(cached)
ok  	deploydeck/internal/config	(cached)
ok  	deploydeck/internal/delta	(cached)
ok  	deploydeck/internal/exec	(cached)
ok  	deploydeck/internal/git	(cached)
ok  	deploydeck/internal/prereq	(cached)
ok  	deploydeck/internal/runs	(cached)
ok  	deploydeck/internal/salesforce	(cached)
```
(`go build`/`go vet`/`gofmt -l .` produced no output; on the immediately
prior uncached run the packages timed at cmd 6.163s, app 4.242s, delta
7.588s, git 48.245s, prereq 4.064s, salesforce 2.601s — all `ok`.)

Command 2 — real-org validate+report, verbatim (condensed to key lines):
```
$ export PATH="/usr/local/go/bin:$PATH" && DEPLOYDECK_E2E_ORG=AM-DEV-EDITION go test -run 'E2EOrg|RealOrg|E2E_Org' ./... -v -count=1 -timeout 20m
=== RUN   TestE2ERealOrg_Smoke
--- PASS: TestE2ERealOrg_Smoke (3.57s)
ok  	deploydeck/internal/prereq	4.227s
=== RUN   TestE2ERealOrg_ValidateAndReport
    real_org_e2e_test.go:201: captured jobId: 0Affj00000L13r0CAB
    real_org_e2e_test.go:239: poll status="Pending" components=0/0(err 0) tests=0/0(err 0)
    real_org_e2e_test.go:239: poll status="Failed" components=0/3(err 3) tests=0/0(err 0)
--- PASS: TestE2ERealOrg_ValidateAndReport (11.87s)
ok  	deploydeck/internal/salesforce	12.897s
(other packages: [no tests to run], all ok)
```

**Report JSON shape verdict**: the real `sf project deploy report --json`
output MATCHED the inferred `reportResultEnvelope` struct VERBATIM —
`result.details.componentFailures[]` with `{fullName, componentType,
problem}` and `result.details.runTestResult.failures` as the test-failure
container. All 3 real component failures parsed correctly. **No struct-tag
fix was required.**

## Commits

Batch 1:
1. `5a3c55f` `docs(delta-validation): add change proposal, spec, and design`
2. `86be517` `feat(config): add delta and polling config`
3. `d47f663` `feat(delta): sgd delta generation service`
4. `9333d45` `feat(delta): package.xml/destructiveChanges.xml summary`
5. `1b5e47c` `test(delta): real-sgd integration against test-e2e-org fixture`
6. `4852ed8` `fix(config): only default delta.outputDir once sourceDirs is set`

Batch 2:
7. `31c84c4` `feat(salesforce): async deploy validate`
8. `970265c` `feat(runs): minimal run persistence`
9. `9c77413` `feat(salesforce): deploy report + bounded polling primitives`

Batch 3 (this FINAL batch):
10. `a18cf21` `feat(app): wire delta generation, package review and async validation states`
11. `e25db27` `test(delta,salesforce): real e2e for multi-source-dir delta and org validate+report`
12. (this doc + tasks/proposal checkbox updates) `docs(delta-validation): mark phases 7-9 complete`

## Status

53/53 tasks.md items complete (Phases 1-9 fully done and green).
All batches complete. The change is fully implemented, green under
`-race`, and confirmed against the real org — ready for `sdd-verify`.
