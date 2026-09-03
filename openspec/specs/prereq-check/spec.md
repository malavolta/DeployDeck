# Capability: Prerequisite Check

## Overview

The prerequisite check is the first phase of the DeployDeck flow. It validates the local environment, Git repository state, Salesforce CLI configuration, and plugin versions before any promotion work begins. A blocking prerequisite failure prevents the promotion flow from starting; informative checks warn without blocking.

## Requirements

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

### Requirement: Binary, Plugin, And Minimum Version Validation

The system SHALL validate installed versions of `git`, `sf`, and the `sfdx-git-delta` plugin against `minVersions` from configuration, and SHALL validate plugin presence via `sf plugins --json` or an equivalent output.

#### Scenario: Version below minimum blocks with upgrade guidance
- GIVEN an installed `git`/`sf`/`sfdx-git-delta` version below the configured `minVersions`
- WHEN the environment is validated
- THEN the check reports blocking `Status` showing current version, required version, and an update `FixCommand`

#### Scenario: Missing sfdx-git-delta plugin shows install command
- GIVEN the `sfdx-git-delta` plugin is not installed
- WHEN the environment is validated
- THEN the check reports the suggested install `FixCommand`

### Requirement: Repository Membership And Remote Validation

The system SHALL validate that the current directory belongs to a Git repository and that an `origin` remote is configured.

#### Scenario: Missing origin blocks the flow
- GIVEN the repository has no `origin` remote configured
- WHEN the prerequisite check runs
- THEN the check reports blocking `Status` with a corrective action

### Requirement: Working Tree Cleanliness Check

The system SHALL detect a dirty working tree and block branch-modifying operations.

#### Scenario: Dirty working tree blocks promotion start
- GIVEN the working tree has uncommitted changes
- WHEN the user attempts to start a promotion
- THEN branch modification is blocked

### Requirement: `.deploydeck/` Gitignore Enforcement (Blocking)

The system SHALL block the flow when `.deploydeck/` is not present in the `.gitignore` governing the artifacts root, and SHALL offer to add the entry there. WHEN the artifacts root and the git root coincide (flat layout, the default), this is observably the repository's top-level `.gitignore`, unchanged from before.

(Previously: stated as "the repository's `.gitignore`", which the checker resolved unconditionally at the git root — wrong when the artifacts root is a nested SFDX project subdirectory, causing a false block and an untracked git-root `.gitignore` from the offered fix.)

#### Scenario: Ungitignored .deploydeck blocks the flow
- GIVEN `.gitignore` governing the artifacts root does not contain a `.deploydeck/` entry
- WHEN the environment is validated
- THEN the flow is blocked and the user is offered to add the entry

#### Scenario: Nested layout checks the artifacts-root gitignore, not the git root's
- GIVEN a nested repo where the artifacts root differs from the git root, and `.deploydeck/` is already gitignored at the artifacts root but not at the git root
- WHEN the environment is validated
- THEN the check passes, because it reads the `.gitignore` governing the artifacts root

#### Scenario: The offered fix writes at the artifacts root, not the git root
- GIVEN a nested repo missing the `.deploydeck/` entry at the artifacts root
- WHEN the user accepts the offered fix
- THEN the entry is added to the `.gitignore` governing the artifacts root, and no untracked `.gitignore` is created at the git root

### Requirement: Salesforce Alias Validation

The system SHALL validate configured Salesforce aliases via `sf org list --json` and block validation against a sandbox whose alias does not exist.

#### Scenario: Missing alias blocks that sandbox
- GIVEN a configured Salesforce alias that does not exist in `sf org list --json`
- WHEN the environment is validated
- THEN the missing alias is shown and validation against that sandbox is blocked

### Requirement: Single-Instance Lock Acquisition

The system SHALL acquire an exclusive lock at `.deploydeck/lock` before starting, and SHALL block startup, naming the owning process, when another instance already holds it.

#### Scenario: Lock already held blocks startup
- GIVEN another DeployDeck instance holds `.deploydeck/lock` for this repo
- WHEN a new instance attempts to start
- THEN startup is blocked and the message identifies the owning process

### Requirement: `deploydeck doctor` CLI Subcommand

