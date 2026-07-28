# Exploration: HU-019 — Pipeline de Release de DeployDeck (`release`)

Eighth slice, stacked on `cleanup`. DIFFERENT in kind from prior slices: it's CI/release infra (GitHub Actions + goreleaser) + a small Go core (version embedding, `--version`, non-blocking update-check). User scoped this to the **CI-testable core**, deferring real-publish infra as documented decisions.

## Hard environment constraints (verified — scope is built around these)
- **`goreleaser` NOT installed locally** → `.goreleaser.yaml` validated ONLY in CI (`goreleaser check` / `--snapshot`), no local test. Accepted exception (CI IS the test, per the story's own Test E2E).
- **Module path is bare `module deploydeck`** (`go.mod:1`) and **NO `origin` remote** (`.git/config` has no remote block). So BOTH `go install <module>@latest` AND goreleaser's real publish (tap/bucket/Releases) are blocked on the SAME unmade infra decision (real repo URL + 2 aux repos + write token). Real publish CANNOT be exercised in this slice.
- Greenfield: no `.github/workflows/`, no `.goreleaser.yaml`, no root `README.md`.

## Current state (code-anchored)
- **`cmd/deploydeck/main.go`**: Cobra composition root. `Deps{NewChecker, RunTUI}` function-field DI (`:32-41`); `newRootCmd(deps)` (`:46-67`) — NO `root.Version` set today (Cobra's free `--version` is inert). `defaultChecker`/`defaultRunTUI` (`:186-239`) are the composition functions where real services get wired — the exact place a `defaultCheckUpdate` belongs. `root_test.go` tests via fake `Deps` + `cmd.SetArgs` + `Execute()`.
- **`internal/app`**: `app.Deps` (`app.go:207-244`) documents the "nil disables feature → degrades gracefully" convention repeatedly. `Model.Init()` (`:452-454`) returns a single `tea.Cmd` (`runPrereqCmd`). Bubble Tea runs each Cmd as an independent goroutine feeding a `tea.Msg` back — this IS the non-blocking mechanism; `tea.Batch(runPrereqCmd, checkUpdateCmd)` fires both without blocking. `commands.go:20-41` sets the msg-struct + named-timeout convention (`reportCallTimeout` etc.). `view.go` `View()` (`:20`) is a flat `switch m.state`; `header`/`footer` are FREE functions (no state threaded) — a global banner is a single prepend at the top of `View()`.
- **No HTTP anywhere**: `internal/exec.Runner` is the process-exec boundary (NOT http). `net/http`/`httptest`: zero hits in the repo. The update-check's `net/http.Client` is a new, separate concern.
- **Deps**: `go.mod` has only cobra + bubbletea family; ZERO semver dep. Real-org e2e self-skip without `DEPLOYDECK_E2E_ORG`, so a plain `ci.yml` `go test ./...` excludes them automatically.

## Resolved decisions
1. **Version vars → NEW `internal/version` package** (`Version`/`Commit`/`Date`, default `"dev"`), not `package main` vars — importable without violating the Main→App→… dependency direction. ldflags path `-X deploydeck/internal/version.Version=...`.
2. **Update-check placement**: fire as a 2nd `tea.Cmd` from `Init()` — `tea.Batch(m.runPrereqCmd(), m.checkUpdateCmd())`. `checkUpdateCmd` wraps a `context.WithTimeout(bg, updateCheckTimeout≈3s)` call; timeout/network-error/non-2xx (incl. private-repo 401) ALL swallow identically → notice stays unset (no distinct branch — story treats down/slow/unauthorized the same: silently skip). Surface via `View()` prepending `m.updateBanner()` (empty when none) — one change, shows on whatever screen is active. **`app.Deps.CheckUpdate func(ctx) (hasUpdate bool, latest string, err error)` — SCALAR signature** so no `internal/version`/`internal/update` types leak into `internal/app` (import-cycle discipline; comparison lives in `main.go`'s `defaultCheckUpdate`, mirroring `defaultChecker`).
3. **Semver → hand-rolled `internal/update.HasNewer(current, latest)`** (strip `v`, split MAJOR.MINOR.PATCH, int-compare) — zero dep, table-testable. `golang.org/x/mod/semver` flagged as the upgrade path only if prerelease/build-metadata logic grows.
4. **goreleaser scope — SHIP NOW**: `.goreleaser.yaml` (builds macOS amd64+arm64/Linux/Windows, archives, checksums, ldflags injecting `internal/version` vars, `brews:`/`scoops:` stanzas with clearly-marked PLACEHOLDER org/repo) + `release.yml` (goreleaser action on semver tag) + `ci.yml` step running `goreleaser check` + `goreleaser release --snapshot --clean` (dry-run → `dist/` binaries + formula + manifest, no publish) — verbatim the story's Test E2E. **DEFER (infra-gated, documented)**: real `brews:`/`scoops:` push (needs the 2 aux repos + write token), therefore AC3 (`brew`/`scoop install`) entirely + AC2's "formula/manifest ACTUALLY updated" clause.
5. **Bare module path → leave + document** as step 1 of the activation checklist (no real repo URL exists to set; only matters for `go install`/imports, NOT for `goreleaser check`/`--snapshot`).
6. **Scope OUT**: creating `homebrew-tap`/`scoop-bucket` repos; the write token; real tag→publish; `brew`/`scoop install` UX; the public/private repo decision (+ its 401/token UX fork); winget/chocolatey (story's own "Descartados"). Ship an **activation-checklist doc** (not code): (a) pick real module path once `origin` exists, (b) create the 2 aux repos, (c) provision+wire the write token secret, (d) decide public vs private + update README install docs, (e) cut the first real tag.

## New/affected areas
- NEW `internal/version/` (vars), NEW `internal/update/` (`Checker{BaseURL,*http.Client}`, `Latest(ctx)`, `HasNewer`).
- `cmd/deploydeck/main.go` (set `root.Version`, `defaultCheckUpdate`, extend `Deps`/wiring) + `root_test.go` (`--version` test).
- `internal/app/{app.go (CheckUpdate field), commands.go (updateCheckDoneMsg/checkUpdateCmd/updateCheckTimeout), update.go (case), view.go (updateBanner)}`.
- NEW `.goreleaser.yaml`, `.github/workflows/{ci,release}.yml`, root `README.md`, activation-checklist doc. Minor `openspec/config.yaml` `testing.ci.scope` addendum.

## ACs → testability (`docs/HISTORIAS.md:1177-1195`)
- **Go-unit NOW** (strict TDD): `--version` shows `version.Version` (root_test.go pattern; default `"dev"`); newer version → non-blocking notice via `httptest.Server` canned `{"tag_name":...}` + injected `BaseURL`, and feed `updateCheckDoneMsg` into `Update()` to assert banner set/unset; down/slow endpoint → feed `{err}` / slow httptest → assert startup NOT gated. `HasNewer` table test.
- **CI-only, testable NOW**: `ci.yml` runs build/vet/test on PR; `goreleaser check` + `--snapshot` produce `dist/` artifacts (asserted in the CI job).
- **DEFERRED / infra-gated**: real formula/manifest push (AC2 partial), `brew`/`scoop install` (AC3) — no aux repos/token exist. Must NOT be silently marked done.

## Suggested capabilities (propose/spec to finalize names)
- `release-versioning` (version embedding + `--version`), `update-notification` (non-blocking startup check + banner), `release-pipeline` (CI + goreleaser config + `--snapshot` validation + activation checklist). No existing living spec covers any of these.

## Risks
- goreleaser has NO local validation loop (CI is the first signal) — accepted exception, flag in proposal/design so reviewers don't expect a local yaml test.
- No `origin` → real publish/`go install` blocked on the same repo-URL decision; slice exercises only the CI-testable core.
- Public/private repo UX fork (story's pending decision) gates AC3 — flag prominently in the activation checklist, don't silently default.
- No `actionlint`-equivalent local check for the workflow YAML — same CI-is-the-test caveat.
- Import-cycle discipline: keep `app.Deps.CheckUpdate` scalar-only; do NOT import `internal/version`/`internal/update` into `internal/app`.

## Ready for Proposal: yes
Split deliverables explicitly: "ships now" (version+`--version`, update-check+banner, `.goreleaser.yaml`+`goreleaser check`/`--snapshot` in CI, `ci.yml`/`release.yml`, README) vs "documented activation checklist" (aux repos, token, module-path rename, public/private decision, real publish, AC2 "actually updated", all of AC3) — so infra-gated ACs aren't treated as done.
