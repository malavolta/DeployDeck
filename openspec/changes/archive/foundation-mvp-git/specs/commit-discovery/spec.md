# Delta for Commit Discovery

## ADDED Requirements

### Requirement: Ticket-Based Commit Message Search
The system SHALL search commits via `git log --grep <ticket>` and list all commits whose message contains the ticket. (HU-002)

#### Scenario: Ticket found in commit messages
- GIVEN commits exist whose message contains the ticket
- WHEN the user searches by ticket
- THEN the related commits are listed

#### Scenario: Commit mentioning multiple tickets is found for any of them
- GIVEN a commit message mentions two tickets
- WHEN the user searches by either ticket
- THEN that commit appears in the results

### Requirement: Candidate Branch Search By Name
The system SHALL find local and remote branches whose name contains the ticket.

#### Scenario: Ticket found in branch name
- GIVEN branches exist whose name contains the ticket
- WHEN the user searches by ticket
- THEN the candidate branches are listed

### Requirement: Single Source Branch Enforcement
The system SHALL require the user to choose exactly one source branch per run when multiple candidate branches exist; commits from different branches SHALL NOT be mixed in one run.

#### Scenario: Multiple candidates require a single choice
- GIVEN commits exist across more than one candidate branch
- WHEN the user continues past discovery
- THEN the user must select a single source branch for the run

### Requirement: Suggested Default Source For Environment Promotions
For promotions between two configured sandbox environments (e.g. `INT -> UAT`), the system SHALL suggest the previous/validated environment as the default source branch, not the feature branch, while still allowing the user to override the suggestion and always enforcing single-source-branch selection. (RF-002)

#### Scenario: Environment promotion suggests the previous environment as default source
- GIVEN a promotion between two configured sandbox environments
- WHEN source selection is shown
- THEN the previous environment branch is suggested as the default source, not a feature branch

#### Scenario: Suggested default source can be overridden
- GIVEN the previous-environment branch is suggested as the default source
- WHEN the user selects a different candidate branch instead
- THEN the override is accepted and single-source-branch enforcement still applies

### Requirement: Topological Commit Ordering
The system SHALL order discovered commits using `git rev-list --reverse --topo-order origin/<target>..origin/<source>`, not by author or commit date.

#### Scenario: Topological order overrides inverted author dates
- GIVEN seeded commits whose author dates are inverted relative to their topological order
- WHEN results are ordered
- THEN the final order is topological, not chronological

### Requirement: Merge Commit Detection
The system SHALL detect commits with more than one parent and mark them as merge commits, blocked from selection.

#### Scenario: Merge commit is flagged and blocked
- GIVEN a merge commit is among the discovered commits
- WHEN results are rendered
- THEN it is visually identified as a merge commit and appears blocked

### Requirement: Already-Applied Detection By SHA And Content Equivalence
The system SHALL mark a commit as already applied when its SHA is an ancestor of the target (`git merge-base --is-ancestor`), and SHALL mark a commit as equivalent-already-applied when `git cherry` reports it with `-`. Neither case SHALL be selected by default.

#### Scenario: Same-SHA commit marked already applied
- GIVEN a commit present in the target branch by identical SHA
- WHEN it appears in the results
- THEN it is marked already applied and not selected by default

#### Scenario: Cherry-picked-elsewhere commit marked equivalent
- GIVEN a commit whose content was already cherry-picked to the target under a different SHA
- WHEN `git cherry` classifies it
- THEN it is marked equivalent-already-applied and not selected by default

### Requirement: Search Diagnostics And Warnings
The system SHALL warn when the source branch no longer exists and search relies only on message grep, SHALL warn when history suggests squash merges (equivalence undetectable statically), and SHALL show actionable alternatives when a ticket search returns no results.

#### Scenario: Deleted source branch narrows search to grep only
- GIVEN the source branch has been deleted
- WHEN the user searches by ticket
- THEN the system warns that only message-grep results are visible

#### Scenario: Squash-merge history triggers an equivalence warning
- GIVEN the repository history shows squash merges
- WHEN commits are evaluated for equivalence
- THEN a limitation warning is shown

#### Scenario: Ticket with no results shows alternatives
- GIVEN a ticket with no matching commits or branches
- WHEN the search completes
- THEN the TUI offers manual search, changing the ticket, or selecting a source branch directly
