# Design: HU-019 — Release Pipeline, Version Awareness, Update Notice

## Technical Approach

Three independent seams, anchored to the real composition root (`cmd/deploydeck/main.go`) and TUI (`internal/app`), plus CI/release infra validated only in CI:

1. **`internal/version`** — leaf package of build-time vars (`Version`/`Commit`/`Date`, default `"dev"`), ldflags-injected, wired to Cobra `root.Version`.
2. **`internal/update`** — `Checker.Latest(ctx)` (HTTP GET GitHub Releases) + hand-rolled zero-dep `HasNewer`.
3. **`internal/app` wiring** — a **scalar** `Deps.CheckUpdate` fired as a 2nd `tea.Cmd` from `Init`; banner surfaced by one `View` prepend. Composition (`Checker` build + `HasNewer` compare + `version.Version` read) lives in `main.go`'s new `defaultCheckUpdate`, mirroring `defaultChecker`/`defaultRunTUI`.

Respects `docs/ARQUITECTURA.md` Main→App→leaf direction: `internal/app` imports neither `internal/version` nor `internal/update` nor `net/http`; only main composes them. HTTP (not a subprocess) is confined to `internal/update`, so the `internal/exec`-only rule is untouched.

## Architecture Decisions

### ADR-1 · Version vars: package vs `package main`
| | |
|---|---|
| **Choice** | New leaf `internal/version` (`Version`/`Commit`/`Date`), imported by main. |
| **Rejected** | Vars in `package main` (`-X main.Version`). |
| **Rationale** | main-only vars aren't importable by future callers without breaking the Main→App direction; a leaf keeps ldflags path stable (`-X deploydeck/internal/version.Version=`) and lets `String()` be unit-tested independently. |

### ADR-2 · `CheckUpdate` scalar signature (import-cycle discipline)
| | |
|---|---|
| **Choice** | `Deps.CheckUpdate func(ctx) (hasUpdate bool, latest string, err error)` — stdlib types only; nil disables (existing degrade convention). |
| **Rejected** | Passing `update.Checker`/`version.Version` types into `internal/app`. |
| **Rationale** | A typed signature forces `internal/app` to import sibling leaves that main composes — an inverted dependency and latent cycle. Scalar keeps the version→app→update comparison in `main.defaultCheckUpdate`; app stays I/O-free and HTTP-free. |

### ADR-3 · Silent skip on any check failure
| | |
|---|---|
| **Choice** | `onUpdateCheckDone`: `err != nil` OR `!hasUpdate` → do nothing; banner only on `hasUpdate`. |
| **Rejected** | Distinct branches / error banner for timeout vs 401 vs 5xx. |
| **Rationale** | Story treats down/slow/unauthorized identically (non-blocking courtesy). One branch = no nagging, no failure UI, testable as set-vs-unset. 3s `context.WithTimeout` bounds it; Bubble Tea's goroutine Cmd makes it inherently non-blocking. |

### ADR-4 · Malformed / `"dev"` current → no-nag
| | |
|---|---|
| **Choice** | `HasNewer` strips `v`, splits `MAJOR.MINOR.PATCH`, int-compares; any parse failure (incl. `"dev"`) → `false`. |
| **Rejected** | Panic/error on malformed; `golang.org/x/mod/semver`. |
| **Rationale** | Default dev builds (`Version=="dev"`) must never nag; malformed remote tags must never panic a TUI. Zero-dep keeps `go.mod` clean (`x/mod` is the flagged upgrade path only if prerelease logic grows). |

### ADR-5 · Ships-now vs infra-gated split
| | |
|---|---|
| **Choice** | Ship: version/`--version`, update-check+banner, `.goreleaser.yaml`, `ci.yml`/`release.yml`, README — all CI-validated (`goreleaser check` + `--snapshot`). Defer as documented checklist: aux repos, write token, module-path rename, public/private decision, real publish (AC2 "actually updated", all AC3). |
| **Rejected** | Silently marking AC2/AC3 done; blocking the slice on `origin`. |
| **Rationale** | `goreleaser` isn't installed locally and there's no `origin` — real publish and `go install` share one unmade infra decision. CI is the story's own Test E2E; placeholders are clearly commented, not fake-passing. |

