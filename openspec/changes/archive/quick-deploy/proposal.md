# Proposal: HU-015 — Quick Deploy Opcional (`quick-deploy`)

**Phase**: Futuro (out of the enumerated Fase 1–5 delivery list in `openspec/config.yaml`). HU-015 is documented "Fase Futuro / Prioridad Baja"; this is intentional — the proposal does NOT claim a numbered phase.

## Intent

After a validation succeeds, Salesforce lets you "quick deploy" that validated result — reusing its `job-id`, no re-run — within 10 days. HU-015 DETECTS eligible successful validations and SHOWS the suggested `sf project deploy quick --job-id <id> --target-org <alias>` command. **Suggest-only by default.** Objetivo guardrail: this is NOT a default deploy tool. Execution is strictly OPT-IN behind config + strong confirmation + a production block.

## Scope

### In Scope
- **Additive `Record.TestLevel string`** (`json:"testLevel,omitempty"`, NO SchemaVersion bump), set from `plan.TestLevel` at run creation. Old `run.json` loads with `""` → fail-safe INELIGIBLE.
- **Pure `quickDeployEligible(rec, now) (eligible bool, reason string)`** in `internal/runs` (mirrors `isRePromoteEligible`): eligible ⇔ Status Succeeded/SucceededPartial AND required tests ran (TestLevel) AND age < 10 days. `reason` explains non-eligibility (age vs test-level).
- **`StateRunHistory` per-row key** on an eligible row → a quick-deploy view showing the suggested command (never auto-run).
- **`QuickDeployConfig{ AllowExecution bool; AllowProduction bool }}`** — both default `false` → suggest-only + prod-blocked; zero-value-safe (no `applyDefaults` entry).
- **OPT-IN execute path**, behind `AllowExecution` + (non-production OR `AllowProduction`) + typed strong-confirm on a DEDICATED confirm field: new `salesforce.QuickDeploy` (`sf project deploy quick --job-id <id> --target-org <alias>`, mirrors `CancelDeploy`), then `Writer.MarkQuickDeployed` records the action (`quick.json` companion + Record, mirroring `MarkCanceled`). Built NOW, FULLY FAKED — the story's Test E2E asserts it.

### Out of Scope
- Real production quick deploy (faked / alias-local only; never a real prod deploy in tests).
- Any change to validation/report machinery.
- Making DeployDeck a default deploy tool (Objetivo guardrail). winget/etc.

## Capabilities

> Contract for sdd-spec.

### New Capabilities
- `quick-deploy`: eligibility detection + suggested-command surface + gated opt-in faked execute with action registration.

### Modified Capabilities
- `run-persistence`: additive `Record.TestLevel` field.
- `run-history`: new per-row quick-deploy action on an eligible run.

(No other living spec changes; `run-retention`/RF-002 config reused unchanged.)

## Approach

Compose existing idioms. `Record.TestLevel` is the ONLY real data gap (`plan.TestLevel` at `deployment_plan.go:26` is never persisted today). Eligibility is a pure, table-testable function. Surface reuses the `StateRunHistory` per-row key + strong-confirm patterns. Execution reuses the `exec.Runner` positional-args and `MarkCanceled` companion-file idioms — faked in CI, gated so zero-value config can never execute.

## Flagged for sdd-design

Pin the EXACT Salesforce TestLevel values DeployDeck's HU-004 flow produces — inspect `internal/git` `ConfirmTargetSelection` / target-selection code for the actual strings set (`NoTestRun` / `RunLocalTests` / `RunAllTestsInOrg` / `RunSpecifiedTests`) so "required tests ran" maps to real values, not assumptions.

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `internal/runs` (Record, Writer) | Modified | `TestLevel` field + `quickDeployEligible` + `MarkQuickDeployed` (`quick.json`) |
| `internal/salesforce` | New | `QuickDeploy` method (`sf project deploy quick`) |
| `internal/config` | New | `QuickDeployConfig{AllowExecution, AllowProduction}` |
| `internal/app` (TUI) | Modified | Per-row key, quick-deploy view, dedicated strong-confirm |
| e2e test | New | Fixture + fake `sf project deploy quick` + injected clock |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| Additive-schema break | Low | `omitempty`, no bump; old runs load fail-safe-INELIGIBLE |
| Confirm-buffer collision | Med | DEDICATED confirm field (not shared `deleteConfirm`/`cancelInput`) |
| Config zero-value executes | Low | `false`/`false` MUST mean suggest-only + prod-blocked |
| Real prod deploy in tests | Low | Execute path FAKED in CI; real only via local alias (Futuro) |
| >400-line footprint | High | sdd-tasks group for batched apply; single stacked branch, no origin |

## Rollback Plan

Additive-only. Revert the slice: `Record.TestLevel` is `omitempty` (existing runs unaffected); no schema migration. Removing the feature leaves prior run.json files valid. No shared Git/Salesforce state is mutated in suggest-only default; execute is faked in CI.

## Dependencies

- Stacked on `release` (ninth slice). Reuses `Branches`/`environmentPipelineOrder` (RF-002) to identify "production".

## Success Criteria

- [ ] Eligible run (Succeeded + required tests + <10 days) shows the command with its `--job-id`.
- [ ] Non-eligible by age (>10 days) or by test-level → not offered (both variants).
- [ ] Production target → not executed without `AllowProduction`.
- [ ] No strong confirm → no deploy.
- [ ] `AllowExecution` + strong confirm + permitted target → runs `sf project deploy quick --job-id <correct> --target-org <alias>`; action registered (`quick.json` + Record).
- [ ] Old run.json (no TestLevel) → INELIGIBLE (fail-safe).
