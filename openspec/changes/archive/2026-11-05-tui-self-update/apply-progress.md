# Apply Progress: tui-self-update

Status: all units implemented except Unit 6 (ADR-7), which was cut. Nothing committed (user decides). Size: 800 changed lines excluding openspec (`git diff --stat origin/main -- . ':(exclude)openspec'`, new files counted via `git add -N`), recorded as `size:exception` within the 800-line user budget.

## Work-unit boundaries (suggested commits, tests + code together)

| Unit | Files |
|---|---|
| 1 | internal/update/{checker.go,checker_test.go,method.go,method_test.go} |
| 2 | internal/update/{cache.go,cache_test.go} |
| 3 | internal/app/{app.go,keys.go,view.go,self_update_test.go} |
| 4+5 | cmd/deploydeck/{selfupdate.go,selfupdate_test.go,main.go,main_test.go} (Unit 4 = runUpgrade; Unit 5 = composeUpdate/newCheckUpdate/postRun/main wiring) |
| 7 | README.md |

## TDD evidence

| Unit | RED (observed) | GREEN |
|---|---|---|
| 1 | `go test ./internal/update` build failed: undefined update.Method/DetectMethod/UpgradeArgs | ok after method.go + ownerRepo |
| 2 | build failed: undefined update.Cache | ok after cache.go |
| 3 | build failed: unknown Deps.CanUpgrade/ReleasesURL, undefined UpgradeRequested/UpdateLatest | ok; boundary_test unchanged and passing |
| 4 | build failed: undefined runUpgrade | ok after selfupdate.go |
| 5 | vet failed: decideUpdate arity, undefined composeUpdate/postRun/newCheckUpdate | ok after composition |

## Deviations

- ADR-7 (Unit 6) cut: the diff was already ~886 lines before docs. Requirement "Build-Info Version Fallback For The Check" and its 4 scenarios removed from the update-notification delta; MODIFIED semver requirement reworded. `decideUpdate(current, latest, err)` still takes `current` explicitly (needed by `newCheckUpdate`).
- `runUpgrade` before-message omits the version (`Actualizando DeployDeck: <argv>`) because the pinned signature has no `latest` parameter.
- `composeUpdate`/`newCheckUpdate` signatures are flat (baseURL, cacheDir, current, exe, goos, getenv, lookPath) and return an `updateWiring` struct; extra helper `postRun(final tea.Model, ...)` hosts the post-run decision so it is testable.
- No existing English-banner test pinned the old text (only `9.9.9` substrings); none needed updating. The old `defaultCheckUpdate` test was replaced by `TestNewCheckUpdate_FailedCheckNotCached`.
- `docs/ACTIVATION-CHECKLIST.md` step (a) was already marked DONE; no change.
- Compact test tables were used to stay at 800.
