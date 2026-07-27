# Design: Delta + Validation (HU-007, 008, 010, 011)

## Technical Approach

Extend the post-`StatePickVerification` flow with a new `internal/delta` service (sgd over `exec.Runner`), two new `internal/salesforce` files (`validate.go`/`report.go` on the existing `Client`/`decodeEnvelope`/`New(runner)`), a minimal `internal/runs` writer, and new `internal/app` states. All external commands stay behind services; `app` never execs (enforced by `boundary_test.go`). HU-007 entry reuses `git.DeltaAndValidationAllowed` / `Model.DeltaAllowed()` — never re-derived. Polling mirrors the existing `tickCmd`+`repoStateCmd`+`onTick` pattern: `tea.Tick` schedules single `ReportDeploy` calls; the CLI/repo is the source of truth, the model only reflects it.

## Architecture Decisions

| Decision | Choice | Rejected | Rationale |
|---|---|---|---|
| sgd multi-dir | ONE `sf sgd source delta` with repeated `--source-dir` | per-dir call + merge packages | Spike proved 6.45.1 merges in one call; merge code is dead complexity |
| delta placement | new `internal/delta` sibling over `exec.Runner` | grow `salesforce` shim | sgd is a distinct concern from the read-only `sf` shim; keeps shim thin |
| polling driver | `tea.Tick` firing single `ReportDeploy` per tick | blocking loop in `salesforce` | matches existing cherry-pick re-poll; keeps shim a 1-call decoder; testable via `Update` |
| terminal timeout | model `pollDeadline` from injected `Deps.Now` + `PollTimeoutSeconds` | real `time.Sleep` | deterministic, real-time-free tests |
| `QueueReview` | constant kept, `PackageReview → ValidationStart` on confirm (pass-through) | omit the state | forward-compat for HU-009 without dead UI |
| empty package | `emptyConfirmed` flag on `PackageReview` blocks confirm until explicit override | dedicated confirm state | one extra field vs a whole screen |
| run persistence | minimal `run.json`+raw, `SchemaVersion:1` | pull HU-013 forward | meets ACs; HU-013 extends by scanning the same dir |

## Data Flow

    PickVerification ──confirm(DeltaAllowed)──▶ DeltaGeneration
      delta.Generate(from=origin/<target>,to=HEAD,sourceDirs…) ─▶ package/package.xml (+destructiveChanges/)
      RegisterDeltaArtifacts(plan) ; Parse+Summarize ─▶ PackageReview
    PackageReview ──confirm──▶ (QueueReview inert) ─▶ ValidationStart
      sf.ValidateDeploy ─▶ jobId ─▶ runs.Create(run.json+validate.json) ─▶ ValidationPolling
    ValidationPolling ◀─tea.Tick── sf.ReportDeploy(jobId) ─▶ runs.AppendReport(raw)
      IsTerminal? ─▶ Succeeded | Failed | Canceled     (else reschedule; Now()>deadline ─▶ Failed:timeout)

## File Changes

| File | Action | Description |
|---|---|---|
| `internal/delta/service.go` | Create | `New(runner)`, `Generate(ctx,Request) (Result,error)`: build args, run, discover artifacts |
| `internal/delta/package.go` | Create | `ParsePackage`/`ParseDestructive` (encoding/xml), `Summarize` → `PackageSummary` |
| `internal/salesforce/validate.go` | Create | `ValidateDeploy` → `sf project deploy validate --async --json` |
| `internal/salesforce/report.go` | Create | `ReportDeploy` → `sf project deploy report --json`, `IsTerminal` |
| `internal/salesforce/client.go` | Modify | add `ValidateDeploy`/`ReportDeploy` to `Client` (New unchanged) |
| `internal/runs/writer.go` | Create | `NewWriter`, `Create`, `AppendReport`, versioned `Record` |
| `internal/config/config.go` | Modify | `Delta DeltaConfig`, `PollIntervalSeconds`, `PollTimeoutSeconds` |
| `internal/config/{load,validate}.go` | Modify | defaults (interval 10, timeout 3600, outputDir) + `Validate` rules |
| `internal/git/deployment_plan.go` | Modify | `PackageXMLPath`/`DestructiveChangesPath` + `RegisterDeltaArtifacts` |
| `internal/app/{app,update,commands}.go` | Modify | new states, `Deps.{Delta,Runs,Now}`, delta/validate/report cmds |

