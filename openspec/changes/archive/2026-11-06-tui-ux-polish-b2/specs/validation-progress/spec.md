# Validation Progress Delta (tui-ux-polish-b2)

Tag: `[U5]`. Scenarios use a fake `Deps.Now` clock and `Model.Update`/`View` under the Ascii profile.

## ADDED Requirements

### Requirement: [U5] Polling Shows Elapsed Time And Last-Poll Age

While a validation job is polling, the screen MUST show the elapsed time since polling started as `mm:ss` (zero-padded; minutes keep growing past 59) and the age of the last successful report as `hace Ns` (whole seconds). Both values MUST be computed from the injected clock (`Deps.Now`), never from the wall clock. Before the first report has arrived, the last-poll age MUST NOT be shown as a number. Elapsed time and age update on each render driven by the existing tick or report messages and add no new polling requests. Re-attaching to an existing job restarts elapsed from the time of re-attach. Terminal states stop showing the live lines.

#### Scenario: Elapsed is formatted mm:ss
- GIVEN polling started at fake time T and the clock now reads T+65s
- WHEN the view renders
- THEN it shows `01:05`

#### Scenario: Last poll is relative
- GIVEN a report received at fake time T and the clock now reads T+7s
- WHEN the view renders
- THEN it shows `hace 7s`

#### Scenario: Last poll refreshes on a new report
- GIVEN `hace 7s` is shown
- WHEN a new report arrives and the clock reads that instant
- THEN the view shows `hace 0s`

#### Scenario: No report yet
- GIVEN polling just started and no report has arrived
- WHEN the view renders
- THEN elapsed shows `00:00` and no `hace Ns` number is shown

#### Scenario: Long jobs keep counting
- GIVEN the clock reads 61 minutes and 5 seconds after start
- WHEN the view renders
- THEN it shows `61:05`

#### Scenario: Terminal state drops the live lines
- GIVEN polling reaches a terminal state
- WHEN the result renders
- THEN neither the elapsed time nor `hace Ns` is shown

## MODIFIED Requirements

None.

## REMOVED Requirements

None.
