# Delta for Validation Progress

## ADDED Requirements

### Requirement: Terminal Result Screen Shows The Failure Headline

On a terminal Failed or Canceled result the screen MUST show the failure headline (same text as history and detail) above the counters. While the job is polling (`Queued`/`InProgress`/`Pending`) no headline MUST be shown. On timeout the existing timeout line (`Timeout: la validación no alcanzó un estado terminal a tiempo.`) MUST remain; the headline is derived from the last known report if one exists, otherwise omitted; the status is shown as-is.

#### Scenario: Failed coverage run shows headline
- GIVEN polling reaches `Failed` with org-wide coverage 66% < 75%
- WHEN the result renders
- THEN `Falló por cobertura: 66% global < 75%` appears above the counters

#### Scenario: Polling has no headline
- GIVEN the last report status is `InProgress`
- WHEN the screen renders
- THEN no failure headline is shown

#### Scenario: Timeout with a last report
- GIVEN polling timed out while the last report was non-terminal (a timeout only occurs while the status is non-terminal)
- WHEN the result renders
- THEN the existing timeout line appears and no failure headline is shown

#### Scenario: Timeout without any report
- GIVEN polling timed out before any report arrived
- WHEN the result renders
- THEN the timeout line appears and no headline
