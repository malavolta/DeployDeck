# Delta for Deploy Validation

## MODIFIED Requirements

### Requirement: CLI Error Surfaces Message And Raw JSON

The system SHALL show an error message and, when available, the raw JSON returned by the Salesforce CLI when `sf project deploy validate` fails at launch, without breaking the flow. It SHALL also persist that raw envelope as a companion file for the run (mirroring the `run-persistence` writer companion pattern) and surface the persisted file's path alongside the message.
(Previously: showed message + raw JSON but never persisted the raw envelope or surfaced a path to it — HU-010's original acceptance criterion was left unfulfilled.)

#### Scenario: CLI error shows message and raw JSON
- GIVEN the Salesforce CLI returns an error
- WHEN validation is launched
- THEN an error message is shown together with the raw JSON, and the flow continues rather than crashing

#### Scenario: Launch error shows an actionable message and the persisted-raw path
- GIVEN `sf project deploy validate` fails at launch, before any `jobId` is obtained
- WHEN the error is displayed
- THEN it shows an actionable message together with the file-system path of the persisted raw-envelope companion (D7, HU-010 AC)
