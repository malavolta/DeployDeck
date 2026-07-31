# Exploration: source-resolution — promotion source-branch resolution (dedupe + current-branch inference)

Fixes a dogfooding bug + adds a UX improvement in how promotion discovery resolves
the single source branch. Fase 5 (Productización); relates to HU-002/HU-004.

## Current State (verified, file:line)

Promotion discovery resolves a single source branch in two composed passes
(`internal/app/commands.go:472-501` `discoverCmd`):

1. `git.Discover(ctx, dir, {Ticket})` → `CandidateBranches`
   (`internal/git/service_branches.go:34-41`) lists every local AND remote-tracking
   branch matching `*<ticket>*` via `branchesByPattern` (`service_branches.go:47-69`),
   appending both forms as SEPARATE, undeduplicated `Branch{Name}` entries (e.g.
   `DEMO-2-mi-cambio` local + `origin/DEMO-2-mi-cambio` remote count as 2).
2. `resolveSource(candidates, cfg, target)` (`internal/app/flow.go:38-53`) asks
   `git.SuggestDefaultSource` (`internal/git/source_suggestion.go:26-54`) for RF-002's
   "previous pipeline environment" default; when the target is the FIRST configured
   pipeline stage (`idx <= 0`, line 39) it returns `false` and `selected` stays `""`.
3. `git.SelectSingleSource(candidates, selected)` (`internal/git/source_selection.go:30-48`):
   0 candidates → error; **exactly 1 → auto-resolved (ignores `selected`, case 1, line 34-35)**;
   2+ with `selected == ""` → `ErrMultipleSourceBranches`.
4. On `!ok`, `discoverCmd` degrades to a message-only `discoverDoneMsg{result: base}`
   (`commands.go:486-489`) where `base.OrderedCommits` is `nil` (the first `Discover`
   never set `Target`/`Source`).
5. `onDiscoverDone` (`internal/app/update.go:480-491`) sets `m.items` ONLY from
   `msg.result.OrderedCommits`, never `MatchedCommits`. `viewSelection`
   (`internal/app/view.go:245-262`) renders `"No se encontraron commits en el rango."`
   whenever `len(m.items) == 0`, plus `m.discovery.Alternatives` — which
   `NoResultsAlternatives` (`internal/git/discovery.go:83-88`) only populates when BOTH
   `MatchedCommits` and `CandidateBranches` are empty. In the ambiguous-source case both
   are non-empty → `Alternatives` is `nil` → genuine dead end, no escape hatch.

**Repro (verified)**: a pushed feature branch present both locally and as `origin/<name>`,
promoted to the first pipeline env, hits step 3's `ErrMultipleSourceBranches` purely
because the SAME logical branch was counted twice. `git log origin/INT..origin/<branch>`
is non-empty, and a message match exists — but the screen dead-ends.

**Key finding**: `SelectSingleSource` case 1 auto-resolves a single candidate WITHOUT
consulting `selected`. So deduping local+remote of the same branch down to 1 candidate
**fixes the reported bug by itself**. Current-branch preference is a genuinely
independent, additive improvement for ACTUALLY distinct candidates (e.g.
`feature/DEMO-2-a` and `hotfix/DEMO-2-b` both matching the ticket).

**Current branch already captured**: `internal/git.Service.CurrentBranch`
(`service_cleanup.go:26-42`, `rev-parse --abbrev-ref HEAD`, returns literal `"HEAD"` on
detached) is already called at startup via `originalBranchCmd` (`commands.go:862-873`) →
lands on `m.originalBranch` (`update.go:175-181`), fired from `onPrereqDone`
(`update.go:132`) BEFORE ticket input/discovery, and nothing checks out a different
branch before `StateBranchCreation`. `quitCmd` already trusts this field
(`commands.go:938`: `m.originalBranch != "" && m.originalBranch != "HEAD"`). So feature
(b) needs ZERO new git calls and no exec-boundary risk.

## Affected Areas

- `internal/git/service_branches.go` — `CandidateBranches` gains a dedupe step for
  local vs remote-tracking of the same branch, scoped to THIS method only (new
  unexported helper, e.g. `dedupeByLocalName`, called after `branchesByPattern` returns).
- `internal/git/service_branches_test.go` — new same-name local+remote dedupe test; the
  existing different-names test must keep passing unmodified (over-aggression guard).
- `internal/app/flow.go` — `resolveSource` gains a `currentBranch string` param + current-
  branch-preference logic, ordered AFTER `SuggestDefaultSource` (so env-to-env /
  re-promotion keep their suggestion priority).
- `internal/app/flow_test.go` — **NEW file** (none exists today): direct unit coverage of
  `resolveSource`, closing the "no covering tests found" gap.
- `internal/app/commands.go` — `discoverCmd` captures `m.originalBranch` into a local var
  and threads it into `resolveSource`; NO new git/exec call.
- `internal/app/flow_e2e_test.go` / `internal/git/discovery_e2e_test.go` — extend with the
  repro scenario (local+origin same branch, first-env target → non-empty range) and a
  genuinely-distinct-candidates scenario proving current-branch preference.

**MUST NOT touch**: `branchesByPattern`/`ListBranches` (`service_branches.go:20-27,47-69`)
and their callers. `ListBranches` feeds the standalone-delta source picker
(`commands.go:1184` `standaloneBranchesCmd`), which is TESTED to require local+remote to
stay as separate rows: `standalone_delta_test.go:33-53`
(`TestStandaloneBranchesCmd_ComposesListBranches`, DeepEqual against both `main` and
`origin/main`) and `standalone_modes_e2e_test.go:175-192` (`pickStandaloneBase` requires
`br.Remote == true`). Deduping at the shared level would break these passing tests.

