# Verification Report: promotion-polish

**Change**: `promotion-polish` (4 fixes for 1.0.0 — D1 exclude promotion branches, D2 bright colors, D3 Spanish guard + no notice-bleed, D4 cherry-pick `-x`)
**Mode**: Strict TDD (runner: `PATH=/usr/local/go/bin:$PATH go test ./... -race`)
**Artifact store**: OpenSpec (file-based)
**Branch**: `feat/promotion-polish` (applied, not committed)
**Verdict**: **PASS WITH WARNINGS** — 0 CRITICAL, 1 WARNING, 2 SUGGESTION

## Completeness

| Dimension | Result |
|---|---|
| Tasks complete | 16/16 checked; all match actual code state |
| Requirements | 4 (commit-discovery x1 MODIFIED, tui-presentation x1 MODIFIED + x1 ADDED, cherry-pick x1 ADDED) |
| Scenarios | 12 total |
| Artifacts read | proposal, design, 3 delta specs, tasks, apply-progress |

## Build / Test / Static Evidence

| Command | Exit | Result |
|---|---|---|
| `go test ./... -race -count=1` | 0 | 13/13 packages `ok` |
| `go vet ./...` | 0 | clean |
| `gofmt -l internal` | 0 | no files listed (all formatted) |

6 new test functions re-run isolated and verbose — all PASS:
`TestPromotionBranchPrefix_TableDriven` (3 sub), `TestService_CandidateBranches_ExcludesPromotionBranch`,
`TestMark_NonAsciiUsesBrightColors`, `TestConfirmSelection_EmptyGuardNoticeIsSpanish`,
`TestBackTransitions_ClearStaleNotice` (7 sub), `TestCherryPickArgs_IncludesDashX`.

## Spec Compliance Matrix

| # | Requirement / Scenario | Covering test (runtime) | Status |
|---|---|---|---|
| **commit-discovery — Candidate Branch Search By Name (MODIFIED)** | | | |
| 1 | Ticket found in branch name | `TestService_CandidateBranches_ReturnsLocalAndRemoteMatchesByTicketName` | PASS |
| 2 | Pushed feature branch → single candidate + OrderedCommits non-empty | `TestHU002_Discover_E2E_SourceResolutionDedupe` (twin run, real git, asserts 1 candidate + 1 ordered commit) | PASS |
| 3 | Genuinely distinct branches not collapsed | `TestService_CandidateBranches_DoesNotOverCollapseDistinctNames` + e2e distinct-run | PASS |
| 4 | Tool's promotion branch excluded — bare + `origin/` | `TestService_CandidateBranches_ExcludesPromotionBranch` (real cfg, both forms excluded, feature survives, exactly 1 candidate) | PASS |
| 5 | Leftover promotion branch no longer false multi-candidate dead end + OrderedCommits non-empty | Exclusion runtime-verified (scenario 4, real cfg); non-empty range runtime-verified (scenario 2) — same code paths, but no single Discover-level test plants a leftover `deploy/*` with real cfg AND asserts non-empty `OrderedCommits` | PASS (by composition) — SUGGESTION |
| **tui-presentation — Semantic Color And Visual Hierarchy (MODIFIED)** | | | |
| 6 | Failed `XX` bright red (ANSI 9) at runtime, plain under test | `TestMark_NonAsciiUsesBrightColors` (`\x1b[91m` present, `\x1b[31m` absent) + `TestMark_AsciiProfileIsPlain` | PASS |
| 7 | OK bright green (ANSI 10) at runtime, plain under test | same (`\x1b[92m` present, `\x1b[32m` absent) | PASS |
| **tui-presentation — Localized Empty-Selection Guard (ADDED)** | | | |
| 8 | Guard notice shown in Spanish | `TestConfirmSelection_EmptyGuardNoticeIsSpanish` (asserts exact string + state unchanged) | PASS |
| 9 | Notice does not persist after going back | `TestBackTransitions_ClearStaleNotice` (7 sub-cases, notice=="" after each back key) | PASS |
| 10 | Notice does not bleed onto unrelated screen (Doctor) | `TestBackTransitions` `keyTicket esc`/`empty-q -> prereq` (Doctor state); `viewPrereq` renders notice only when non-empty | PASS |
| **cherry-pick — Source Provenance Trailer (ADDED)** | | | |
| 11 | Promoted commit message carries `(cherry picked from commit <sha>)` | `TestCherryPickArgs_IncludesDashX` (proves `-x` passed, positioned after `cherry-pick`, before revs). Real-git e2e run `-x` at runtime but assert only `--format=%s` subject, never the `%b` body trailer | PASS (by proxy) — SUGGESTION |
| 12 | Trailer present regardless of conflict path (clean OR `--continue`) | No covering runtime test — clean path runs `-x` (unasserted trailer); `--continue`/`--skip` `-x`-persistence claim is unexercised | UNTESTED — WARNING |

