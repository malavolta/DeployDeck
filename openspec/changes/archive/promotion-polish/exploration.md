# Exploration — promotion-polish (4 fixes for 1.0.0)

Bundle of 4 fixes found while dogfooding v0.2.6. Facts below verified against the
codebase via CodeGraph.

## Fix 1 — Exclude the tool's own `deploy/*` branches from source candidates (ROOT of the "0 commits" bug)

**Root cause (confirmed with live evidence):** `CandidateBranches(dir, ticket)`
(`internal/git/service_branches.go:39`) lists candidate SOURCE branches via
`git branch --list '*<ticket>*'` (local + remote, deduped). A leftover promotion
branch like `deploy/DEMO-2-to-INT` CONTAINS the ticket, so it is returned as a
"candidate source branch". With 2 candidates (`DEMO-2-mi-cambio` + `deploy/...`),
`resolveSource` (`internal/app/flow.go:53`) hits the current-branch confirm
(`resolveNeedsConfirm`); declining it degrades to message-only → `OrderedCommits`
empty → the selection screen shows "No se encontraron commits en el rango"
(0 commits) even though git shows the commits ARE in the range. This is why the
user must delete `deploy/...` before every re-run.

**Fix:** exclude branches that match the tool's own promotion-branch shape from
`CandidateBranches`. The promotion format is `config.BranchFormat`
(default `deploy/{{ticket}}-to-{{target}}`, rendered by `RenderBranchName`). Derive
a matcher from it — either the literal prefix up to the first `{{` token
(`deploy/`), or a glob/regex with the tokens as wildcards — and drop candidates
(local AND `origin/`) matching it. Never a legitimate source. Scoped to
`CandidateBranches` only; `ListBranches`/`branchesByPattern` untouched.

## Fix 2 — Brighter/more-visible semantic colors

**Current:** `internal/app/style.go` uses `lipgloss.Color("2")` (green), `"3"`
(amber), `"1")` (red) — the ANSI BASE palette (0–7), which renders muted in dark
themes (the "no veo los colores" report: green [OK] on Doctor is there but subtle).

**Fix:** switch to the BRIGHT ANSI variants (`"10"` bright-green, `"9"` bright-red,
`"11"` bright-yellow) or explicit truecolor hex, for more pop. The `color_test.go`
Ascii `TestMain` still forces plain text under tests → the 115 assertions are
unaffected. Optionally dim the context bar / footer keys (`styleDim`) so status
color stands out more by contrast — keep restrained.

## Fix 3 — English notice + notice-bleed across screens

**Current:** the guard notice `"select at least one commit before continuing"`
(set in `internal/app/keys.go`, the confirm-selection / empty-selection path) is in
ENGLISH, and it PERSISTS across screen navigation — it renders on the ticket and
Doctor screens (seen in the user's screenshots) because `m.notice` is not cleared
on the `esc`/back transitions.

**Fix:** (a) translate it (e.g. "selecciona al menos un commit para continuar");
(b) clear `m.notice` on back/navigation transitions so a notice never bleeds onto
an unrelated screen. Audit the `case "esc"` / back handlers that change `m.state`
without resetting `m.notice`. Part of the ongoing Spanish-copy correctness of the
`tui-presentation` capability.

## Fix 4 — `git cherry-pick -x` provenance

**Current:** `cherryPickArgs` (`internal/git/service_cherrypick.go:179`) runs
`cherry-pick <revs>` — no `-x`, so promoted commits carry no provenance link.
DeployDeck already detects "already applied" by CONTENT (git cherry + patch-id +
SHA ancestry) against the TARGET, and re-promotion remaps by patch-id — but there
is no explicit source-SHA trailer.

**Fix:** add `-x` → `cherry-pick -x <revs>`, appending
`(cherry picked from commit <sha>)` to each promoted commit. Benefits: (a) human
traceability in the PR (reviewer sees each commit's origin); (b) a durable,
in-history equivalence signal that survives conflict-resolutions which change the
patch-id. Enables the later incremental-promotion feature (Change B) to detect
"already on the deploy branch" via the trailer. Update the cherry-pick arg-assertion
tests (`service_cherrypick_test.go`) to include `-x`.

## Risks / notes

- Fix 1: the BranchFormat matcher must be robust to a custom `branchFormat`; use the
  token-prefix or a rendered-with-wildcards matcher, not a hardcoded "deploy/".
- Fix 3: only clear `m.notice` where a notice SHOULD not persist; don't clear
  notices that a screen legitimately owns (verify per transition).
- Fix 4: `-x` uses the FULL sha in the trailer; harmless for validation but changes
  commit messages — no behavioral risk, but arg-assertion tests must update.
- Strict TDD throughout; boundary invariant preserved (all changes are git-args /
  view / config-derived logic — no new exec/http imports).
- Delivery: single-PR, size:exception auto-accepted (per the 1.0.0 auto directive).

Artifact store: OpenSpec (Engram MCP unavailable to SDD sub-agents).