## Interfaces / Contracts

```go
// internal/delta
type Request struct{ Dir, From, To, OutputDir string; SourceDirs []string; IgnoreFile, IgnoreDestructiveFile string }
type Result  struct{ PackageXMLPath, DestructiveChangesPath, Raw string } // Destructive "" when absent
func (s *Service) Generate(ctx, Request) (Result, error) // runner err | ExitCode!=0 → error carrying Raw, no artifacts
// args: sf sgd source delta --from <From> --to <To> --output-dir <OutputDir> --generate-delta
//       (--source-dir X)... [--ignore-file] [--ignore-destructive-file]   (real sgd flags, verified vs `sf sgd source delta --help`)
// discovery: <out>/package/package.xml ; <out>/destructiveChanges/destructiveChanges.xml (only if present)

type PackageSummary struct { // HU-008 model
  Types, DestructiveTypes []MetadataTypeSummary // {Name string; Count int}; destructive kept SEPARATE
  HasDestructive bool; SensitiveTypes, OutsideSourceDirs []string; Empty bool
}
func Summarize(pkg, destructive Package, changedFiles, sourceDirs []string) PackageSummary
// Empty = zero members in pkg; Sensitive = {Profile,PermissionSet,Flow,CustomObject,CustomField}
// OutsideSourceDirs = changedFiles not under any sourceDirs (changedFiles from git diff --name-only From..To)

// internal/salesforce
type ValidateRequest struct{ Dir, ManifestPath, PostDestructivePath, TargetOrg, TestLevel string; Tests []string }
type ValidateResult  struct{ JobID, Raw string } // JobID = envelope result.id
func (c *client) ValidateDeploy(ctx, ValidateRequest) (ValidateResult, error)
// conditional flags: PostDestructivePath!="" → --post-destructive-changes; each Tests entry → repeated --tests
// error: ExitCode!=0 → decodeEnvelope for message; unparseable → surface raw stdout/stderr; always keep Raw

type DeployReport struct { Status string
  NumberComponentsTotal, NumberComponentsDeployed, NumberComponentErrors int
  NumberTestsTotal, NumberTestsCompleted, NumberTestErrors int
  ComponentFailures []ComponentFailure // {Component,Type,Message} ← fullName,componentType,problem
  TestFailures      []TestFailure      // {Class,Method,Message} ← name,methodName,message
  Raw string }
func (c *client) ReportDeploy(ctx, jobID, targetOrg, dir string) (DeployReport, error)
func IsTerminal(status string) bool // {Succeeded,SucceededPartial,Failed,Canceled}

// internal/runs — MINIMAL, extensible for HU-013
type Record struct{ SchemaVersion int; RunID, Ticket, Target, Alias, JobID, Status string; CreatedAt, UpdatedAt time.Time }
func (w *Writer) Create(rec Record, validateRaw []byte) (dir string, err error) // .deploydeck/runs/<run-id>/{run.json,validate.json}
func (w *Writer) AppendReport(runID, status string, reportRaw []byte) error      // update run.json + persist EACH poll's raw report as report-<NNN>.json (HU-011: every raw report saved, not overwritten)
// run-id: <ticket>-to-<target>-<yyyymmddHHMMSS>. HU-013 adds list/retention/resume by scanning run.json.

// internal/config
type DeltaConfig struct{ OutputDir string; SourceDirs []string; IgnoreFile, IgnoreDestructiveFile string }
// Config += Delta, PollIntervalSeconds (def 10), PollTimeoutSeconds (def 3600)
// Validate: interval>0; timeout>0; if any Delta field set → SourceDirs non-empty

// internal/git.DeploymentPlan += PackageXMLPath, DestructiveChangesPath (+ RegisterDeltaArtifacts, mirrors RegisterPromotionBranch)
```

### Polling loop (ValidationPolling)

