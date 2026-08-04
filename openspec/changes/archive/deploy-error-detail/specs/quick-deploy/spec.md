# Delta for Quick Deploy

## ADDED Requirements

### Requirement: Failed Quick Deploy Persists Raw Response And Shows An Actionable Message

When `sf project deploy quick` fails during opt-in execution, the system SHALL show an actionable error message and SHALL persist the raw failure response as a companion file for the run, mirroring the cancel-failure persistence pattern.

#### Scenario: Quick deploy failure shows an actionable message
- GIVEN opt-in execution is authorized (`AllowExecution` true, target permitted, typed strong confirmation given)
- WHEN `sf project deploy quick` fails
- THEN an actionable error message is shown to the user

#### Scenario: Failed quick deploy persists the raw response
- GIVEN the quick-deploy failure above
- WHEN it is processed
- THEN the raw failure response is persisted as a companion file for the run