## Data Flow

    ldflags -X deploydeck/internal/version.Version={{.Version}}
        └─▶ version.Version ─▶ root.Version = version.String() ─▶ `deploydeck --version`

    Init() = tea.Batch( runPrereqCmd(), checkUpdateCmd() )      [each its own goroutine → non-blocking]
                                            │
       main.defaultCheckUpdate(ctx):        ▼  (ctx = WithTimeout 3s)
         update.Checker.Latest(ctx) ──HTTP GET──▶ GitHub Releases API  ({"tag_name"})
         update.HasNewer(version.Version, latest)
                     │ (bool, latest, err)  ← SCALAR, no leaf types
                     ▼
       updateCheckDoneMsg ─▶ Update → onUpdateCheckDone
            err|!hasUpdate → skip · hasUpdate → m.updateAvailable/m.updateLatest
                     ▼
       View() = m.updateBanner() + m.viewBody()   (banner "" unless updateAvailable)

## File Changes

| File | Action | Description |
|------|--------|-------------|
| `internal/version/version.go` | Create | `Version="dev"`,`Commit`,`Date` vars + `String()` |
| `internal/update/update.go` | Create | `Checker{BaseURL,*http.Client}`, `Latest`, `HasNewer`, PLACEHOLDER owner/repo const |
| `cmd/deploydeck/main.go` | Modify | `root.Version=version.String()`; `defaultCheckUpdate`; wire `app.Deps.CheckUpdate` in `defaultRunTUI`; +`context`/`net/http`/`internal/update`/`internal/version` imports |
| `cmd/deploydeck/root_test.go` | Modify | `--version` default + injected tests |
| `internal/app/app.go` | Modify | `Deps.CheckUpdate` field; `updateAvailable`/`updateLatest` Model fields; `Init`→`tea.Batch(...)` |
| `internal/app/commands.go` | Modify | `updateCheckTimeout≈3s` const; `updateCheckDoneMsg`; `checkUpdateCmd` |
| `internal/app/update.go` | Modify | `case updateCheckDoneMsg` → `onUpdateCheckDone` |
| `internal/app/view.go` | Modify | rename switch → `viewBody()`; add `View()=updateBanner()+viewBody()`; `updateBanner()` |
| `internal/app/*_test.go` | Create | checkUpdateCmd/onUpdateCheckDone/updateBanner/Init tests |
| `.goreleaser.yaml` | Create | builds (darwin/linux amd64+arm64, windows amd64), archives (tar.gz + zip win), checksums, ldflags, PLACEHOLDER `brews`/`scoops` |
| `.github/workflows/ci.yml` | Create | build + `go vet` + `go test -race`; goreleaser `check`+`--snapshot` job asserting `dist/` |
| `.github/workflows/release.yml` | Create | goreleaser release on `v*` tag (write-token secret PLACEHOLDER) |
| `README.md`, `docs/ACTIVATION-CHECKLIST.md` | Create | install/update docs + infra activation steps |
| `openspec/config.yaml` | Modify | `testing.ci.scope` addendum |

## Interfaces / Contracts

```go
// internal/version
var (Version = "dev"; Commit = ""; Date = "")
func String() string // "dev" or "1.2.3 (commit abc, built <Date>)"

// internal/update  (owner/repo = PLACEHOLDER const, activation-gated step a)
type Checker struct{ BaseURL string; HTTPClient *http.Client }
func (c Checker) Latest(ctx context.Context) (string, error) // GET BaseURL+/repos/OWNER/REPO/releases/latest; non-2xx (incl 401)→err
func HasNewer(current, latest string) bool                   // strip v, split M.m.p, int-cmp; malformed/"dev"→false

// internal/app
type Deps struct{ /* … */ CheckUpdate func(ctx context.Context) (bool, string, error) } // nil disables
func (m Model) Init() tea.Cmd { return tea.Batch(m.runPrereqCmd(), m.checkUpdateCmd()) } // Batch drops nils
func (m Model) checkUpdateCmd() tea.Cmd // nil if Deps.CheckUpdate==nil; else WithTimeout(bg,updateCheckTimeout)→updateCheckDoneMsg

// cmd/deploydeck/main.go
func defaultCheckUpdate(ctx context.Context) (bool, string, error) {
    c := update.Checker{BaseURL: "https://api.github.com", HTTPClient: &http.Client{}}
    latest, err := c.Latest(ctx); if err != nil { return false, "", err }
    return update.HasNewer(version.Version, latest), latest, nil
}
// newRootCmd: root.Version = version.String()   // default Cobra template → "deploydeck version <…>"
// defaultRunTUI: app.Deps{ …, CheckUpdate: defaultCheckUpdate }  // wired here, NOT on cmd Deps
```

