# Capability: Deploy Gate

## Overview

Give teams an opt-in, per-environment governance check that DeployDeck
enforces locally before it performs a real deployment (quick-deploy). An
environment opts in by declaring a gate in configuration; once enabled,
DeployDeck verifies the deploy's pull request against up to four
independently toggleable conditions and blocks the deploy if any enabled
condition is unmet. An environment with no gate configured, or a gate left
disabled, deploys exactly as the `quick-deploy` capability already
describes. This is a cooperative local control — see Design Notes for the
trust model.

## Requirements

### Requirement: Per-Environment Gate Configuration

The system SHALL support a `gates` configuration map keyed by branch or glob
pattern, mirroring the existing sandbox-configuration keying convention. The
map SHALL be default-off and zero-value-safe: an environment with no
matching entry has no gate. Each gate entry SHALL declare `enabled`, an
`approvers` list, `minApprovals` (default 1), and three independently
toggleable conditions — `requireResolvedThreads`, `requireValidationComment`,
`requireSignature` — each defaulting to on when the gate is enabled.
Configuration validation SHALL reject an enabled gate whose `minApprovals`
is less than 1 or whose `approvers` list is empty.

#### Scenario: Gated environment resolves its configuration by exact match then glob
- GIVEN a `gates` entry matching a target branch exactly, and a separate glob entry that would also match
- WHEN the gate configuration is resolved for that target
- THEN the exact-match entry is used

#### Scenario: An environment with no matching gate entry is ungated
- GIVEN a target branch with no exact or glob match in `gates`
- WHEN the gate configuration is resolved for that target
- THEN no gate applies to the deploy

#### Scenario: An enabled gate with invalid approval settings is rejected
- GIVEN a `gates` entry with `enabled: true` and either `minApprovals` less than 1 or an empty `approvers` list
- WHEN configuration is validated
- THEN validation fails with an actionable error

### Requirement: Gate Runs Before Quick-Deploy Dispatch

When the deploy's target environment has an enabled gate, the system SHALL
evaluate the gate after the typed strong confirmation and before the real
deploy is dispatched. The deploy SHALL proceed only if every condition
enabled on that gate passes. A target with no enabled gate SHALL deploy
unchanged (see the `quick-deploy` capability).

#### Scenario: A gated deploy proceeds when all enabled conditions pass
- GIVEN a target environment with an enabled gate whose enabled conditions all currently pass
- WHEN the typed strong confirmation is given
- THEN the deploy is dispatched

#### Scenario: A gated deploy is blocked when any enabled condition fails
- GIVEN a target environment with an enabled gate where at least one enabled condition currently fails
- WHEN the typed strong confirmation is given
- THEN the deploy is NOT dispatched

### Requirement: Approval Condition

The system SHALL count a PR as meeting the approval condition only when at
least `minApprovals` distinct approver-list logins have `APPROVED` as their
LATEST review state. Login comparison SHALL be normalized: a leading `@` is
stripped and comparison is case-insensitive. Approvals from logins not on
the list SHALL NOT count toward `minApprovals`. A later `CHANGES_REQUESTED`
review from a listed login SHALL negate that login's earlier approval for
this condition.

#### Scenario: Enough listed approvals satisfy the condition
- GIVEN a gate with `minApprovals: 2` and an approver list, and at least 2 distinct listed logins whose latest review is `APPROVED`
- WHEN the approval condition is evaluated
- THEN it passes

#### Scenario: Approvals from non-listed users do not count
- GIVEN a gate with `minApprovals: 1` and an approver list, and only non-listed logins have approved
- WHEN the approval condition is evaluated
- THEN it fails

#### Scenario: A listed reviewer's later changes-requested negates their approval
- GIVEN a listed login whose review history is `APPROVED` then later `CHANGES_REQUESTED`, with no other qualifying approvals
- WHEN the approval condition is evaluated
- THEN it fails, because that login's latest state is `CHANGES_REQUESTED`

### Requirement: Unresolved-Threads Condition

