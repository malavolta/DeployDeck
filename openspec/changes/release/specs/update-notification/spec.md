# Update Notification Specification

## Purpose

Non-blocking startup check that informs the user when a newer DeployDeck
version exists, without ever delaying or gating the TUI.

## ADDED Requirements

### Requirement: Non-Blocking Startup Check

The system MUST run the update-availability check concurrently with TUI
startup. The check MUST NOT delay, block, or gate the TUI's initial render
or its readiness for user interaction, regardless of how long the check
takes.

(HU-019 Tarea, docs/HISTORIAS.md:1174; AC5, docs/HISTORIAS.md:1183)

#### Scenario: Check runs concurrently with startup

- GIVEN the update endpoint is reachable and responds normally
- WHEN the TUI starts
- THEN the TUI becomes interactive without waiting for the update check to
  complete

#### Scenario: Slow or unreachable endpoint never blocks startup

- GIVEN the update endpoint is down, slow, unreachable, or times out
- WHEN the TUI starts
- THEN startup proceeds normally with no delay attributable to the check

(Test E2E variant, docs/HISTORIAS.md:1194)

### Requirement: Semver-Based Newer-Version Detection

The system MUST determine whether a newer version is available by
comparing the running version against the latest known release using
semantic-version comparison (`HasNewer`).

(Test E2E, docs/HISTORIAS.md:1191,1193)

#### Scenario: Newer version available shows a notice

- GIVEN the latest known release version is semver-greater than the
  running version
- WHEN the update check completes
- THEN a non-blocking notice/banner is shown to the user

#### Scenario: Same or older version shows no notice

- GIVEN the latest known release version is equal to or semver-older than
  the running version
- WHEN the update check completes
- THEN no notice is shown

### Requirement: Silent Skip on Check Failure

The system MUST treat any check failure — timeout, network error,
unreachable host, non-2xx response, or unauthorized (401) response —
identically: it MUST skip the notice silently, without surfacing an error
to the user and without distinguishing the failure cause in the UI.

(AC5 + Nota decision pendiente, docs/HISTORIAS.md:1183,1185; Test E2E
variant, docs/HISTORIAS.md:1194)

#### Scenario: Down or slow endpoint yields no notice and no error

- GIVEN the update endpoint is down, slow past its timeout, or returns a
  non-2xx/401 response
- WHEN the update check completes or times out
- THEN no notice is shown and no error message is surfaced to the user
