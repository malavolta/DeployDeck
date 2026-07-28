# Design: HU-017 — Branch & Run Cleanup (`cleanup`)

## Technical Approach

Add ~7 exec-free `git.Service` methods (all via `newRequest`, called only inside `tea.Cmd` closures so `TestApp_NeverImportsExecSeam` holds), a `quitCmd()` that restores the original branch synchronously before quitting, a new `StateBranchCleanup` screen for orphan `deploy/*` batch cleanup + retention, and an inline current-branch delete at the success/abort terminals. Retention reuses the UNCHANGED `runs.Writer.Prune`. No schema, config, or `Record` change. Implements proposal scope 1–5; realizes new capability `branch-cleanup`.

## Architecture Decisions

### Quit-restore mechanism + guards
| Option | Tradeoff | Decision |
|--|--|--|
| Sync `Checkout` inside `quitCmd` closure, then return `tea.Quit()`'s `QuitMsg` | Bubble Tea runs the cmd to completion before delivering the msg → checkout finishes before `Program.Run()` returns; zero `main.go` change | chosen |
| Restore in `main.go` after `Run()` | app must leak `originalBranch`; breaks single-source-of-truth | rejected |

Guards (skip restore), pure/model-state in `quitCmd`: `deps.Git==nil` (unit tests) OR `repoState.InProgress` (mid-conflict `q` must NOT checkout — git refuses unmerged paths and HU-013 reads `CHERRY_PICK_HEAD` off the branch) OR `originalBranch ∈ {"","HEAD"}`. Git-dependent guards in the closure: re-query `CurrentBranch` (`== original` → skip) and `BranchExists(original)` (gone → skip). `currentBranch` is RE-QUERIED not stored — the flow checks the deploy branch out (`CreatePromotionBranch`), so the branch at quit ≠ startup.

Capture: `onPrereqDone` returns `tea.Batch(resumeDetectCmd(), originalBranchCmd())`; `originalBranchCmd` runs `CurrentBranch` → `originalBranchMsg` → `m.originalBranch`. *Refines exploration* ("captured at onPrereqDone"): the reducer is pure, so capture is a batched cmd + msg, not a sync call. `deps.Git==nil` → `originalBranchCmd` returns nil (degrades cleanly).

### `deploy/*` lister — single-pass
| Option | Tradeoff | Decision |
|--|--|--|
| ONE `git for-each-ref --format='%(refname:short)\|%(committerdate:iso-strict)' refs/heads/deploy/* refs/remotes/origin/deploy/*` | one subprocess (both refspecs); `Pushed`=`origin/<name>` present in the remote set (ref-existence, same semantics as `revParseVerify`/`RemoteHead`) | chosen |
| N+1 per-branch `origin/<name>` resolve | one exec per branch | rejected |
| `%(upstream)` on heads only | depends on `-u` having run; fragile | rejected |

Parsing is a pure `parseDeployBranches([]byte) []DeployBranch` (unit-testable, no git).

### `-D` force delete
Choice `git branch -D`. Alternative `-d` (safe). Rationale: the app gates unpushed + strong-confirm FIRST; git's `-d` would double-gate and mis-refuse squash/rebase-merged branches (new SHA → `--is-ancestor` false-negative). Force is deliberate and post-gate.

### Strong-confirm reuse
Reuse the exact typed-input idiom of `keyCancelConfirm` (`keys.go:552-579`): `backspace`/`KeyRunes` buffer build + case-sensitive `==` gate. Use a DEDICATED field `deleteConfirm` and word `BORRAR` — NOT HU-012's `cancelInput`/`CANCELAR` — avoiding shared-buffer contamination and the mismatch of typing "cancel" to delete. Unpushed gate: on `d`, fire `unpushedCountCmd(name)`; `>0` → strong (`BORRAR`), else normal (`y`) confirm — one call per attempt, no N+1.

## Interfaces / Contracts

```go
// internal/git/service_cleanup.go — all via newRequest; exit-code-as-data mirrors revParseVerify
CurrentBranch(ctx, dir) (string, error)              // rev-parse --abbrev-ref HEAD ("HEAD" when detached)
Checkout(ctx, dir, branch) error                     // checkout <branch>  (no -b)
DeleteLocalBranch(ctx, dir, branch) error            // branch -D <branch>
DeleteRemoteBranch(ctx, dir, branch) error           // push origin --delete <branch>
UnpushedCommitCount(ctx, dir, branch) (int, error)   // origin/<b> resolves → rev-list --count origin/<b>..<b>;
                                                     // else → rev-list --count <b> --not --remotes=origin
IsMergedInto(ctx, dir, branch, target) (bool, error) // merge-base --is-ancestor <branch> origin/<target> (LABEL only)
ListDeployBranches(ctx, dir) ([]DeployBranch, error) // single for-each-ref (above)
type DeployBranch struct { Name string; LastCommit time.Time; Pushed bool }
```

```go
func (m Model) quitCmd() tea.Cmd {
    if m.deps.Git == nil || m.repoState.InProgress ||
        m.originalBranch == "" || m.originalBranch == "HEAD" {
        return tea.Quit
    }
    g, dir, ctx, orig := m.deps.Git, m.deps.Dir, m.ctx(), m.originalBranch
    del, name, pushed := m.pendingDeleteCurrent, m.branchName, m.currentPushed
    return func() tea.Msg {
        if cur, err := g.CurrentBranch(ctx, dir); err == nil && cur != orig {
            if ok, err := g.BranchExists(ctx, dir, orig); err == nil && ok &&
                g.Checkout(ctx, dir, orig) == nil && del { // must leave branch before deleting it
                _ = g.DeleteLocalBranch(ctx, dir, name)
                if pushed { _ = g.DeleteRemoteBranch(ctx, dir, name) }
            }
        }
        return tea.Quit() // QuitMsg
    }
}
```

