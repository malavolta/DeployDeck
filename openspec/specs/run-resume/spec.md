# Run Resume

## Purpose

HU-013 (`docs/HISTORIAS.md:814-894`) closes the HU-006 (cherry-pick reopen) and HU-011 (resume-by-jobId) deferrals: on startup, after prerequisites pass, DeployDeck detects a resumable run and offers to continue it instead of always starting fresh. There is no literal `StateSuspended` — accepting a resume routes directly into `StateCherryPickConflict` or `StateValidationPolling` with rehydrated `Model` fields, reusing `git.Service.RepoState` and `salesforce.Client.ReportDeploy` (no new exec seams).

## Requirements

### Requirement: Startup Resume Detection After Prerequisites

After prerequisites pass, the system SHALL check for a resumable run: (a) an in-progress cherry-pick (`RepoState.InProgress`) matching a persisted run, or (b) a run with a non-terminal `jobId`. When found, it SHALL offer to resume; declining SHALL proceed to the normal flow. When nothing is resumable, the system SHALL proceed directly to the normal flow without a blocking prompt.

#### Scenario: In-progress cherry-pick offers resume
- GIVEN a persisted run in `CherryPickConflict` and a matching `CHERRY_PICK_HEAD` in the repo
- WHEN DeployDeck starts and prerequisites pass
- THEN the user is offered to resume the conflict screen (HU-013 AC `docs/HISTORIAS.md:846`)

#### Scenario: No resumable run skips the prompt
- GIVEN no in-progress cherry-pick and no run with a non-terminal jobId
- WHEN DeployDeck starts
- THEN the flow proceeds directly to the normal ticket-input screen

#### Scenario: User declines the resume offer
- GIVEN a resumable run is detected
- WHEN the user declines
- THEN the flow proceeds to the normal flow instead of resuming

### Requirement: Accepted Resume Routes Directly Into Conflict Or Polling

Accepting a resume offer SHALL route directly into `StateCherryPickConflict` (rehydrated with ticket, pick index/total, current commit) or `StateValidationPolling` (re-attached to the run's `jobId`), with no intermediate suspended state.

#### Scenario: Accepted cherry-pick resume rehydrates context
- GIVEN the user accepts resume for a conflicted run
- WHEN the conflict screen opens
- THEN it shows the run's ticket and pick N of M (HU-013 AC `docs/HISTORIAS.md:846`)

#### Scenario: Accepted jobId resume re-attaches polling
- GIVEN the user accepts resume for a run with a non-terminal `jobId`
- WHEN resume is accepted
- THEN `deploy report` polling re-attaches for that `jobId` (HU-013 AC `docs/HISTORIAS.md:844`)

### Requirement: Resync When The Repo No Longer Matches The Persisted Record

When a persisted run's phase is `CherryPickConflict` but the repo no longer shows an in-progress cherry-pick (resolved or aborted externally), the system SHALL reconcile the record with the real repo state before continuing, rather than offering a stale conflict screen.

#### Scenario: Externally resolved conflict resyncs instead of offering a stale screen
- GIVEN a persisted run in `CherryPickConflict` but no `CHERRY_PICK_HEAD` is present in the repo
- WHEN DeployDeck starts
- THEN the record is resynced with the real repo state and the stale conflict screen is not offered (HU-013 AC `docs/HISTORIAS.md:847`)
