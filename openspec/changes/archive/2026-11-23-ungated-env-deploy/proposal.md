# Proposal: Ungated Environment Deploy

## Intent

Developers deploying to a test environment such as DEV assume they must go through the strict PR gate. They don't: ungated targets already skip it, but this is undocumented. After a successful validation, they also have to dig through run history to deploy. This change offers the deploy from the validation result screen and documents the ungated path.

**Delivery phase:** Fase 5 Productizacion (documentation, UX). It extends HU-015 quick deploy.

## Scope

### In Scope
- `x` "desplegar" on `StateSucceeded`. It is shown only when all of these hold:
  - the target is ungated (`config.GateFor` is false);
  - the target is not production;
  - `runs.QuickDeployEligible` passes.
- Pressing `x` opens the existing `StateQuickDeploy` with the `DESPLEGAR` typed confirmation. No new state.
- Ineligible runs show the reason via `quickDeployReasonES`.
- When `allowExecution: false`, the offer is shown as unavailable, with the reason. It never executes.
- An origin flag makes Esc/q return to `StateSucceeded`.
- Docs:
  - a README section, "Deploying to a test environment";
  - HU-020 plus its e2e table row in `docs/HISTORIAS.md`;
  - a mockup tweak in `docs/MOCKUPS_TUI.md`.

### Out of Scope
- A new config opt-in key.
- Changing `git.IsProductionTarget`.
- Changing the gated flow (`keys.go` ~1597).
- Changing the run-history `x` path.
- Auto-deploy without confirmation.

## Capabilities

### New Capabilities
None

### Modified Capabilities
- `deploy-gate`: ungated non-production targets get a result-screen deploy offer. Gated targets never get it.
- `quick-deploy`: quick deploy can be entered from the validation result, with return-to-origin navigation.
- `tui-presentation`: `StateSucceeded` footer hint, offer/reason line, Esc/q matrix origin variant.

## Approach

When `x` is pressed, load the just-finished run record into `m.runs`/`m.runsCursor`. Then reuse `StateQuickDeploy`, `quickDeployExecAllowed`, and the in-flight guards (`quickDeployingRunID`). Add a `quickDeployOrigin` flag for Esc/q. Design must confirm that Status, JobID, Alias, and TestLevel are persisted by terminal time.

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `internal/app/keys.go` | Modified | `keySucceeded` `x`, quick-deploy Esc/q origin |
| `internal/app/keymap.go`, `view.go` | Modified | Hint and offer/reason line |
| `internal/app/*_test.go` | Modified | Footer parity, esc/q matrix, keymap states, quick deploy |
| `README.md`, `docs/HISTORIAS.md`, `docs/MOCKUPS_TUI.md` | Modified | Documentation |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| A test branch that is not listed in `branches` is treated as production, so the offer is hidden | Med | Document this footgun in the README |
| The offer leaks onto gated targets | Low | Gate check inside the offer predicate; negative tests |
| The run record is incomplete at terminal time | Low | Verify in design; fall back to hiding the offer |

## Rollback Plan

Revert the single PR. No Git branch state, persisted run schema, or sandbox state changes. Quick deploy runs only after typed confirmation and its effects are the same as the existing run-history path. A deploy that has already executed is reverted through normal Salesforce means.

## Dependencies

- Existing HU-015 quick deploy and deploy-gate implementation.

## Success Criteria

- [ ] On an ungated, non-production, eligible target, validation success then `x` then `DESPLEGAR` deploys the run, and Esc returns to the result screen.
- [ ] Gated and production targets show no offer. `allowExecution: false` never executes.
- [ ] The README, HU-020, and the mockup describe the flow. `go test ./...` passes.