When enabled, the system SHALL require that no unresolved review thread
remains on the PR. Thread-resolution status SHALL be read from GitHub. WHEN
thread-resolution status cannot be determined (a lookup or authentication
failure), the condition SHALL FAIL — it SHALL NOT be treated as passed or
as unknown-but-ignorable.

#### Scenario: All threads resolved passes the condition
- GIVEN every review thread on the PR is resolved
- WHEN the unresolved-threads condition is evaluated
- THEN it passes

#### Scenario: One unresolved thread blocks the condition
- GIVEN at least one review thread on the PR is unresolved
- WHEN the unresolved-threads condition is evaluated
- THEN it fails

#### Scenario: Thread-resolution status unavailable fails closed
- GIVEN thread-resolution status cannot be determined due to a lookup or authentication failure
- WHEN the unresolved-threads condition is evaluated
- THEN it fails

#### Scenario: More than 100 review threads fails closed
- GIVEN the PR has more than 100 review threads, so the first page of results is truncated and thread-resolution status is undeterminable beyond it
- WHEN the unresolved-threads condition is evaluated
- THEN it fails, the same as any other lookup failure

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

### Requirement: Signature Condition

The system SHALL pass the signature condition only when the PR body carries
a valid provenance signature, verified using the same verification
semantics as PR-provenance verification.

#### Scenario: A valid signature passes
- GIVEN the PR body carries a marker that verifies successfully
- WHEN the signature condition is evaluated
- THEN it passes

#### Scenario: A missing or invalid signature fails
- GIVEN the PR body carries no marker, or a marker whose signature does not match
- WHEN the signature condition is evaluated
- THEN it fails

#### Scenario: A dev-signed marker fails
- GIVEN the PR body carries a dev-signed marker
- WHEN the signature condition is evaluated
- THEN it fails, because a dev-signed marker is unverifiable, not valid

### Requirement: Fail-Closed PR Resolution

To evaluate a gate, the system SHALL first resolve the run's PR using its
recorded PR URL; when no PR URL is recorded, it SHALL fall back to locating
the PR by the run's branch. WHEN neither path resolves a PR, the gate SHALL
BLOCK the deploy with a clear message rather than skip evaluation.

#### Scenario: A recorded PR URL resolves the PR
- GIVEN the run has a recorded PR URL
- WHEN the gate resolves the PR
- THEN that PR is used for condition evaluation

#### Scenario: An empty PR URL resolves via the branch fallback
- GIVEN the run has no recorded PR URL, but a PR exists for the run's branch
- WHEN the gate resolves the PR
- THEN the branch-derived PR is used for condition evaluation

#### Scenario: No resolvable PR blocks the deploy
- GIVEN the run has no recorded PR URL and no PR is found for the run's branch
- WHEN the gate attempts to resolve the PR
- THEN the deploy is blocked with a clear message, and no condition is evaluated as passed

### Requirement: Gate-Block Screen

WHEN a gated deploy is blocked, the system SHALL show a screen listing
every unmet condition, not only the first one encountered. The system
SHALL NOT provide an in-app override to bypass a blocked gate.

#### Scenario: All unmet conditions are listed together
- GIVEN a gated deploy fails more than one enabled condition
- WHEN the gate-block screen renders
- THEN every unmet condition is listed, not just the first

#### Scenario: No override exists
- GIVEN the gate-block screen is shown
- WHEN the user looks for a way to proceed anyway
- THEN no in-app override or bypass action is available

## Design Notes

- Trust model: this is a COOPERATIVE local control, the same honesty class
  as PR provenance — it deters casual bypass and enforces team discipline
  but is bypassable by editing configuration or the binary. It is NOT a
  hard security boundary; the hard equivalent (GitHub branch protection
  plus a required CI check) is explicitly out of scope.
- Team/org-membership expansion of `approvers` is out of scope: the
  approver list is a literal set of logins, not a group reference.
- Aggregate coverage in the validation comment is a computed figure (sum
  of covered / sum of total across per-class data), not a value stored
  elsewhere.
- The signature condition reuses PR-provenance's existing verification
  outcome classification; it does not introduce a new signing scheme.
