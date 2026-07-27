# Delta for Validation Progress

Closes the HU-011 resume-by-jobId gap noted in the main spec ("exit `q` leaves job active/resumable", `validation-progress/spec.md:75`). At archive time this note stands as satisfied, not deferred. The existing `User Exit Leaves The Job Active And Resumable` requirement is UNCHANGED and kept as-is by this delta.

## ADDED Requirements

### Requirement: Re-Attach Polling To A Non-Terminal jobId On A Later Launch

When `run-resume` offers and the user accepts resume for a run whose persisted `jobId` has not reached a terminal state, the system SHALL re-attach `deploy report` polling for that `jobId` at the configured interval, reusing the same polling, terminal-state-detection, and timeout requirements as a freshly started validation.

#### Scenario: Resumed run re-attaches to deploy report polling
- GIVEN a persisted run with a `jobId` not in `{Succeeded, SucceededPartial, Failed, Canceled}`
- WHEN the user accepts the resume offer
- THEN `deploy report` polling re-attaches for that `jobId` at the configured interval (HU-013 AC `docs/HISTORIAS.md:844`)

#### Scenario: Resumed job already terminal is not offered for polling resume
- GIVEN a persisted run whose `jobId` already reached a terminal state
- WHEN startup resume-detection runs
- THEN that run is not offered for polling re-attach (it remains browsable in history but is not resumable via polling)
