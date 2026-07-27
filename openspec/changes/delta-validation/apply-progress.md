# Apply Progress: Delta + Validation (HU-007, 008, 010, 011)

**Batch**: 1 of N (this batch covers Phase 1-3 only, per orchestrator scope)
**Mode**: Strict TDD
**Delivery**: single-pr under `size:exception` (maintainer-approved, per tasks.md's Review Workload Forecast)
**Branch**: `delta-validation`

## Scope Covered This Batch

Phase 1 (Foundations: config/plan/changed-files), Phase 2 (HU-007
`internal/delta` delta generation), Phase 3 (HU-008 `Summarize`
package-summary). Phases 4-9 (HU-010 validate, run-persistence, HU-011
report, `internal/app` wiring, real e2e, final verification) are **NOT**
started — explicitly out of scope for this batch per the orchestrator's
instruction.

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

**19/53 tasks.md items complete** (Phases 1-3 fully done; Phases 4-9
remain for the next batch).

## Files Changed

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

## Issues Found

None beyond the two deviations above (both resolved within this batch,
not deferred).

## Remaining Tasks

Phase 4 (HU-010 `salesforce.ValidateDeploy`) through Phase 9 (final
verification) — 34 remaining `tasks.md` items. Explicitly NOT started per
this batch's scope (orchestrator instruction: "Phase 1-3 ONLY. Do NOT
start Phase 4+").

## Workload / PR Boundary

- Mode: single PR under `size:exception` (delivery=`single-pr`,
  chain-strategy=`size-exception`, per tasks.md's Review Workload
  Forecast — maintainer-approved, ~2.5k lines within the 40,000 budget)
- Current work unit: Units 1-3 of 8 (Foundations, HU-007 service,
  HU-008 summary) — this batch
- Boundary: starts from the archived HU-001..006 foundation (clean,
  green); ends with Phase 3 complete and green, Phase 4+ untouched
- Estimated review budget impact: 1,457 changed lines in `internal/`
  across 6 commits (`git diff --stat 1ed0063 HEAD -- internal/`), plus
  567 lines of `openspec/` planning docs (excluded from the authored
  code budget) — well within the granted exception

## Final Verification (this batch's scope)

```
$ export PATH="/usr/local/go/bin:$PATH" && go build ./... && go vet ./... && gofmt -l . && go test -race ./...
ok  	deploydeck/cmd/deploydeck	6.347s
ok  	deploydeck/internal/app	4.340s
ok  	deploydeck/internal/config	1.356s
ok  	deploydeck/internal/delta	4.921s
ok  	deploydeck/internal/exec	1.359s
ok  	deploydeck/internal/git	50.159s
ok  	deploydeck/internal/prereq	3.581s
ok  	deploydeck/internal/salesforce	1.488s
```

`go build`, `go vet`, and `gofmt -l .` all produced no output (clean).
`go test -race ./...` — run WITHOUT `-short`, so it also exercises every
pre-existing real-git integration test plus the new real-sgd integration
test — is fully green. `PollTimeoutSeconds`-driven behavior (HU-011
polling) and `internal/app` wiring are Phase 4+ and not yet exercised;
Phase 9's own `go test -race ./...` / `go vet ./...` / `gofmt -l .` /
proposal-checklist tasks remain for the final batch.

Real-sgd integration test, isolated:
```
$ go test ./internal/delta/... -run TestService_Generate_RealSgd_TempRepo -v -count=1
=== RUN   TestService_Generate_RealSgd_TempRepo
--- PASS: TestService_Generate_RealSgd_TempRepo (3.12s)
PASS
ok  	deploydeck/internal/delta	3.509s
```

## Commits (this batch)

1. `5a3c55f` `docs(delta-validation): add change proposal, spec, and design`
2. `86be517` `feat(config): add delta and polling config`
3. `d47f663` `feat(delta): sgd delta generation service`
4. `9333d45` `feat(delta): package.xml/destructiveChanges.xml summary`
5. `1b5e47c` `test(delta): real-sgd integration against test-e2e-org fixture`
6. `4852ed8` `fix(config): only default delta.outputDir once sourceDirs is set`

## Status

19/53 tasks.md items complete (Phases 1-3 fully done and green).
Ready for the next `sdd-apply` batch (Phase 4: HU-010
`salesforce.ValidateDeploy`).
