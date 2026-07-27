# Exploration: `foundation-mvp-git` — Go bootstrap + MVP Git (HU-001..HU-006)

## Current State

Pre-code repository: only planning docs exist (`EPICA.md`, `docs/HISTORIAS.md`,
`docs/ARQUITECTURA.md`, `docs/MOCKUPS_TUI.md`). No `go.mod`, no `cmd/`/`internal/`
tree, no `.gitignore` at repo root, no `origin` remote. `openspec/config.yaml:7`
explicitly states "pre-code — only planning docs exist; no go.mod yet (created on
first change)". This change IS that first change.

## 1. Scope: IN vs OUT

**IN** (Fase 1 "MVP Git" — `EPICA.md:776-785`, recommended order `docs/HISTORIAS.md:1178-1183`):

- Go project bootstrap (`go.mod`, module skeleton, `.gitignore`)
- HU-001 Validar Prerequisitos Locales (`docs/HISTORIAS.md:11-93`)
- HU-002 Buscar Commits Por Ticket (`docs/HISTORIAS.md:94-164`)
- HU-003 Seleccionar Commits Manualmente (`docs/HISTORIAS.md:166-225`)
- HU-004 Seleccionar Destino Y Sandbox (`docs/HISTORIAS.md:227-292`)
- HU-005 Crear Rama Temporal De Promocion (`docs/HISTORIAS.md:294-350`)
- HU-006 Ejecutar Cherry-Pick Controlado + post-pick verification (`docs/HISTORIAS.md:352-438`)

**OUT** (`EPICA.md:786-825`, `docs/HISTORIAS.md:1184-1196`):

- HU-007 Generar Delta Package (Fase 2) — **the sgd multi-source-dir spike does NOT belong here**;
  it is a task inside HU-007 (`docs/HISTORIAS.md:458`, `EPICA.md:842`). Zero `sfdx-git-delta`
  touchpoints in this slice.
- HU-008 Resumir Package (Fase 2)
- HU-009/HU-012 Cola de Deploys (Fase 4)
- HU-010/HU-011 Validacion Salesforce (Fase 3)
- HU-013/014/016/017/019 Productizacion; HU-015/018 Futuro

## 2. Per-HU acceptance criteria + E2E harness (all six run in CI)

Legend (`docs/HISTORIAS.md:1246-1257`): `temp` = repo git temporal; `sf-fake` = `exec`
falso con JSON enlatado; `fs` = fixtures filesystem; `alias`/`sgd`/`gh-fake` NOT used in this slice.

| HU | AC source | Harness | CI |
|---|---|---|---|
| HU-001 | `docs/HISTORIAS.md:45-54` (8 GWT) | `temp + sf-fake` | si |
| HU-002 | `docs/HISTORIAS.md:123-131` (7 GWT) | `temp` | si |
| HU-003 | `docs/HISTORIAS.md:192-200` (7 GWT) | `temp` | si |
| HU-004 | `docs/HISTORIAS.md:252-258` (5 GWT) | `fs + temp + sf-fake` | si |
| HU-005 | `docs/HISTORIAS.md:318-323` (4 GWT) | `temp` | si |
| HU-006 | `docs/HISTORIAS.md:386-400` (12 GWT — largest) | `temp` | si |

Seed details that drive design:

- HU-002 seeds invert author-date vs topological order (`:161`) — dates can't be trusted for ordering (`:151`).
- HU-005 seeds a remote that advances after clone → only `git fetch` yields the right base (`:347-348`);
  a two-repo (local + local bare "remote") integration setup.
- HU-006 reuses `newTempRepo` as "el ejemplo de referencia" for the whole Git family (`:434`) —
  HU-002/003/005/006 share one harness helper.

Only external-command surface in this slice: `git` itself + three faked read-only `sf` calls
(`sf --version`, `sf plugins --json`, `sf org list --json`) for HU-001 prereqs and HU-004 alias check.
No `sf deploy`/`sgd` anywhere.

## 3. Scaffolding this change must create

From `docs/ARQUITECTURA.md:26-43` and `EPICA.md:534-588`:

- `go.mod` — module path decided as placeholder `deploydeck` (renameable later via `go mod edit -module`).
- `cmd/deploydeck/main.go` — entry point; Cobra wired early (HU-001 needs a `deploydeck doctor`
  subcommand with distinct non-zero exit on blockers, `docs/HISTORIAS.md:43`, `:90`).
- `internal/exec` — `CommandRequest`/`CommandResult` (`docs/ARQUITECTURA.md:255-275`), sole external-command
  boundary, non-interactive Git env `GIT_EDITOR=true`, `GIT_TERMINAL_PROMPT=0`, `GIT_PAGER=cat` (`:284`).
- `internal/config` — YAML load/defaults/validate (`docs/ARQUITECTURA.md:108-113`, schema `:287-339`).
- `internal/git` — status/branches/commits/cherry_pick/push (`EPICA.md:547-551`).
- `internal/app` — TUI skeleton; "No ejecuta comandos externos directamente" (`docs/ARQUITECTURA.md:56-59`).
- `.gitignore` including `.deploydeck/` — HU-001 doctor blocks if not ignored (`docs/HISTORIAS.md:37`, `:52`).
- Minimal `internal/salesforce` shim — only the 3 read-only calls above; full service deferred to Fase 3.
- `internal/prereq` — HU-001 (`docs/HISTORIAS.md:27`) allows a dedicated package or embedding in `internal/app`;
  not in `ARQUITECTURA.md`'s diagram — placement is an open decision.