The system SHALL expose the same prerequisite checks as a `deploydeck doctor` CLI subcommand, which SHALL exit with a non-zero exit code, distinct from the success case, when any blocking check fails.

#### Scenario: Doctor exits non-zero on blockers
- GIVEN at least one blocking `PrereqCheck`
- WHEN `deploydeck doctor` runs
- THEN the process exits with a non-zero exit code distinct from success

### Requirement: Git Hooks Interference Detection (Informative)

The system SHALL detect repository Git hooks that could interfere with checkout or cherry-pick operations and SHALL report them as an informative, non-blocking `PrereqCheck`.

#### Scenario: Interfering hook is reported informationally
- GIVEN the repository has a Git hook that could interfere with checkout or cherry-pick
- WHEN the prerequisite check runs
- THEN the hook is reported as an informative, non-blocking `PrereqCheck`

### Requirement: `gh` CLI Availability And Authentication Check (Informative)

The system SHALL report `gh` CLI availability and authentication status as an
informative, non-blocking `PrereqCheck`, classifying it as absent,
present-unauthenticated, or present-authenticated, and SHALL NEVER block the
flow based on this check.

#### Scenario: gh absent is reported informationally
- GIVEN the `gh` binary is not installed
- WHEN the prerequisite check runs
- THEN an informative, non-blocking `PrereqCheck` reports `gh` as absent

#### Scenario: gh present but unauthenticated is reported informationally
- GIVEN `gh` is installed but not authenticated
- WHEN the prerequisite check runs
- THEN an informative, non-blocking `PrereqCheck` reports `gh` as
  present-unauthenticated

#### Scenario: gh present and authenticated is reported informationally
- GIVEN `gh` is installed and authenticated
- WHEN the prerequisite check runs
- THEN an informative, non-blocking `PrereqCheck` reports `gh` as
  present-authenticated

#### Scenario: gh check never blocks the flow
- GIVEN any `gh` availability/auth state, including absent
- WHEN the prerequisite check runs
- THEN the flow is not blocked by this check

### Requirement: AI Endpoint And Model Availability Check (Informative)

The system SHALL report AI endpoint reachability and configured-model
availability (`GET <endpoint>/v1/models`) as an informative, non-blocking
`PrereqCheck`, and SHALL NEVER report `StatusBlocking` for this check. WHEN
no `ai` configuration is present or `Checker.AI` is nil, the check SHALL be
skipped, reporting `StatusOK`. This closes the remaining part of HU-014's
deferred "Idea Futura" (`docs/HISTORIAS.md:977-978`, "Chequeo
INFORMATIVO/no bloqueante en el doctor").

#### Scenario: Absent config or nil client skips the check
- GIVEN no `ai` configuration is present, or `Checker.AI` is nil
- WHEN the prerequisite check runs
- THEN `CheckAI` is skipped and reports `StatusOK`

#### Scenario: Reachable endpoint with configured model listed
- GIVEN the AI endpoint is reachable and `GET <endpoint>/v1/models` lists
  the configured model
- WHEN the prerequisite check runs
- THEN an informative `PrereqCheck` reports `StatusOK`

#### Scenario: Reachable endpoint with configured model missing
- GIVEN the AI endpoint is reachable but `GET <endpoint>/v1/models` does
  not list the configured model
- WHEN the prerequisite check runs
- THEN an informative `PrereqCheck` reports `StatusWarning`

#### Scenario: Unreachable endpoint is reported informationally
- GIVEN the AI endpoint is unreachable or times out
- WHEN the prerequisite check runs
- THEN an informative `PrereqCheck` reports `StatusWarning`

#### Scenario: AI check never blocks the flow
- GIVEN any AI reachability/model-availability state, including
  unreachable
- WHEN the prerequisite check runs
- THEN the flow is not blocked by this check

## Design Notes

- The informative, non-blocking `gh` CLI availability/authentication check is implemented by HU-014 (PR automation), as it enables the push-pr-preparation capability.
- The informative, non-blocking AI endpoint/model availability check is implemented by HU-014 (PR automation), as it enables the ai-pr-summary capability.
- The lock implementation uses atomic file writes and stale-takeover detection via process start-time probing to ensure safe multi-instance coordination.
