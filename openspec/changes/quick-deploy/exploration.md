# Exploration: HU-015 — Quick Deploy Opcional (`quick-deploy`)

Ninth slice, stacked on `release`. HU-015 (`docs/HISTORIAS.md:1004-1037`) is **Fase Futuro / Prioridad Baja / Operativa**. Objetivo (explicit guardrail): support controlled release scenarios WITHOUT turning DeployDeck into a default deploy tool. Core deliverable: DETECT eligible successful validations + SHOW the suggested `sf project deploy quick --job-id <id> --target-org <alias>` command; NEVER execute by default. Execution is an OPT-IN capability behind config + strong confirmation + a production block. Composes existing capabilities; the ONLY real data gap is an additive `Record.TestLevel`.

## Current state (code-anchored)
- **`internal/runs.Record`** grows additively (JobID, Commits, PickIndex/Phase, PRUrl, SourceRunID). It HAS `CreatedAt`/`UpdatedAt time.Time` (age source) and `Status`/`JobID`. It does NOT persist the deploy **TestLevel** — that lives only on `git.DeploymentPlan.TestLevel` (`deployment_plan.go:26`, set by ConfirmTargetSelection HU-004) during the flow, never written to `run.json`. **This is the one additive gap.**
- **`internal/runs.Writer`**: `Create(rec, validateRaw)` (writes `validate.json` companion), `Save`, `Load`, `AppendReport` (report-NNN.json + status/UpdatedAt), `MarkCanceled` (HU-012 — records an action by writing a `cancel.json` companion verbatim + updating the Record). `MarkCanceled` is the exact idiom for recording a quick-deploy action (a `MarkQuickDeployed` writing `quick.json` + updating the Record).
- **`internal/salesforce`**: `Client` interface + `ValidateDeploy` (`validate --async --json` → JobID), `ReportDeploy` (`report --job-id --json` → DeployReport), `CancelDeploy`, `ListDeployQueue` — all compose args positionally via `exec.Runner`. NO `QuickDeploy` yet. `sf project deploy quick --job-id <id> --target-org <alias>` is a new method mirroring `CancelDeploy`'s shape (for the OPT-IN execute path only).
- **`internal/config`**: `Config`/`RunsConfig` + `Load`/`applyDefaults` (zero-value defaults). `environmentPipelineOrder`/`Branches` (RF-002, reused by HU-016 `NextEnvironmentBranch`) identify the "production" env. No `QuickDeploy` config yet.
- **TUI**: `StateRunHistory` has the per-row key pattern (`Enter`=resume, `d`=delete, `r`=re-promote from HU-016) — the entry point for a per-row quick-deploy affordance on an eligible row. The typed `CANCELAR`/`BORRAR` strong-confirm idiom (`keyCancelConfirm`) is reused for the opt-in execute. Injected clock exists (`m.now()`/`Now`) for the 10-day age rule.
- **Eligibility idiom**: `isRePromoteEligible(rec)` (HU-016) is the pure-function template — a `quickDeployEligible(rec, now, isProduction)` mirrors it.

