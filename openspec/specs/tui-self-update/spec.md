# TUI Self-Update Specification

## Purpose

Let the user upgrade DeployDeck from the main menu with one key. The TUI quits cleanly (restoring the original branch), then the process runs the install-method-specific upgrade command with inherited stdio. `internal/app` never executes processes; it only exposes scalars and an `UpgradeRequested()` getter. In-place self-replace for manual binaries is out of scope (follow-up `tui-self-update-manual`).

## Requirements

### Requirement: Install Method Detection

The system MUST classify the running executable into exactly one method: `Brew`, `Scoop`, `GoInstall` or `Manual`, via a pure function of the resolved executable path, the OS name and environment values (no filesystem or process access inside the function). Matching MUST normalize path separators and case where the OS requires it, and MUST check the most specific matches first:

- `Brew`: path under a Homebrew prefix (for example `/opt/homebrew/`, `/usr/local/Cellar/`, `/usr/local/Caskroom/`, `/home/linuxbrew/.linuxbrew/`).
- `Scoop`: path containing a `scoop` apps/shims segment (Windows).
- `GoInstall`: path under `$GOBIN`, or `$GOPATH/bin`, or `~/go/bin` when those apply.
- `Manual`: anything else, including an unresolvable path.

#### Scenario: Homebrew paths on macOS and Linux

- GIVEN executable paths `/opt/homebrew/Caskroom/deploydeck/1.0.0/deploydeck` and `/home/linuxbrew/.linuxbrew/bin/deploydeck`
- WHEN the method is detected
- THEN both are `Brew`

#### Scenario: Scoop path on Windows

- GIVEN goos `windows` and path `C:\Users\u\scoop\apps\deploydeck\current\deploydeck.exe`
- WHEN the method is detected
- THEN the method is `Scoop`

#### Scenario: Go install path

- GIVEN path `/home/u/go/bin/deploydeck` with `GOPATH` unset and home `/home/u`, or a path under an explicit `GOBIN`
- WHEN the method is detected
- THEN the method is `GoInstall`

#### Scenario: Unknown location is Manual

- GIVEN path `/usr/local/bin/deploydeck` or an empty path
- WHEN the method is detected
- THEN the method is `Manual`

### Requirement: Upgrade Command Construction

The system MUST build the upgrade command as an argv slice (never a shell string) from the install method and the latest version:

- `Brew`: `brew upgrade --cask deploydeck` with no preceding `brew update`.
- `Scoop`: `scoop update deploydeck`.
- `GoInstall`: `go install github.com/malavolta/DeployDeck/cmd/deploydeck@<latest tag>`, using the tagged version and not `@latest`.
- `Manual`: no command (nil).

#### Scenario: Brew argv

- GIVEN method `Brew` and latest `v1.3.0`
- WHEN the upgrade argv is built
- THEN it is `["brew","upgrade","--cask","deploydeck"]`

#### Scenario: Go install uses the tag

- GIVEN method `GoInstall` and latest `v1.3.0`
- WHEN the upgrade argv is built
- THEN the last element ends with `/cmd/deploydeck@v1.3.0`

#### Scenario: Manual has no command

- GIVEN method `Manual`
- WHEN the upgrade argv is built
- THEN the result is nil

### Requirement: Update Key On Main Menu Only

The `u` key MUST trigger an upgrade request only while the model is in `StateMainMenu`, an update is available, and an upgrade command is wired (non-empty). On trigger the model MUST record an upgrade request (readable through the exported `UpgradeRequested()` getter) and MUST quit through the same quit path as `q`/`esc` (so the original branch is restored). On every other state, `u` MUST keep its existing behavior and MUST NOT request an upgrade.

#### Scenario: u on main menu requests upgrade and quits

- GIVEN `StateMainMenu`, an update available, an upgrade command wired and no operation in progress
- WHEN the user presses `u`
- THEN `UpgradeRequested()` returns true and the returned command is the quit command

#### Scenario: u on another screen does nothing

- GIVEN an update available and a state other than `StateMainMenu`
- WHEN the user presses `u`
- THEN `UpgradeRequested()` returns false and the state's existing handling of `u` is unchanged

### Requirement: Guard While An Operation Is In Progress

