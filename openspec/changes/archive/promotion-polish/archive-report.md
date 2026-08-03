# Archive Report — promotion-polish

**Status:** archived · **Delivery:** single-PR (size:exception) · **Mode:** strict TDD · **Milestone:** 1.0.0

## Summary

Four dogfooding fixes found on v0.2.6, bundled for 1.0.0:
1. **D1 — exclude the tool's own promotion branches from source candidates** (fixes the recurring
   "0 commits in range" dead-end). A leftover `deploy/<ticket>-to-<target>` matched the ticket glob
   and became a spurious second source candidate → declining the resulting confirm degraded to 0
   commits. New `git.PromotionBranchMatcher(cfg)` renders `config.BranchFormat` into an anchored
   regex (tokens → `[^/]+`) and `CandidateBranches` (now `cfg`-threaded via `DiscoverOptions.Cfg`)
   drops branches matching the full SHAPE — bare and `origin/`-prefixed — BEFORE dedupe. Empty-format
   guard skips filtering (never drops everything).
2. **D2 — brighter semantic colors**: `style.go` OK/warn/err → bright ANSI `10`/`11`/`9`; the Ascii
   `TestMain` keeps all assertions plain.
3. **D3 — Spanish empty-selection guard notice + no cross-screen bleed**: translated, and `m.notice`
   cleared on 6 back/nav transitions so it never renders on the ticket/Doctor screens.
4. **D4 — `git cherry-pick -x` provenance**: `(cherry picked from commit <sha>)` trailer on every
   pick, for traceability and a durable equivalence signal.

## SDD trail

explore (orchestrator-authored) → propose → spec + design → tasks (16) → apply (16/16, strict TDD) →
verify (PASS, 0 CRITICAL) → adversarial reliability review → remediation → archive. Artifact store:
OpenSpec (Engram MCP unavailable).

## Review & remediation

- Reliability review found ONE real WARNING: D1 excluded by the format's static PREFIX, which also
  dropped legitimate branches sharing that prefix (e.g. `deploy/DEMO-2` without the `-to-<target>`
  shape). Remediated: switched to FULL-SHAPE regex matching (`PromotionBranchMatcher`), so
  prefix-only branches are kept. Added tests for both.
- Verify WARNING (D4 conflict-path trailer untested) remediated with a real-git e2e that induces a
  conflict + `--continue` and asserts the `(cherry picked from ...)` trailer survives.
- Verify SUGGESTION (plan preview lacked `-x`) fixed so the shown command matches the executed one.

## Spec merge note

The three delta specs (`commit-discovery`, `tui-presentation`, `cherry-pick` — all MODIFIED/ADDED
requirements on EXISTING capabilities) were merged ADDITIVELY into the living specs (replace the
modified requirement block, append the added ones), not wholesale-replaced. Living specs grew
187→205 / 122→154 / 156→170; no pre-existing requirement was lost. The commit-discovery requirement
text was updated to describe the remediated FULL-SHAPE matching (not the pre-remediation prefix).

## Verification

- `go build ./...`, `go vet ./...`, `gofmt -l internal` clean.
- `go test ./... -race -count=1` green across all 13 packages (independently re-run by the orchestrator).
- Exec-boundary invariant preserved (regex/view/config-derived logic; no new exec/http imports).
- Diffstat (code): ~326 lines + the additive spec merges.

## Follow-up

The incremental append-to-existing-PR workflow is the SEPARATE `incremental-promotion` change (1.0.0);
D4's `-x` trailer seeds its already-on-branch detection.
