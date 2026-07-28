# Proposal: HU-019 — DeployDeck Release Pipeline

## Intent

Give DeployDeck a **repeatable release pipeline** (CI + goreleaser) and **in-binary version awareness**, replacing manual builds. `goreleaser` is not installed locally and there is no `origin` remote, so this slice ships the **CI-testable core** and documents real-publish infra as an activation checklist — not code.

## Scope

### In Scope (ships now)
- **`internal/version`** package (`Version`/`Commit`/`Date`, default `"dev"`), wired to Cobra `root.Version` so `deploydeck --version` works; ldflags `-X deploydeck/internal/version.Version=...`.
- **`internal/update`** — non-blocking startup check: `Checker{BaseURL, *http.Client}`, `Latest(ctx)`, hand-rolled zero-dep `HasNewer(current, latest)`. Fired as a 2nd `tea.Cmd` via `tea.Batch(runPrereqCmd, checkUpdateCmd)` in `Init()`; `updateCheckTimeout`≈3s; timeout/network/non-2xx/401 all swallow → notice unset; surfaced by `View()` prepending `m.updateBanner()`. `app.Deps.CheckUpdate func(ctx)(bool,string,error)` — **scalar signature** (no version/update types leak into `internal/app`).
- **`.goreleaser.yaml`** (macOS amd64+arm64/Linux/Windows, archives, checksums, ldflags, `brews:`/`scoops:` with PLACEHOLDER org/repo), **`.github/workflows/ci.yml`** (PR: build/vet/test + `goreleaser check` + `goreleaser release --snapshot --clean` asserting `dist/`), **`.github/workflows/release.yml`** (semver tag → goreleaser action), root **`README.md`** install/update docs.

### Out of Scope (deferred — activation-checklist doc, NOT code)
- Create `homebrew-tap`/`scoop-bucket` repos; provision + wire the write token; real tag→publish; `brew`/`scoop install` UX (AC3); AC2's "formula/manifest actually updated" clause; public/private repo decision (+ its 401/token UX fork); rename bare `deploydeck` module path (blocked on no `origin`); winget/chocolatey (story "Descartados").

## Capabilities

> Contract for sdd-spec. Confirmed against `openspec/specs/`: no existing living spec covers these.

### New Capabilities
- `release-versioning`: version embedding + `deploydeck --version`.
- `update-notification`: non-blocking startup update-check + banner.
- `release-pipeline`: CI + goreleaser config + `--snapshot` validation + activation checklist.

### Modified Capabilities
- None.

## Approach

Version vars live in a new leaf `internal/version` (respects Main→App direction). Update-check is a Bubble Tea goroutine Cmd — inherently non-blocking; failures are swallowed identically (down/slow/unauthorized all skip). Semver is a hand-rolled int-compare (upgrade to `golang.org/x/mod/semver` only if prerelease logic grows). goreleaser/workflow YAML are **CI-validated only** (no local loop — accepted exception, CI is the test per the story's Test E2E).

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `internal/version/` | New | version vars |
| `internal/update/` | New | `Checker`, `Latest`, `HasNewer` |
| `cmd/deploydeck/main.go` | Modified | `root.Version`, `defaultCheckUpdate`, `Deps` wiring |
| `internal/app/{app,commands,update,view}.go` | Modified | `CheckUpdate` field, `updateCheckDoneMsg`/`checkUpdateCmd`, case, banner |
| `.goreleaser.yaml`, `.github/workflows/{ci,release}.yml`, `README.md`, activation-checklist doc | New | CI/release infra + docs |
| `openspec/config.yaml` | Modified | `testing.ci.scope` addendum |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| goreleaser has no local validation loop | High | Accepted exception — CI is first/only signal; flag so reviewers expect no local yaml test |
| No `origin` blocks real publish + `go install` | High | Same repo-URL decision; slice exercises only the CI-testable core |
| Public/private decision gates AC3 | Med | Flag prominently in activation checklist — do NOT silently default |
| Import cycle if `CheckUpdate` leaks types | Med | Keep signature scalar; comparison stays in `main.go` |
| No local `actionlint` for workflow YAML | Med | Same CI-is-the-test caveat |

## Rollback Plan

New packages/files (`internal/version`, `internal/update`, `.goreleaser.yaml`, workflows, README, checklist) are deletable in isolation. `main.go`/`internal/app` edits revert cleanly: `CheckUpdate` nil disables the check (graceful-degrade convention), unset `root.Version` restores inert `--version`. No persisted state, no migration.

## Dependencies

- CI runner with `goreleaser` action (no local install required).
- No new Go deps (semver hand-rolled; `net/http`/`httptest` are stdlib).

## Success Criteria

**Verified now (strict-TDD Go units + CI):**
- [ ] `--version` shows `version.Version` (default `"dev"`) — `root_test.go` pattern.
- [ ] Newer canned version → non-blocking banner set; down/slow endpoint → startup NOT gated (`httptest.Server` + `updateCheckDoneMsg` into `Update()`).
- [ ] `HasNewer` table test passes.
- [ ] `ci.yml` runs build/vet/test on PR; `goreleaser check` + `--snapshot` produce `dist/` artifacts.

**Infra-gated / deferred (explicitly NOT claimed done):**
- [ ] AC2 "formula/manifest actually updated" + AC3 (`brew`/`scoop install`) — blocked on aux repos + write token; documented in activation checklist only.
