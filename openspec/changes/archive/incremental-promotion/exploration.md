# Exploration — incremental-promotion

Reuse an existing `deploy/<ticket>-to-<target>` branch and append only the NEW commits (updating the
open PR), instead of recreating from scratch or forcing the user to delete the branch. Verified
against the code.

## Current state

- Flow: `StateCommitSelection → StateTargetSelection → StatePlanPreview → StateBranchCreation →
  StateCherryPicking → ... → StatePushPreparation`.
- `CreatePromotionBranch` (`service_promotion.go:38`): `fetch` → `BranchExists` → `checkout -b <branch>
  origin/<target>`. If the branch already resolves (local or origin/), returns `ErrPromotionBranchExists`
  BEFORE any git state change. Caller `branchCreateCmd`→`onBranchCreated` (`update.go:567`) today routes
  ANY error to `StateError` (dead end) — NO reuse UX exists (despite the error's aspirational doc).
- Prior art: `promotion-polish` (Change A, archived) added `cherry-pick -x` SPECIFICALLY to seed this
  feature — every picked commit carries `(cherry picked from commit <sha>)`, robust across
  conflict-resolution edits that break patch-id.
- HU-016 re-promotion (`RemapCommitsByPatchID`, `startRePromoteInto`) is the closest analog (prior-run
  SHAs → seeded selection) but always fresh-branch, different target — NOT incremental.

## The 6 seams (verified)

1. **Collision routing**: `onBranchCreated` special-cases `errors.Is(err, git.ErrPromotionBranchExists)`
   → new confirm state (mirrors how `onDiscoverDone` handles `resolveNeedsConfirm`). Reuse checkout uses
   the EXISTING `Service.Checkout(dir, branch)` (`service_cleanup.go:56`, `git checkout <branch> --`) — no
   new git primitive for the checkout.
2. **Already-on-branch detection**: layered, mirroring `ClassifyEquivalence`'s priority chain, but pointed
   at the DEPLOY branch instead of `origin/<target>`: `IsAncestor(sha, deploy-branch)` (exists) → **`-x`
   trailer match (PRIMARY** — parse `git log <deploy-branch>` for `(cherry picked from commit <SHA>)`;
   NET-NEW small pure parser mirroring `ParseCherryOutput`) → `Cherry(<deploy-branch>, <source>)` fallback
   (existing method, new target). Trailer is primary because it's exact and survives conflict edits.
3. **Cherry-pick of only new commits**: `CherryPick` picks onto whatever HEAD is — needs ZERO changes.
   Only the app layer filters `plan.SelectedCommits` to the not-already-on-branch subset before
   `cherryPickCmd`. Conflict machinery (`ContinueCherryPick`/`Skip`/`Abort`/`StateCherryPickConflict`/
   `RepoState`) is position-agnostic → unchanged.
4. **Push / existing PR**: `Push` (`git push -u`) is idempotent → unchanged. GAP: no `gh pr view/list`
   primitive exists — `CreatePR` would FAIL with "a PR already exists". NEW primitive required:
   `github.Client.PRForBranch(ctx, branch) (url string, open bool, err error)` via `gh pr view <branch>
   --json url,state`, exit-code-as-data like `AuthStatus` (found / not-found-as-data / error). Local
   `Record.PRUrl` is a fast non-authoritative hint only.
5. **Run records / range**: `CommitsInRange` (`origin/target..origin/source`) is the ENVIRONMENT range —
   unchanged (incremental still shows the full range; only the post-selection filter changes). `Record`
   has no `PromotionBranch` field — reconstruct via `RenderBranchName(cfg.BranchFormat, Ticket, Target)`;
   finding "prior run for this branch" = a small NEW pure re-render-and-match helper over `runs.List()`.
   Represent the increment by mutating the SAME RunID in place (extend `Commits`, bump `PickTotal`/
   `UpdatedAt`, keep `CreatedAt`) — NOT a new `SourceRunID` record (that's for cross-env re-promotion).
6. **UX**: new `StateBranchCollision` confirm state (reuse & append / delete & recreate / cancel),
   entered only from `onBranchCreated`'s error branch — mirrors `StateCancelConfirm`/`StateSourceConfirm`
   dedicated-confirm shape, never a silent fall-through.

## Recommended approach — reactive reuse-on-collision (MVP)

Keep the flow identical through `StatePlanPreview`; on `ErrPromotionBranchExists` → `StateBranchCollision`.
On "reuse & append": `Checkout` the branch, run the layered detection to filter `SelectedCommits`, pick
only the remainder, push, and gate PR creation on `PRForBranch` (skip create if a PR is already open).
(A later Approach 2 — a run-history "continue this run" entry — reuses these same primitives.)

## Key decisions for propose/design

- (a) reuse-vs-recreate UX = new `StateBranchCollision` confirm.
- (b) detection = layered IsAncestor → `-x` trailer (primary) → Cherry (fallback).
- (c) PR detection = new `github.Client.PRForBranch` (gh pr view), 4-state graceful degrade
  (found-open / found-closed / none / gh-absent-or-unauth → fall through to manual push+confirm).
- (d) conflict handling = unchanged.
- (e) run records = same-RunID mutation in place.

## Risks

- Boundary: new primitives land in `internal/git` (trailer parser) and `internal/github` (`PRForBranch`),
  never inline in `internal/app`.
- `PRForBranch` must degrade gracefully (gh absent/unauth → don't hard-block; fall through).
- Diverged remote deploy branch (someone pushed to origin/<deploy> since local checkout) — reuse's
  fetch/fast-forward against `origin/<deploy-branch>` is an OPEN design question (decide in design).
- Empty-after-filter selection (everything already on branch) → `CherryPick` hard-errors on empty; catch
  at app layer with an explicit notice, not a raw git error.
- No `Record.PromotionBranch` → new re-render-and-match helper (own test).
- Sizeable strict-TDD surface; likely >400 lines → single-PR with size:exception (per 1.0.0 auto).
- `verifyCmd`'s base = `origin/<target>` is CORRECT as-is for incremental (verify vs environment, not
  branch tip) — confirmed no change.

Artifact store: OpenSpec (Engram MCP unavailable).