**ldflags** (`.goreleaser.yaml`): `-s -w -X deploydeck/internal/version.Version={{.Version}} -X deploydeck/internal/version.Commit={{.Commit}} -X deploydeck/internal/version.Date={{.Date}}`. No flags (e.g. `go run`) → `Version=="dev"` → inert-safe `--version` and no-nag.

**Wiring note (cmd Deps vs app.Deps):** `defaultRunTUI` already builds `app.Deps` and references `defaultChecker` directly — `CheckUpdate: defaultCheckUpdate` goes in the same literal. The cmd-level `Deps{NewChecker,RunTUI}` is **not** extended: the update-check is TUI-startup behavior (not a Cobra path like `doctor`), so it needs no cmd-layer injection seam; `root.Version` is set inside `newRootCmd`.

## Testing Strategy

| Layer | What | Approach |
|-------|------|----------|
| Unit | `HasNewer` | Table: newer/older/equal, missing patch, leading `v`, `"dev"`, non-numeric → all deterministic |
| Unit | `Latest` | `httptest.Server` w/ injected `BaseURL`: canned `{"tag_name":"v1.2.3"}`→tag; 401→err; slow handler + short ctx → err, no hang |
| Unit | `version.String()` | Table on injected vars (optional) |
| CLI | `--version` | `root_test.go` pattern: `SetArgs(["--version"])`+`Execute`, capture `OutOrStdout`; default→`"dev"`, set+restore `version.Version="9.9.9"` (no `t.Parallel`)→`"9.9.9"` |
| TUI unit | `checkUpdateCmd` | nil `Deps.CheckUpdate`→nil Cmd; fake→yields `updateCheckDoneMsg` |
| TUI unit | `onUpdateCheckDone` | `{hasUpdate:true}`→fields set, banner renders; `{err}`/`{hasUpdate:false}`→unset, banner `""` |
| TUI unit | `Init` non-block | `Init()` non-nil; drive slow/failing update msg into `Update`, assert prereq flow unaffected (direct `Model.Update`, per go-testing skill) |
| **CI-only (NO local test)** | `.goreleaser.yaml`, `ci.yml`, `release.yml`, README, checklist | `goreleaser check` + `release --snapshot --clean` in CI produce `dist/`; no local goreleaser/actionlint |

No Go test touches the real network, a real org, or goreleaser.

## Threat Matrix

N/A — no routing, shell, subprocess, VCS/PR automation, executable-file classification, or process-integration boundary is added in app code (CI/goreleaser subprocesses are pipeline config, not the binary's integration surface). One new outbound boundary — `net/http` GET in `internal/update`: URL from a compile-time const (no untrusted input → no SSRF/injection), bounded by a 3s context, all errors swallowed, no credentials sent (public-repo path; private-repo 401 also swallowed, its token UX deferred to the activation checklist).

## Migration / Rollout

No data migration, no persisted state. All new files delete in isolation; `main.go`/`internal/app` edits revert cleanly (`CheckUpdate` nil disables the check, unset `root.Version` restores inert `--version`). Real publish is activation-gated behind `docs/ACTIVATION-CHECKLIST.md` (module-path rename, 2 aux repos, write token, public/private decision, first real tag).

## Open Questions

- [ ] Real module path / `owner/repo` — blocked on no `origin`; placeholder now, checklist step a (does NOT block `goreleaser check`/`--snapshot`).
- [ ] Public vs private repo — gates AC3 install UX + the 401/token fork; flagged in the checklist, not silently defaulted.
