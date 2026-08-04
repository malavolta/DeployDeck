# Design: Incremental Promotion (reuse-on-collision)

## Technical Approach

Reactive reuse: keep the flow identical through `StatePlanPreview`. `onBranchCreated`
(`update.go:567`) special-cases `errors.Is(err, git.ErrPromotionBranchExists)` → new
`StateBranchCollision` confirm; every other error still lands on `StateError`. On
"reuse & append": `Service.Checkout` the branch, fast-forward-guard it against the
already-fetched `origin/<deploy>`, filter `plan.SelectedCommits` to the not-yet-present
subset (layered classifier), cherry-pick the remainder, mutate the SAME RunID, then gate
PR creation on `github.Client.PRForBranch`. New primitives live only in `internal/git`
and `internal/github`; `CherryPick`/`CherryPick` conflict machinery/`Push` are untouched.
Maps to proposal D1–D5.

## Architecture Decisions

| Decision | Choice | Rejected | Rationale |
|---|---|---|---|
| Collision UX | `StateBranchCollision` confirm (r reuse / d delete+recreate / c cancel), routed only from the `ErrPromotionBranchExists` branch | silent reuse; overload `StateError` | mirrors `StateSourceConfirm`/`StateCancelConfirm`; never silent |
| Already-on-branch filter | git-layer `Service.FilterNotOnBranch` composing `IsAncestor(sha, deploy)` → **`ParseCherryPickTrailers`** (primary) → `Cherry(deploy, source)` (fallback) | cherry-authoritative; patch-id first; app-inline exec | trailer is exact + survives conflict-resolution edits that break patch-id (cherry-first would re-pick them into a spurious re-conflict on the common path); known limitation — a commit reverted/rewritten in place stays hidden by its trailer, escape hatch is delete & recreate; boundary stays in `git` |
| PR detection | `PRForBranch` via `gh pr view <branch> --json url,state`; consulted only on the authenticated path | `gh pr create` then swallow "already exists" | avoids create failure; gh-degrade reuses existing `authState` gate |
| Increment persistence | mutate prior run in place (append `Commits`, bump `PickTotal`/`UpdatedAt`, keep `CreatedAt`/`RunID`) via pure `FindRunForBranch` | new `SourceRunID` record; `SchemaVersion` bump | `SourceRunID` is cross-env; same branch = same run; old `run.json` still loads |
| Diverged remote (Q1) | fetch is already done at collision; `IsAncestor`-layered ff: ff-only when behind, no-op when ahead/up-to-date, **hard error on non-ff divergence** | force-push; in-flow rebase | never clobber remote work; hard error is the right MVP slice |
| Closed/merged PR (Q2) | fall through to normal create/compare (with a notice) | reopen; notify-only | least surprising; a fresh PR re-tracks the appended commits |
| Empty-after-filter | app-layer notice, then `StatePushPreparation` (idempotent push + `PRForBranch`) | raw `CherryPick` empty error | never expose git internals; PR still inspectable |

## Data Flow

    StatePlanPreview → branchCreateCmd → onBranchCreated
      err==nil ─────────────────────────→ (existing) create run + cherryPickCmd
      ErrPromotionBranchExists ─→ StateBranchCollision
          r → reuseBranchCmd: Checkout → FastForwardBranch → FilterNotOnBranch
                → reuseReadyMsg → onReuseReady
                    diverged/err → StateError (clear message)
                    empty        → notice → StatePushPreparation
                    remainder    → FindRunForBranch → mutate+Save → StateCherryPicking
          d → DeleteLocalBranch + branchCreateCmd (recreate)
          c/esc → StatePlanPreview
    ... StatePushPreparation: preparePRCmd also calls PRForBranch when m.reusing →
          open → skip create, show url;  closed/merged → note + normal create;
          none → normal create;  gh absent/unauth → existing compare degrade

## File Changes

