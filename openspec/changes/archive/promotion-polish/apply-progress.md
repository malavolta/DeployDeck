# Apply Progress: promotion-polish

Mode: Strict TDD
Baseline: `go build ./...` clean, `go test ./... -race -count=1` all green (13 packages) before any change.
Final: `go build ./...` clean, `go test ./... -race -count=1` all green (13 packages) after all 4 fixes.

## Status

**Complete — 16/16 tasks done.** All 4 fixes (D1–D4) implemented, RED→GREEN verified for
each, full suite green after every phase.

`git diff --stat`: 14 files changed, 307 insertions(+), 19 deletions(-) — 326 changed
lines, within the pre-accepted `size:exception` / Low-risk forecast (~220-320 estimate).

## TDD Cycle Evidence

| Task | Test File | Layer | Safety Net | RED | GREEN | TRIANGULATE | REFACTOR |
|------|-----------|-------|------------|-----|-------|-------------|----------|
| 1.1 | `internal/git/promotion_branch_test.go` (`TestPromotionBranchPrefix_TableDriven`) | Unit | ✅ full suite green pre-change | ✅ compile fail (`undefined: git.PromotionBranchPrefix`) | ✅ passed | ✅ 3 cases (default/custom/token-leading) | ➖ none needed |
| 1.2 | `internal/git/service_branches_test.go` (`TestService_CandidateBranches_ExcludesPromotionBranch`) | Unit (real git, temp repo) | ✅ (same run) | ✅ compile fail (new 4th `cfg` arg) | ✅ passed | ➖ single scenario matches spec's one exclusion case | ➖ none needed |
| 1.3 | `internal/git/promotion_branch.go` | — | — | — | ✅ `go build` clean | — | — |
| 1.4 | `internal/git/service_branches.go` | — | — | — | ✅ `go build` clean | — | — |
| 1.5 | Verify | — | — | — | ✅ `internal/git` + `internal/app` full packages green | — | — |
| 2.1 | `internal/app/style_test.go` (`TestMark_NonAsciiUsesBrightColors`) | Unit | ✅ existing `TestMark_*`/`TestHeader_*` green pre-change | ✅ failed with exact bright-escape-missing / base-escape-present messages | ✅ passed | ✅ 3 tokens (OK/!!/XX), each asserting bright-present AND base-absent | ➖ none needed |
| 2.2 | `internal/app/style.go` | — | — | — | ✅ `go build` clean | — | — |
| 2.3 | Verify | — | — | — | ✅ full 13-package suite green | — | — |
| 3.1 | `internal/app/keys_test.go` (`TestConfirmSelection_EmptyGuardNoticeIsSpanish`) | Unit | ✅ existing `keys_test.go` suite green pre-change | ✅ failed (English string) | ✅ passed | ➖ single guard scenario (spec has one) | ➖ none needed |
| 3.2 | `internal/app/keys_test.go` (`TestBackTransitions_ClearStaleNotice`) | Unit | ✅ (same run) | ✅ failed on all 7 sub-cases (stale notice not cleared) | ✅ passed on all 7 | ✅ 7 sub-cases (6 design rows; `keyTicket` esc + empty-q tested separately) | ➖ none needed |
| 3.3 | `internal/app/keys.go` | — | — | — | ✅ `go build` clean | — | — |
| 3.4 | `internal/app/keys.go` | — | — | — | ✅ `go build` clean | — | — |
| 3.5 | Verify | — | — | — | ✅ full 13-package suite green; grepped for stale English string, none found | — | — |
| 4.1 | `internal/git/service_cherrypick_test.go` (`TestCherryPickArgs_IncludesDashX`) | Unit (mocked `capturingRunner`) | ✅ existing `CherryPick*`/`ContinueCherryPick` tests green pre-change | ✅ failed (`-x` absent, positioning checks failed) | ✅ passed | ➖ single scenario (arg-composition, no branching) | ➖ none needed |
| 4.2 | `internal/git/service_cherrypick.go` | — | — | — | ✅ `go build` clean | — | — |
| 4.3 | Verify | — | — | — | ✅ full 13-package suite green, incl. all real-git e2e cherry-pick scenarios | — | — |

### Test Summary
- **Total tests written**: 6 new test functions (`TestPromotionBranchPrefix_TableDriven`,
  `TestService_CandidateBranches_ExcludesPromotionBranch`,
  `TestMark_NonAsciiUsesBrightColors`, `TestConfirmSelection_EmptyGuardNoticeIsSpanish`,
  `TestBackTransitions_ClearStaleNotice` [7 sub-cases], `TestCherryPickArgs_IncludesDashX`)
- **Total tests passing**: all of the above + full pre-existing suite (13 packages)
- **Layers used**: Unit (6 new functions, 1 using a real temp-repo git harness)
- **Approval tests** (refactoring): None — no refactoring tasks, all 4 fixes are additive/behavioral
- **Pure functions created**: 2 (`git.PromotionBranchPrefix`, `git.excludePromotionBranches`)

## Deviations from Design