## Correctness (source inspection vs specs)

| Fix | Finding |
|---|---|
| **D1** | `PromotionBranchPrefix(cfg)` (`promotion_branch.go:72`) returns literal prefix up to first `{{`; returns `""` when token-leading/empty (empty-prefix guard present). `excludePromotionBranches` (`service_branches.go:67`) drops `HasPrefix(name, prefix)` **or** `HasPrefix(name, "origin/"+prefix)`, applied at `CandidateBranches` line 55 **before** `dedupeByLocalName` line 56 (correct per design). Guard: `if prefix == "" { return branches }` — never nukes the whole set. Scoped to `CandidateBranches` only; `ListBranches`/`branchesByPattern` untouched. **Correct.** |
| **D2** | `style.go` `styleOK=Color("10")`, `styleWarn=Color("11")`, `styleErr=Color("9")`. Ascii invariant intact: `TestMain` (`color_test.go:17`) forces `termenv.Ascii` for the whole binary; `mark()` structure unchanged. **Correct.** |
| **D3** | Spanish guard string at `keys.go:946` = `"selecciona al menos un commit para continuar"` (matches spec). `m.notice = ""` present on all 6 design back-transitions: `keySelection` esc (936), `keyTicket` esc (877) + empty-`q` (873), `keyTarget` esc (984), `keyPlanPreview` esc (1017), `keySourceConfirm` esc (911), `keyQueueReview` esc (1172). Each clears then sets `m.state` and returns — no destination handler runs in the same step, so no legitimately screen-owned notice is wiped (verified `keyTicket` "enter"-empty notice, `confirmTarget` block notices, `onQueueDone` permission notice all set on other paths). **Correct.** |
| **D4** | `cherryPickArgs` (`service_cherrypick.go:184`) = `gpgSignOff + "cherry-pick" + "-x" + revs...` — `-x` immediately after `cherry-pick`, before revs. `continueArgs`/`skipArgs` unchanged (no `-x`). **Correct** (behavior sound; runtime trailer assertion missing — see issues). |

## D1 Deviation — Design-Conformance Judgment: **CONFORMANT**

The documented deviation (`CandidateBranches` gains a `cfg config.Config` param + `DiscoverOptions.Cfg` field threaded through `discovery.go` + 3 app call sites) is judged **design-conformant and minimal**:

- **Threading confirmed correct end-to-end** (the filter actually runs in the real discovery path, not just unit tests):
  `discoverCmd` (`commands.go:497` and `:511-516`, both pass `Cfg: cfg`) and `confirmSourceCmd` (`:548-553`, with `cfg := m.deps.Config` fetched at `:537`) → `g.Discover(..., DiscoverOptions{Cfg: cfg})` → `Discover` (`discovery.go:153`) calls `s.CandidateBranches(ctx, dir, opts.Ticket, opts.Cfg)` → `excludePromotionBranches(branches, PromotionBranchPrefix(cfg))`. Real app path exercises the filter with the user's configured `branchFormat`.
- **Consistent with design intent**: design.md explicitly rejected hardcoding `"deploy/"`; making the filter config-driven necessarily requires cfg to reach `CandidateBranches`. This is the smallest change that gives the fix real effect.
- **Zero-value safety confirmed** for the 5 updated test call sites passing `config.Config{}`: `PromotionBranchPrefix(config.Config{})` → `""` (no `{{` found), and `excludePromotionBranches`'s `if prefix == ""` guard returns branches unmodified — so prior unfiltered behavior is preserved. `DiscoverOptions.Cfg` is additive/keyed-literal-safe; every existing construction defaults to zero-value cfg.