Extract pure `shouldRestore(inProgress bool, original, current string, exists bool) bool` for unit tests. App additions: `State` const `StateBranchCleanup`; `cleanupPhase` enum (loading|browsing|confirm|strongConfirm|pruneConfirm); fields `originalBranch, currentPushed, pendingDeleteCurrent, cleanupBranches []git.DeployBranch, cleanupCursor, cleanupPhase, deleteConfirm, cleanupNotice`; msgs `originalBranchMsg, deployBranchesMsg, deleteDoneMsg, unpushedMsg, pruneDoneMsg`; cmds `originalBranchCmd, listDeployBranchesCmd, deleteOrphanCmd(name,pushed), unpushedCountCmd(name), pruneRunsCmd`. `keyTicket` gains `b`→`StateBranchCleanup`+load; `keyBranchCleanup` (nav/`d`/`p`/`q`/`esc`→ticket); `keySucceeded`+terminal block gain `d` (inline delete → confirm → `pendingDeleteCurrent=true` + `quitCmd`; delete happens INSIDE `quitCmd` after the restore-checkout, since you cannot delete the branch you are on). Retention: `pruneRunsCmd`=`deps.Runs.Prune(cfg.Runs.KeepLast, cfg.Runs.KeepDays, m.now())`; `now` from injected `m.now()` (existing pattern); `Prune` untouched. Swap the **13** `return m, tea.Quit` in `keys.go` → `m.quitCmd()` (*refines exploration's "17"*; real count is 13 in `keys.go`). `update.go:26` ctrl+c stays a hard `tea.Quit` (no restore) — interrupt semantics.

## Data Flow
```
onPrereqDone ─Batch─▶ originalBranchCmd ─▶ originalBranchMsg ─▶ m.originalBranch
StateTicketInput ─b─▶ StateBranchCleanup ─listDeployBranchesCmd─▶ deployBranchesMsg ─▶ list(age,push)
   ─d─▶ unpushedCountCmd ─▶ (>0 strongConfirm | else confirm) ─▶ deleteOrphanCmd ─▶ reload
   ─p─▶ pruneConfirm ─▶ pruneRunsCmd ─▶ Writer.Prune (unchanged)
any q/enter/esc ─▶ quitCmd ─▶ [guarded Checkout(original) (+ optional current-branch delete)] ─▶ QuitMsg
```

## File Changes
| File | Action | Description |
|--|--|--|
| `internal/git/service_cleanup.go` | Create | 7 methods + `DeployBranch` + `parseDeployBranches` |
| `internal/app/app.go` | Modify | `StateBranchCleanup`, `cleanupPhase`, new fields |
| `internal/app/keys.go` | Modify | `b` entry, `keyBranchCleanup`, inline `d`, swap 13 quit sites |
| `internal/app/update.go` | Modify | batch capture in `onPrereqDone`; new msg handlers |
| `internal/app/commands.go` | Modify | new cmds + `quitCmd` |
| `internal/app/view.go` | Modify | `viewBranchCleanup` + `View()` case |
| `openspec/specs/branch-cleanup/spec.md` | Create | new capability spec |

## Testing Strategy
| Layer | What | How |
|--|--|--|
| Unit | `parseDeployBranches`; `shouldRestore` guards; orphan-correlation `selectOrphans(branches,current)`; `UnpushedCommitCount` form selection | table-driven, no git |
| Integration | restore-on-quit; **abort-mid-conflict NO-restore (MANDATORY)**; delete local+remote ref in bare origin; unpushed count; orphan list age+push; `IsMergedInto` merged/not; retention prune | `newTempRepoWithRemote`, `-short`-skip |
| App | `b`→cleanup load; nav; confirm/strongConfirm; `q`/`esc`→ticket; terminal `d`→delete-then-quit order; quit swap preserves transitions | direct `Model.Update` |
| Invariant | `TestApp_NeverImportsExecSeam` still passes | all git via `git.Service` inside `tea.Cmd` |

## Threat Matrix
| Boundary | Cases | Applicability | Design response | RED tests |
|--|--|--|--|--|
| Git repo selection | relative/abs dir | Applicable | every method resolves via `RepoRoot`, like all `git.Service` | integration on temp repo |
| Commit state | unmerged / mid-conflict | Applicable | `InProgress` guard blocks restore; `-D` only post-gate | abort-mid-conflict NO-restore |
| Push state | tracking, first push, no remote ref | Applicable | `UnpushedCommitCount` two-form; `DeleteRemoteBranch` only when `Pushed` | unpushed-count + delete-remote-ref |
| Documentation-like paths | — | N/A: no file classification/exec | — | — |
| PR commands | — | N/A: no gh/PR change (out of scope) | — | — |

## Migration / Rollout
No migration. Purely additive; revert the branch to undo. Retention CLI, config, and `Record` schema untouched.

## Open Questions
- [ ] Strong-confirm word `BORRAR` (recommended) vs reuse `CANCELAR` — cosmetic, resolve in tasks.
- [ ] `update.go:26` ctrl+c restore? Design says no (interrupt = hard exit); confirm acceptable.