## Resolved decisions
1. **Data gap → add additive `Record.TestLevel string` (`json:"testLevel,omitempty"`, no SchemaVersion bump)**, populated from `plan.TestLevel` at run-creation (`onBranchCreated`, alongside the existing fields). Age = `now.Sub(rec.CreatedAt) < 10*24h`. JobID already persisted (the validation job to quick-deploy). Backward-compat: old run.json loads with TestLevel "" → treated as "required tests NOT known to have run" → NOT eligible (fail-safe).
2. **Eligibility = pure function** `quickDeployEligible(rec Record, now time.Time) (eligible bool, reason string)` in `internal/runs` (testable like `selectPruneCandidates`): eligible ⇔ `Status ∈ {Succeeded, SucceededPartial}` AND TestLevel indicates REQUIRED tests ran (e.g. `RunLocalTests`/`RunAllTestsInOrg` — NOT `NoTestRun`/`RunSpecifiedTests`-without-required) AND `age < 10 days`. Return a `reason` so the view can explain non-eligibility (age vs test-level).
3. **Surface = `StateRunHistory`, a new per-row key** (e.g. `x` for "quick deploy" — verify it's free in `keyRunHistory`) on an eligible row → a quick-deploy view showing the SUGGESTED COMMAND (never auto-run). Suggest-only is the default; the opt-in execute is a sub-action.
4. **Config = new `QuickDeployConfig{ AllowExecution bool; AllowProduction bool }`** (both default `false` → suggest-only + prod-blocked; zero-value-safe like the other config sections — no `applyDefaults` entry needed since false is the safe default). Production identified via the `Branches`/`environmentPipelineOrder` "production" mapping.
5. **Execution path IN this slice, behind the gate, FULLY FAKED**: the story's Test E2E explicitly asserts the execute path (`sf project deploy quick` with the correct `--job-id`, action registered). Build it: only when `AllowExecution` AND (target not production OR `AllowProduction`) AND a typed strong-confirm passes → run the new `salesforce.QuickDeploy` (faked in tests) → `MarkQuickDeployed` records it. Without the gate/confirm → suggest-only, no exec.
6. **Scope OUT**: real production quick deploy (only faked/alias-local); making DeployDeck a default deploy tool (Objetivo guardrail); any change to validation/report machinery. Also OUT: winget/etc. **Note for proposal**: HU-015 is "Fase Futuro", OUTSIDE `openspec/config.yaml`'s Fase 1–5 enumerated list — the proposal must reconcile/note this (state phase as "Futuro (out of the Fase 1–5 delivery list)").

## New capability + surface
- **NEW spec domain `quick-deploy`** (no existing spec covers it). Modified (additive deltas): `run-persistence` (`Record.TestLevel`), possibly `run-history` (the `x` action). Reuses `run-retention`/RF-002 config unchanged.
- New: `Record.TestLevel` + `Writer.MarkQuickDeployed`; `runs.quickDeployEligible`; `salesforce.QuickDeploy`; `config.QuickDeployConfig`; app `StateQuickDeploy` (or a sub-mode on run-history) + key + strong-confirm + view.

## ACs → testability (`docs/HISTORIAS.md:1018-1037`; runs fixture + fake `sf project deploy quick` + injected clock; NO real org, CI: fake)
- eligible (Succeeded + required tests + <10 days) → shows command with its `--job-id`.
- >10 days OR required tests not run → NOT offered (two variants: age, test-level).
- production target → not executed without `AllowProduction`.
- no strong confirm → no deploy.
- `AllowExecution` + strong confirm + permitted target → runs `sf project deploy quick --job-id <correct> --target-org <alias>`, action registered on the run (`quick.json` + Record).
- backward-compat: old run.json (no TestLevel) → not eligible (fail-safe).

## Risks
- **Additive-schema discipline** on `Record.TestLevel` (omitempty, no bump; old runs load fail-safe-ineligible).
- **Confirm-buffer collision**: reuse the strong-confirm idiom with a DEDICATED field (don't share `deleteConfirm`/`cancelInput`) — same lesson as HU-017's `BORRAR` field.
- **Config zero-value safety**: `AllowExecution=false`/`AllowProduction=false` defaults MUST mean suggest-only + prod-blocked (never accidentally execute).
- **Fake-only execution**: the execute path is faked in CI; real `sf project deploy quick` only via a local alias (Futuro). Never a real production deploy in tests.
- **Fase Futuro vs config.yaml phase list** — reconcile in proposal.
- Likely >400 lines (Record field + eligibility + salesforce method + config + app state/key/view/confirm + e2e) → sdd-tasks should group for batched apply; single stacked branch, no origin.

## Ready for Proposal: yes
Anchor on the REAL `Record` (add `TestLevel`), reuse `isRePromoteEligible`/`MarkCanceled`/strong-confirm idioms, default to suggest-only + prod-blocked, build the faked execute path behind the gate.
