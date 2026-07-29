# Design: HU-015 — Quick Deploy Opcional (`quick-deploy`)

## Technical Approach

Compose existing idioms; the ONE real data gap is persisting the deploy test level.
`git.DeploymentPlan.TestLevel` (`deployment_plan.go:26`) is set at `confirmTarget`
(`keys.go:533` → `ConfirmTargetSelection`, `target_selection.go:125`) from
`SandboxConfig.TestLevel` (`config.go:47`) but never reaches `run.json`. We add an
additive `Record.TestLevel`, a pure `runs.QuickDeployEligible`, a `salesforce.QuickDeploy`
mirroring `CancelDeploy`, a zero-value-safe `config.QuickDeployConfig`, a `git.IsProductionTarget`
helper, and a gated `StateQuickDeploy` surface with a dedicated typed strong-confirm.
Default is suggest-only + production-blocked. Maps to proposal In-Scope 1:1; feeds spec
domains `quick-deploy` (new), `run-persistence` + `run-history` (additive deltas).

## Architecture Decisions

### ADR-1 — Pinned TestLevel → eligible mapping (REAL values)

DeployDeck emits the four sf-CLI test levels documented at `validate.go:14-17,36-37`
(const `testLevelRunSpecifiedTests = "RunSpecifiedTests"`). The literal string is
whatever `SandboxConfig.TestLevel` holds per sandbox; these four are the domain.

| `Record.TestLevel` (real value) | required tests ran? | eligible | Rationale |
|---|---|---|---|
| `RunLocalTests` | yes | ✅ | Runs all local Apex — satisfies prod deploy test gate |
| `RunAllTestsInOrg` | yes | ✅ | Superset of local |
| `RunSpecifiedTests` | unknown coverage | ❌ | DeployDeck can't prove the specified set covers required tests → fail-safe |
| `NoTestRun` | no | ❌ | No tests executed |
| `""` (old run.json, pre-HU-015) | unknown | ❌ | Fail-safe backward-compat (`omitempty`) |