## 4. Intra-slice build order (matches `docs/HISTORIAS.md:1178-1183`)

1. `go.mod` + `cmd/deploydeck` skeleton + `.gitignore` (with `.deploydeck/`)
2. `internal/exec` (no deps) — foundation for all command execution
3. `internal/config` (no deps) — needed before HU-004, useful for HU-001 (`minVersions`)
4. `internal/git` core (repo validation, branch listing, working-tree status)
5. **HU-001** — composes exec + minimal salesforce shim + git + config; `.deploydeck/lock`, `.gitignore` check, `doctor` CLI
6. **HU-002** — commit search/parsing, patch-id/`git cherry` equivalence, topo-order (`git rev-list --reverse --topo-order`)
7. **HU-003** — selection model on HU-002 output + per-file dependency warning
8. **HU-004** — destination/sandbox: config + `git rev-parse origin/<target>` + minimal salesforce alias check
9. **HU-005** — temp branch: `git fetch` + `checkout -b`, depends on config `branchFormat` + HU-004 target
10. **HU-006** — cherry-pick engine (sequential pick, conflict detect/classify, live re-poll,
    `--continue`/`--skip`/`--abort`, rerere hint, post-pick verification). Largest, most stateful (talla `L`).

## 5. Open decisions touching this slice

- **DEC-001** (`docs/ARQUITECTURA.md:392-398`) — merge/squash policy is a **team process recommendation**,
  not a DeployDeck behavior. It matters because HU-006 content-equivalence (`git cherry`/patch-id) degrades
  under squash (`docs/HISTORIAS.md:152`). Regardless of DEC-001, HU-006 MUST implement the squash safety-net
  (empty-cherry-pick → `--skip`, `docs/HISTORIAS.md:380`, `:398`) — already in scope, does NOT block on DEC-001.
- **sgd multi-source-dir spike** — confirmed OUT (Fase 2).
- **`internal/prereq` boundary** — dedicated package vs embedded in `internal/app` (recommend dedicated).
- **`internal/salesforce` shim scope** — read-only prereq/alias checks only; state explicitly to avoid Fase 3 rework.
- **Go module path** — resolved: placeholder `deploydeck`.
- **`.deploydeck/lock` liveness detection** — PID/liveness mechanism unspecified in docs; design-time decision.
- Minor doc gap: HU-006's mockup pointer omits "Verificacion Post Cherry-Pick" (`docs/MOCKUPS_TUI.md:223-244`)
  even though post-pick verification is an explicit AC (`:399`) — use that mockup for the `PickVerification` state.

State-machine subset (`docs/ARQUITECTURA.md:161-198`):
`PrereqCheck → TicketInput → CommitDiscovery → CommitSelection → TargetSelection → PlanPreview →
BranchCreation → CherryPicking ⇄ CherryPickConflict/Suspended/Aborted → PickVerification`, with
`PickVerification → CommitSelection` on partial-promotion. Scope edge = where `PickVerification →
DeltaGeneration` would start (Fase 2/HU-007, OUT).

## 6. Testability / strict-TDD breakdown

**Pure-unit testable** (table-driven): ticket/commit-message parsing; merge-commit detection (parent count>1);
equivalence classification given precomputed `git cherry`/patch-id output; config YAML parse/validate/defaults
(`t.TempDir()`); branch-name templating; `PrereqCheck` assembly given faked `exec` JSON; `CommitSelectionItem`
disabled/reason logic; per-file dependency-warning logic.

**Needs temp-git-repo integration harness** (skippable via `testing.Short()`): HU-001 repo-membership/origin/
dirty-tree/`.gitignore` checks; HU-002 `git log --grep`/`rev-list --topo-order`/`cherry`/`patch-id`; HU-003 real
`git diff`; HU-004 `git rev-parse origin/<target>` + config + sf-fake; HU-005 two-repo (local + bare remote) fetch
setup; HU-006 real cherry-pick, conflict classification, `CHERRY_PICK_HEAD`/`.git/sequencer` reconciliation,
`--continue`/`--skip`/`--abort`, rerere, post-pick verification.

**Highest strict-TDD risk**: HU-006 (talla `L`, 12 AC, largest seed) — plan incremental RED→GREEN per acceptance
criterion, share the `newTempRepo` helper across HU-002/003/005/006 (`:434`).

## Approaches (resolved)

1. Minimal `internal/salesforce` shim now (3 read-only calls), full service deferred to Fase 3. **CHOSEN.**
2. Inline the 3 faked calls without a package — rejected (contradicts module diagram, Fase 3 rewrite).
3. `internal/prereq` as separate package vs embedded — **recommend separate package** (cleaner exec boundary,
   reusable between TUI and `deploydeck doctor` CLI).

## Recommendation

Minimal `internal/salesforce` shim + separate `internal/prereq` package; module path placeholder `deploydeck`;
follow the §4 build order (matches the docs); DEC-001 and the sgd spike are context/out-of-scope so the PR budget
guard has a clean boundary at HU-006 → `PickVerification`.

## Ready for Proposal

Yes. The proposal must state: (1) module path = `deploydeck` (placeholder), (2) `internal/salesforce` shim scope
(read-only only), (3) `internal/prereq` as separate package, (4) HU-006 → Fase 2 boundary as the scope edge.
