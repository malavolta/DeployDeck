# Delta for deploy-gate

## ADDED Requirements

### Requirement: gh-Unavailable Gate Block Names gh As The Cause

WHEN a gated deploy is evaluated and `gh` is unavailable (absent,
present-unauthenticated, or timed-out for the repository's origin host), the
gate SHALL BLOCK the deploy (fail closed) and the gate-block screen SHALL
report a single condition naming `gh` as the cause (e.g. "gh no disponible"),
distinguishing absent/unauthenticated from could-not-verify. The block
message SHALL NOT suggest that approvals, threads, validation comment, or
signature are missing, because none of them could be evaluated. The system
SHALL NOT provide an override for this block. Gate pass/fail semantics for
available `gh` are unchanged.

#### Scenario: Absent gh blocks naming gh
- GIVEN a target with an enabled gate and `gh` is not installed
- WHEN the typed strong confirmation is given
- THEN the deploy is NOT dispatched and the block screen shows one condition
  naming `gh` as unavailable, with no approvals-missing message

#### Scenario: Unauthenticated gh blocks naming gh
- GIVEN a target with an enabled gate and `gh` is not authenticated for the
  origin host
- WHEN the typed strong confirmation is given
- THEN the deploy is NOT dispatched and the block message names `gh`
  authentication as the cause

#### Scenario: Timed-out gh blocks naming gh as unverifiable
- GIVEN a target with an enabled gate and the `gh` check times out
- WHEN the typed strong confirmation is given
- THEN the deploy is NOT dispatched and the block message says `gh` could not
  be verified, not that approvals are missing

#### Scenario: No override for the gh-unavailable block
- GIVEN the gate-block screen shows the gh-unavailable condition
- WHEN the user looks for a way to proceed anyway
- THEN no in-app override or bypass action is available

#### Scenario: Available gh leaves gate evaluation unchanged
- GIVEN a gated target and `gh` is authenticated for the origin host
- WHEN the gate is evaluated
- THEN conditions are evaluated and listed exactly as before

## MODIFIED Requirements

### Requirement: Gate-Block Screen

WHEN a gated deploy is blocked, the system SHALL show a screen listing
every unmet condition, not only the first one encountered, except that WHEN
the block is caused by `gh` being unavailable the screen SHALL list the single
gh-unavailable condition instead of the individual conditions (see "gh-Unavailable
Gate Block Names gh As The Cause"). The system SHALL NOT provide an in-app
override to bypass a blocked gate.

(Previously: always listed every unmet condition, with no gh-unavailable
case.)

#### Scenario: All unmet conditions are listed together
- GIVEN a gated deploy fails more than one enabled condition and `gh` is available
- WHEN the gate-block screen renders
- THEN every unmet condition is listed, not just the first

#### Scenario: No override exists
- GIVEN the gate-block screen is shown
- WHEN the user looks for a way to proceed anyway
- THEN no in-app override or bypass action is available