| File | Action | Description |
|---|---|---|
| `internal/git/trailers.go` | Create | pure `ParseCherryPickTrailers(gitLog []byte) map[string]bool` (`(cherry picked from commit <sha>)`) |
| `internal/git/service_reuse.go` | Create | `FilterNotOnBranch` (layered) + `FastForwardBranch` (`merge --ff-only`, FFResult as data) |
| `internal/github/client.go` | Modify | add `PRForBranch` to `Client` interface + `client` impl |
| `internal/runs/finder.go` | Create | pure `FindRunForBranch(records, format, branchName)` re-rendering `RenderBranchName` over `List()` |
| `internal/app/app.go` | Modify | `StateBranchCollision` const; `reusing bool` field |
| `internal/app/update.go` | Modify | `onBranchCreated` routing; `onReuseReady`; increment mutation; `onPrepDone` PR gate |
| `internal/app/commands.go` | Modify | `reuseBranchCmd`; extend `preparePRCmd` with `PRForBranch` |
| `internal/app/keys.go` | Modify | route `StateBranchCollision` → `keyBranchCollision` |
| `internal/app/view.go` | Modify | `StateBranchCollision` screen |

## Interfaces / Contracts

```go
// internal/git
func ParseCherryPickTrailers(gitLog []byte) map[string]bool // sourceSHA -> present
func (s *Service) FilterNotOnBranch(ctx, dir, deployBranch, sourceRef string,
    commits []DiscoveredCommit) (remaining []DiscoveredCommit, err error)
type FFResult int // FFUpToDate|FFFastForwarded|FFLocalAhead|FFRemoteAbsent|FFDiverged
func (s *Service) FastForwardBranch(ctx, dir, branch string) (FFResult, error) // diverged = data
// internal/github
PRForBranch(ctx, branch string) (url string, open bool, err error) // gh pr view --json url,state
// internal/runs
func FindRunForBranch(records []Record, format, branchName string) (Record, bool)
```

`PRForBranch` outcomes: OPEN → skip create (set `prURL`); CLOSED/MERGED → `open=false`,
fall through to create + notice; no PR (exit≠0 under authed gh) → `open=false,err=nil`;
Runner error (gh absent) → `err!=nil` (caller degrades via existing `authState`, so it is
only invoked when `authState==Authenticated`).

## Testing Strategy (strict TDD — RED first)

| Layer | What | Test file |
|---|---|---|
| Unit | `ParseCherryPickTrailers` (trailer / no-trailer / multiline / malformed) | `internal/git/trailers_test.go` |
| Unit | `FindRunForBranch` (match / no-match / newest-of-many) | `internal/runs/finder_test.go` |
| Integration | `FilterNotOnBranch` layered (ancestor / trailer / cherry / empty) | `internal/git/service_reuse_test.go` |
| Integration | `FastForwardBranch` (behind→ff, ahead→noop, absent, **diverged→data**) | `internal/git/service_reuse_test.go` |
| Integration | `PRForBranch` (open/closed/none) via fake Runner | `internal/github/pr_for_branch_test.go` |
| Unit | collision routing: `ErrPromotionBranchExists`→`StateBranchCollision`; other err→`StateError` | `internal/app/branch_collision_test.go` |
| Unit | `keyBranchCollision` r/d/c; empty-after-filter notice; diverged hard error; PR-open skip-create | `internal/app/branch_collision_test.go` |
| E2E | reuse appends only new commits, mutates same RunID, updates open PR (fake gh) | `internal/app/incremental_promotion_e2e_test.go` |

Extend the app-test fake `gh` client with `PRForBranch`.

## Threat Matrix

| Boundary | Applicability | Design response | Planned RED test |
|---|---|---|---|
| Documentation-like paths | N/A: no file-type classification | — | — |
| Git repository selection | Applicable: reuse ops resolve via `RepoRoot(dir)` like every `Service` method | no `git -C`/relative cwd introduced | `service_reuse_test.go` runs in a temp repo dir |
| Commit state | Applicable: reuse checks out an existing branch, then cherry-picks | filter runs before pick; empty index → app notice, never raw pick | empty-after-filter test |
| Push state | Applicable: reused branch already tracks origin | `Push` unchanged (idempotent `-u`); ff-only, never force | `FastForwardBranch` diverged=hard-error test |
| PR commands | Applicable: new `gh pr view` + gated `gh pr create` | discrete-slice args, no shell, no env-prefix; `--head` unchanged; skip create when open | `PRForBranch` + PR-open skip-create tests |

## Migration / Rollout

No migration. Additive and gated on collision; no `SchemaVersion` bump. Revert restores
the `StateError` dead-end; existing runs/branches unaffected.

## Open Questions

- [x] Diverged non-ff deploy branch → hard error (ff-or-error), user resolves manually.
- [x] Closed/merged open PR → fall through to normal create/compare with a notice.