## TDD Compliance

| Check | Result | Details |
|---|---|---|
| TDD evidence reported | PASS | Full TDD Cycle Evidence table in apply-progress |
| All tasks have tests | PASS | 6 new test functions for 4 fixes; test files all exist |
| RED confirmed (tests exist) | PASS | 6/6 verified present in codebase |
| GREEN confirmed (tests pass) | PASS | 6/6 pass on isolated verbose re-run |
| Triangulation adequate | PASS | Prefix 3 cases; colors 3 tokens (present AND absent dual-assert); back-transitions 7 sub-cases; single-scenario fixes correctly single-cased |
| Safety net for modified files | PASS | apply-progress records full-suite green pre-change |

## Test Layer Distribution

| Layer | Tests | Notes |
|---|---|---|
| Unit (pure/mocked) | 4 new fns | `PromotionBranchPrefix`, `mark` colors, Spanish guard, `cherryPickArgs` (capturingRunner) |
| Unit (real temp-repo git) | 1 new fn | `TestService_CandidateBranches_ExcludesPromotionBranch` |
| Integration/reducer | 1 new fn | `TestBackTransitions_ClearStaleNotice` via `m.Update(keyPress(...))` |
| Existing real-git e2e (regression) | many | dedupe/discovery/cherry-pick e2e suites re-run green |

## Assertion Quality

All assertions verify real behavior. No tautologies, no ghost loops (no assertions inside possibly-empty loops), no smoke-only tests, no orphan empty-checks (empty/exclusion assertions are paired with non-empty survivor assertions), no mock-heavy imbalance. Bright-color test asserts both bright-present AND base-absent (strong). `-x` test asserts exact positional index (`xIdx == pickIdx+1`), stronger than membership.

**Assertion quality**: All assertions verify real behavior.

## Quality Metrics

**Linter/vet**: `go vet ./...` clean. **Formatter**: `gofmt -l internal` clean. **Coverage tool**: not configured for this run — coverage analysis skipped (not a failure).

## Issues

### CRITICAL
None.

### WARNING
- **D4 scenario 2 — "trailer present regardless of conflict resolution path" is UNTESTED.** No test exercises the `--continue`/`--skip` path to confirm the `(cherry picked from commit <sha>)` trailer survives. The design claims "the sequencer remembers `-x` across `--continue`/`--skip`" — this is documented git behavior, but no runtime test verifies it, and `continueArgs`/`skipArgs` deliberately omit `-x`. Behaviorally low-risk (deterministic git flag), but the spec scenario explicitly calls out the conflict path. Non-blocking for archive; recommend a real-git conflict+continue e2e asserting the body trailer.

### SUGGESTION
- **D4 scenario 1 — trailer verified only by proxy.** `TestCherryPickArgs_IncludesDashX` proves `-x` is passed; the real-git e2e (`TestHU006_CherryPick_E2E`) runs it but asserts only `git log --format=%s` (subject), never the `%b` body where the trailer lives. Add a `--format=%b` (or `%B`) assertion checking `(cherry picked from commit <sha>)` to lock the observable behavior directly.
- **D1 scenario 5 — reproduce the exact reported bug end-to-end.** The leftover-`deploy/*` exclusion (scenario 4, unit + real cfg) and non-empty `OrderedCommits` (scenario 2, e2e) are each runtime-verified over the same code paths, but no single `Discover`-level test plants a leftover `deploy/DEMO-2-to-INT` with a real `branchFormat` cfg AND asserts a single candidate + non-empty `OrderedCommits`. A dedicated integration test would nail the reported "0 commits dead end" narrative in one place.
- Residual (already flagged in design Open Questions, out of scope): `view.go` plan-preview prints `git cherry-pick <sha>` without `-x` — cosmetic mismatch only.

## Final Verdict

**PASS WITH WARNINGS.** All 16 tasks complete and match code state; full suite green (13 packages), vet + gofmt clean. All four fixes are correctly implemented per spec and design; the D1 signature-threading deviation is design-conformant and verified correct end-to-end in the real app path. The single WARNING is a runtime-coverage gap on a deterministic git-flag behavior (D4 `-x` trailer across `--continue`), not a functional defect — it does not block archive. No CRITICAL issues.
