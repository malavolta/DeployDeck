# Proposal: promotion-polish (4 fixes for 1.0.0)

Delivery phase: Fase 5 Productizacion (1.0.0 polish), touching Fase 1 Git
discovery + cherry-pick. Single PR, size:exception accepted, strict TDD.

## Intent

Four dogfooding defects from v0.2.6 block a clean 1.0.0: a recurring "0 commits"
bug forcing manual branch deletion, muted status colors, an English notice that
bleeds across screens, and promoted commits with no provenance link. Each is
small, code-grounded, and low-risk; bundling avoids four trivial PRs.

## Scope

### In Scope
- **D1** — Exclude the tool's own promotion branches from source candidates.
- **D2** — Brighter semantic status colors.
- **D3** — Spanish guard notice + stop notice-bleed across screens.
- **D4** — `git cherry-pick -x` provenance trailer.

### Out of Scope
- The incremental append-to-deploy-branch feature (separate change
  `incremental-promotion`; D4 only *seeds* its trailer-based detection).
- `ListBranches` / `branchesByPattern` behavior; footer/context-bar redesign.

## Capabilities

### New Capabilities
- None.

### Modified Capabilities
- `commit-discovery`: candidate source search MUST exclude tool-owned promotion branches.
- `tui-presentation`: brighter semantic colors; guard notice localized and cleared on navigation.
- `cherry-pick`: promoted commits MUST carry a source-SHA provenance trailer.

## Approach

- **D1**: derive the matcher from `config.BranchFormat` — literal prefix up to
  the first `{{` token (default `deploy/`), robust to custom formats, no
  hardcoded `deploy/`. Drop bare and `origin/`-prefixed matches. Scoped to
  `CandidateBranches` only (`service_branches.go`).
- **D2**: `style.go` moves ANSI base 2/3/1 → bright ANSI 10/9/11 (theme-friendly;
  hex rejected to respect user terminal themes). Ascii `TestMain` invariant kept
  → 115 color assertions unaffected.
- **D3**: translate to "selecciona al menos un commit para continuar"; clear
  `m.notice` only on back/nav transitions that change `m.state` where a notice
  must not persist (audit per transition; never clear a screen-owned notice).
- **D4**: append `-x` to `cherryPickArgs` (`service_cherrypick.go`); update
  arg-assertion tests.

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `internal/git/service_branches.go` | Modified | Candidate exclusion (D1) |
| `internal/config` (BranchFormat) | Read | Derive matcher prefix (D1) |
| `internal/app/style.go` | Modified | Bright colors (D2) |
| `internal/app/keys.go` + back handlers | Modified | Notice text + clearing (D3) |
| `internal/git/service_cherrypick.go` | Modified | Add `-x` (D4) |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| D1 matcher too broad, hides real source | Low | Prefix from BranchFormat; test bare + origin/ forms |
| D3 clears a legitimately-owned notice | Med | Clear per-transition only; test each screen |
| D4 changes commit messages | Low | Trailer only; no behavior; update arg tests |

## Rollback Plan

Pure revert of the single PR; each fix is independent and self-contained. No
migration, no persisted-state change. Reverting restores prior git args, colors,
copy, and candidate list with no cleanup.

## Success Criteria

- [ ] Re-running a promotion with a leftover `deploy/*` branch present yields the correct non-empty commit range (no manual deletion).
- [ ] Status colors render visibly in a dark-theme TTY; 115 color assertions still pass.
- [ ] Guard notice reads in Spanish and never renders on ticket/Doctor screens.
- [ ] Promoted commits carry `(cherry picked from commit <sha>)`; arg tests assert `-x`.
