# Capability: Incremental Promotion

## Overview

When the promotion branch for a ticket/target pair already exists (`deploy/<ticket>-to-<target>`), incremental promotion lets the user reuse it and append only the commits not already applied, updating the same run and its open PR, instead of forcing a delete-and-recreate that discards review history.

## Requirements

### Requirement: Branch Collision Offers Reuse, Recreate, Or Cancel

When branch creation fails because the target branch name already exists (locally or on `origin`), the system SHALL enter a dedicated collision decision and SHALL offer exactly three explicit choices: reuse & append, delete & recreate, or cancel. The system SHALL NOT dead-end into an unrecoverable error state on this collision.

#### Scenario: Collision presents all three choices
- GIVEN branch creation fails because the branch name already exists
- WHEN the collision decision is entered
- THEN reuse & append, delete & recreate, and cancel are all offered

#### Scenario: Cancel leaves the existing branch and PR untouched
- GIVEN the collision decision is shown
- WHEN the user selects cancel
- THEN neither the existing branch nor any of its PR history is modified

#### Scenario: Delete & recreate replaces the branch fresh
- GIVEN the collision decision is shown
- WHEN the user selects delete & recreate
- THEN the existing branch is deleted and a fresh branch is created from `origin/<target>`, discarding its prior commit history and PR linkage

### Requirement: Layered Already-On-Branch Detection

On reuse, before cherry-picking, the system SHALL classify each selected commit as already present on the deploy branch or not, using an ordered layered check: ancestry (`IsAncestor`) first, then the `-x` provenance trailer (`(cherry picked from commit <sha>)`) as the primary re-application signal, then cherry-equivalence (`git cherry`) as a fallback when a source ref is available. The trailer is primary because it is exact and survives conflict-resolution edits that break patch-id equivalence; the cherry-equivalence layer backstops content reproduced on the deploy branch without a trailer. The trailer's other role — provenance/traceability recorded on every `-x` cherry-pick — is unaffected by this ordering.

#### Scenario: Commit already merged by ancestry is detected
- GIVEN a selected commit is an ancestor of the deploy branch's tip
- WHEN already-on-branch detection runs
- THEN the commit is classified as already present

#### Scenario: Commit reapplied under a different SHA is detected via its trailer
- GIVEN a selected commit's SHA appears in a `(cherry picked from commit <sha>)` trailer on the deploy branch, though its resulting SHA differs
- WHEN already-on-branch detection runs
- THEN the commit is classified as already present via the trailer

#### Scenario: Content-equivalent commit without a trailer is detected via cherry-equivalence
- GIVEN a selected commit's content is equivalent (same patch) to a commit already on the deploy branch but carries no `-x` trailer
- WHEN already-on-branch detection runs with a source ref available
- THEN the commit is classified as already present via `git cherry`

### Requirement: Not-Yet-Present Commits Filtered Before Cherry-Pick

The system SHALL cherry-pick only the subset of selected commits classified as not-yet-present on the deploy branch; commits already present SHALL be excluded from the pick sequence.

#### Scenario: Only new commits are cherry-picked
- GIVEN a selection of 5 commits where 2 are already present on the deploy branch
- WHEN the reused branch's cherry-pick sequence runs
- THEN only the remaining 3 not-yet-present commits are applied

### Requirement: Empty-After-Filter Shows An Explicit Notice

When every selected commit is classified as already present on the deploy branch, the system SHALL show an explicit notice stating that nothing new remains to apply, and SHALL NOT surface a raw cherry-pick error.

#### Scenario: All selected commits already present shows a clear notice
- GIVEN every selected commit is already present on the reused deploy branch
- WHEN the reuse flow evaluates the filtered selection
- THEN an explicit "nothing new to apply" notice is shown instead of a raw git error

### Requirement: Diverged Remote Deploy Branch Blocks With A Clear Error

On reuse, the system SHALL fetch and fast-forward the local deploy branch to `origin/<deploy-branch>` when the remote is strictly ahead. When the remote deploy branch has diverged (non-fast-forwardable), the system SHALL surface a clear error and SHALL NOT overwrite the remote branch silently.

#### Scenario: Remote strictly ahead is fast-forwarded
- GIVEN `origin/<deploy-branch>` is strictly ahead of the local reused branch
- WHEN reuse fetches remote state
- THEN the local branch is fast-forwarded to match `origin/<deploy-branch>` before cherry-picking resumes

#### Scenario: Diverged remote surfaces a clear error
- GIVEN `origin/<deploy-branch>` has commits not reachable from the local reused branch and vice versa
- WHEN reuse fetches remote state
- THEN a clear divergence error is shown and no push or overwrite is attempted

### Requirement: Reuse Targets The Prior Run For The Same Branch

On reuse, the system SHALL identify the prior run record associated with the deploy branch name and SHALL route the increment to that run rather than starting an unrelated new run.

#### Scenario: Reuse locates the matching prior run
- GIVEN a prior run exists whose rendered branch name matches the deploy branch being reused
- WHEN reuse is confirmed
- THEN that prior run is identified as the target for the increment

#### Scenario: No prior run found is treated as a fresh increment target
- GIVEN no prior run record renders to the deploy branch name being reused
- WHEN reuse is confirmed
- THEN the flow proceeds without a matching prior run to increment (no error)

## Design Notes

- Detection order is ancestry → `-x` trailer (primary) → cherry-equivalence (fallback): the trailer is exact and survives conflict-resolution edits that break patch-id equivalence, so it is the primary re-application signal; cherry-equivalence backstops content reproduced without a trailer when a source ref is available.
  - Known limitation: the trailer records that a `-x` cherry-pick happened, not that its content still survives on the current tip. If a promoted commit is later reverted or rewritten IN PLACE on the deploy branch, the trailer keeps it classified as already-present and it will not be re-applied on reuse. This is an accepted MVP tradeoff — making cherry authoritative instead would re-pick every conflict-resolution-edited commit (whose patch-id no longer matches its source) and drop the user into a spurious re-conflict on the common path, a worse failure mode. The escape hatch for the reverted-in-place case is delete & recreate. (Empirically, a plain `git revert`-on-top does not even change `git cherry`'s classification — the original commit's patch-id stays in the deploy branch's set — so cherry-authoritative would not fix that case either.)
- `CherryPick`, its conflict machinery, and `Checkout` are unchanged; filtering happens before invocation, at the app layer.
