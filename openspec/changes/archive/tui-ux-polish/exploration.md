# Exploration — tui-ux-polish

UX/UI audit of the DeployDeck TUI produced 8 fixes. This exploration maps how each is
implementable against `internal/app`, and resolves the key risk (color vs. the test suite).

## Current state

- `internal/app/view.go` (~1120 lines): every screen is a `view*()` method on `Model`
  returning a plain string (strings.Builder + fmt.Sprintf), routed by `viewBody()`'s switch
  on `m.state`.
- Shared seams: `header(title)` (25 call sites) and `footer(keys)` (26 call sites) — every
  screen uses both. `statusMark()` (OK/!!/XX/--) and `conflictMark()` (D/B/U).
- Status convention `[OK]`/`[!!]`/`[XX]`/`[i]` is NOT centralized: `statusMark` returns them
  in `viewPrereq`, but ~10 other screens hand-write the literal bracket tokens. Color mapping
  must key off these bracketed tokens, not only `statusMark`.
- Tests: NO `teatest`, NO `.golden` files. All view tests assert via `strings.Contains(v, "...")`
  — **115 occurrences across 25 files**; 106 `.View()` calls. No full-string `==` equality
  (only `v == ""` checks). So whitespace/line-count changes inside `header`/`footer` are safe;
  only literal-text changes can break a test.

## KEY DECISION — semantic color without breaking tests (top risk, RESOLVED)

Adding ANSI escape codes to `View()` output would break the 115 `strings.Contains` assertions.
Resolution (Approach A, recommended and adopted):

- `github.com/charmbracelet/lipgloss v1.1.0` is **already an indirect go.mod dependency**
  (+ `colorprofile` v0.2.3). Making it direct is just `go mod tidy` — no new module, no version
  risk. lipgloss is a pure rendering lib (no exec/net) → does NOT violate `boundary_test.go`.
- Add a net-new test file with `TestMain` that calls `lipgloss.SetColorProfile(termenv.Ascii)`
  (no existing `TestMain` in `internal/app`). This forces every `View()` under `go test` to
  render byte-identical plain text → **zero changes to the 115 existing assertions.**
- Production (`main.go` real terminal) auto-detects the TTY/`NO_COLOR` profile independently —
  untouched.
- Add `internal/app/style.go` with named styles (styleOK/styleWarn/styleErr/styleBold/styleDim)
  used at the `header`/`footer`/status-token seams.
- Rejected: relying on implicit non-TTY auto-detection under `go test` (upstream lipgloss #439:
  flaky, directory-dependent). Rejected: rewriting 115 assertions / migrating to golden files
  (a separate, much larger refactor — out of scope here).

## The 8 fixes — implementability

1. **Semantic color + hierarchy** — foundation; via style.go + wrapping the `[OK/!!/XX/i]`
   tokens (10+ sites) and header titles. Lands first.
2. **Context bar** (repo · branch · org) — repo=`m.deps.Dir` (often `.`); branch=`m.originalBranch`
   (async, empty on first frames → degrade gracefully); **org has no single source** — only
   resolved once a target/sandbox is chosen (`m.plan.SandboxAlias`), so on menu/ticket/select
   screens there is no org yet. DESIGN DECISION NEEDED: placeholder ("—") vs. omit the org segment
   until resolved.
3. **Standardized footers + always-quit** — `footer()` 26 sites. `viewTicket` genuinely lacks a
   `q` case (typing `q` appends to the buffer). Fix via the existing GUARDED idiom
   (`keyPackageSelect`'s `q`): `if m.ticket != "" { append } else { quit }`. NUANCE: `q` has two
   meanings today — quit-app (prereq/error/menu) vs. go-back-to-menu (packageSelect/deltaSource).
   Standardization must PRESERVE the distinction, not flatten it.
4. **Better empty states** — `viewSelection` already special-cases `len(items)==0` for the body
   but its `footer()` is unconditional ("Space marcar / Enter continuar" with nothing to mark).
   Localized fix: conditional footer branch (precedent: `viewPackageReview`'s footer switch).
5. **Accent/¿¡ copy sweep** — WIDER than view.go: `m.notice` literals are set in `keys.go` (40)
   and `update.go` (6). `viewSourceConfirm` is the one already-correct reference (¿…? + [s/N]);
   its test asserts the exact accented string. Verify per-string (some titles/notices asserted
   verbatim).
6. **Spinner** — NO `charmbracelet/bubbles` dep; do NOT add it. Reuse the existing `tickCmd`
   idiom (`tea.Tick`) + a `spinnerFrame` field + `spinnerTickMsg`. Target static states:
   CommitDiscovery, BranchCreation, CherryPicking, DeltaGeneration, ValidationStart.
   (ValidationPolling already ticks.)
7. **Column headers** — QueueReview/RunHistory/BranchCleanup have no header row; purely additive
   one `fmt.Sprintf` label line matching existing `%-Ns` widths. Cheapest fix.
8. **Menu descriptions + glossary** — `menuEntry` (app.go) has no description field; add it + the
   3-entry var + render loop. Unify "rama base"/"rama sugerida"/"rama origen" vocabulary.

## Design decisions to carry into propose

1. Adopt Approach A for color (TestMain forces Ascii profile). CONFIRMED feasible.
2. Context bar target-org on pre-target screens: placeholder vs. omit — decide in propose/design.
3. Footer `q`: preserve quit-vs-goback distinction; guarded-q on the 3 free-text buffer screens
   (ticket, cancel CANCELAR, quick DESPLEGAR).

## Size & slicing

Estimated ~630–960 lines (incl. RED tests under strict TDD) vs. the 800-line single-PR budget →
at-or-over budget; surface to tasks/orchestrator for a size:exception vs. slice decision.

Recommended internal commit/slice order (within one PR):
1. Color infra + `TestMain` Ascii safety net + go.mod tidy (foundation).
2. Footer standardization + context bar (both header/footer-seam changes).
3. Empty states + column headers + menu descriptions (mechanical, additive).
4. Accent/¿¡ copy sweep (isolate; touches most files; easiest to review/revert alone).
5. Spinner (self-contained new feature).

## Risks

- ANSI-vs-tests: mitigated (Approach A). Residual: any NEW test asserting colored output must be
  written against the Ascii-forced plain string.
- `q` collides with free-text buffers on 3 screens → guarded per screen, not global.
- `q` has two meanings (quit vs go-back) → footer grammar must preserve it.
- Org not universally available for the context bar → placeholder/omit decision.
- Copy sweep leaks into keys.go/update.go, not just view.go.
- Combined size ~630–960 vs 800 budget → forecast decision after tasks.

Artifact store: OpenSpec (Engram MCP tools were not available to the SDD sub-agents).
