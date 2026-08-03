# Tasks: promotion-polish (4 fixes for 1.0.0)

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | ~220-320 (5 files modified, 0 new; RED tests in 4 existing `*_test.go` files) |
| 400-line budget risk | Low |
| Chained PRs recommended | No |
| Suggested split | Single PR, 4 sequential commits (D1→D2→D3→D4, independent) |
| Delivery strategy | single-pr |
| Chain strategy | size-exception |

Decision needed before apply: No
Chained PRs recommended: No
Chain strategy: size-exception
400-line budget risk: Low

Rationale: `size:exception` is pre-accepted for this 1.0.0-polish PR; estimate is
well under the 400-line budget anyway (4 small, independent, code-local fixes,
no new files). No further decision is needed before `sdd-apply`.

### Suggested Work Units (sequential commits inside the single PR)

| Unit | Goal | Likely PR | Focused test command | Runtime harness | Rollback boundary |
|------|------|-----------|----------------------|-----------------|-------------------|
| 1 | D1 — exclude `deploy/*` candidates | PR 1 (commit 1) | `go test ./internal/git/... -run "PromotionBranchPrefix\|CandidateBranches"` | N/A — pure filter, no exec boundary | Revert `promotion_branch.go`'s new func + `service_branches.go`'s filter block |
| 2 | D2 — bright semantic colors | PR 1 (commit 2) | `go test ./internal/app/... -run TestMark` | N/A — pure render, Ascii `TestMain` covers regression | Revert 3 `lipgloss.Color(...)` literals in `style.go` |
| 3 | D3 — Spanish notice + no bleed | PR 1 (commit 3) | `go test ./internal/app/... -run "Notice\|GuardedQ\|Selection\|Ticket\|Target\|PlanPreview\|SourceConfirm\|QueueReview"` | `go test ./internal/app/... -race` (existing e2e flow suites double as regression harness) | Revert the string literal + 6 `m.notice = ""` insertions in `keys.go` |
| 4 | D4 — `cherry-pick -x` | PR 1 (commit 4) | `go test ./internal/git/... -run CherryPick` | N/A — arg composition only, no live-git harness needed (already exec-mocked) | Revert the `-x` insertion in `cherryPickArgs` |

## Phase 1: D1 — Exclude Promotion Branches (commit-discovery)

- [x] 1.1 RED: `internal/git/promotion_branch_test.go` — `TestPromotionBranchPrefix`: default `cfg.BranchFormat` → `"deploy/"`; custom `"promo/{{ticket}}"` → `"promo/"`; token-leading format → `""`. Compile fails (`PromotionBranchPrefix` undefined).
- [x] 1.2 RED: `internal/git/service_branches_test.go` — `TestCandidateBranches_ExcludesPromotionBranch`: pattern set with local + `origin/` `deploy/DEMO-2-to-INT` plus a real `DEMO-2` feature branch; assert both `deploy/...` forms are excluded, feature branch survives.
- [x] 1.3 GREEN: `internal/git/promotion_branch.go` — add `func PromotionBranchPrefix(cfg config.Config) string` (literal prefix of `cfg.BranchFormat` up to first `"{{"`).
- [x] 1.4 GREEN: `internal/git/service_branches.go`'s `CandidateBranches` — after `branchesByPattern`, before `dedupeByLocalName`, drop entries with `strings.HasPrefix(name, prefix)` or `strings.HasPrefix(name, "origin/"+prefix)`; skip filter when `prefix == ""`. (Deviation, documented: `CandidateBranches` gained a `cfg config.Config` parameter — required for the filter to be config-driven rather than hardcoded — threaded through `DiscoverOptions.Cfg` and the 3 `Discover(...)` call sites in `internal/app/commands.go` so the fix is real end-to-end, not just unit-testable in isolation. 5 existing test call sites updated mechanically with `config.Config{}`.)
- [x] 1.5 Verify: `go build ./...` && `PATH=/usr/local/go/bin:$PATH go test ./... -race` — GREEN (`internal/git` + `internal/app` packages re-run in full, both green).

