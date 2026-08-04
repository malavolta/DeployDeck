# Archive Report — incremental-promotion

**Status:** archived · **Delivery:** single-PR (size:exception) · **Mode:** strict TDD · **Milestone:** 1.0.0

## Summary

Reuse-on-collision incremental promotion. When the promotion branch for a ticket/target pair
already exists (`deploy/<ticket>-to-<target>`), the flow no longer dead-ends: it enters a
`StateBranchCollision` decision offering **reuse & append**, **delete & recreate**, or **cancel**.
On reuse it checks out the branch, fast-forwards it against `origin/<deploy>` (never force),
filters the selection to the not-yet-present commits (layered detection), cherry-picks only the
remainder, mutates the SAME run record in place, and gates PR creation on an existing-open-PR
check so it appends to the open PR instead of failing on `gh pr create`.

New capability `incremental-promotion` (6 requirements); MODIFIED `promotion-branch` (collision
choice set) and `push-pr-preparation` (open-PR check before create); ADDED `run-persistence`
(in-place increment mutation). New primitives live only in `internal/git`
(`FilterNotOnBranch`, `FastForwardBranch`, `ParseCherryPickTrailers`, `LocalBranchExists`,
`RemoteBranchExists`) and `internal/github` (`PRForBranch`); `internal/runs` gained the pure
`FindRunForBranch`. `CherryPick`/conflict machinery/`Push`/`Checkout` are untouched. The
exec-boundary invariant holds (`internal/app` never gained exec/http imports).

## SDD trail

explore (orchestrator-authored) → propose → spec + design → tasks → apply (strict TDD) →
verify → 3 parallel adversarial reviews (reliability, resilience, sdd-verify) → remediation →
archive. Artifact store: OpenSpec (Engram MCP unavailable).

## Review & remediation

Three reviews landed:

- **Resilience — CLEAN.** `FastForwardBranch` proven never-force (two `IsAncestor` checks classify
  BEFORE any mutation; only a strict-descendant origin triggers `merge --ff-only`; diverged is pure
  data). `PRForBranch` degrades safely (exit-code-as-data, gated behind the authenticated path).
- **sdd-verify — PASS (0 CRITICAL), one WARNING (W1):** `deleteAndRecreateBranchCmd` called
  `DeleteLocalBranch` unconditionally, so a pure origin-only collision dead-ended to `StateError`,
  and it never deleted the remote deploy branch (so recreate would re-collide on a pushed branch).
- **Reliability — one WARNING, one SUGGESTION:**
  - WARNING: `FilterNotOnBranch` checked the `-x` trailer before `git cherry`, allegedly letting a
    reverted-but-still-in-log commit be wrongly skipped.
  - SUGGESTION: the in-place increment makes `rec.Commits` cumulative and `PickTotal =
    len(rec.Commits)`, so a later resume over-counts "pick N of M" and re-verifies prior-session
    commits.

**Remediation:**

- **W1 (verify) — FIXED.** Extracted `deleteExistingDeployBranch`: it exists-guards the local
  (`LocalBranchExists`) and remote (`RemoteBranchExists`) branches independently and deletes only
  what is present before recreate. Delete & recreate now works for origin-only and local+remote
  collisions. The `StateBranchCollision` copy now states `d` removes the branch **local AND
  remote** and closes its PR. RED→GREEN with unit + integration tests.

- **Reliability WARNING — investigated and REVERTED (won't-fix, evidence-based).** The proposed
  reorder (make `git cherry` authoritative, demote the trailer to a fallback) was implemented, then
  disproven with real-git reproduction:
  1. A plain `git revert` on the deploy branch does **not** flip `git cherry`'s classification —
     `git cherry` tests patch-id SET MEMBERSHIP over every commit ever unique to the branch, and a
     revert only *adds* a negating commit; the original commit's patch-id stays in the set, so
     cherry still reports `-` (present). **Neither ordering re-applies the reverted commit** — the
     WARNING's scenario is not fixable by reordering.
  2. Making cherry authoritative **regresses the common path**: a conflict-resolution-edited pick
     has a different patch-id, so `git cherry` reports it `+`; cherry-first would RE-PICK it and
     drop the user into a **spurious re-conflict** for a commit that is already applied. The
     original trailer-first design deliberately avoids exactly this.

  Both points were reproduced with standalone git scripts. `FilterNotOnBranch` was restored to the
  deliberate trailer-primary ordering (ancestry → `-x` trailer → cherry fallback); the spec, design
  decision row, and code comments now document the accepted **known limitation** (a promoted commit
  reverted/rewritten *in place* stays hidden by its trailer; escape hatch = delete & recreate) and
  the empirical rationale.

## Follow-up (accepted, non-blocking)

- Reliability SUGGESTION (resume over-count): the in-place cumulative `Commits`/`PickTotal` is the
  intended record shape (the run reflects all promoted commits), but a *resume* of an incremental
  run rehydrates the full cumulative selection, so "pick N of M" over-counts and prior-session
  commits are re-verified. No data loss and no wrong pick is applied. Deferred to a focused
  resume-of-incremental change that distinguishes cumulative-record commits from current-session
  picks.

## Spec merge note

The four delta specs were merged ADDITIVELY into the living specs (replace only the named MODIFIED
requirement block; append ADDED ones; the NEW `incremental-promotion` capability is a fresh living
spec). No pre-existing requirement was lost: `promotion-branch` 76→81, `push-pr-preparation`
166→174, `run-persistence` 189→213, new `incremental-promotion` = 97. Delta-only `(Previously: …)`
annotations were dropped so each living spec states current truth.

## Verification

- `go build ./...`, `go vet ./...`, `gofmt -l internal` clean.
- `go test ./... -race -count=1` green across all 13 packages (re-run by the orchestrator AFTER the
  Fix-1 revert).
- Exec-boundary invariant preserved (all new subprocess calls behind `git.Service`/`github.Client`).
