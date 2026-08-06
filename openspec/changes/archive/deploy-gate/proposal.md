# Proposal: Deploy Gate (per-environment, opt-in governance check before quick-deploy)

## Intent

Give teams an opt-in, per-environment gate that DeployDeck enforces LOCALLY before it performs
the real deployment (quick-deploy). For an environment listed in config, the run's PR must satisfy
four governance conditions or the deploy is blocked. Success: a gated env cannot be quick-deployed
unless its PR is approved, thread-clean, DeployDeck-validated, and provenance-signed; ungated envs
are unchanged. Milestone: v1.3.0.

## Problem / Motivation

- Quick-deploy IS the real deploy (`salesforce.QuickDeploy`, `quick.go:41-68`), fired from
  `keyQuickDeploy` "enter" (`keys.go:1432-1511`); the only gates today are an in-flight guard,
  `QuickDeployEligible`, `quickDeployExecAllowed`, and a typed `"DESPLEGAR"` (`keys.go:1455-1467`).
  Nothing checks the PR's review state, so a validated-but-unreviewed change deploys.
- DeployDeck reads no approvals, no thread-resolution, and posts no PR record today
  (`github.Client` has only `AuthStatus/CreatePR/PRForBranch/PRDetails`, `client.go:26-63`).
- There is no durable audit record that a given validation actually passed inside DeployDeck.

## Threat / Trust model (stated honestly)

This is a COOPERATIVE local control, same honesty class as PR provenance: it enforces team
discipline and deters casual bypass, but is bypassable by editing config or the binary. It is NOT
a hard security boundary. The hard equivalent — GitHub branch protection + a required CI check —
is explicitly out of scope and noted as the future path.

## Scope

### In Scope
- New per-env `Gates` config keyed by branch/glob (mirrors `Sandboxes`/`SandboxFor`), default-off,
  zero-value-safe (`QuickDeployConfig` precedent); validation of `minApprovals`≥1 and non-empty
  approver list when a gate is enabled.
- A local gate inserted between the typed `"DESPLEGAR"` confirmation and quick-deploy dispatch.
- Four conditions, each independently toggleable, all default-on when the gate is enabled:
  (1) ≥`minApprovals` (default 1) approvals from the configured approver list; (2) no unresolved
  review threads; (3) a DeployDeck validation comment present; (4) valid provenance signature.
- After a successful CheckOnly, post/upsert an automatic PR validation comment (audit record +
  condition-3 signal) — only when the target env's gate is enabled.
- A gate-block screen listing the unmet conditions.

### Out of Scope
- GitHub Actions / branch-protection enforcement (the hard gate).
- Any new real-deploy command — quick-deploy is it.
- Gating environments not opted-in.
- Team/org-membership expansion of approvers (literal handle list only).
- No in-app override / bypass UI.

## Approach — Decisions

