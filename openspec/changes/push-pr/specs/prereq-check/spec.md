# Delta for Prerequisite Check

## ADDED Requirements

### Requirement: `gh` CLI Availability And Authentication Check (Informative)

The system SHALL report `gh` CLI availability and authentication status as an
informative, non-blocking `PrereqCheck`, classifying it as absent,
present-unauthenticated, or present-authenticated, and SHALL NEVER block the
flow based on this check. This closes the deferral previously noted in this
specification's Design Notes ("the `gh` check is deferred to HU-014"); the
check is implemented by HU-014's `push-pr-preparation` capability
(`docs/HISTORIAS.md:952`).

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

## Note For Archive

`openspec/specs/prereq-check/spec.md`'s "Design Notes" section states the
`gh` check "is intentionally deferred to HU-014". That carve-out prose SHOULD
be updated or removed at archive time, since this delta closes the deferral
by adding the `gh` check as a formal Requirement above.
