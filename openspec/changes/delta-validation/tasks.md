# Tasks: Delta + Validation (HU-007, 008, 010, 011)

Strict TDD. `[U]` table-driven unit, `[I]` integration (temp repo/real sgd), `[T]` TUI (`Model.Update`), `[E2E-ORG]` opt-in (`DEPLOYDECK_E2E_ORG`). Out of scope: HU-009 queue, HU-012 cancel, HU-014 push/PR, HU-015 quick deploy, HU-013 full run history/retention/resume (`internal/runs` stays minimal).

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | ~2,200–2,800 (6 modules touched/created: config, git, delta, salesforce, runs, app + tests) |
| 400-line budget risk | High |
| Project review budget (session) | 40,000 — estimate is ~7% of budget; single PR fits comfortably |
| Chained PRs recommended | Yes (risk-based) |
| Suggested split | Single PR under `size:exception` (delivery=single-pr); 8 natural internal boundaries below for commit structure / future chaining reference |
| Delivery strategy | single-pr |
| Chain strategy | size-exception |

Decision needed before apply: Yes
Chained PRs recommended: Yes
Chain strategy: size-exception
400-line budget risk: High

### Suggested Work Units

| Unit | Goal | Likely PR | Focused test command | Runtime harness | Rollback boundary |
|---|---|---|---|---|---|
| 1 | Foundations: config/plan/changed-files | PR 1 | `go test ./internal/config/... ./internal/git/...` | N/A — no user-visible behavior yet | Revert `config.go/validate.go/load.go` Delta fields + `deployment_plan.go` additions + `changed_files.go` |
| 2 | HU-007 `internal/delta` service | PR 2 | `go test ./internal/delta/...` | `[I]` real sgd on temp repo seeded from `test-e2e-org` | Delete `internal/delta/service.go`(+test); no dependents yet |
| 3 | HU-008 `Summarize` | PR 3 | `go test ./internal/delta/... -run Summarize` | N/A — pure parser, fixture-proven | Delete `internal/delta/package.go`(+test) |
| 4 | `internal/runs` minimal writer | PR 4 | `go test ./internal/runs/...` | N/A — `t.TempDir` filesystem writer | Delete `internal/runs/` package |
| 5 | HU-010 `salesforce.ValidateDeploy` | PR 5 | `go test ./internal/salesforce/... -run ValidateDeploy` | `[E2E-ORG]` CheckOnly `--async --json` vs personal alias | Delete `validate.go`(+test); drop from `Client` |
| 6 | HU-011 `salesforce.ReportDeploy`/`IsTerminal` | PR 6 | `go test ./internal/salesforce/... -run 'ReportDeploy\|IsTerminal'` | `[E2E-ORG]` report polling vs personal alias | Delete `report.go`(+test); drop from `Client` |
| 7 | `internal/app` wiring (state machine) | PR 7 | `go test ./internal/app/...` | N/A directly — proven via `Model.Update`; manual smoke = run TUI past `PickVerification` | Revert `app.go/update.go/commands.go` additions; flow falls back to stopping at `StatePickVerification` |
| 8 | Real e2e suite | PR 8 | `go test ./internal/delta/... -run E2E`; `DEPLOYDECK_E2E_ORG=<alias> go test ./internal/salesforce/... -run TestE2ERealOrg` | The tests ARE the harness (real sgd; real non-destructive CheckOnly validate/report) | Delete the two e2e test files; no prod code |

## Phase 1: Foundations — config, plan, changed-files helper

- [x] 1.1 [U] RED `internal/config/config_test.go`: `DeltaConfig{OutputDir,SourceDirs,IgnoreFile,IgnoreDestructiveFile}` loads from yaml; `PollIntervalSeconds`/`PollTimeoutSeconds` default to 10/3600 when omitted
- [x] 1.2 [U] GREEN: add `Delta DeltaConfig`, `PollIntervalSeconds`, `PollTimeoutSeconds` to `config.Config`; defaults in `load.go`
- [x] 1.3 [U] RED `internal/config/validate_test.go`: `Validate()` rejects interval<=0, timeout<=0, and any `Delta` field set with empty `SourceDirs`
- [x] 1.4 [U] GREEN: implement the rules in `validate.go`
- [x] 1.5 [U] RED `internal/git/deployment_plan_test.go`: `RegisterDeltaArtifacts` sets `PackageXMLPath`/`DestructiveChangesPath`, preserves all other fields (mirrors `RegisterPromotionBranch`)
- [x] 1.6 [U] GREEN: add the two fields + `RegisterDeltaArtifacts` to `deployment_plan.go`
- [x] 1.7 [U] RED `internal/git/changed_files_test.go`: exported `ChangedFiles(ctx, dir, from, to)` wraps `git diff --name-only From..To`
- [x] 1.8 [U] GREEN: implement `ChangedFiles` (reuse the existing `diffNameOnly` pattern from `pick_verification.go`)

