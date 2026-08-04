# Proposal: Incremental Promotion (reuse-on-collision)

## Intent

When a `deploy/<ticket>-to-<target>` branch already exists, `CreatePromotionBranch` returns `ErrPromotionBranchExists` and `onBranchCreated` dead-ends into `StateError`. The user must delete the branch — discarding the open PR and its review history — just to append commits that landed on source after the first run. Re-promoting after new commits is a routine workflow; delete-and-recreate is data loss. Success: on collision the user can REUSE the branch, append only the not-yet-present commits, and update the existing PR — no manual git, no lost review. Delivery phase: Fase 1 (MVP Git).

## Scope

### In Scope
- New `StateBranchCollision` confirm (reuse & append / delete & recreate / cancel), entered ONLY from `onBranchCreated`'s `ErrPromotionBranchExists` branch.
- Layered already-on-branch detection → filter `plan.SelectedCommits` to the not-yet-present subset before cherry-pick.
- New primitives: `internal/git` `-x` trailer parser; `internal/github` `PRForBranch`.
- Increment persisted by mutating the SAME RunID in place.
- Empty-after-filter and diverged-deploy-branch guards.

### Out of Scope
- Proactive run-history "continue this run" entry (Approach 2) — later change.
- Conflict machinery, `CommitsInRange`, `verifyCmd` base (`origin/<target>`), `CherryPick`, `Checkout`, `Push` — unchanged (confirmed by exploration).
- No `SchemaVersion` bump; increment is NOT a new `SourceRunID` record.

## Decisions (D1–D5)

| # | Decision | Rationale |
|---|----------|-----------|
| D1 | Collision UX = new `StateBranchCollision` confirm; reuse checkout via existing `Service.Checkout`. Never silent. | Mirrors `StateCancelConfirm`/`StateSourceConfirm`; no new checkout primitive. |
| D2 | Layered classifier vs DEPLOY branch: `IsAncestor` → `-x` trailer match (PRIMARY, net-new `git log` parser) → `Cherry` fallback. Filter selection; `CherryPick` unchanged. | Trailer is exact and survives conflict-edits that break patch-id. |
| D3 | New `github.Client.PRForBranch` via `gh pr view --json url,state`, exit-code-as-data, 4-state degrade (open/closed/none/gh-absent→manual push+confirm). Skip create when open. `Record.PRUrl` is a non-authoritative hint. | Avoids `gh pr create` "already exists" failure; degrades like `AuthStatus`. |
| D4 | Mutate same RunID (extend `Commits`, bump `PickTotal`/`UpdatedAt`, keep `CreatedAt`) via a pure `RenderBranchName`-over-`List()` prior-run finder. | `SourceRunID` is for cross-env re-promotion, not the same branch. |
| D5 | Empty-after-filter → explicit app-layer notice, not a raw `CherryPick` empty error. Diverged remote deploy branch: fetch and fast-forward the reused branch to `origin/<deploy>` when ahead; on non-ff divergence surface a clear error, never silent overwrite. | Never expose raw git failures; never clobber remote work. |

## Capabilities

### New Capabilities
- `incremental-promotion`: `StateBranchCollision` decision, layered already-on-branch detection + not-yet-present filtering, empty-after-filter guard, diverged-branch fetch/ff guard, same-RunID increment orchestration.

### Modified Capabilities
- `promotion-branch`: "Existing Temp Branch Collision Handling" now specifies reuse & append / delete & recreate / cancel (reuse is the new path).
- `push-pr-preparation`: gate PR creation on `PRForBranch` — skip create when a PR is already open; 4-state graceful degrade.
- `run-persistence`: incremental append mutates the same run record in place (extend `Commits`, bump `PickTotal`/`UpdatedAt`, preserve `CreatedAt`).
- `cherry-pick`: NOT modified — spec unchanged. `CherryPick` and conflict machinery are position-agnostic; filtering lives in `incremental-promotion`. (Corrects the scope brief after grounded reading.)

## Affected Areas

| Area | Impact | Change |
|------|--------|--------|
| `internal/app/update.go` (`onBranchCreated`) | Modified | Route `ErrPromotionBranchExists` → `StateBranchCollision`; filter + increment |
| `internal/git` | New | `-x` trailer parser; reuse layered detection |
| `internal/github` | New | `PRForBranch` |
| `internal/runs` | Modified | Prior-run-for-branch finder; same-RunID mutation |

## Risks

| Risk | Likelihood | Mitigation |
|------|-----------|------------|
| Diverged non-ff remote deploy branch overwritten | Med | D5: fast-forward only; clear error on divergence |
| `PRForBranch` hard-blocks when `gh` absent/unauth | Med | 4-state degrade → manual push+confirm |
| Boundary leak (exec inline in `internal/app`) | Low | Primitives in `git`/`github` only |
| >400 changed lines (strict-TDD surface) | High | Single-PR, size:exception pre-accepted (1.0.0) |

## Rollback Plan

Behavior is additive and gated on collision. Revert the change to restore `StateError` dead-end; existing runs/branches unaffected (no schema bump, no `SourceRunID`). No migration.

## Success Criteria

- [ ] Collision offers reuse & append / delete & recreate / cancel; never silent.
- [ ] Reuse appends only not-yet-present commits and updates the open PR (no duplicate PR).
- [ ] Empty-after-filter shows an explicit notice, not a raw git error.
- [ ] Diverged non-ff deploy branch surfaces a clear error, never a silent overwrite.
- [ ] Increment mutates the same RunID; old `run.json` still loads.

## Proposal Question Round (executor is not in direct user contact)

One genuine product gap remains; spec/design should confirm:
1. Diverged non-ff deploy branch — is "hard error, user resolves manually" the right first slice, or should reuse offer an in-flow rebase/force option? (Assumption: hard error; recommend for MVP.)
2. Reused branch whose open PR is CLOSED/MERGED — reopen, create a fresh PR, or notify-only? (Assumption: fall through to manual push+confirm.)
