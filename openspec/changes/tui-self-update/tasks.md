# Tasks: Update DeployDeck From Inside The TUI

Test command: `go test ./...` (Go lives at `/usr/local/go/bin`, not on PATH). Strict TDD: each behavior task is a RED test written and seen failing, followed by its GREEN implementation. Table tests, `t.TempDir()`, fakes and `bytes.Buffer` per the go-testing skill. No test runs `brew`, `scoop`, `go install` or the real GitHub API. Commits follow the work-unit-commits skill: one reviewable unit per commit, tests and implementation together, conventional commit messages, no AI attribution.

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | ~790 (production ~310: update 145, main 115, app 50; tests ~455; docs ~25) |
| Applied budget (session) | 800 lines |
| 400-line budget risk | High (exceeds 400, but stays inside the 800 session budget with about 10 lines of headroom) |
| Chained PRs recommended | No |
| Suggested split | Single PR, 7 commits (work units below) |
| Delivery strategy | single-pr |
| Chain strategy | n/a |

Decision needed before apply: No
Chained PRs recommended: No
400-line budget risk: High

First cut lever if the running total passes 800: ADR-7, the build-info version fallback (Unit 6, tasks 7.1-7.4, about -35 lines). It touches only the `dev` -> `go install` comparison, so cutting it leaves the other behavior intact. Cutting it also removes the "Build-Info Version Fallback For The Check" requirement. In that case drop its 4 scenarios and record the delta in the verify report. Check the running diff size (`git diff --stat`) after Unit 5 and decide then.

### Suggested Work Units

| Unit | Goal | Commit | Focused test command | Rollback boundary |
|------|------|--------|----------------------|-------------------|
| 1 | Real repo + method detection + argv (Phase 1) | `feat(update): detect install method and build upgrade argv` | `go test ./internal/update/...` | Revert commit; pure functions, no callers yet |
| 2 | 24h cache (Phase 3) | `feat(update): cache the latest-release tag for 24 hours` | `go test ./internal/update/...` | Revert commit; no callers yet |
| 3 | App port: `u` key, guard, banner, footer (Phase 4) | `feat(app): add u key and Spanish update banner` | `go test ./internal/app/...` | Revert commit; Deps fields default to zero values |
| 4 | `runUpgrade` + Windows print-only (Phase 5) | `feat(cmd): run the upgrade after the TUI quits` | `go test ./cmd/deploydeck/...` | Revert commit |
| 5 | Composition: opt-out, cache wiring, post-run hook (Phase 6) | `feat(cmd): wire update check cache, opt-out and post-run upgrade` | `go test ./cmd/deploydeck/...` | Revert commit |
| 6 | Build-info version fallback, ADR-7 (Phase 7) | `feat(cmd): fall back to build info version for the update check` | `go test ./cmd/deploydeck/...` | Revert commit; this is the cut lever |
| 7 | Docs + full verification (Phases 8-9) | `docs: document in-TUI update and opt-out` | `go test ./... && go build ./...` | Revert commit |

Order is strictly sequential. Units 1 and 2 touch disjoint files in `internal/update` and Unit 3 touches only `internal/app`, so they could run in parallel in separate worktrees. The single-writer rule applies, so they run sequentially. Unit 4 needs Unit 1 (`UpgradeArgs`, `ReleasesURL`). Unit 5 needs Units 1-4. Unit 6 needs Unit 5, because it changes `decideUpdate`'s signature.

## Phase 1: Real repository and install-method detection (Unit 1)

