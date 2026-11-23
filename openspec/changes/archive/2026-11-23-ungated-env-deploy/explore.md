# Exploration: ungated-env-deploy

## Summary

The ungated deploy path already works. A target with no `gates` entry, or with `enabled: false`, skips the PR, approval and signature checks. Push and PR are opt-in. A DEV sandbox can be configured without any code change.

Four things are missing:

1. Documentation of the developer use case.
2. A "deploy now" offer on the validation result screen.
3. Guidance on a fail-closed classification. `IsProductionTarget` treats a branch that is not listed under `branches` as production.
4. Explicit statement of the ungated default in the README.

## Q1. Does the promotion flow force push and PR?

No.

- `onReportDone` routes a terminal status to `StateSucceeded`. See `update.go:1238` and `terminalState` in `app.go:902-912`.
- `keySucceeded` (`keys.go:97-139`) offers these keys:
  - `p` for push, which leads to `StatePushPreparation`.
  - `d` to delete the branch.
  - Quit.
- Each side effect in `keyPushPreparation` (`keys.go:434-547`) needs an explicit key.
- The validation comment is posted only for gated targets. See `maybeTriggerValidationCommentCmd` in `update.go:1261-1270`.
- Quick deploy from run history (`x`, `keys.go:1418-1436`) consults `GateFor(rec.Target)` at Enter (`keys.go:1597`):
  - Ungated targets go straight to `quickDeployCmd`.
  - Gated targets go to `gateCheckCmd`.

## Q2. `branches` config and targets

- `git.ListDestinations(cfg)` iterates over `cfg.Branches` (`git/target_selection.go:52-71`). The keys are free-form strings.
- `environmentPipelineOrder = {integration, uat, production}` (`git/source_suggestion.go:12`) only drives suggestions.
- `confirmTarget` (`keys.go:1106-1123`) only requires that `SandboxFor(branch)` resolves.
- `IsProductionTarget` (`git/source_suggestion.go:121-130`) is fail-closed:
  - A target that is not mapped in `cfg.Branches` counts as production and needs `allowProduction`.
  - A branch whose value is `main` is also production.
- The DEV branch must therefore be listed under `branches` with a key other than `production`.

## Q3. Quick deploy prerequisites and "deploy now" feasibility

`runs.QuickDeployEligible` (`internal/runs/quick_deploy.go:37-51`) requires all of the following:

- The status is `Succeeded` or `SucceededPartial`.
- Required tests ran, so `NoTestRun` is not eligible.
- The run is less than 10 days old.
- The run has not been quick-deployed yet.

`quickDeployExecAllowed` (`keys.go:1468`) is `AllowExecution && (!isProd || AllowProduction)`.

`StateQuickDeploy` is driven by `m.runs[m.runsCursor]`. Esc or q returns to `StateRunHistory` (`keys.go:1624,1630`).

The result-screen offer is feasible. It needs three pieces:

1. Load the just-finished run record into `m.runs` and `m.runsCursor`.
2. Add an origin flag so that Esc or q returns to `StateSucceeded`.
3. Reuse the `DESPLEGAR` flow.

To verify during design: the run record's Status, JobID, Alias and TestLevel are persisted by terminal time.

## Q4. Keymap and parity

- `keymap.go:186-192` holds the `StateSucceeded` bindings. `keys.Quick` already exists at `keymap.go:218`.
- Tests to touch:
  - `footer_parity_test.go`
  - `esc_q_matrix_test.go:103`, which needs an origin variant
  - `keymap_states_test.go:82`
  - `quick_deploy_test.go`
  - `run_history_menu_test.go`
- No new state is needed, so `stateCount` stays the same.

## Q5. Docs and specs

- **README.md:** add a subsection on deploying to a test environment. The relevant lines are 136-169 and 187.
- **docs/HISTORIAS.md:** add HU-020, since the last story is HU-019 at line 1151. Also add a row to the e2e table at line 1287 and review the stale HU-015 wording at line 987.
- **docs/MOCKUPS_TUI.md:** update the quick deploy mockup at line 420.
- **Spec deltas:**
  - `deploy-gate`
  - `quick-deploy`
  - `tui-presentation`
  - Optionally `push-pr-preparation` and `target-selection`

## Q6. Risks

- **Gated targets:** never offer the result-screen deploy for them, and keep `keys.go:1597` unchanged.
- **Production:** `quickDeployExecAllowed` and `IsProductionTarget` still apply. Hide the offer for production targets.
- **Typed confirmation:** keep the `DESPLEGAR` confirmation.
- **Ineligible runs** (`NoTestRun`, expired, already deployed): show the reason using `quickDeployReasonES`.
- **Shared in-flight guards:** `quickDeployingRunID` and `gateCheckingRunID` must keep applying.

## Recommended scope

**Code:**

- Add `x` on `StateSucceeded`, shown only for runs that are ungated, not production and eligible.
- Load the run record when the key is pressed.
- Add an Esc/q origin flag that returns to the result screen.
- Add a footer hint.
- Add a result-screen line.
- Add tests for all of the above.

**Docs:**

- A README subsection.
- HU-020 and its e2e table row.
- The mockup update.
- Spec deltas for `deploy-gate`, `quick-deploy` and `tui-presentation`.

**Estimate:** about 415–610 changed lines after applying the 1.6x correction.
