# Design: Source Resolution — Dedupe Candidates + Current-Branch Confirm

## Technical Approach

Two surgical, independent changes behind the existing seams (proposal D1 + D2). D1 collapses a
local branch and its `origin/` twin into one candidate inside `CandidateBranches` only, so
`SelectSingleSource` case 1 auto-resolves and the reported bug disappears with zero UI. D2 adds an
explicit TUI confirm (`StateSourceConfirm`) that fires only when discovery is genuinely ambiguous
(2+ distinct candidates, no pipeline suggestion) and the checked-out branch matches one. `internal/app`
stays off the exec seam: the confirm reuses the already-captured `m.originalBranch`; git logic stays in
`internal/git`. Discovery keeps funneling through the unmodified `git.SelectSingleSource`.

## Architecture Decisions

### ADR-1 — D1 dedupe scoped to `CandidateBranches`
**Choice**: New unexported `dedupeByLocalName([]Branch) []Branch` called at the END of
`CandidateBranches` (after `branchesByPattern` returns). It drops each remote `origin/X` when a local
`X` is present (compares on `strings.TrimPrefix(name,"origin/")`), keeps the bare/local form, preserves
first-seen order, and never collapses distinct names.
**Alternatives**: dedupe inside shared `branchesByPattern`/`ListBranches` — rejected.
**Rationale**: `ListBranches` feeds the standalone-delta picker which is TESTED to need local+remote as
separate rows (`standalone_delta_test.go:33-53`, `standalone_modes_e2e_test.go:175-192`). Scoping to
`CandidateBranches` fixes the bug (1 candidate ⇒ auto-resolve) without touching that contract. Bare form
reads better in the "Rama sugerida" label and matches `CurrentBranch()` output.

### ADR-2 — Current-branch match computed inside `resolveSource`, not an app pre-check
**Choice**: `resolveSource` gains `currentBranch string` and returns a tri-state outcome. Ordering:
`SuggestDefaultSource` first; only if it yields nothing AND `currentBranch != "" && != "HEAD"` AND a
candidate satisfies `sourceRefName(c) == currentBranch`, return that candidate as *needs-confirm*.
**Alternatives**: a separate gating pre-check in `discoverCmd` before selection — rejected.
**Rationale**: keeps the pipeline-first ordering in ONE pure, directly unit-testable function (the new
`flow_test.go`), avoids duplicating candidate matching, and takes `currentBranch` as a plain string
(from `m.originalBranch`) so `internal/app` adds NO git/exec call.

### ADR-3 — Confirm fires on the message boundary, BEFORE the ranged discover
**Choice**: Split the two-pass `discoverCmd`. Pass 1 (candidates + `resolveSource`) returns a
`discoverDoneMsg{confirm:true, source:pending}` when needs-confirm; the ranged discover runs only after
the user presses `s`, via a new `confirmSourceCmd`. Resolved and degrade paths are byte-for-byte
unchanged.
**Alternatives**: prompt mid-goroutine — impossible; a Bubble Tea `tea.Cmd` cannot render a prompt.
**Rationale**: the decision is pure and lands on the reducer boundary; the already-working
resolved path (dedupe→1, or pipeline suggestion) never enters the new state.

### ADR-4 — `StateSourceConfirm` reuses existing fields; invariant preserved
**Choice**: New `StateSourceConfirm`. The pending candidate rides `m.source`; base candidates ride
`m.discovery`. On `s`, `confirmSourceCmd` funnels `git.SelectSingleSource(m.discovery.CandidateBranches,
m.source.Name)` (UNMODIFIED) then runs the ranged `Discover`. No new Model fields; only
`discoverDoneMsg` gains `confirm bool`.
**Rationale**: minimal surface; single-source invariant enforced by the same funnel; mirrors the
existing confirm-screen precedents (`keyCancelConfirm`).

## Data Flow / Sequence (confirm path)

```
keyTicket enter ─▶ StateCommitDiscovery ─▶ discoverCmd()          [pass 1]
  g.Discover{Ticket} → base
  resolveSource(base.Candidates, cfg, prelim, m.originalBranch):
    ├ resolved      & prelim!="" → g.Discover{Ticket,Target,Source} → msg{full, source}
    ├ needs-confirm & prelim!="" → msg{base, source:pending, confirm:true}
    └ else                       → msg{base}                         (degrade, unchanged)
onDiscoverDone:
  confirm → m.discovery=base; m.source=pending; state=StateSourceConfirm
  else    → m.items=…; state=StateCommitSelection                    (unchanged)
StateSourceConfirm  view: "¿Usar la rama actual '<X>' como origen? [s/N]"
  keySourceConfirm:
    s          → state=StateCommitDiscovery; confirmSourceCmd()      [pass 2]
                   SelectSingleSource(cands, pending.Name)=sel
                   g.Discover{Ticket,Target,Source:sourceRefName(sel)} → msg{full, sel}
                   → onDiscoverDone → StateCommitSelection, len(items)>0   ✔ repro fixed
    n/N/enter  → m.source={}; degrade to StateCommitSelection (base.OrderedCommits=nil)
    esc        → state=StateTicketInput
```

