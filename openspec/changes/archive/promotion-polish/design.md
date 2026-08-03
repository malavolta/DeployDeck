# Design: promotion-polish (4 fixes for 1.0.0)

## Technical Approach

Four independent, code-local fixes, each mapping 1:1 to a proposal defect (D1–D4).
No new packages; no new `exec`/`http` imports (boundary invariant preserved — all
changes are git-args, config-derived pure logic, or view/reducer state). Strict
TDD: every fix lands a RED test before the production edit.

## Architecture Decisions

### D1 — Exclude the tool's own promotion branches from source candidates

| | |
|---|---|
| **Choice** | New pure helper `git.PromotionBranchPrefix(cfg)` (in `promotion_branch.go`, beside `RenderBranchName`) returns the literal prefix of `cfg.BranchFormat` up to the first `{{` token (default → `deploy/`). `CandidateBranches` filters it out. |
| **Filter point** | Inside `CandidateBranches`, on the raw slice returned by `branchesByPattern(...)`, **before** `dedupeByLocalName`. Drop any branch where `HasPrefix(name, prefix)` **or** `HasPrefix(name, "origin/"+prefix)` — so bare and `origin/` twins are dropped independently of dedupe collapsing. |
| **Guard** | If derived prefix `== ""` (format starts with a token), skip filtering — never nuke the whole candidate set. |
| **Scope** | `CandidateBranches` only. `ListBranches`/`branchesByPattern` (the standalone-delta base picker) stay untouched. |
| **Rejected** | Hardcoded `"deploy/"` (breaks custom `branchFormat`); render-with-wildcards regex (heavier — the literal prefix always precedes the first token, so it suffices); filtering after dedupe (an origin-only leftover with no local twin still needs the same `origin/`-aware check). |

### D2 — Brighter semantic colors

**Choice**: `style.go` base ANSI → bright ANSI — `styleOK` `"2"→"10"`, `styleWarn`
`"3"→"11"`, `styleErr` `"1"→"9"`. Ascii `TestMain` still forces plain text, so the
115 existing color assertions are unaffected. **Rejected**: truecolor hex (ignores
the user's terminal theme).

### D3 — Spanish guard notice + stop notice-bleed

**Choice (a)**: translate the empty-selection guard at `keys.go:942` to
`"selecciona al menos un commit para continuar"`.
**Choice (b) — the rule**: a BACK/`esc` transition that changes `m.state` and lands
on a screen that renders `m.notice` MUST clear `m.notice` first — guard notices are
transient and screen-local. Never clear a notice the destination handler sets in the
same step. Audited back-transitions that currently do NOT clear (add `m.notice = ""`):

| Handler | Line | Transition |
|---|---|---|
| `keySelection` `esc` | 932–934 | → `StateTicketInput` *(the primary bug path)* |
| `keyTicket` `esc` / empty-`q` | 873–877 | → `StatePrereqCheck`/Doctor *(carries the notice onward)* |
| `keyTarget` `esc` | 979–980 | → `StateCommitSelection` |
| `keyPlanPreview` `esc` | 1011–1012 | → `StateTargetSelection` |
| `keySourceConfirm` `esc` | 908–909 | → `StateTicketInput` |
| `keyQueueReview` `esc` | 1165–1166 | → `StatePackageReview` |

Already clearing (leave as-is): `keyPackageSelect`/`keyCancelConfirm`/`keyQuickDeploy`/
`keyDeleteConfirm`. The first two rows fix the reported bleed; the rest apply the same
rule for consistency. **Rejected**: a blanket clear at the top of `handleKey` — it
would wipe notices a handler legitimately carries forward.

### D4 — `git cherry-pick -x` provenance

**Choice**: `cherryPickArgs` → insert `-x` after `cherry-pick`, before revs:
`-c commit.gpgsign=false cherry-pick -x <revs>`. Only the initial pick;
`continueArgs`/`skipArgs` stay unchanged (the sequencer remembers `-x` across
`--continue`/`--skip`). **Rejected**: appending the trailer manually (git already
does it via `-x`).

## Data Flow (D1)

    CandidateBranches(dir, ticket)
      RepoRoot → branchesByPattern(root, "*ticket*") → [Branch...]
           │  (NEW) drop HasPrefix(name, prefix) || HasPrefix(name, "origin/"+prefix)
           ▼
      dedupeByLocalName → candidates   (deploy/* excluded)

## File Changes

| File | Action | Description |
|------|--------|-------------|
| `internal/git/promotion_branch.go` | Modify | Add pure `PromotionBranchPrefix(cfg)` (D1) |
| `internal/git/service_branches.go` | Modify | Filter promotion-prefix in `CandidateBranches` (D1) |
| `internal/app/style.go` | Modify | Bright ANSI 10/11/9 (D2) |
| `internal/app/keys.go` | Modify | Spanish guard string + clear `m.notice` on back-transitions (D3) |
| `internal/git/service_cherrypick.go` | Modify | `cherryPickArgs` adds `-x` (D4) |

## Interfaces / Contracts

```go
// PromotionBranchPrefix returns the literal prefix of cfg.BranchFormat up to the
// first "{{" token (default "deploy/"); "" when the format begins with a token.
// Pure, git-free.
func PromotionBranchPrefix(cfg config.Config) string
```

## Testing Strategy

| Layer | RED test | Assertion |
|-------|----------|-----------|
| Unit (git) | `CandidateBranches` with a `deploy/X-to-Y` (+ `origin/` twin) in the pattern set | that branch is excluded; a real source branch survives |
| Unit (git) | `PromotionBranchPrefix` | default → `"deploy/"`; custom `promo/{{ticket}}` → `"promo/"`; token-leading → `""` |
| Unit (app) | `mark("OK")` under `termenv.TrueColor` (mirror `style_test.go`) | output contains bright-green escape; Ascii path stays plain |
| Unit (app) | per back-transition (selection→ticket, ticket→prereq, …) set a notice then send `esc` | rendered next screen has no stale notice |
| Unit (git) | `cherryPickArgs(revs)` + extend the gpg-override arg test in `service_cherrypick_test.go` (~L106–126) | args contain `-x`, positioned after `cherry-pick` and before revs |

## Threat Matrix

Boundary touched: git-subprocess **argument composition** (D4) and candidate
**filtering** (D1). No new input or injection surface.

| Boundary | Applicability | Response / RED test |
|---|---|---|
| Documentation-like paths | N/A — no file classification/execution | — |
| Git repository selection | N/A — `RepoRoot`/cwd resolution unchanged | — |
| Commit state | N/A — `-x` adds a trailer only; index/worktree semantics unchanged | — |
| Push state | N/A — no push/refspec change | — |
| PR/command argument composition | Applicable (D4) — `-x` is a literal flag; revs are already-validated SHAs/ranges | arg-assertion test: `-x` present and precedes revs |

## Migration / Rollout

No migration. No persisted-state change. Pure single-PR revert restores prior git
args, colors, copy, and candidate list.

## Open Questions

- [ ] Plan-preview text (`view.go:447`) prints `git cherry-pick <sha>` without `-x` — cosmetic mismatch; out of task scope, flagged as residual.
- [ ] D3 extends clear-on-back to all six back-transitions (not just the two bug paths) — confirmed intentional for consistency under the stated rule.
