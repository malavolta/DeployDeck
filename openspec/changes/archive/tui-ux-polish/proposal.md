# Proposal: TUI UX/UI Polish

Delivery phase: **Fase 5 Productizacion** (per `openspec/config.yaml` rules.proposal).

## Intent

A UX/UI audit of the DeployDeck TUI found 8 defects that make the tool harder to
read, orient in, and trust: no semantic color/hierarchy, no persistent context
(users lose track of repo/branch/org), inconsistent footers with `q` sometimes
un-offered, unhelpful empty states, broken Spanish accents/`¿¡`, no feedback
during long operations, unlabeled dense tables, and undescribed menu entries.
Fix all 8 in one PR so the polish lands as one coherent pass.

## Scope

### In Scope (the 8 fixes)
1. Semantic color + visual hierarchy (foundation).
2. Persistent context bar (repo · branch · org).
3. Standardized footers + always-offer-quit.
4. Better empty states (cause + next action; hide inapplicable keys).
5. Correct Spanish accents + `¿¡` copy sweep (view.go + keys.go + update.go notices).
6. Spinner for long operations.
7. Column headers for dense tables.
8. Menu descriptions + terminology glossary.

### Out of Scope (non-goals)
- Migrating the view test suite to golden files (separate, larger refactor).
- Fixing the "0 commits" discovery behavior (separate possible-bug investigation).
- Any change under `internal/exec`/net/http or module-boundary shifts.

## Capabilities

### New Capabilities
- `tui-presentation`: cross-cutting presentation/interaction requirements — color
  semantics, context bar, footer grammar + always-offer-quit, empty states,
  Spanish copy correctness, spinner feedback, table headers, menu descriptions.

### Modified Capabilities
- None. Domain specs (branch-cleanup, run-history, commit-selection, …) keep their
  behavior; only their rendered presentation changes, which the new capability owns.

## Decisions (cornerstone)

| # | Decision | Rationale |
|---|----------|-----------|
| D1 | **Adopt color Approach A**: make lipgloss (already an indirect go.mod dep) direct via `go mod tidy`; add a net-new `TestMain` in `internal/app` calling `lipgloss.SetColorProfile(termenv.Ascii)`; `internal/app/style.go` holds named styles (styleOK/styleWarn/styleErr/styleBold/styleDim) applied at header/footer/status-token seams. | Ascii profile forces byte-identical plain text under `go test` → **zero rewrites of the 115 `strings.Contains` assertions**. Production auto-detects TTY/`NO_COLOR`. lipgloss is pure-render → boundary invariant intact. Rejected: implicit non-TTY detection (flaky, upstream #439) and rewriting assertions/golden migration (out of scope). |
| D2 | **Context bar = omit-until-resolved**: always show repo·branch; append ·org only once `m.plan.SandboxAlias` (or a picked sandbox) exists. | Org has no source on pre-target screens (menu/ticket/select). Omitting is cleaner than a dangling `—` placeholder and never shows a stale/wrong org. |
| D3 | **Preserve the two `q` meanings** (quit-app vs. go-back-to-menu); never flatten. On the 3 free-text-buffer screens (ticket, cancel/CANCELAR, quick/DESPLEGAR) add `q` via the existing GUARDED idiom (append literal when buffer non-empty, else act). **Non-negotiable.** | `q` must stay typeable in free-text buffers; flattening would break go-back semantics and user muscle memory. |
| D4 | **Correct Spanish everywhere** (tildes + `¿¡`) across all user-facing strings, using `viewSourceConfirm` as the reference; sweep extends into keys.go/update.go notices. | Correctness + consistency; some notices/titles are asserted verbatim, so per-string verification is required. |
| D5 | **Spinner via existing `tea.Tick` idiom** — `spinnerFrame` field + `spinnerTickMsg` wired into static in-progress states (CommitDiscovery, BranchCreation, CherryPicking, DeltaGeneration, ValidationStart). | No new `bubbles` dependency; reuses the proven `tickCmd` pattern (ValidationPolling already ticks). |

## Approach — internal slice/commit order (one PR)

1. Color infra + `TestMain` Ascii safety net + `go mod tidy` (foundation).
2. Footer standardization + context bar (header/footer-seam changes).
3. Empty states + column headers + menu descriptions (mechanical, additive).
4. Accent/`¿¡` copy sweep (isolated; touches most files; easiest to review/revert).
5. Spinner (self-contained).

STRICT TDD: RED-first for every behavioral change; pure-copy edits verified against
existing assertions (new colored-output tests written against the Ascii plain string).

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `internal/app/style.go` | New | Named lipgloss styles. |
| `internal/app/*_test.go` (TestMain) | New | Forces Ascii profile under test. |
| `internal/app/view.go` | Modified | Context bar, footers, empty states, headers, color seams, copy. |
| `internal/app/keys.go`, `update.go` | Modified | Guarded-`q`, spinner ticks, notice copy sweep. |
| `internal/app/app.go` | Modified | `menuEntry` description field + glossary. |
| `go.mod` / `go.sum` | Modified | lipgloss → direct dependency. |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| ANSI escapes break the 115 assertions | Med | D1 Ascii `TestMain`; new colored tests assert plain string. |
| `q` collides with free-text buffers | Med | D3 guarded-`q` per screen, not global. |
| Copy sweep leaks beyond view.go | High | Include keys.go/update.go; per-string verify verbatim-asserted notices. |
| Combined size ~630–960 lines vs. 800 budget | High | **Open item for tasks**: forecast size:exception vs. slice decision. |

## Rollback Plan

Pure presentation change touching no shared Git/Salesforce state. Rollback = revert
the single PR; `go mod tidy` demotes lipgloss back to indirect. No data/migration impact.

## Dependencies

- lipgloss v1.1.0 + colorprofile v0.2.3 (already indirect; promote to direct).

## Success Criteria

- [ ] All 8 fixes implemented; `go test ./... -race` green with zero rewrites of the 115 existing assertions.
- [ ] Color renders in a real TTY and is absent under `NO_COLOR`/non-TTY/`go test`.
- [ ] Context bar shows repo·branch on every screen, ·org appended once resolved.
- [ ] `q` works (quit or go-back) on every screen, including the 3 free-text buffers.
- [ ] Spanish user-facing strings carry correct tildes + `¿¡`.
- [ ] Spinner animates during the 5 targeted long operations.