## Interfaces / Contracts

```go
// internal/git/service_branches.go
func dedupeByLocalName(branches []Branch) []Branch // scoped to CandidateBranches only

// internal/app/flow.go
type resolveOutcome int
const (resolveDegrade resolveOutcome = iota; resolveReady; resolveNeedsConfirm)
func resolveSource(candidates []git.Branch, cfg config.Config, target, currentBranch string) (git.Branch, resolveOutcome)

// internal/app/commands.go
type discoverDoneMsg struct { result git.DiscoverResult; source git.Branch; confirm bool; err error }
func (m Model) confirmSourceCmd() tea.Cmd // pass-2 ranged discover for the confirmed source

// internal/app/app.go
StateSourceConfirm State // new
```

The confirm literal stays Spanish to match existing UI copy (`viewSelection`); the design is English.

## File Changes

| File | Action | Description |
|------|--------|-------------|
| `internal/git/service_branches.go` | Modify | Add `dedupeByLocalName`; call at end of `CandidateBranches` only |
| `internal/git/service_branches_test.go` | Modify | Dedupe unit + over-aggression guard + `ListBranches`-not-deduped guard |
| `internal/app/flow.go` | Modify | `resolveSource` gains `currentBranch`; tri-state ordering (pipeline→current-branch) |
| `internal/app/flow_test.go` | Create | Table-driven `resolveSource` unit coverage |
| `internal/app/commands.go` | Modify | Thread `m.originalBranch`; switch on outcome; add `confirmSourceCmd`; `discoverDoneMsg.confirm` |
| `internal/app/app.go` | Modify | New `StateSourceConfirm` |
| `internal/app/keys.go` | Modify | `handleKey`→`keySourceConfirm` (s / n·N·enter / esc) |
| `internal/app/update.go` | Modify | `onDiscoverDone` routes `confirm`→`StateSourceConfirm` |
| `internal/app/view.go` | Modify | `viewBody`→`viewSourceConfirm` render |
| `internal/git/discovery_e2e_test.go` | Modify | Repro: 1 deduped candidate + non-empty `OrderedCommits`; distinct-candidates via `SelectSingleSource` |
| `internal/app/flow_e2e_test.go` | Modify | Repro reaches `StateCommitSelection`, `len(items)>0`; distinct-candidate confirm scenario |

## Testing Strategy

| Layer | What to Test | Approach |
|-------|-------------|----------|
| Unit (git) | dedupe collapses local+`origin/X`→1; distinct names→2 (over-aggression); `ListBranches` still returns both | real temp repo; `branchesByPattern` unaffected |
| Unit (app) | `resolveSource`: 0 cands; deduped-single auto-resolves ignoring `currentBranch`; 2 distinct + match→needs-confirm; `""`/`"HEAD"`/no-match→degrade; pipeline suggestion beats current-branch | table-driven `flow_test.go` (pure) |
| Unit (app) | `keySourceConfirm` s/n/N/enter/esc; `onDiscoverDone` confirm routing | `Model.Update` |
| E2E (git) | repro non-empty range after dedupe; distinct-candidate selection | `discovery_e2e_test.go` |
| E2E (app) | full flow: repro → `StateCommitSelection` `len(items)>0`; confirm `s` path | `flow_e2e_test.go` (`setupFlowRepo`/`advance`) |

## Threat Matrix

N/A — introduces no new routing, shell command, subprocess, VCS/PR automation, or executable-file
classification. Dedupe is pure post-processing of existing branch-list output; the confirm reuses the
already-captured `m.originalBranch` (no new exec). `internal/app` stays off the exec seam
(`boundary_test.go` holds); the source ref flows through the unchanged `sourceRefName` → `origin/`
prefixing → `service_range.go`. No new user-controlled input reaches any command.

## Migration / Rollout

No migration. Purely additive/behavioral; the new state is reachable only on the previously dead-end
ambiguous path. Rollback = revert the commit.

## Open Questions

None blocking. Residual (out of scope, deferred follow-up): a full source-branch PICKER for
genuinely-distinct candidates with NO current-branch match still dead-ends silently — unchanged, not
worsened.
```