## Phase 2: HU-007 `internal/delta` (delta-generation)

- [x] 2.1 [U] RED `internal/delta/service_test.go`: `Generate` composes ONE `sf sgd source delta --from <From> --to <To> --output-dir <dir> --generate-delta` + one repeated `--source-dir` per configured dir + conditional `--ignore-file`/`--ignore-destructive-file`; `Dir`=repo root, `--from`/`--to` literal refs (`FakeRunner` arg capture — threat-matrix arg-composition + git-repo-selection rows)
- [x] 2.2 [U] GREEN: implement `Request`/`Result` + arg composition in `service.go`
- [x] 2.3 [U] RED: locates `<out>/package/package.xml` always; `<out>/destructiveChanges/destructiveChanges.xml` only when present; `Raw` always captured
- [x] 2.4 [U] GREEN: implement artifact discovery
- [x] 2.5 [U] RED: sgd `ExitCode!=0` → error carrying `Raw`, zero `Result`, no artifact paths returned (downstream validation never triggered)
- [x] 2.6 [U] GREEN: implement the failure path
- [x] 2.7 [I] `internal/delta` real-sgd integration on a temp repo seeded from `test-e2e-org` fixture metadata: multi `--source-dir` merges into one `package.xml`, deletes → `destructiveChanges.xml`, artifacts under `.deploydeck/manifest/delta/<ticket>-to-<target>/`, working tree stays clean (`-short`-skippable, plugin required)

## Phase 3: HU-008 `Summarize` (package-summary)

- [x] 3.1 [U] RED `internal/delta/package_test.go`: `ParsePackage`/`ParseDestructive` parse fixture XML into `Package`
- [x] 3.2 [U] GREEN: implement `ParsePackage`/`ParseDestructive` (`encoding/xml`)
- [x] 3.3 [U] RED `internal/delta/summarize_test.go` (table-driven): per-type counts; `Empty` warn; destructive kept separate with no additive-count inflation; sensitive `{Profile,PermissionSet,Flow,CustomObject,CustomField}` warning; `OutsideSourceDirs` from `changedFiles`
- [x] 3.4 [U] GREEN: implement `Summarize` → `PackageSummary`

## Phase 4: HU-010 `salesforce.ValidateDeploy` (deploy-validation)

- [x] 4.1 [U] RED `internal/salesforce/validate_test.go`: composes `sf project deploy validate --manifest <pkg> --target-org <alias> --test-level <level> --async --json` (`Dir`=repo root); conditional `--post-destructive-changes`; repeated `--tests` only for `RunSpecifiedTests`; `JobID`=envelope `result.id`
- [x] 4.2 [U] GREEN: implement `ValidateRequest`/`ValidateResult`/`ValidateDeploy` in `validate.go`, add to `Client` interface (`client.go`, `New` unchanged)
- [x] 4.3 [U] RED: CLI `ExitCode!=0` → message+`Raw` surfaced, error returned (not panic), flow-continuable
- [x] 4.4 [U] GREEN: implement the error path

## Phase 5: `internal/runs` minimal writer (run-persistence)

- [x] 5.1 [U] RED `internal/runs/writer_test.go`: `Create` writes `.deploydeck/runs/<run-id>/{run.json,validate.json}`, `SchemaVersion:1`, jobId/status/timestamps (`t.TempDir`)
- [x] 5.2 [U] GREEN: implement `Record`/`NewWriter`/`Create`
- [x] 5.3 [U] RED: `AppendReport` persists EACH poll as `report-<NNN>.json` (`001,002,…`, never overwritten), updates `run.json` status/`UpdatedAt`
- [x] 5.4 [U] GREEN: implement `AppendReport` with an incrementing `NNN` counter

## Phase 6: HU-011 `salesforce.ReportDeploy` (validation-progress, CLI half)