- [x] 1.1 RED (`internal/update/checker_test.go`): checker built with its defaults against an `httptest` server records request path `/repos/malavolta/DeployDeck/releases/latest`. Satisfies: Request targets the real repository.
- [x] 1.2 GREEN (`internal/update/checker.go`): `ownerRepo = "malavolta/DeployDeck"`; remove the placeholder comment.
- [x] 1.3 RED (`internal/update/method_test.go`, new): table test for `DetectMethod` with a fake `getenv` map and no filesystem access:
  - `/opt/homebrew/Caskroom/deploydeck/1.0.0/deploydeck`, `/home/linuxbrew/.linuxbrew/bin/deploydeck`, `/usr/local/Cellar/...`
  - Windows `C:\Users\u\scoop\apps\deploydeck\current\deploydeck.exe`, `$SCOOP`-based path, mixed case
  - `/home/u/go/bin/deploydeck` (GOPATH unset, HOME `/home/u`), explicit `GOBIN`, GOPATH list (first entry plus `/bin`)
  - `/usr/local/bin/deploydeck`, empty path
  - a `GOBIN` inside a brew prefix resolves to `Brew` (ADR-4 ordering)
  
  Satisfies: Homebrew paths on macOS and Linux, Scoop path on Windows, Go install path, Unknown location is Manual.
- [x] 1.4 GREEN (`internal/update/method.go`, new): `Method` (zero is `MethodManual`), `DetectMethod` (pure; `strings` and `path` on a slash-normalized path, lower-cased on windows, never `filepath`; order Brew, Scoop, GoInstall, Manual; Brew off windows only, Scoop on windows only), `Tool()`, `ReleasesURL`.
- [x] 1.5 RED (`method_test.go`): table test for `UpgradeArgs`:
  - Brew gives `["brew","upgrade","--cask","deploydeck"]`
  - Scoop gives `["scoop","update","deploydeck"]`
  - GoInstall with `v1.3.0` ends in `/cmd/deploydeck@v1.3.0`; a tag without `v` is normalized
  - invalid tags (`v1.2`, `v1.2.3 && x`, `-x`, empty) give nil; Manual gives nil
  
  Satisfies: Brew argv, Go install uses the tag, Manual has no command; threat-matrix PR-argument composition.
- [x] 1.6 GREEN (`method.go`): `UpgradeArgs` with the `^v?\d+\.\d+\.\d+$` check (ADR-6).
- [ ] 1.7 Commit unit 1; `go test ./internal/update/...` green.

## Phase 3: 24-hour cache (Unit 2)