| # | Decision | Rationale |
|---|----------|-----------|
| D1 | `Config.Gates map[string]GateConfig` keyed by branch/glob, resolved by a new `GateFor(target)` (exact then `path.Match`, mirroring `SandboxFor`); default-off, zero-value-safe. Config owned by the new `deploy-gate` capability, like `QuickDeployConfig` is owned by `quick-deploy`. | Reuses the proven keying convention and the safe zero-value pattern; no separate config capability to modify. |
| D2 | Gate runs locally as an async `gateCheckCmd` (mirrors `quickDeployCmd`, `commands.go:1271-1281`) on a gate sub-state of `StateQuickDeploy`, inserted between `keys.go:1464-1467` and dispatch `keys.go:1477`, re-checked at the point of no return like the `QuickDeployEligible` re-check (1455-1458). | Quick-deploy is the real deploy; the word-check→dispatch seam is the only correct insertion point; async + re-check matches existing gate precedents. |
| D3 | Four conditions, each toggleable in `GateConfig`, all default-on when the gate is enabled. | Opt-in per env, but a gate that is enabled is strict by default. |
| D4 | PR resolution order: `rec.PRUrl` → fallback `PRForBranch(RenderBranchName(...))` (reuse `selectOrphans` re-derivation, `commands.go:367-390`). If NO PR resolves, FAIL-CLOSED and block (mirror `IsProductionTarget` fail-closed). | `PRUrl` is frequently unset (standalone/cross-session); a governance gate must never deploy on an unverifiable PR. |
| D5 | Unresolved threads via `gh api graphql` `reviewThreads.isResolved` (project's first GraphQL call); extend `ParsePRURL` (`compare_url.go:94-99`) to keep the discarded PR number `m[4]`. If GraphQL/auth is unavailable, FAIL-CLOSED. | No REST field exists for thread resolution; for a governance gate, treat-as-unknown is unsafe — fail-closed is the safe default. |
| D6 | The validation comment carries a hidden dedup marker HTML comment `<!-- deploydeck-validation: job:<jobId> ... -->` (like provenance). On re-validation, UPDATE/skip rather than double-post; condition-3 detection keys on the marker, not free text. | First outward write besides `CreatePR`; idempotency prevents comment spam on retries/resumes and gives a stable, forge-resistant signal. |
| D7 | Aggregate coverage in the comment is COMPUTED from `CodeCoverage` (sum covered / sum total); `DeployReport` has no aggregate `%`. | Gives a meaningful single number without a schema change; per-class data already exists (`report.go:57-63`). |
| D8 | Approver matching normalizes handles (strip leading `@`, case-insensitive GitHub login compare); bot/team handles match literally as logins. Approvals read via `gh pr view --json reviews,latestReviews,reviewDecision`; only listed handles count. | No normalization precedent existed; literal-login matching keeps semantics predictable and avoids implying org/team expansion. |
| D9 | On failure, block and render a dedicated gate screen listing every unmet condition; NO in-app override. | A cooperative gate needs no override — bypass is honestly at the config/binary layer, not a hidden button. |
| D10 | The auto-comment is posted only when the target env's gate is enabled. | Avoids PR noise on ungated environments. |
| D11 | Provenance is REUSED, not modified: `ParsePRURL`→`PRDetails`→`ParseMarkers`→`Verify` (the `cmd/deploydeck/pr.go:86-148` composition). | Condition-4 is exactly the existing verify path; no new provenance primitive is needed. |

## Capabilities

### New Capabilities
- `deploy-gate`: the per-env `Gates` config schema + validation; the four gate conditions and their
  toggles; fail-closed PR resolution; block/allow semantics + gate-block screen; the validation
  comment definition (hidden dedup marker, content, idempotent upsert) and when it is posted.

### Modified Capabilities
- `quick-deploy`: a gate check runs between the typed `"DESPLEGAR"` confirmation and dispatch; a
  gated target is blocked unless its resolved gate passes; ungated targets deploy as today.
- `validation-progress`: at terminal successful CheckOnly, trigger the deploy-gate validation
  comment post/upsert to the run's PR when the target env's gate is enabled.

## Affected Areas

| Area | Impact | Change |
|------|--------|--------|
| `internal/config/config.go` (68-79, 103-109) | Modified | Add `Gates map[string]GateConfig` + `GateConfig` struct (mirror `QuickDeployConfig`/`Sandboxes`). |
| `internal/config/validate.go` (19-59) | Modified | Validate `minApprovals`≥1 and non-empty approvers when a gate is enabled. |
| `internal/config/` (new `GateFor`) | New | Branch/glob gate lookup (exact then `path.Match`), mirroring `SandboxFor`. |
| `internal/github/client.go` (26-63) | Modified | New methods: approvals read (`gh pr view --json`), review threads (`gh api graphql`), comment upsert (`gh pr comment`). |
| `internal/github/compare_url.go` (94-99) | Modified | Extend `ParsePRURL` to return the PR number (`m[4]`). |
| `internal/app/keys.go` (1464-1477) | Modified | Insert async gate between word-check and dispatch; re-check at point of no return. |
| `internal/app/app.go` (516-528) | Modified | Gate sub-state on `StateQuickDeploy` (mirror `quickDeployingRunID` guard). |
| `internal/app/commands.go` (1271-1281, 367-390) | Modified | `gateCheckCmd`; branch re-derivation for PR fallback. |
| `internal/app/update.go` (1105-1162) | Modified | `onReportDone` terminal success posts the validation comment (gate-enabled env only). |
| `internal/app/view.go` | Modified | Gate-block screen listing unmet conditions. |

## Risks

| Risk | Likelihood | Mitigation |
|------|-----------|------------|
| First `gh api graphql` call — heavier/different error shape, zero precedent | High | D5: isolate in a Client method; fail-closed on auth/GraphQL error. |
| Approver-handle matching (case, `@`, bot/team) has no normalization precedent | Med | D8: explicit normalize + literal-login rule, documented in the spec. |
| Comment posting is a new outward write with no idempotency precedent | Med | D6: hidden dedup marker + upsert/skip; retries/resumes never double-post. |
| `PRUrl` frequently unset → gate correctness depends on the fallback | High | D4: `PRForBranch(RenderBranchName)` fallback; fail-closed when no PR resolves. |
| Cooperative control is bypassable (config/binary) | Accepted | Trust model stated honestly; hard gate (branch protection + CI) noted as future path. |
| >400 changed lines across config + github + app under strict TDD | High | Single PR, `size:exception` acceptable. |

## Rollback Plan

Default-off and additive. `Gates` is zero-value-safe, so an env with no gate entry behaves exactly
as today and existing `deploydeck.yaml`/`run.json` files are unaffected (no schema bump). Revert is
localized: remove the `gateCheckCmd` call at `keys.go`, the `onReportDone` comment post, and the new
`github.Client` methods; the `internal/salesforce` exec surface is untouched throughout.

## Success Criteria

- [ ] A gated env blocks quick-deploy unless all four (enabled) conditions pass; ungated envs deploy unchanged.
- [ ] No resolvable PR (`rec.PRUrl` empty and `PRForBranch` finds none) → deploy is BLOCKED, not attempted.
- [ ] Approvals count only configured handles (normalized); `minApprovals` default 1 respected.
- [ ] Any unresolved review thread blocks; GraphQL/auth failure blocks (fail-closed).
- [ ] Condition-3 passes only when the hidden-marker validation comment is present; re-validation never double-posts.
- [ ] Condition-4 passes only on a valid provenance signature via the existing verify path.
- [ ] A successful CheckOnly on a gate-enabled env posts/updates the validation comment; ungated envs get none.
- [ ] Gate failure renders a screen listing the unmet conditions; there is no in-app override.

## Delivery Note

Single PR, strict TDD. `size:exception` acceptable given the fan-out across `internal/config`,
`internal/github` (new reads + first GraphQL + first outward comment write), and `internal/app`
(gate state, keys seam, terminal-comment seam, gate screen). Milestone v1.3.0. Next recommended
phases: `sdd-spec` and `sdd-design` (parallel).