## Phase 2: D2 — Bright Semantic Colors (tui-presentation)

- [x] 2.1 RED: `internal/app/style_test.go` — `TestMark_NonAsciiUsesBrightColors`: under `termenv.TrueColor`, `mark("OK")`/`mark("!!")`/`mark("XX")` contain the bright ANSI codes `10`/`11`/`9` (not base `2`/`3`/`1`); Ascii baseline still plain. Fails against current base-ANSI styles. (Asserted the actual rendered SGR escapes — `\x1b[92m`/`\x1b[93m`/`\x1b[91m` present, `\x1b[32m`/`\x1b[33m`/`\x1b[31m` absent — verified empirically that lipgloss/termenv render ANSI codes 0-15 as literal SGR regardless of TrueColor/ANSI/ANSI256 profile, so this is stable across profiles.)
- [x] 2.2 GREEN: `internal/app/style.go` — `styleOK` → `Color("10")`, `styleWarn` → `Color("11")`, `styleErr` → `Color("9")`.
- [x] 2.3 Verify: `go build ./...` && `PATH=/usr/local/go/bin:$PATH go test ./... -race` — GREEN (full 13-package suite).

## Phase 3: D3 — Spanish Notice + Stop Notice-Bleed (tui-presentation)

- [x] 3.1 RED: `internal/app/keys_test.go` — `TestConfirmSelection_EmptyGuardNoticeIsSpanish`: zero-selection `confirmSelection()` sets `m.notice` to `"selecciona al menos un commit para continuar"`. Fails (current string is English).
- [x] 3.2 RED: `internal/app/keys_test.go` — table test over the 6 back-transitions (`keySelection` esc:932, `keyTicket` esc:875/empty-`q`:873, `keyTarget` esc:979, `keyPlanPreview` esc:1011, `keySourceConfirm` esc:908, `keyQueueReview` esc:1165): pre-set `m.notice`, send the key, assert `m.notice == ""` after. Fails on all 6 (none currently clear). (Implemented as `TestBackTransitions_ClearStaleNotice`, 7 sub-cases — `keyTicket`'s esc and empty-`q` paths tested separately since both code paths needed the fix.)
- [x] 3.3 GREEN: `internal/app/keys.go:942` — translate the guard string to `"selecciona al menos un commit para continuar"`.
- [x] 3.4 GREEN: `internal/app/keys.go` — add `m.notice = ""` to `keySelection` esc, `keyTicket` esc/empty-`q`, `keyTarget` esc, `keyPlanPreview` esc, `keySourceConfirm` esc, `keyQueueReview` esc.
- [x] 3.5 Verify: `go build ./...` && `PATH=/usr/local/go/bin:$PATH go test ./... -race` — GREEN (full 13-package suite; confirmed no other reference to the old English string).

## Phase 4: D4 — `cherry-pick -x` Provenance (cherry-pick)

- [x] 4.1 RED: `internal/git/service_cherrypick_test.go` — `TestCherryPickArgs_IncludesDashX`: cherry-pick call `Args` contain `"-x"`, positioned after `"cherry-pick"` and before the revs (reuse `capturingRunner`/`argsContain`/`idxOf` helpers already in the file). Fails (`-x` absent). (Assertion checks `-x` immediately follows `"cherry-pick"` rather than a specific revs-form string, since `Service.CherryPick`'s real `cherryPickRevs` computation — unlike the pure `CherryPickRevisions` helper — produced an explicit SHA list, not the range form, against a `capturingRunner` with no real repo backing.)
- [x] 4.2 GREEN: `internal/git/service_cherrypick.go`'s `cherryPickArgs` — insert `"-x"` after `"cherry-pick"`, before revs. `continueArgs`/`skipArgs` stay unchanged.
- [x] 4.3 Verify: `go build ./...` && `PATH=/usr/local/go/bin:$PATH go test ./... -race` — GREEN (full 13-package suite, including all real-git e2e cherry-pick scenarios).