`Deps.Now func() time.Time` (nil→`time.Now`). Entering `ValidationStart`→success sets `pollDeadline = Now()+PollTimeout`, arms a cancelable session `pollCtx`/`pollCancel` (`context.WithCancel`), and fires ONLY the first `reportCmd`. Polling is strictly **sequential and report-driven**: the next tick is armed by `onReportDone` after the current report returns — never a free-running `tickCmd` that re-arms itself — so a slow `report` can never let ticks pile up into concurrent `sf` subprocesses. A `pollInFlight` guard enforces "at most one `ReportDeploy` outstanding": `onPollTick`/manual-refresh (`r`) are no-ops while a report is in flight, and `onPollTick` fires the next report only when idle. `onReportDone`: clears `pollInFlight`; persists EVERY raw report — success OR a transient error that still returned output — via `runs.AppendReport(raw)` (spec: "each raw report saved"); on transient error keeps state and `scheduleNextPoll`; on success updates report; `IsTerminal` → map {Succeeded,SucceededPartial}→`StateSucceeded`, Failed→`StateFailed`, Canceled→`StateCanceled`, else `scheduleNextPoll`. `scheduleNextPoll` enforces `Now()>pollDeadline → StateFailed(timeout)` before arming the single next tick. Each `ReportDeploy` gets a per-call `context.WithTimeout` derived from `pollCtx`; **any polling exit — terminal, timeout, or user quit (`q`/`ctrl+c`) — cancels `pollCtx`**, tearing down an in-flight read-only `report` subprocess without ever issuing a `deploy cancel` (the SF job stays active and the run resumable). Terminal detection is exit-code-INDEPENDENT: `ReportDeploy` returns a parseable report (recognized non-empty status) as data even when `sf` exits non-zero on a Failed/Canceled deploy, so the loop maps the terminal state immediately instead of mistaking it for a transient error. Tests drive a fake report sequence (`InProgress`→terminal) via `Model.Update(reportDoneMsg{...})` + `FakeRunner`; the injected clock advances past `pollDeadline` — no real time.

## Testing Strategy

| Layer | What | Approach |
|---|---|---|
| Unit | package.xml/destructive parse; Summarize (normal/empty/destructive/sensitive/outside-dirs); validate+report envelope decode; `IsTerminal`; config load+validate; plan fields | table-driven, fixtures, `t.TempDir` |
| Unit (app) | state transitions Delta→PackageReview→ValidationStart→Polling→terminal; empty-package block+override; timeout via injected `Now`; transient-retry | direct `Model.Update(msg)` + `FakeRunner` |
| Integration | HU-007 real sgd on temp repo (deletes → destructiveChanges), artifacts under `.deploydeck/`, working tree clean | `-short`-skippable; plugin required |
| E2E | non-destructive CheckOnly validate + report poll-to-terminal vs personal alias | `DEPLOYDECK_E2E_ORG` gate + `t.Skip`, `NewOSRunner`, per `internal/prereq/real_org_e2e_test.go` |

## Threat Matrix

| Boundary | Applicability | Design response | Planned RED tests |
|---|---|---|---|
| Documentation-like paths | N/A: parses XML manifests, no executable-file classification | — | — |
| Git repo selection | Applicable: sgd/sf run in the repo root | `CommandRequest.Dir`=repo root; `--from origin/<target> --to HEAD` explicit refs, no cwd trust | sgd/validate run with `Dir` set, refs literal |
| Commit state | N/A: delta reads committed refs; validation is CheckOnly; no index/worktree mutation | — | — |
| Push state | N/A: no push (HU-014 out of scope) | — | — |
| PR/argument composition | Applicable: repeated `--source-dir`/`--tests`, conditional `--post-destructive-changes` | args as slice via `exec.Runner`, never a shell; conditional flags only when destructive/`RunSpecifiedTests` | arg-composition table: N source-dirs, with/without destructive, with/without tests |

## Migration / Rollout

No data migration. Additive on branch `delta-validation`; `.deploydeck/runs/` and `.deploydeck/manifest/` are local + gitignored. Revert = drop feature commits; foundation flow (≤`StatePickVerification`) untouched. CI must add `sf plugins install sfdx-git-delta`.

## Open Questions

- [ ] Confirm exact `sf project deploy validate --async --json` result key for jobId (`result.id`) across pinned CLI; `Raw` is retained regardless so a mapping fix is localized.
- [ ] `PollTimeoutSeconds` introduced as an additive config field (default 3600) to satisfy the "hard timeout" requirement — confirm acceptable vs a fixed constant.