- [x] 6.1 [U] RED `internal/salesforce/report_test.go`: composes `sf project deploy report --job-id <id> --target-org <alias> --json`; parses `ComponentFailures` (`fullName,componentType,problem`) + `TestFailures` (`name,methodName,message`)
- [x] 6.2 [U] GREEN: implement `ReportDeploy` in `report.go`, add to `Client`
- [x] 6.3 [U] RED: `IsTerminal` table — `{Succeeded,SucceededPartial,Failed,Canceled}`=true, else false
- [x] 6.4 [U] GREEN: implement `IsTerminal`

## Phase 7: `internal/app` wiring (state machine, both HU-007..011 UI halves)

- [x] 7.1 [T] RED `internal/app`: `PickVerification` confirm requires `Model.DeltaAllowed()`; false blocks entry to `DeltaGeneration`
- [x] 7.2 [T] GREEN: add `State` consts `DeltaGeneration..Canceled`, `Deps.{Delta,Runs,Now}`, gate wiring
- [x] 7.3 [T] RED: `DeltaGeneration` → `deltaCmd` (`Generate`+`Summarize`) → `PackageReview`; sgd failure keeps user on `DeltaGeneration` with `Raw` shown, no validation call
- [x] 7.4 [T] GREEN: implement `deltaCmd` + transition
- [x] 7.5 [T] RED: `PackageReview` blocks confirm while `summary.Empty` until explicit override (`emptyConfirmed`); override then proceeds
- [x] 7.6 [T] GREEN: implement the `emptyConfirmed` flag + gate
- [x] 7.7 [T] RED: `PackageReview` confirm → `QueueReview` (inert, zero queue query) → `ValidationStart` same tick
- [x] 7.8 [T] GREEN: implement the inert pass-through
- [x] 7.9 [T] RED: `ValidationStart` → `sf.ValidateDeploy` → jobId → `runs.Create` → `ValidationPolling`; CLI error shows message+`Raw`, flow stays alive (no crash/terminal-error state)
- [x] 7.10 [T] GREEN: implement `validateCmd` + immediate persistence
- [x] 7.11 [T] RED: `ValidationPolling` — `tea.Tick`-driven single `ReportDeploy`/tick; injected `Deps.Now` sets `pollDeadline` from `PollTimeoutSeconds`; `Now()>deadline` → `StateFailed`(timeout); transient report error retries within deadline; each raw report → `runs.AppendReport`
- [x] 7.12 [T] GREEN: implement `reportCmd`/`onTick`/`onReportDone` polling loop
- [x] 7.13 [T] RED: terminal mapping `{Succeeded,SucceededPartial}`→`StateSucceeded`, `Failed`→`StateFailed`, `Canceled`→`StateCanceled`; polling stops
- [x] 7.14 [T] GREEN: implement terminal mapping
- [x] 7.15 [T] RED: user exit during `ValidationPolling` leaves the Salesforce job active/resumable — no cancel/abort command issued
- [x] 7.16 Verify `boundary_test.go` (app never execs directly) still passes unmodified

## Phase 8: Real e2e

- [x] 8.1 [I] `internal/delta` real-sgd temp-repo integration seeded from the `test-e2e-org` fixture (already committed) — full HU-007 path incl. multi-dir + destructive
- [x] 8.2 [E2E-ORG] new `internal/salesforce/real_org_e2e_test.go`, same convention as `internal/prereq/real_org_e2e_test.go` (env-gate + `t.Skip`, `NewOSRunner`) — non-destructive CheckOnly `ValidateDeploy --async` then `ReportDeploy` polling to terminal against a personal alias; never a real deploy

## Phase 9: Final verification

- [ ] 9.1 `go test -race ./...` green
- [ ] 9.2 `go vet ./...` clean
- [ ] 9.3 `gofmt -l .` empty output
- [ ] 9.4 Check off `proposal.md` Success Criteria against implemented behavior

## HU / Test-Type Mapping

| Phase | HU | Test types |
|---|---|---|
| 1 | shared foundation | `[U]` |
| 2 | HU-007 | `[U]` `[I]` |
| 3 | HU-008 | `[U]` |
| 4 | HU-010 | `[U]` |
| 5 | run-persistence (HU-010/011 support) | `[U]` |
| 6 | HU-011 (CLI half) | `[U]` |
| 7 | HU-007/008/010/011 (UI half) | `[T]` |
| 8 | HU-007, HU-010/011 | `[I]` `[E2E-ORG]` |