1. **D1 signature threading (necessary, not scope creep)**: design.md's File Changes
   table for D1 lists only `internal/git/promotion_branch.go` and
   `internal/git/service_branches.go`. Making `CandidateBranches`'s filter genuinely
   config-driven (never hardcoding `"deploy/"`, per the design's own rejected-alternative
   note) requires `CandidateBranches` to receive `cfg config.Config`. This was threaded
   through:
   - `CandidateBranches(ctx, dir, ticket string, cfg config.Config)` — new 4th param.
   - `git.DiscoverOptions` gained a `Cfg config.Config` field (additive, keyed-literal-safe
     — every existing `DiscoverOptions{...}` construction site compiles unchanged and
     defaults to a zero-value `cfg`, which `PromotionBranchPrefix` maps to `""`, which the
     filter's guard treats as "skip filtering" — so no existing behavior changed).
   - `internal/app/commands.go`'s 3 `g.Discover(...)` call sites (`discoverCmd` x2,
     `confirmSourceCmd` x1) now pass `Cfg: cfg` — required so the real running app
     actually excludes `deploy/*` candidates end-to-end, not just in an isolated unit
     test. `confirmSourceCmd` gained a `cfg := m.deps.Config` local (it didn't fetch cfg
     before).
   - 5 existing test call sites (`service_branches_test.go` x3,
     `discovery_deleted_branch_test.go` x1, `discovery_e2e_test.go` x2) mechanically
     updated to pass `config.Config{}` as the new 4th arg — zero-value cfg preserves their
     pre-existing (unfiltered) behavior/intent unchanged.

   This is flagged as a deviation because it touches more files than design.md's D1 File
   Changes table states, but it is the minimal change required for the bug fix to have
   any real effect in the running app (the design's own "Rejected: hardcoded 'deploy/'"
   note implies cfg-awareness was always intended to reach the real call path).

2. **D4 RED test assertion (test-shape only, not a behavior deviation)**: task 4.1's RED
   test could not assert `-x` positioned before a specific "range form" rev string —
   `Service.CherryPick`'s real `cherryPickRevs` (unlike the pure `CherryPickRevisions`
   helper covered by `TestCherryPickRevisions_TableDriven`) resolves against a
   `capturingRunner` with no real backing git repo and produced an explicit SHA list, not
   the contiguous range form. The test instead asserts `-x` immediately follows
   `"cherry-pick"` (before any rev, regardless of the revs' shape), which is a stronger
   and more precise positional assertion than the original phrasing implied.

3. **Residual, unchanged (per design's own Open Questions)**: `view.go`'s plan-preview
   text (`git cherry-pick <sha>`) still omits `-x` in the printed preview command —
   design.md explicitly flags this as "cosmetic mismatch; out of task scope" and it was
   left untouched, consistent with the 16-task list (no task assigns it).

## Issues Found

None — no pre-existing test failures encountered; no infrastructure blockers.

## Files Changed

| File | Action | Fix |
|------|--------|-----|
| `internal/git/promotion_branch.go` | Modified | D1: added `PromotionBranchPrefix(cfg)` |
| `internal/git/promotion_branch_test.go` | Modified | D1: RED test `TestPromotionBranchPrefix_TableDriven` |
| `internal/git/service_branches.go` | Modified | D1: `CandidateBranches` gains `cfg` param + `excludePromotionBranches` filter |
| `internal/git/service_branches_test.go` | Modified | D1: RED test + 3 existing call sites updated |
| `internal/git/discovery.go` | Modified | D1 (deviation): `DiscoverOptions.Cfg` field threaded to `CandidateBranches` |
| `internal/git/discovery_deleted_branch_test.go` | Modified | D1 (deviation): existing call site updated |
| `internal/git/discovery_e2e_test.go` | Modified | D1 (deviation): 2 existing call sites updated |
| `internal/app/commands.go` | Modified | D1 (deviation): 3 `Discover(...)` call sites now pass `Cfg: cfg` |
| `internal/app/style.go` | Modified | D2: bright ANSI 10/11/9 |
| `internal/app/style_test.go` | Modified | D2: RED test `TestMark_NonAsciiUsesBrightColors` |
| `internal/app/keys.go` | Modified | D3: Spanish guard string + `m.notice = ""` on 6 back-transitions |
| `internal/app/keys_test.go` | Modified | D3: 2 RED tests (`TestConfirmSelection_EmptyGuardNoticeIsSpanish`, `TestBackTransitions_ClearStaleNotice`) |
| `internal/git/service_cherrypick.go` | Modified | D4: `cherryPickArgs` inserts `-x` |
| `internal/git/service_cherrypick_test.go` | Modified | D4: RED test `TestCherryPickArgs_IncludesDashX` |

## Workload / PR Boundary

- Mode: single PR, `size:exception` (pre-accepted, Low budget risk — 326 changed lines,
  under the 400-line budget anyway)
- Current work unit: all 4 (D1→D2→D3→D4), delivered as one apply batch per the tasks
  artifact's "Suggested Work Units" (sequential commits inside the single PR — commit
  boundaries left to the orchestrator, not created here per instructions: do not commit)
- Boundary: starts at the pre-change baseline (full suite green), ends at all 16 tasks
  complete with the full suite green
- Estimated review budget impact: Low — matches forecast

## Final Verification

```
PATH=/usr/local/go/bin:$PATH go build ./...   # clean, no output
PATH=/usr/local/go/bin:$PATH go test ./... -race -count=1
ok  	github.com/malavolta/DeployDeck/cmd/deploydeck	6.470s
ok  	github.com/malavolta/DeployDeck/internal/ai	1.890s
ok  	github.com/malavolta/DeployDeck/internal/app	28.234s
ok  	github.com/malavolta/DeployDeck/internal/config	1.191s
ok  	github.com/malavolta/DeployDeck/internal/delta	8.471s
ok  	github.com/malavolta/DeployDeck/internal/exec	1.987s
ok  	github.com/malavolta/DeployDeck/internal/git	54.325s
ok  	github.com/malavolta/DeployDeck/internal/github	1.974s
ok  	github.com/malavolta/DeployDeck/internal/prereq	4.472s
ok  	github.com/malavolta/DeployDeck/internal/runs	2.154s
ok  	github.com/malavolta/DeployDeck/internal/salesforce	1.916s
ok  	github.com/malavolta/DeployDeck/internal/update	1.916s
ok  	github.com/malavolta/DeployDeck/internal/version	1.162s
```
