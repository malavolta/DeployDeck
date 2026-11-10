# Delta for prereq-check

## MODIFIED Requirements

### Requirement: `gh` CLI Availability And Authentication Check (Informative)

The system SHALL report `gh` CLI availability and authentication status as an
informative, non-blocking `PrereqCheck`, classifying it as absent,
present-unauthenticated, present-authenticated, or timed-out (could not
verify), and SHALL NEVER block the flow based on this check. The `gh` check
is a warning-class check: it SHALL report `StatusOK` or `StatusWarning` only,
never `StatusBlocking`.

The authentication probe SHALL be scoped to the host of the repository's
`origin` remote (`gh auth status --hostname <host>`), so that the state of
other configured hosts MUST NOT affect the result. WHEN `origin` is missing
or its host cannot be derived, the probe SHALL fall back to a probe without
`--hostname`. The probe SHALL be bounded by a timeout of about 5 seconds. A
deadline-exceeded probe SHALL be classified as timed-out and SHALL NEVER be
reported as unauthenticated.

For every non-authenticated state (absent, unauthenticated, timed-out), the
warning `Detail` SHALL be written in English and SHALL state the
consequences: PR creation falls back to the compare URL, the deploy gate and
the validation comment are unavailable, and push still works. The timed-out
`Detail` SHALL say that authentication could not be verified, not that the
user is unauthenticated.

(Previously: three states with no host scoping, no timeout, and no required
consequence text; one unreachable host could stall the check and flip the
result.)

#### Scenario: gh absent is reported informationally
- GIVEN the `gh` binary is not installed
- WHEN the prerequisite check runs
- THEN an informative `StatusWarning` `PrereqCheck` reports `gh` as absent and
  lists the consequences

#### Scenario: gh present but unauthenticated is reported informationally
- GIVEN `gh` is installed but not authenticated for the origin host
- WHEN the prerequisite check runs
- THEN an informative `StatusWarning` `PrereqCheck` reports `gh` as
  present-unauthenticated and lists the consequences

#### Scenario: gh present and authenticated is reported informationally
- GIVEN `gh` is installed and authenticated for the origin host
- WHEN the prerequisite check runs
- THEN the `PrereqCheck` reports `StatusOK`

#### Scenario: Warning text lists consequences and that push still works
- GIVEN `gh` is absent, unauthenticated, or timed-out
- WHEN the prerequisite check runs
- THEN `Detail` states that PR creation falls back to the compare URL, that
  the deploy gate and validation comment are unavailable, and that push still
  works

#### Scenario: Probe is scoped to the origin host
- GIVEN `origin` is `git@github.com:org/repo.git`
- WHEN the prerequisite check runs
- THEN the probe runs `gh auth status --hostname github.com`

#### Scenario: Unhealthy extra host does not affect the result
- GIVEN `gh` is authenticated for the origin host and another configured host
  (e.g. `github.ibm.com`) is unreachable
- WHEN the prerequisite check runs
- THEN the `PrereqCheck` reports `StatusOK`

#### Scenario: No origin or unrecognized origin falls back to a host-less probe
- GIVEN `origin` is missing or its host cannot be derived
- WHEN the prerequisite check runs
- THEN the probe runs without `--hostname`, still bounded by the timeout

#### Scenario: Probe timeout is reported as could-not-verify
- GIVEN the `gh auth status` probe does not finish within about 5 seconds
- WHEN the prerequisite check runs
- THEN the check completes at about the timeout, reports `StatusWarning` with
  a could-not-verify `Detail`, and does NOT say "not authenticated"

#### Scenario: gh check never blocks the flow
- GIVEN any `gh` state, including absent and timed-out
- WHEN the prerequisite check runs
- THEN the check never reports `StatusBlocking` and the flow is not blocked

## ADDED Requirements

### Requirement: Doctor Screen Loading Indicator

While the prerequisite checks are running and no results are available yet,
the Doctor screen SHALL show the existing spinner together with the text
"Comprobando prerequisitos…", animated by the existing spinner tick loop. The
loading state SHALL reuse the existing footer bindings and SHALL NOT change
the footer. The indicator SHALL disappear as soon as the check results are
shown.

#### Scenario: Spinner shown immediately while checks run
- GIVEN the user opens the Doctor screen and no check results have arrived
- WHEN the screen renders
- THEN the spinner and "Comprobando prerequisitos…" are shown and the spinner
  tick loop is active

#### Scenario: Loading indicator replaced by results
- GIVEN the Doctor screen is showing the loading indicator
- WHEN the check results arrive
- THEN the results list is shown and the spinner tick loop stops

#### Scenario: Footer is unchanged while loading
- GIVEN the Doctor screen is in the loading state
- WHEN the footer renders
- THEN it shows the same bindings as the existing Doctor screen