**Choice**: `requiredTestsRan` = `TestLevel ∈ {RunLocalTests, RunAllTestsInOrg}`.
**Rejected**: treating `RunSpecifiedTests` as eligible (can't verify required-test coverage);
matches E2E seed "no elegible por test level" (`HISTORIAS.md:1033`).

### ADR-2 — Dedicated strong-confirm field

| Option | Decision |
|---|---|
| Reuse `cancelInput`/`deleteConfirm` | ❌ Rejected — confirm-buffer collision (HU-017 lesson) |
| New `Model.quickConfirm string`, word `DESPLEGAR` | ✅ Chosen — dedicated, case-sensitive exact match, mirrors `cancelConfirmWord="CANCELAR"` (`keys.go:809,820`) |

### ADR-3 — Config zero-value-safe defaults

| Field | Default (zero) | Meaning | applyDefaults entry |
|---|---|---|---|
| `QuickDeploy.AllowExecution` | `false` | suggest-only | none — `false` IS safe |
| `QuickDeploy.AllowProduction` | `false` | prod-blocked | none |

**Choice**: `QuickDeployConfig{AllowExecution, AllowProduction bool}` on `Config` as
`QuickDeploy` (`yaml:"quickDeploy"`). No `applyDefaults` (`load.go:36`) entry, no `Validate`
rule (any bool combo valid). **Rejected**: pointer/`*bool` defaults — needless, false-safe already.

### ADR-4 — `MarkQuickDeployed` companion-file shape

| Aspect | Decision | Mirrors |
|---|---|---|
| Companion file | `quick.json` written verbatim (`quickRaw []byte`) | `cancel.json` (`writer.go:236`) |
| Record marker | new `QuickDeployedAt time.Time` (`omitempty`), set `time.Now()` | `MarkCanceled` UpdatedAt (`writer.go:230-231`) |
| `Status` | NOT overwritten | (unlike `MarkCanceled` "Canceled") — preserve the `Succeeded` provenance |
| Unknown runID | explicit error | `readRecord` (`writer.go:222-242`) |

**Rationale**: overwriting `Status` would erase the successful-validation fact and break
re-promote eligibility (`isRePromoteEligible`, `keys.go:801`). `QuickDeployedAt` is additive
and enables a double-deploy guard (see Deviations).

**Other decisions**: eligibility lives in `internal/runs` (exported `QuickDeployEligible`,
callable from app, testable like `selectPruneCandidates`) — mirrors `isRePromoteEligible`'s
purity, not its package. Production = `git.IsProductionTarget(cfg, rec.Target)` reusing
`environmentKeyForBranch`/`environmentPipelineOrder` (`source_suggestion.go:12,96`), i.e. env
key `"production"`. Surface key = `x` (verified FREE in `keyRunHistory`: used are
up/k, down/j, `d`, enter, `r`, q/esc — `keys.go:751-791`).

## Data Flow

    confirmTarget ──plan.TestLevel──► onBranchCreated (update.go:516)
        │                                   runs.Record{…, TestLevel: m.plan.TestLevel}
        ▼
    StateRunHistory ──[x on row]──► QuickDeployEligible(rec, m.now())
        eligible ────► StateQuickDeploy (shows: sf project deploy quick --job-id <rec.JobID> --target-org <rec.Alias>)
           │ suggest-only default
           │ gate: AllowExecution && (!IsProductionTarget || AllowProduction)
           ▼ type DESPLEGAR (quickConfirm) ──► quickDeployCmd (tea.Cmd, deps only)
                 m.deps.SF.QuickDeploy(ctx, alias, jobID) ──ok──► m.deps.Runs.MarkQuickDeployed(runID, raw)
                                                                     └─ quick.json + QuickDeployedAt

## File Changes

| File | Action | Description |
|------|--------|-------------|
| `internal/runs/writer.go` | Modify | Add `Record.TestLevel`, `QuickDeployedAt`; `MarkQuickDeployed` (mirror `MarkCanceled`) |
| `internal/runs/quick_deploy.go` | Create | Pure `QuickDeployEligible(rec, now) (bool, string)` + `requiredTestsRan` |
| `internal/salesforce/quick.go` | Create | `QuickDeployResult` + `QuickDeploy` (mirror `cancel.go`) |
| `internal/salesforce/client.go` | Modify | Add `QuickDeploy` to `Client` interface |
| `internal/config/config.go` | Modify | `QuickDeployConfig` + `Config.QuickDeploy` |
| `internal/git/source_suggestion.go` | Modify | Exported `IsProductionTarget(cfg, target)` |
| `internal/app/app.go` | Modify | `StateQuickDeploy` const; `quickConfirm` field |
| `internal/app/keys.go` | Modify | `x` in `keyRunHistory`; `keyQuickDeploy`; `quickDeployConfirmWord="DESPLEGAR"` |
| `internal/app/update.go` | Modify | Thread `TestLevel` at `runs.Record{}` (line 516); `onQuickDeployDone` |
| `internal/app/commands.go` | Modify | `quickDeployCmd` (mirror `cancelCmd`, deps-only) |
| `internal/app/view.go` | Modify | `StateQuickDeploy` render (command + gate/confirm) |
| test fakes (`delta_validation_test.go`/`resume_test.go` fake Client) | Modify | Implement new `QuickDeploy` interface method |

## Interfaces / Contracts

```go
// internal/runs
func QuickDeployEligible(rec Record, now time.Time) (eligible bool, reason string)
// Succeeded/SucceededPartial && requiredTestsRan(rec.TestLevel) && now.Sub(CreatedAt) < 10*24h
// reason ∈ {"not a successful validation","required tests not run","older than 10 days"}
func (w *Writer) MarkQuickDeployed(runID string, quickRaw []byte) error

// internal/salesforce (mirrors CancelDeploy positional args, no shell)
func (c *client) QuickDeploy(ctx context.Context, jobID, targetOrg string) (QuickDeployResult, error)
// Args: {"project","deploy","quick","--job-id",jobID,"--target-org",targetOrg,"--json"}

// internal/config
type QuickDeployConfig struct { AllowExecution, AllowProduction bool } // yaml allowExecution/allowProduction
// internal/git
func IsProductionTarget(cfg config.Config, target string) bool
// internal/app (pure gate)
func quickDeployExecAllowed(cfg config.Config, isProd bool) bool
```

## Testing Strategy

| Layer | What | Approach |
|-------|------|----------|
| Unit (runs) | `QuickDeployEligible` table: eligible / >10d / `RunSpecifiedTests` / `NoTestRun` / `""` / non-success; `requiredTestsRan`; `TestLevel` JSON round-trip; `MarkQuickDeployed` (`quick.json`+`QuickDeployedAt`, `t.TempDir()`) | table-driven, injected `now` |
| Unit (config/git) | `QuickDeploy` zero-value → both false; `IsProductionTarget` per Branches map | table-driven |
| Unit (salesforce) | `QuickDeploy` arg composition via recording `FakeRunner.Calls`; success+non-zero-exit(Raw) | `argcomposition_test.go`/`boundary_test.go` (no-shell, discrete args) |
| Unit (app) | `x` gated on eligibility; suggest-only default; prod block; no-confirm→no-exec; gate+`DESPLEGAR`→`QuickDeploy(jobID,alias)` + `MarkQuickDeployed` | direct `Model.Update()` |
| E2E | runs fixture + fake `sf project deploy quick` + injected clock; all 6 ACs; NO real org | fixture + `FakeRunner`, `-short` skip |

## Threat Matrix

Applicable — new subprocess `sf project deploy quick` composed positionally.

| Boundary | Applicability | Design response | Planned RED test |
|---|---|---|---|
| PR/argument composition | Applicable | `jobID`/`alias` as DISCRETE slice args, never shell-joined; `jobID`=`rec.JobID` (own run), `alias`=`rec.Alias` — never user-typed | recording-runner arg assertion (`QuickDeploy`) |
| Git repository selection | Applicable | Writer rooted at explicit `baseDir`; no cwd trust | `MarkQuickDeployed` under `t.TempDir()` |
| Documentation-like paths | N/A | No file-classification/exec-of-content boundary | — |
| Commit state | N/A | No git index/commit mutation | — |
| Push state | N/A | No push/ref resolution | — |

## Migration / Rollout

No migration. `Record.TestLevel`/`QuickDeployedAt` are `omitempty`, no `SchemaVersion` bump;
old `run.json` loads clean → fail-safe INELIGIBLE. Revert = delete the slice; prior files valid.

## Deviations / Refinements (flagged)

- **Eligibility placement**: exploration wrote `runs.quickDeployEligible` (lowercase); it MUST
  be exported (`QuickDeployEligible`) to be called from `internal/app`. No behavioral change.
- **Double-deploy guard (refinement)**: adding `rec.QuickDeployedAt.IsZero()` to eligibility
  prevents re-offering an already-quick-deployed run. Beyond the LOCKED 3-predicate — recommend
  spec/tasks adopt it; not silently added here.
- **Production detection**: pinned to config env-key `"production"` (per locked decision), NOT the
  literal `IsProductionBranch("main")` (`target_selection.go:18`). Consider OR-ing both for
  defense-in-depth if a repo omits a `production` branch mapping.

## Open Questions

- [ ] Adopt the `QuickDeployedAt.IsZero()` double-deploy guard into the eligibility contract? (spec)
- [ ] Single `StateQuickDeploy` (inline `quickConfirm` buffer) vs. separate `StateQuickDeployConfirm` — design picks single-state to minimize footprint; confirm at spec.
