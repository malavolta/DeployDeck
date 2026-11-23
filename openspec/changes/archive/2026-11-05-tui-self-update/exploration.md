# Exploration: tui-self-update

Goal: when the TUI opens, check for a newer release and let the user update from inside the TUI.

## 0. Headline findings

- **Finding 0 (blocker, pre-existing): the update check is dead in production.** `internal/update/checker.go:19` is `const ownerRepo = "OWNER/REPO" // PLACEHOLDER`. Requests hit `/repos/OWNER/REPO/releases/latest`, get a 404, and the check silently skips, so the banner has never fired for a real release. Fix: `malavolta/DeployDeck`. Tests inject `BaseURL` through httptest and do not pin the placeholder. (Verified by the orchestrator.)
- **Finding 1: `go install` users never see a banner.** The `version.Version` ldflags exist only in goreleaser (`.goreleaser.yaml:45-50`). A `go install …@vX.Y.Z` binary reports `"dev"`, and `HasNewer` returns false for `"dev"` (`internal/update/hasnewer.go:19-22`). Fix: when `version.Version == "dev"`, fall back to `runtime/debug.ReadBuildInfo().Main.Version`; `(devel)` or empty still means dev.
- **Finding 2: quit-then-run beats `tea.ExecProcess`.** On Windows the running exe is locked, which breaks `scoop update` and `go install` while the TUI is alive. ExecProcess would also be a second sanctioned exception, after `editHandoff`. Instead, quit through the existing `quitCmd()` (which restores the original branch), then `main` runs the package-manager command with inherited stdio after `program.Run()` returns. `internal/app` gains no exec.

## 1. Current code paths

- `cmd/deploydeck/main.go`:
  - `githubAPIBaseURL` at :34-36.
  - `defaultRunTUI` at :255-294 builds `app.Deps`, with `CheckUpdate: defaultCheckUpdate` at :283. `program.Run()` discards the final model (:291-293).
  - `defaultCheckUpdate` at :301-305 and `decideUpdate` at :312-317.
  - `editHandoff` at :323-329 is the only `tea.ExecProcess` use.
- `internal/update`:
  - `Checker.Latest` at checker.go:45-73: GET releases/latest, 1 MiB body limit, reads `tag_name`.
  - `HasNewer` at hasnewer.go:19: MAJOR.MINOR.PATCH comparison; pre-release tags return false.
- `internal/app`:
  - `Deps.CheckUpdate` at app.go:337; `Deps.Now` at :292.
  - Model fields `updateAvailable` and `updateLatest` at app.go:598-604.
  - `Init()` batches `runPrereqCmd` and `checkUpdateCmd` (app.go:784-786).
  - `checkUpdateCmd` at commands.go:463-474, with a 3s timeout.
  - `onUpdateCheckDone` at update.go:159-166.
  - `View()=updateBanner()+viewBody()` at view.go:20-33. The banner is English, has no key hint, and no test covers it.
  - `keyMainMenu` at keys.go:519-589 has the D1 `repoState.InProgress` guard and handles `q`/`esc` via `quitCmd()`.
  - `viewMainMenu` at view.go:188-207 renders the notice and the footer.
- Boundary: `internal/app/boundary_test.go:52-57` forbids `os/exec`, `net/http`, `internal/version` and `internal/update` in `internal/app`.
- `internal/exec.Runner` captures output, so it is unsuitable for an interactive upgrade with streaming stdio.
- Distribution (`.goreleaser.yaml`): tar.gz archives (zip on Windows), `checksums.txt` (unsigned), a Homebrew cask with a quarantine-strip postflight, and a Scoop manifest.
- Docs to update: README `## Update` (:54-65 and :186), and `docs/ACTIVATION-CHECKLIST.md:79-80`.

## 2. Where each piece lives

| Piece | Location |
|---|---|
| Fix `ownerRepo` | `internal/update/checker.go:19` |
| Install-method detection (pure) | new `internal/update/method.go`: `DetectMethod(exePath, goos, env) Method` returning `Brew`, `Scoop`, `GoInstall` or `Manual`. Matches on a normalized path. |
| Upgrade argv | same file: `UpgradeArgs(method, latest) []string`, nil for Manual |
| 24h cache | new `internal/update/cache.go`: JSON `{checked_at, latest}` under `os.UserCacheDir()/deploydeck/update-check.json`. Atomic write, errors degrade to a live check, failures are not cached, and only the latest tag is cached. |
| Opt-out | main: `DEPLOYDECK_NO_UPDATE_CHECK` non-empty sets `Deps.CheckUpdate = nil` |
| Version fallback | main or `internal/version`: buildinfo fallback |
| Composition | main: cache plus checker; resolve the upgrade via `os.Executable` + `EvalSymlinks` + `LookPath` |
| App port | `internal/app`: Deps scalars for the upgrade (argv availability and hint), a model flag `upgradeRequested`, and an exported getter `UpgradeRequested()` |
| Key | `keyMainMenu` `case "u"`. Guard: an update is available, the upgrade is wired, and `!repoState.InProgress` (otherwise show a Spanish notice). Then set the flag and call `quitCmd()`. |
| Banner | Spanish copy. Show the `u` hint only on the main menu. Manual installs get the releases URL. |
| Post-run | `defaultRunTUI` keeps the final model; if an upgrade was requested, `runUpgrade(argv, stdout, stderr, run)` runs it. On failure, print the manual command. |

The `u` key works only on StateMainMenu, the only non-destructive top-level landing, so it never fires during a cherry-pick or validation.

## 3. Testing seams

- Detection: table tests over exe paths for macOS, Linux and Windows.
- Cache: `t.TempDir()` and a fake clock, covering fresh, stale, corrupt and unwritable cases.
- Checker: httptest.
- App: drive `Model.Update` directly with `updateCheckDoneMsg` followed by the `u` key.
- Main: `runUpgrade` with a fake run func; version-fallback table; opt-out via `t.Setenv`.
- No real package manager ever runs in tests.

## 4. Approaches

| Approach | Verdict |
|---|---|
| A. Banner hint only (~100 lines) | Not "update from inside the TUI"; kept as the fallback for Manual installs |
| **B. Detect + quit-then-run (~700 lines)** | **Recommended**: safe on Windows, keeps the boundary intact, testable |
| C. Detect + `tea.ExecProcess` | The Windows exe is locked, the binary is swapped under a live process, and it adds a second exception |
| D. In-process self-replace | New dependencies, archive extraction, a Windows rename dance, and same-origin checksums (integrity only). Defer to a follow-up change. |

## 5. Risks

- A stale brew tap with `HOMEBREW_NO_AUTO_UPDATE` set may report "already up to date"; on failure, print the manual commands.
- The `go` toolchain may be missing or too old: fall back to the hint, and use the tagged version rather than `@latest`.
- Detection is heuristic: a tarball binary placed in `~/go/bin` is treated as go install. Check the specific matches first.
- Dev builds show no banner, which is the existing behavior.
- Prerelease tags never trigger an update.
- The banner language changes to Spanish.

## 6. Estimate

About 305 production lines and 445 test lines, roughly 750 in total, inside the 800-line budget.
