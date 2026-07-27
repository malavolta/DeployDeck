# Delta for Run Persistence

Source: HU-010 (`docs/HISTORIAS.md:659`) and HU-011 (`docs/HISTORIAS.md:718`) acceptance criteria that require `.deploydeck/runs/`. MINIMAL slice scoped to this change; full lifecycle belongs to HU-013.

## ADDED Requirements

### Requirement: Minimal Run Record On jobId Receipt

The system SHALL create `.deploydeck/runs/<run-id>/` as soon as a validation `jobId` is received, and SHALL hold, at minimum, the `jobId`, the current status, and the raw validate/report JSON for that run.

#### Scenario: jobId receipt creates the run directory
- GIVEN a validation call returns a `jobId`
- WHEN the `jobId` is received
- THEN `.deploydeck/runs/<run-id>/` is created holding the `jobId`, current status, and raw validate JSON

#### Scenario: Report polling updates the persisted status and raw JSON
- GIVEN an existing run directory for a `jobId`
- WHEN a new `deploy report` is polled
- THEN the run's status and raw report JSON are updated on disk

## Out of Scope (Deferred to HU-013)

This slice is a minimal writer only. Full run history browsing/listing, retention/cleanup policy, and resume-by-jobId are explicitly OUT of this change and are owned by HU-013, which extends this writer rather than rewriting it.
