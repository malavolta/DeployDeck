# Delta for Prerequisite Check

## ADDED Requirements

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
