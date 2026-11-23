# Proposal: Update DeployDeck From Inside The TUI

> **Amendment (2026-11-05):** ADR-7 (build-info version fallback, `resolveCurrentVersion`) was CUT during apply to stay within the 800-line review budget. Only `decideUpdate(current, latest, err)` taking `current` explicitly was kept. The requirement was removed from the update-notification spec delta. Follow-up change recorded in state.yaml.

**Delivery phase**: Fase 5 Productizacion, extending HU-019 (update notice).

## Intent

The startup update check has never worked in production: `ownerRepo = "OWNER/REPO"` (`internal/update/checker.go:19`) returns 404, so the check silently skips. `go install` binaries report `"dev"` and never see a banner. Users who do see a notice must leave and look up the upgrade command themselves. Goal: detect a newer release on startup and let the user upgrade with one key.

## Scope

### In Scope
- Fix `ownerRepo` to `malavolta/DeployDeck`.
- Detect the install method (`Brew`, `Scoop`, `GoInstall`, `Manual`) and build its upgrade argv. Pure functions.
- Use a 24h cache in `os.UserCacheDir()/deploydeck/`. Failed checks are not cached.
- Add the `DEPLOYDECK_NO_UPDATE_CHECK` opt-out.
- Add a `runtime/debug.ReadBuildInfo` fallback when `version.Version == "dev"`, used for the update check only. This is the first item to cut if the forecast exceeds 800 lines.
- Translate the banner to Spanish, with a `u` hint on the main menu and the releases URL for `Manual` installs.
- Add a `u` key on `StateMainMenu` only. While `repoState.InProgress`, it is blocked with a Spanish notice.
- Update the README `## Update` section and `docs/ACTIVATION-CHECKLIST.md`.

### Out of Scope
- In-place self-replace for manual binaries. Planned follow-up: `tui-self-update-manual`.
- Updating without user action, telemetry, prerelease channels.
- Changing `--version` output.

## Capabilities

### New Capabilities
- `tui-self-update`: install-method detection, the `u` key and its guards, the quit-then-run upgrade, and the failure fallback.

### Modified Capabilities
- `update-notification`: real repository, 24h cache, opt-out, buildinfo version fallback, Spanish banner with an action hint.

## Approach (decisions)

| Decision | Choice | Rationale |
|---|---|---|
| Execution | `u` sets a flag and calls `quitCmd()`. After `program.Run()` returns, `main` runs the argv with inherited stdio. | Windows locks the running exe. Avoids a second `tea.ExecProcess` exception. Restores the original branch. |
| Brew | `brew upgrade --cask` with no `brew update` | Faster and predictable. If it fails, DeployDeck prints the manual commands. |
| Non-zero exit | Print the manual commands | No retries and no hidden state |
| Banner language | Spanish. Tests pinning the English text are updated. | Matches the app |
| Boundary | `internal/app` gets only Deps scalars plus `UpgradeRequested()`. `boundary_test.go` stays unchanged. | No `os/exec`, `net/http` or `internal/update` imports in `internal/app` |

## Affected Areas

| Area | Impact |
|---|---|
| `internal/update/checker.go`, `method.go`, `cache.go` | Modified/New |
| `cmd/deploydeck/main.go` | Modified: composition, opt-out, fallback, `runUpgrade` |
| `internal/app/{app,keys,view,update}.go` | Modified |
| `README.md`, `docs/ACTIVATION-CHECKLIST.md` | Modified |

## Risks

| Risk | Likelihood | Mitigation |
|---|---|---|
| A stale brew tap reports "up to date" | Med | Print the manual commands. Documented in the README. |
| Heuristic detection is wrong | Med | Check specific matches first. A wrong guess only means the manual command is shown. |
| `go` toolchain missing or too old | Low | Install the tagged version rather than `@latest`, and fall back to the hint |
| Forecast exceeds the budget | Low | Cut the buildinfo fallback |

## Rollback Plan

No shared Git state or Salesforce sandbox is touched, because `quitCmd()` restores the branch before the upgrade runs. The cache file is disposable. Revert the PR. Users can set `DEPLOYDECK_NO_UPDATE_CHECK` as an immediate kill switch.

## Dependencies

- A published GitHub release on `malavolta/DeployDeck`.

## Success Criteria

- [ ] A real newer release shows the Spanish banner.
- [ ] Pressing `u` on the main menu quits the TUI, runs the right command, and prints the manual commands on failure.
- [ ] `u` is blocked during an in-progress operation and does nothing on other screens.
- [ ] Tests never run a real package manager. `go test ./...` and the boundary test pass.
- [ ] One PR of about 750 lines (about 305 production, 445 test), within the 800-line budget.

## Proposal question round

Auto mode: the orchestrator answered the exploration's questions (decisions above). For user review:
- Should `--version` also use the buildinfo fallback?
- Is the `brew upgrade` behavior without `brew update` acceptable?
