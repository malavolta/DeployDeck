# Apply Progress: Delta + Validation (HU-007, 008, 010, 011)

**Batch**: 2 of N (Phases 4-6 this batch; Phases 1-3 completed in batch 1)
**Mode**: Strict TDD
**Delivery**: single-pr under `size:exception` (maintainer-approved, per tasks.md's Review Workload Forecast)
**Branch**: `delta-validation`

## Scope Covered So Far

Phase 1 (Foundations: config/plan/changed-files), Phase 2 (HU-007
`internal/delta` delta generation), Phase 3 (HU-008 `Summarize`
package-summary) — batch 1. Phase 4 (HU-010 `salesforce.ValidateDeploy`),
Phase 5 (`internal/runs` minimal writer), Phase 6 (HU-011
`salesforce.ReportDeploy` + `IsTerminal`) — batch 2 (this batch). Phase 7
(`internal/app` wiring), Phase 8 (real e2e), Phase 9 (final verification)
are **NOT** started — explicitly out of scope for this batch per the
orchestrator's instruction ("Phases 4-6 ONLY... Do NOT start Phase 7+").

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

**31/53 tasks.md items complete** (Phases 1-6 fully done; Phases 7-9
remain for the next batch).

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
7. **`reportResultEnvelope`'s nested `details.componentFailures`/
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

## Issues Found

None beyond the deviations above (all resolved within their respective
batch, not deferred, except deviation 7 which is explicitly flagged for
Phase 8 confirmation).

## Remaining Tasks

Phase 7 (`internal/app` wiring: `DeltaGeneration`, `PackageReview`,
`QueueReview`, `ValidationStart`, `ValidationPolling` states +
`Deps.{Delta,Runs,Now}` + `deltaCmd`/`validateCmd`/`reportCmd`/`onTick`/
`onReportDone`) through Phase 9 (final verification) — 22 remaining
`tasks.md` items. Explicitly NOT started per this batch's scope
(orchestrator instruction: "Phases 4-6 ONLY... Do NOT start Phase 7+").

## Workload / PR Boundary

- Mode: single PR under `size:exception` (delivery=`single-pr`,
  chain-strategy=`size-exception`, per tasks.md's Review Workload
  Forecast — maintainer-approved, ~2.5k lines within the 40,000 budget)
- Current work unit: Units 4-6 of 8 (HU-010 validate, `internal/runs`
  writer, HU-011 report+`IsTerminal`) — this batch
- Boundary: starts from batch 1's clean, green Phase 1-3 state; ends with
  Phase 6 complete and green, Phase 7+ untouched
- Estimated review budget impact: this batch added 3 commits touching
  `internal/salesforce/{client,validate,report}.go`(+tests) and
  `internal/runs/writer.go`(+test) — `git diff --stat 4852ed8 HEAD --
  internal/` shows the batch's authored addition, well within the granted
  exception (full running total still far under the 40,000 session
  budget)

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

## Commits

Batch 1:
1. `5a3c55f` `docs(delta-validation): add change proposal, spec, and design`
2. `86be517` `feat(config): add delta and polling config`
3. `d47f663` `feat(delta): sgd delta generation service`
4. `9333d45` `feat(delta): package.xml/destructiveChanges.xml summary`
5. `1b5e47c` `test(delta): real-sgd integration against test-e2e-org fixture`
6. `4852ed8` `fix(config): only default delta.outputDir once sourceDirs is set`

Batch 2 (this batch):
7. `31c84c4` `feat(salesforce): async deploy validate`
8. `970265c` `feat(runs): minimal run persistence`
9. `9c77413` `feat(salesforce): deploy report + bounded polling primitives`

## Status

31/53 tasks.md items complete (Phases 1-6 fully done and green).
Ready for the next `sdd-apply` batch (Phase 7: `internal/app` wiring —
`DeltaGeneration` through `ValidationPolling` states, `Deps.{Delta,Runs,
Now}`, and the actual `tea.Tick`-driven poll loop with timeout/
transient-retry).
