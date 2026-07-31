# Delta for Commit Discovery

## MODIFIED Requirements

### Requirement: Candidate Branch Search By Name

The system SHALL find local and remote branches whose name contains the ticket. When a local branch and its remote-tracking counterpart (e.g. `X` and `origin/X`) both match the ticket and represent the same logical branch, the system MUST treat them as a single candidate; genuinely distinct branches (different logical names) MUST NOT be collapsed.
(Previously: local and remote-tracking branches of the same logical name were listed as separate, undeduplicated candidates, causing a single pushed feature branch to be counted twice.)

#### Scenario: Ticket found in branch name
- GIVEN branches exist whose name contains the ticket
- WHEN the user searches by ticket
- THEN the candidate branches are listed

#### Scenario: Pushed feature branch resolves to a single candidate
- GIVEN a feature branch for the ticket exists both locally and as its remote-tracking counterpart (`origin/<branch>`)
- WHEN the user promotes to the first pipeline environment (no pipeline-order default applies)
- THEN discovery yields exactly one candidate for that branch
- AND the ticket's commits are discovered (`OrderedCommits` is non-empty), not a dead end

#### Scenario: Genuinely distinct branches are not collapsed
- GIVEN two different branches (different logical names, e.g. `feature/TICKET-a` and `hotfix/TICKET-b`) both match the ticket
- WHEN the user searches by ticket
- THEN both remain separate candidates and single-source selection still applies

## ADDED Requirements

### Requirement: Current-Branch Source Confirmation

WHEN no pipeline-order default source applies (per `Suggested Default Source For Environment Promotions` or `Known Source For Re-Promotion Bypasses Suggested Default Source`) AND the user's current branch matches one of the candidate branches, the system SHOULD offer that branch as the source via an explicit confirmation prompt (e.g. "use current branch `<X>` as source?"). Accepting the prompt uses that branch as the selected source; declining leaves the source unresolved among the remaining candidates. A configured pipeline default source (environment-to-environment suggestion or re-promotion's known source) MUST take priority over this prompt and, when present, the prompt MUST NOT be shown.

#### Scenario: User confirms the current branch as source
- GIVEN no pipeline default source applies and the user is checked out on one of the candidate branches
- WHEN discovery reaches source resolution
- THEN the system prompts to confirm using the current branch as the source
- AND WHEN the user accepts
- THEN that branch is used as the source and its commits are discovered

#### Scenario: User declines the current-branch suggestion
- GIVEN the current-branch confirmation prompt is shown
- WHEN the user declines
- THEN the current branch is not used as the source and selection proceeds among the remaining candidates without it being auto-applied

#### Scenario: No prompt when current branch is not eligible
- GIVEN the user is in a detached HEAD state, or the current branch does not match any candidate branch
- WHEN discovery reaches source resolution
- THEN no current-branch confirmation prompt is shown, and behavior is unchanged from before this requirement

#### Scenario: Pipeline default source takes priority over the prompt
- GIVEN a pipeline-order default source applies (environment-to-environment suggestion or a re-promotion's known source)
- WHEN discovery reaches source resolution
- THEN the pipeline default source is used and the current-branch confirmation prompt is not shown, even if the current branch also matches a candidate