## Approaches

1. **(a) Dedupe local vs remote inside `CandidateBranches` only** — merge pairs where
   local `X` and remote-tracking `origin/X` both exist into one entry (keep the bare/local
   form: reads better in the "Rama sugerida: %s" label and matches `CurrentBranch()`'s
   plain-name output). Pros: minimal, surgical, fixes the bug on its own (1 deduped
   candidate auto-resolves); zero change for distinct branches or `ListBranches`. Cons:
   none if scoped correctly. Effort: Low.
2. **(b) Prefer `CurrentBranch` when among candidates and no pipeline suggestion** — thread
   `m.originalBranch` into `resolveSource`; when `SuggestDefaultSource` returns `false` and
   `currentBranch != ""/"HEAD"`, set `selected` to the matching candidate (compare via
   `sourceRefName(c) == currentBranch`, dedupe-representation-agnostic). Pros: reuses an
   already-captured field, no new exec call, independently valuable for real multi-branch
   ambiguity, pipeline suggestion still wins. Cons: silently uses the checked-out branch —
   mild surprise risk; no help when the user is on neither candidate. Effort: Low.
3. **(c) Confirm prompt vs silent inference for (b)** — existing precedent
   (`SuggestDefaultSource`) is fully silent, surfaced via the "Rama sugerida: %s" label
   (`view.go:249-251`) with Esc as escape. A confirm dialog needs a new TUI state.
   **Recommendation: silent inference** (consistent, no new state). Flag for the proposal
   as a decision point in case the user wants a confirm gate for ambient-checkout inference.
4. **Source-branch picker for genuinely-still-ambiguous cases** — real gap
   (`NoResultsAlternatives` doesn't cover "matches found but ambiguous"), but needs a NEW
   TUI screen → materially larger. **Defer as a separate follow-up change**, not bundled.

## Recommendation

Combine **(a) + (b)**, with **(c) = silent inference** (matches existing
`SuggestDefaultSource` UX), and the **picker (4) deferred**. (a) alone fixes the bug; (b)
is additive value at negligible cost (current-branch data already captured pre-flow).

- (a): `internal/git/service_branches.go`, new unexported helper called at the end of
  `CandidateBranches` only.
- (b): `internal/app/flow.go` `resolveSource` (new `currentBranch` param, ordered after
  `SuggestDefaultSource`); `commands.go` `discoverCmd` threads `m.originalBranch` in — no
  exec/git call added, `internal/app` stays off the exec seam.

## Test Strategy (strict TDD)

1. **Git layer** — `service_branches_test.go`: new
   `TestService_CandidateBranches_DedupesLocalAndRemoteTrackingOfSameBranch` (real temp
   repo, same-name branch pushed to origin, assert `len(candidates) == 1`); existing
   different-names test keeps passing (over-aggression guard);
   `TestStandaloneBranchesCmd_ComposesListBranches` keeps passing (no leak into
   `ListBranches`).
2. **App layer, NEW `internal/app/flow_test.go`** — table-driven `resolveSource`: zero
   candidates; deduped single candidate auto-resolves regardless of `currentBranch`; 2
   distinct candidates + `currentBranch` matching one → selected; `currentBranch ==
   ""/"HEAD"` → falls through; `currentBranch` not among candidates → falls through;
   pipeline suggestion (env-to-env) beats `currentBranch` (re-promotion regression guard).
3. **Git-layer E2E** — extend `discovery_e2e_test.go`: repro (local+origin same branch,
   first-env target) now returns 1 deduped candidate + non-empty `OrderedCommits`; a
   distinct-candidates scenario proves (b) via `SelectSingleSource` directly.
4. **App-layer full-flow E2E** — extend `flow_e2e_test.go` (reuse
   `setupFlowRepo`/`gitRun`/`advance`): reproduce the repro through `m.discoverCmd()`,
   asserting `m.state == StateCommitSelection` and `len(m.items) > 0` instead of the dead end.

## Risks

- **Scope leakage** (mitigated by design): dedupe must never touch
  `branchesByPattern`/`ListBranches` — proven necessary by the standalone-delta tests.
- **Detached HEAD** (`CurrentBranch` → `"HEAD"`): reuse the `quitCmd` guard convention
  (`commands.go:938`).
- **Current branch not among candidates**: falls through to unchanged behavior.
- **Env-branch-as-source / re-promotion** (`startRePromoteInto`, `update.go:514-529`):
  `SuggestDefaultSource` MUST keep priority over current-branch inference.
- **Single-source invariant** ("never mix commits from >1 branch"): preserved — both (a)
  and (b) only narrow `candidates`/`selected` before the unmodified `SelectSingleSource`.
- **Pre-existing (not introduced)**: an unpushed-only local candidate still hard-fails
  discovery (range is always `origin/<target>..origin/<source>`, `service_range.go:24`,
  `discovery.go:200-201`). (b) doesn't increase exposure (dedupe only merges branches that
  already have a pushed remote-tracking counterpart). No fix here.
- **Genuinely ambiguous, no current-branch match** still dead-ends silently — pre-existing
  gap; the source-picker follow-up is the fix, out of scope.
- Estimated diff (git dedupe helper + test, `flow.go` change + new `flow_test.go`, 1-line
  `commands.go` wiring, e2e extensions) is comfortably within the 800-line budget — single PR.

## Ready for Proposal

Yes. Scope: (a) dedupe + (b) current-branch preference (silent inference); note the
source-branch picker as an out-of-scope follow-up; flag decision (c) (silent vs
confirm-prompt) for the user if they want it revisited.
