# Design: Update DeployDeck From Inside The TUI

> **Amendment (2026-11-05):** ADR-7 (build-info version fallback, `resolveCurrentVersion`) was CUT during apply to stay within the 800-line review budget. Only `decideUpdate(current, latest, err)` taking `current` explicitly was kept. The requirement was removed from the update-notification spec delta. Follow-up change recorded in state.yaml.

## Technical Approach

All process, network and filesystem work stays in `internal/update` (pure detection, the cache) and `cmd/deploydeck` (composition and the post-run step). `internal/app` gets two scalar Deps fields, one flag and two getters. `boundary_test.go` stays unchanged. Pressing `u` sets the flag and returns `quitCmd()`. When `program.Run()` returns, `defaultRunTUI` reads the final model and calls `runUpgrade`. This covers both spec deltas (`tui-self-update`, `update-notification`).

## Architecture Decisions

| # | Decision | Choice | Rejected | Rationale |
|---|---|---|---|---|
| ADR-1 | Where the upgrade runs | Post-run in `defaultRunTUI`, after `program.Run()` returns | `tea.ExecProcess` from app | Adding ExecProcess would make it a second sanctioned exception next to `editHandoff`, and it would swap the binary under a live TUI. Running after quit lets `quitCmd()` restore the branch first. |
| ADR-2 | App port | `Deps.CanUpgrade bool` + `Deps.ReleasesURL string`. Model `upgradeRequested`. Getters `UpgradeRequested() bool` and `UpdateLatest() string` | passing argv or a func into app | App only has to decide whether `u` is live and what to render. Argv needs `latest`, which main reads back through `UpdateLatest()`. Main never captures it from the check goroutine, so there is no data race. `CanUpgrade` is how the spec's "upgrade command wired" is met. |
| ADR-3 | Detection | Pure `DetectMethod(exePath, goos string, getenv func(string) string) Method`. Matches with `strings` and `path` on a slash-normalized path, lower-cased on windows. Never uses `filepath`. | `filepath`-based matching | Windows table cases have to pass on darwin CI, and `filepath` behaves differently per OS. |
| ADR-4 | Match order | Brew → Scoop → GoInstall → Manual. Brew is checked only off windows, Scoop only on windows. | GoInstall first | Most specific first. A `GOBIN` pointing into a Homebrew prefix must still resolve to Brew. A wrong guess only shows the manual command. |
| ADR-5 | Windows execution | When `goos == "windows"`, `runUpgrade` prints the command and does not run it. `u` still quits and restores the branch. | running anyway; a detached child plus exit | DeployDeck's process is still alive during post-run, so its exe stays locked. `go install` cannot overwrite it, and Scoop refuses to update apps that are still running. Running anyway would always fail. A detached child races Scoop's process check and garbles the console. The spec delta includes this Windows scenario. |
| ADR-6 | Go install target | `@<tag>` normalized to start with `v`. Must match `^v?\d+\.\d+\.\d+$`, otherwise `UpgradeArgs` returns nil, which leads to the manual fallback | `@latest` | The tag comes from the network or cache and ends up in argv. Strict validation keeps it data-only, and the tag pins the version that was checked. |
| ADR-7 | Version fallback | `resolveCurrentVersion(v, readBuildInfo)`, applied only to the comparison. `decideUpdate(current, latest, err)` takes `current` explicitly. | changing `version.String()` | `--version` stays the same. Passing the value explicitly removes the global mutation from `TestDecideUpdate`. This is the first cut lever. |
| ADR-8 | Upgrade failure | Print the method's command and the releases URL to stderr, then return nil, so the exit status is unchanged | `exitError{1}`; retries | The spec requires the exit status to stay the same. No hidden state is left behind. |

## Data Flow

    main ─ composeUpdate(getenv, exe, goos, lookPath, cacheDir, current)
             ├─ opt-out? → CheckUpdate=nil
             ├─ CheckUpdate = Cache{Dir,Now}.Latest(ctx, Checker.Latest) → decideUpdate
             └─ Method=DetectMethod(...); CanUpgrade = Method!=Manual && lookPath(Method.Tool())==nil
           app.New(deps) → Init: checkUpdateCmd → onUpdateCheckDone (unchanged)
           StateMainMenu, `u`: !updateAvailable||!CanUpgrade → no-op
                               repoState.InProgress → notice, no quit
                               else upgradeRequested=true; return quitCmd()
           final, _ := program.Run()
           if final.(app.Model).UpgradeRequested():
               runUpgrade(UpgradeArgs(Method, UpdateLatest()), goos, os.Stdout, os.Stderr, runInherited)

