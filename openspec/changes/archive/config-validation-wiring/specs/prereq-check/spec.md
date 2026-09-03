# Delta for Prerequisite Check

Source: HU-001 (`docs/HISTORIAS.md:11-93`) — activates HU-001's `PrereqCheck{Name, Status, Detail, FixCommand}` reporting contract for a check the story's acceptance criteria implies but never named: a malformed `deploydeck.yaml` blocks with a corrective action, the same pattern already used for missing `git`/`sf`/plugin/alias/lock.

## ADDED Requirements

### Requirement: Config Validity Check (Blocking)

The system MUST run `Config.Validate()` as a blocking prerequisite check named `config file`. This check MUST run FIRST in the report, before `CheckVersions`, because `Config.MinVersions` and `Config.Sandboxes` are consumed by checks later in the list, and a malformed config should surface before any check that depends on it.

WHEN `Validate()` returns an error, the check MUST report `Status = StatusBlocking`, `Detail` set to that error's message verbatim, and `FixCommand` naming the RESOLVED config file path.

The check MUST only ever run against a `Config` that has already been through `applyDefaults` (i.e., produced by `config.Load`), never a bare `config.Config{}` literal. Omitting `pollIntervalSeconds`/`pollTimeoutSeconds` from a config file is the normal way to author it, so calling `Validate()` before defaults are applied fails those two fields spuriously on every real config — verified against a live user config, the README example, and the ARQUITECTURA example, all three of which fail without defaults and all three of which pass with them.

Because `Detail` is rendered by a single-line `Fprintf`, the check reports only the FIRST error `Validate()` returns. A config violating N rules requires N run-fix cycles, one violation surfaced per rerun, not a combined multi-line detail.

#### Scenario: Malformed config blocks with first error and fix path

- GIVEN a resolved config file whose loaded, defaulted `Config` fails `Validate()` on its first violated rule
- WHEN the prerequisite check runs
- THEN the check reports `Name = "config file"`, `Status = StatusBlocking`, `Detail` equal to the `Validate()` error verbatim, and `FixCommand` naming the resolved config file path

#### Scenario: Config check runs before CheckVersions

- GIVEN a fresh `Checker.Check()` invocation
- WHEN the returned `[]PrereqCheck` is inspected
- THEN the `config file` entry appears before any `CheckVersions` entry

#### Scenario: Loaded (defaulted) minimal config passes

- GIVEN a minimal config file loaded via `config.Load` (so `applyDefaults` ran)
- WHEN the config validity check runs
- THEN it reports `Status = StatusOK`

#### Scenario: Bare struct literal is rejected (regression pin)

- GIVEN a `config.Config{}` literal built directly, without going through `Load`/`applyDefaults`
- WHEN `Validate()` is called on it
- THEN it returns an error naming `pollIntervalSeconds`, pinning that `Validate()` MUST NOT be called on an un-defaulted `Config`

#### Scenario: Multiple violations require multiple fix cycles

- GIVEN a config violating two or more `Validate()` rules
- WHEN the user fixes the first reported violation and reruns the check
- THEN the check reports the NEXT violation alone, not a combined multi-line detail

## MODIFIED Requirements

### Requirement: Prerequisite Check Execution And Reporting

The system SHALL run all configured local-prerequisite checks (config validity, git, sf, `sfdx-git-delta`, repo membership, origin, working tree, gitignore, aliases, lock) and report each as a `PrereqCheck` with `Status` (OK/warning/blocking), `Detail`, and `FixCommand` where applicable. The config validity check SHALL run first, before all others, since the version and alias checks consume `MinVersions`/`Sandboxes` from that same config.

(Previously: the check list omitted config validity and defined no first-check ordering constraint.)

#### Scenario: All prerequisites pass

- GIVEN a correctly configured local environment
- WHEN the user opens DeployDeck
- THEN all critical `PrereqCheck` entries report `Status = OK`

#### Scenario: Missing git binary blocks the flow

- GIVEN `git` is not available on PATH
- WHEN the prerequisite check runs
- THEN the flow is blocked and a corrective `FixCommand` is shown