While `repoState.InProgress` is true, pressing `u` on the main menu MUST NOT request an upgrade and MUST NOT quit. The system MUST show a Spanish notice explaining that the update is blocked until the in-progress operation is finished. The notice MUST use the same notice mechanism as the other main-menu guards.

#### Scenario: Blocked during in-progress operation

- GIVEN `StateMainMenu`, an update available, an upgrade command wired and `repoState.InProgress = true`
- WHEN the user presses `u`
- THEN `UpgradeRequested()` returns false, no quit command is returned, and the rendered main menu contains a Spanish notice stating the update is blocked by the operation in progress

#### Scenario: Allowed again after the operation ends

- GIVEN the previous blocked press and `repoState.InProgress` now false
- WHEN the user presses `u`
- THEN `UpgradeRequested()` returns true and the quit command is returned

### Requirement: No-Op Without Update Or Wiring

Pressing `u` on the main menu MUST do nothing (no quit, no request, no error) when no update is available, or when no upgrade command is wired (Manual install, opt-out, or nil dependency). Models built without the upgrade dependencies (existing tests) MUST behave as before.

#### Scenario: No update available

- GIVEN `StateMainMenu` with no update available
- WHEN the user presses `u`
- THEN `UpgradeRequested()` returns false and no quit command is returned

#### Scenario: Update available but nothing wired

- GIVEN `StateMainMenu`, an update available and no upgrade command wired
- WHEN the user presses `u`
- THEN `UpgradeRequested()` returns false and no quit command is returned

### Requirement: Quit-Then-Run Execution

After the TUI program returns, if `UpgradeRequested()` is true, the composition root MUST run the upgrade argv with inherited stdin, stdout and stderr, in the already-restored terminal. The command MUST NOT run while the TUI is alive and MUST NOT run when no upgrade was requested. The execution seam MUST be injectable so that tests never invoke a real package manager. `internal/app` MUST NOT import `os/exec`, `net/http`, `internal/version` or `internal/update`.

#### Scenario: Requested upgrade runs after quit

- GIVEN an upgrade was requested with argv `["brew","upgrade","--cask","deploydeck"]` and a fake run function returning success
- WHEN the post-run step executes
- THEN the fake receives exactly that argv once, stdio is the process stdio, and nothing is printed as a failure

#### Scenario: Windows prints the command instead of running it

On Windows the DeployDeck executable stays locked while the process is alive, including during the post-run step: `go install` cannot overwrite it and Scoop refuses to update a running app. Running the command would always fail.

- GIVEN an upgrade was requested on `goos == "windows"` with argv `["scoop","update","deploydeck"]`
- WHEN the post-run step executes
- THEN the run function is never called, the exact command is printed for the user to run after DeployDeck exits, and the exit status is unchanged

#### Scenario: No request means no run

- GIVEN `UpgradeRequested()` is false
- WHEN the post-run step executes
- THEN the run function is never called

#### Scenario: Boundary preserved

- GIVEN the existing `internal/app` import boundary test
- WHEN it runs
- THEN it passes unchanged

### Requirement: Failure Fallback Prints Manual Commands

When the upgrade command fails to start or exits non-zero, the system MUST NOT retry, MUST print the manual upgrade instructions to stderr, and MUST NOT leave persisted state. The instructions MUST include the install-method command for the method (Brew, Scoop, GoInstall) and the releases URL `https://github.com/malavolta/DeployDeck/releases/latest`. The message MUST be in Spanish. The process exit status of DeployDeck itself is not changed by an upgrade failure.

#### Scenario: Non-zero exit prints manual commands

- GIVEN an upgrade was requested for method `Brew` and the fake run function returns an error
- WHEN the post-run step executes
- THEN stderr contains `brew upgrade --cask deploydeck` and the releases URL, and the run function was called exactly once

#### Scenario: Command cannot start

- GIVEN the fake run function returns a "not found" style error
- WHEN the post-run step executes
- THEN the same manual instructions are printed and no retry occurs

### Requirement: Manual Install Is Hint Only

For `Manual` installs the system MUST NOT attempt any in-process or external upgrade. The only affordance is the banner hint with the releases URL (see `update-notification`).

#### Scenario: Manual install never runs a command

- GIVEN method `Manual`, an update available and the main menu displayed
- WHEN the user presses `u`
- THEN nothing runs, `UpgradeRequested()` returns false, and the banner shows the releases URL