## Interfaces / Contracts

```go
// internal/update/method.go
type Method int // MethodManual (zero), MethodBrew, MethodScoop, MethodGoInstall
func DetectMethod(exePath, goos string, getenv func(string) string) Method
func (m Method) Tool() string                    // "brew" | "scoop" | "go" | ""
func UpgradeArgs(m Method, latest string) []string // nil for Manual or an invalid tag
const ReleasesURL = "https://github.com/malavolta/DeployDeck/releases/latest"

// internal/update/cache.go
const CacheTTL = 24 * time.Hour
type Cache struct { Dir string; Now func() time.Time } // Dir "" disables the cache
func (c Cache) Latest(ctx context.Context, fetch func(context.Context) (string, error)) (string, error)

// cmd/deploydeck/selfupdate.go
type runFunc func(argv []string) error // default: osexec.Command, Stdin/Stdout/Stderr = os.*
func runUpgrade(argv []string, goos string, stdout, stderr io.Writer, run runFunc)
```

**Detection rules.** These apply after normalizing the path with `\`→`/`.
- **Brew**: the path contains `/Caskroom/` or `/Cellar/`, or has the prefix `/opt/homebrew/` or `/home/linuxbrew/.linuxbrew/`.
- **Scoop**: the path contains `/scoop/apps/` or `/scoop/shims/`, or starts with `$SCOOP/apps/`.
- **GoInstall**: `path.Dir(exe)` equals `$GOBIN`. Otherwise it equals the first `$GOPATH` entry (split on `;` on windows, `:` elsewhere) plus `/bin`. Otherwise it equals `$HOME` (or `$USERPROFILE` on windows) plus `/go/bin`.

Main gets `exe` from `os.Executable` and `filepath.EvalSymlinks`. An error yields `""`, which is Manual.

**Cache.** The file is `<UserCacheDir>/deploydeck/update-check.json` with content `{"checked_at":"<RFC3339>","latest":"v1.3.0"}`.
- **Fresh** means `0 <= Now-checked_at < 24h`. A timestamp in the future counts as stale.
- **Degrading**: any read or parse error falls through to a live fetch.
- **Atomic write**: `MkdirAll` (0o755), then `CreateTemp` in the same directory, then `Rename`. The temp file is removed on error.
- **What is not cached**: fetch errors and an empty `latest`. Write errors are ignored.
- **What is cached**: the tag only, never `hasUpdate`, so after an upgrade the comparison runs against the new version.
- **Location failure**: if `os.UserCacheDir` fails, `Dir` is `""` and every check is live.

**Opt-out.** If `getenv("DEPLOYDECK_NO_UPDATE_CHECK") != ""`, then `CheckUpdate` is nil. No network request is made and the cache is never touched.

## Copy (Spanish)

| Where | Text |
|---|---|
| Banner, any state | `Nueva versión de DeployDeck disponible: %s` |
| Suffix, main menu, `CanUpgrade` | ` · pulsa u para actualizar` |
| Suffix, main menu, Manual | ` · descárgala en <ReleasesURL>` |
| Main-menu footer, update available and `CanUpgrade` | `Enter seleccionar   ↑/↓ navegar   u actualizar   q salir` |
| In-progress notice | `termina la operación en curso antes de actualizar DeployDeck` |
| Post-run, before | `Actualizando DeployDeck a %s: <argv joined>` |
| Post-run, success | `Listo. Ejecuta deploydeck --version para confirmar la versión instalada.` |
| Post-run, failure (stderr) | `No se pudo actualizar DeployDeck (%v). Actualiza manualmente con:` + method command + `o descarga la versión desde <ReleasesURL>` |
| Post-run, windows | `Para completar la actualización ejecuta: <argv joined>` |

The success message is deliberately neutral, because a stale brew tap can exit 0 with "already up to date".

## File Changes

| File | Action | Description |
|---|---|---|
| `internal/update/checker.go` | Modify | `ownerRepo = "malavolta/DeployDeck"`; remove the placeholder comment |
| `internal/update/method.go` | Create | `Method`, `DetectMethod`, `Tool`, `UpgradeArgs`, `ReleasesURL` |
| `internal/update/cache.go` | Create | `Cache`, `CacheTTL`, atomic write |
| `cmd/deploydeck/selfupdate.go` | Create | `composeUpdate`, `resolveCurrentVersion`, `resolveExePath`, `runUpgrade`, `runInherited` |
| `cmd/deploydeck/main.go` | Modify | `defaultRunTUI` wires `composeUpdate`, keeps the final model, calls `runUpgrade`. `decideUpdate` gains `current`. `defaultCheckUpdate` is replaced by a `newCheckUpdate(baseURL, cacheDir, current)` builder. |
| `internal/app/app.go` | Modify | Deps fields, `upgradeRequested`, getters |
| `internal/app/keys.go` | Modify | `case "u"` in `keyMainMenu` |
| `internal/app/view.go` | Modify | Spanish banner with state-aware suffix; conditional footer |
| `README.md`, `docs/ACTIVATION-CHECKLIST.md` | Modify | `## Update` mentions `u`, the opt-out and the Windows behavior; mark activation step (a) done |