(Phase 2 intentionally absent: detection and argv share Phase 1's file and commit.)


- [x] 3.1 RED (`internal/update/cache_test.go`, new). Use `t.TempDir()`, a fake `Now` and a counting fetch:
  - fresh (1h old): fetch is never called, result is `v1.2.0`. Satisfies: Fresh cache avoids the network.
  - stale (25h old): one fetch, result `v1.3.0`, file rewritten with the current timestamp. Satisfies: Stale cache triggers a live check and refresh.
  - future timestamp counts as stale (one fetch).
  - corrupt JSON: fetch once, file overwritten with valid content. Satisfies: Corrupt cache degrades to a live check.
  - fetch error: error returned, no file created. Satisfies: Failed check is not cached. Include an empty `latest` result: not cached.
  - unwritable dir (a regular file placed where the dir should be): result still `v1.3.0`, no error. Satisfies: Unwritable cache directory does not break the check.
  - `Dir == ""`: always live, nothing written.
- [x] 3.2 GREEN (`internal/update/cache.go`, new): `CacheTTL`, `Cache{Dir, Now}`, `Latest` (fresh means `0 <= Now-checked_at < 24h`; read or parse errors fall through to live; atomic write via `MkdirAll` 0o755, `CreateTemp` in the same dir, `Rename`, temp file removed on error; write errors ignored; only the tag is cached).
- [ ] 3.3 Commit unit 2; `go test ./internal/update/...` green.

## Phase 4: App port, key, guard, banner (Unit 3)

Tests drive `Model.Update` with `updateCheckDoneMsg` and then `tea.KeyMsg`. Existing banner tests that pin the English text must be updated in this unit.

- [x] 4.1 RED (`internal/app/self_update_test.go`, new):
  - `u` on main menu with update, `CanUpgrade` and no operation in progress: `UpgradeRequested()` true; running the returned cmd yields `tea.QuitMsg`. Satisfies: u on main menu requests upgrade and quits.
  - `u` in another state with an update: `UpgradeRequested()` false and the state's existing `u` handling is unchanged. Satisfies: u on another screen does nothing.
  - `repoState.InProgress` true: no request, nil cmd, rendered menu contains `termina la operación en curso antes de actualizar DeployDeck`. Then InProgress false and `u` again: request true plus quit cmd. Satisfies: Blocked during in-progress operation, Allowed again after the operation ends.
  - no update available: no request, nil cmd. Satisfies: No update available.
  - update available, `CanUpgrade == false`: no request, nil cmd. Satisfies: Update available but nothing wired, Manual install never runs a command (app half).
  - existing tests with zero-value Deps behave as before.
- [x] 4.2 GREEN: `app.go` adds `Deps.CanUpgrade bool`, `Deps.ReleasesURL string`, model field `upgradeRequested`, getters `UpgradeRequested()` and `UpdateLatest()`. `keys.go` adds `case "u"` in `keyMainMenu` (no-op if `!updateAvailable || !CanUpgrade`; `InProgress` sets the existing main-menu notice without quitting; otherwise sets the flag and returns `quitCmd()`).
- [x] 4.3 RED (`internal/app/view_test.go` or the new file; update tests that pin the English banner):
  - main menu, upgradable: contains `v1.3.0`, `Nueva versión de DeployDeck disponible`, `pulsa u para actualizar`, and the footer `u actualizar`. Satisfies: Banner on main menu for an upgradable install.
  - main menu, Manual (`CanUpgrade` false, `ReleasesURL` set): contains `v1.3.0` and `https://github.com/malavolta/DeployDeck/releases/latest`; does not contain `pulsa u`. Satisfies: Banner on main menu for a manual install.
  - other state with an update: banner shows the version but no `pulsa u` hint. Satisfies: No u hint outside the main menu.
  - no update: no banner text in any view. Satisfies: No banner without an update.
  - the old English banner text is gone.
- [x] 4.4 GREEN (`view.go`): Spanish banner with a state-aware suffix, conditional main-menu footer (copy table in design).
- [x] 4.5 Verify `internal/app/boundary_test.go` passes unchanged. Satisfies: Boundary preserved.
- [ ] 4.6 Commit unit 3; `go test ./internal/app/...` green.

## Phase 5: runUpgrade (Unit 4)

- [x] 5.1 RED (`cmd/deploydeck/selfupdate_test.go`, new). Use a fake `runFunc` and `bytes.Buffer`:
  - success with `["brew","upgrade","--cask","deploydeck"]`: fake receives exactly that argv once, stderr empty. Satisfies: Requested upgrade runs after quit.
  - error (Brew): stderr contains `brew upgrade --cask deploydeck` and the releases URL, in Spanish, one call, no retry. Satisfies: Non-zero exit prints manual commands.
  - "not found" style error: same manual text, one call. Satisfies: Command cannot start.
  - nil argv (Manual or invalid tag): manual text, no call. Satisfies: Manual install never runs a command (post-run half); threat-matrix nil-argv case.
  - `goos == "windows"` with `["scoop","update","deploydeck"]`: run function never called, stdout contains `Para completar la actualización ejecuta: scoop update deploydeck`, no error. Satisfies: Windows prints the command instead of running it.
- [x] 5.2 GREEN (`cmd/deploydeck/selfupdate.go`, new): `runFunc`, `runUpgrade` (before message, success message, failure message to stderr, Windows print-only, returns nothing so exit status is unchanged), `runInherited` (`osexec.Command` with Stdin/Stdout/Stderr = `os.*`).
- [ ] 5.3 Commit unit 4; `go test ./cmd/deploydeck/...` green.

## Phase 6: Composition and post-run step (Unit 5)

- [x] 6.1 RED (`cmd/deploydeck/selfupdate_test.go`): `composeUpdate` with fake `getenv`, `lookPath`, `exe`:
  - `DEPLOYDECK_NO_UPDATE_CHECK=1`: `CheckUpdate` is nil, the server fixture fails the test if called. Satisfies: Opt-out disables the check.
  - `DEPLOYDECK_NO_UPDATE_CHECK=""`: `CheckUpdate` wired. Satisfies: Empty value does not opt out.
  - Manual method or a `lookPath` miss: `CanUpgrade == false`; Brew with the tool found: true.
- [x] 6.2 RED (`cmd/deploydeck/main_test.go`): rewrite the `defaultCheckUpdate` cancellation test to target `newCheckUpdate(baseURL, cacheDir, current)` with a pre-canceled ctx and a `t.TempDir()` cache, so a real cache file cannot hide the error. Add a post-run test: final model with `UpgradeRequested()` false never calls the run function. Satisfies: No request means no run, Failed check is not cached (end to end).
- [x] 6.3 GREEN: `selfupdate.go` adds `composeUpdate` (opt-out, `Cache{Dir,Now}.Latest(ctx, Checker.Latest)`, `DetectMethod` plus `lookPath`, `os.UserCacheDir` failure gives `Dir ""`) and `resolveExePath` (`os.Executable` plus `EvalSymlinks`, error gives `""`). `main.go` replaces `defaultCheckUpdate` with `newCheckUpdate`, `defaultRunTUI` wires `composeUpdate`, keeps the final model from `program.Run()`, and calls `runUpgrade(UpgradeArgs(Method, UpdateLatest()), goos, os.Stdout, os.Stderr, runInherited)` only when `UpgradeRequested()`.
- [ ] 6.4 Commit unit 5; `go test ./cmd/deploydeck/...` green. Then run `git diff --stat main` against the 800-line budget and decide whether to apply the ADR-7 cut.

## Phase 7: Build-info version fallback, ADR-7 (Unit 6) - CUT, not implemented

Cut at the Unit 5 size check (diff already ~886 lines before docs). `decideUpdate(current, ...)` already takes `current` explicitly (done in Unit 5); the `resolveCurrentVersion` fallback, its tests and spec scenarios are dropped.

- [~] 7.1 (CUT) RED (`cmd/deploydeck/selfupdate_test.go`): table for `resolveCurrentVersion(v, readBuildInfo)`:
  - `dev` plus build info `v1.2.3` gives `v1.2.3`. Satisfies: go install build compares with its module version.
  - `dev` plus `(devel)` or `""` gives `dev`, and `decideUpdate("dev", latest, nil)` yields no banner. Satisfies: Devel or empty build info stays dev.
  - `v1.4.0` plus build info `v1.2.3` gives `v1.4.0`. Satisfies: Ldflags version wins over build info.
- [~] 7.2 (CUT) RED (`main_test.go`): existing `TestDecideUpdate` table passes `current` explicitly (no mutation of the `version.Version` global); keep the newer, same/older and pre-release cases. Satisfies the MODIFIED requirement scenarios: Newer version available shows a notice, Same or older version shows no notice, Pre-release tag never triggers a notice. Add a `--version` test asserting output is unchanged with `version.Version = "dev"`. Satisfies: --version is unaffected.
- [~] 7.3 (CUT) GREEN: `decideUpdate(current, latest, err)` takes `current` explicitly. `resolveCurrentVersion` is called only when building the update check, and `version.String()` is untouched.
- [~] 7.4 (CUT) Commit unit 6; `go test ./cmd/deploydeck/...` green.

## Phase 8: Docs (Unit 7)

- [x] 8.1 `README.md` `## Update`: mention `u` in the TUI, `DEPLOYDECK_NO_UPDATE_CHECK`, the 24h cache, and that on Windows the command is printed instead of run. Keep the cask-not-formula note.
- [x] 8.2 `docs/ACTIVATION-CHECKLIST.md`: mark activation step (a) (real repository) done.

## Phase 9: Verification (Unit 7)

- [x] 9.1 `go test ./...` full suite green, including `internal/app/boundary_test.go` unchanged.
- [x] 9.2 `go build ./...` and `go vet ./...`.
- [x] 9.3 Spec-coverage pass: every scenario in both spec files maps to a test above (both spec files, including the MODIFIED semver scenarios and the amended Windows print-only scenario).
- [x] 9.4 Final line count `git diff --stat main`: confirm at or under 800; otherwise apply the ADR-7 cut (Phase 7) and note it for sdd-verify.

---

Out of scope: in-place self-replace for manual binaries (follow-up `tui-self-update-manual`). The merge of the spec deltas into `openspec/specs/` is `sdd-archive` scope.
