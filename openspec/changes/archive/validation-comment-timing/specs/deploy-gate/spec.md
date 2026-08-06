# Delta for Deploy Gate

## MODIFIED Requirements

### Requirement: Validation-Comment Condition And Posting

After a successful CheckOnly validation completes on an environment whose
gate is enabled, the system SHALL post or update a comment on the run's PR
carrying a hidden dedup marker keyed to the validation job, plus
human-readable detail: job id, component and test error counts, and an
aggregate coverage figure computed from the report's per-class coverage
data — posted as soon as the run's PR exists. The comment SHALL be posted
from whichever of the following occurs first with the PR present: (a)
validation success, when the run's PR already exists at that point (e.g.
the reuse/incremental flow updating an already-open PR), or (b) PR
creation, when a terminal-successful validation already completed this
session before the PR existed (the normal validate-then-push-then-create-PR
flow). Re-validation of the same job, and re-evaluation at PR creation after
the comment already exists, SHALL update or skip the existing comment
rather than posting a duplicate — the hidden dedup marker keeps posting
exactly-once regardless of which of the two points actually posts it. The
validation-comment condition SHALL pass only when a marker comment is
present on the PR, and SHALL fail otherwise. On an environment with no
enabled gate, no comment SHALL be posted.
(Previously: the comment was posted/updated only at validation success,
which left it permanently unreachable whenever validation completed before
the PR existed — the normal flow, since the PR is created afterward via a
separate, explicit action.)

#### Scenario: Successful validation on a gated environment posts the marker comment
- GIVEN a validation job completes successfully on an environment with an enabled gate
- WHEN the terminal successful state is processed
- THEN a comment carrying the hidden dedup marker and human-readable detail is posted to the run's PR

#### Scenario: Re-validation does not duplicate the comment
- GIVEN a marker comment already exists on the PR for a job that is re-validated
- WHEN the re-validation completes successfully
- THEN the existing comment is updated or left as-is, and no duplicate comment is created

#### Scenario: The condition passes only when the marker comment is present
- GIVEN a PR with no marker comment
- WHEN the validation-comment condition is evaluated
- THEN it fails; it passes only once a marker comment exists on the PR

#### Scenario: Validation on an ungated environment posts no comment
- GIVEN a validation job completes successfully on an environment with no enabled gate
- WHEN the terminal successful state is processed
- THEN no comment is posted to the PR

#### Scenario: Validation completes before the PR exists, comment posts at PR creation
- GIVEN a validation job completes successfully on a gate-enabled environment, and the run's PR does not yet exist at that point
- WHEN the run's PR is subsequently created
- THEN the marker comment carrying the human-readable detail is posted to the PR at PR-creation time, since that is the first point where the PR is present

#### Scenario: Comment is posted exactly once across both trigger points (no duplicate)
- GIVEN a validation job completes successfully on a gate-enabled environment and the run's PR is created afterward
- WHEN the validation-success point runs (no PR yet, no post) and the PR-creation point subsequently runs (PR present, marker comment posted)
- THEN exactly one marker comment exists on the PR, and neither trigger point posts a second one
