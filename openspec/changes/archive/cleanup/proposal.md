# Proposal: HU-017 — Branch & Run Cleanup (`cleanup`)

## Intent

DeployDeck accumulates dead `deploy/*` branches and never returns the user to their starting branch: 17 `tea.Quit` sites do zero cleanup, no branch-identity/mutation plumbing exists, and the complete run-retention mechanism is CLI-only (`deploydeck runs prune`, never TUI-reachable). HU-016 deferred temp-branch cleanup here. HU-017 adds branch-lifecycle hygiene and surfaces retention in the TUI.

## Scope

### In Scope
1. **Restore original branch** on finish/abort via new `Model.quitCmd()` (captures `originalBranch` at `onPrereqDone`; swaps all 17 `tea.Quit`), **guarded**: skip when `repoState.InProgress`, detached/empty `HEAD`, or `original == current`.
2. **Delete current run's temp branch** (local + remote-if-pushed) inline at `StateSucceeded`/`StateAborted`, behind confirmation.
3. New **`StateBranchCleanup`** screen (entry key `b` from `StateTicketInput`; `q`/`esc` returns) listing orphan `deploy/*` with AGE + PUSH-STATUS for batch cleanup.
4. **Invoke existing `runs.Prune`** from that screen (not auto-fired, not CLI-only; rule/config/schema UNCHANGED — new caller only).
5. **Strong confirm** (typed-CANCELAR idiom) before deleting any branch with unpushed commits; normal confirm otherwise.

Push status is derived **live from git** (`origin/<branch>` resolves; `rev-list --count`) — no persisted `Pushed` field, so `run-persistence`/`run-history`/`Record` schema are untouched.

### Out of Scope
- Real PR-merge gh/API polling (org-free harness → git-native best-effort "merged" LABEL only).
- Cross-repo/multi-remote (only `origin`); HU-018 (menu)/HU-019; any change to retention rule, defaults, or `Record` schema.

## Capabilities

### New Capabilities
- `branch-cleanup`: branch-lifecycle hygiene — original-branch restore, temp-branch deletion (inline + batch orphans), push/unpushed/age detection, best-effort merged label, and the TUI entry point that invokes existing retention.

### Modified Capabilities
- None. `branch-cleanup` **references** `run-retention` (unchanged — new caller only). No delta to `run-persistence`, `run-history`, or `promotion-branch`.

## Approach

Add ~6 exec-free `git.Service` methods (all via `newRequest`, called only through `tea.Cmd`): `CurrentBranch`, `Checkout` (plain), delete-local (`branch -D`), delete-remote (`push origin --delete`), unpushed/ahead-count, `IsMergedInto` (`merge-base --is-ancestor`), and a single `for-each-ref` `deploy/*`-with-age lister (avoids N+1). `-D` force is deliberate: the app gates on unpushed + strong-confirm first, and git's `-d` would mis-refuse squash-merged branches. `quitCmd()` runs `Checkout` synchronously in the closure before returning `tea.Quit`'s msg — zero `main.go` change.

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `internal/git/service*.go` | Modified | ~6 new exec-free methods; add age field to branch struct |
| `internal/app/{app,keys,update}.go` | Modified | `StateBranchCleanup`, `quitCmd()`, inline delete, `b` binding, capture `originalBranch` |
| `internal/app` retention call | New | Invoke `runs.Prune` from cleanup screen |
| `openspec/specs/branch-cleanup/spec.md` | New | New capability spec |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| Squash/rebase-merge blind spot (`--is-ancestor` false "not merged") | Med | Label only → extra confirm, never a wrong delete; document |
| Restore-vs-resume guard regresses HU-013 | Med | `InProgress` guard load-bearing; mandatory abort-mid-conflict NO-restore negative test |
| Larger multi-file slice exceeds review budget | Med | Batch apply as ordered task groups in one stacked branch |

## Rollback Plan

Single stacked branch, purely additive. Revert the branch: `git.Service`, `tea.Quit` sites, and states return to prior form; new spec removed. Retention CLI, `Record` schema, and other specs untouched — no data migration.

## Dependencies

- Stacked on `re-promote`. Reuses existing retention (`selectPruneCandidates`/`Writer.Prune`) and test harness (`newTempRepoWithRemote`). No org, no new deps.

## Success Criteria

- [ ] Finish/abort restores original branch; abort mid-conflict does NOT (guard test).
- [ ] Confirm deletes current temp branch locally + remote-if-pushed (bare remote).
- [ ] Orphan `deploy/*` listed with age + push-status; batch cleanup works.
- [ ] Unpushed branch refuses normal confirm, deletes only on strong confirm.
- [ ] Retention prune reachable from TUI; rule/defaults/schema unchanged.