## Testing Strategy (strict TDD, RED first)

| Seam | Test | Approach |
|---|---|---|
| `DetectMethod` | macOS/Linux brew, Caskroom, Scoop (including a custom `$SCOOP` and mixed case), GOBIN, GOPATH list, default `~/go/bin`, `/usr/local/bin`, empty path, a GOBIN inside a brew prefix (ordering) | table test with a fake `getenv` map |
| `UpgradeArgs` | exact argv per method; tag without `v`; invalid tags (`v1.2`, `v1.2.3 && x`, `-x`) return nil; Manual returns nil | table |
| `Cache.Latest` | fresh (fetch fails the test if called), stale (one fetch, file rewritten), future timestamp, corrupt JSON, fetch error leaves no file, unwritable dir (file placed where the dir should be), `Dir==""` | `t.TempDir()`, fake `Now`, counting fetch |
| `Checker` | request path is `/repos/malavolta/DeployDeck/releases/latest` | httptest |
| `composeUpdate` | opt-out yields nil `CheckUpdate`; an empty value stays wired; a missing tool or Manual yields `CanUpgrade=false` | fake getenv and lookPath |
| `resolveCurrentVersion` / `decideUpdate` | `v1.2.3`, `(devel)`, `""`, ldflags wins; existing table passes `current` explicitly | table |
| `newCheckUpdate` | pre-canceled ctx degrades. It must use a `t.TempDir()` cache, otherwise a real cache file would hide the error. | rewrite of the existing test |
| `runUpgrade` | success: argv passed once, nothing on stderr; error: stderr contains the command and the URL, one call; nil argv: manual text and no call; windows: printed, no call | fake `runFunc`, `bytes.Buffer` |
| App | `u` on the main menu sets the request and returns quit (run the cmd and assert `tea.QuitMsg`); blocked by InProgress (notice, nil cmd), then allowed; no update or no `CanUpgrade` is a no-op; `u` elsewhere does not request; banner fragments per state and method; no banner without an update | drive `Model.Update` with `updateCheckDoneMsg` and then `tea.KeyMsg` |

No test runs `brew`, `scoop`, `go install` or the real GitHub API.

## Threat Matrix

| Boundary | Applicability | Design response | RED test |
|---|---|---|---|
| Documentation-like paths | N/A. The change does no file classification and executes no repository files. | — | — |
| Git repository selection | N/A. No new git call is added; restore reuses `quitCmd()` unchanged. | — | — |
| Commit state | N/A | — | — |
| Push state | N/A | — | — |
| PR commands / argument composition | **Applicable.** The upgrade argv includes a tag that comes from the network or cache. | Argv is a slice and no shell is involved. `argv[0]` is a fixed literal resolved through PATH by `LookPath`, never taken from the exe path. ADR-6 validates the tag strictly, and an invalid tag yields nil, which leads to the manual fallback. | the invalid-tag `UpgradeArgs` cases; the nil-argv `runUpgrade` case |

## Migration / Rollout

No migration is needed. The cache file is disposable. `DEPLOYDECK_NO_UPDATE_CHECK` works as a kill switch.

## Budget

The forecast is about 790 lines: about 310 production (update 145, main 115, app 50), about 455 tests and about 25 docs. If the total runs over, cut ADR-7 first (about −35 lines).

## Risks / Open Questions

- [x] **ADR-5 spec deviation resolved**: the orchestrator amended the spec delta with the "Windows prints the command instead of running it" scenario (2026-11-05).
- [ ] The ADR-5 claim about Scoop's running-process refusal has not been tested on a real Windows host.
- [ ] The budget is tight, with about 10 lines of headroom. If tasks forecast more, cut ADR-7.
- [ ] Detection is heuristic. For example, a tarball binary copied into `~/go/bin` is detected as GoInstall. The cost is limited to the manual fallback.
