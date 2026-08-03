# Capability: Commit Discovery

## Overview

Commit discovery finds relevant commits for a given ticket by searching Git history and branch names, then classifies them for selection. It applies ticket-based message search, branch-name search, and equivalence detection to surface all potentially relevant commits in topological order.

## Requirements

### Requirement: Ticket-Based Commit Message Search

The system SHALL search commits via `git log --grep <ticket>` and list all commits whose message contains the ticket.

#### Scenario: Ticket found in commit messages
- GIVEN commits exist whose message contains the ticket
- WHEN the user searches by ticket
- THEN the related commits are listed

#### Scenario: Commit mentioning multiple tickets is found for any of them
- GIVEN a commit message mentions two tickets
- WHEN the user searches by either ticket
- THEN that commit appears in the results

### Requirement: Candidate Branch Search By Name

The system SHALL find local and remote branches whose name contains the ticket, EXCLUDING branches that match the tool's own promotion-branch SHAPE — the full `config.BranchFormat` rendered with its template tokens (`{{ticket}}`/`{{target}}`) as branch-segment wildcards, e.g. `deploy/<ticket>-to-<target>` — in both bare and `origin/`-prefixed form. A branch that merely shares the format's prefix but not its full shape (e.g. `deploy/DEMO-2` without the `-to-<target>` segment) is a legitimate source and MUST NOT be excluded. When a local branch and its remote-tracking counterpart (e.g. `X` and `origin/X`) both match the ticket and represent the same logical branch, the system MUST treat them as a single candidate; genuinely distinct branches (different logical names) MUST NOT be collapsed.
(Previously: candidate search matched any branch containing the ticket with no exclusion, so a leftover `deploy/<ticket>-to-<target>` branch from a prior promotion was returned as a second, spurious source candidate.)

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

#### Scenario: Tool's own promotion branch is excluded, bare and origin/-prefixed
- GIVEN a leftover promotion branch `deploy/DEMO-2-to-INT` exists locally and/or as `origin/deploy/DEMO-2-to-INT`, alongside the ticket's real feature branch
- WHEN the user searches by ticket `DEMO-2`
- THEN both the bare and `origin/`-prefixed forms of `deploy/DEMO-2-to-INT` are excluded from candidates
- AND only the genuine feature branch remains a candidate

#### Scenario: Prefix-only branch is kept as a legitimate source
- GIVEN a branch `deploy/DEMO-2` (matches the format prefix but lacks the `-to-<target>` shape) matches ticket `DEMO-2`
- WHEN the user searches by ticket
- THEN it is NOT excluded and remains a selectable source candidate

#### Scenario: Leftover promotion branch no longer causes a false multi-candidate dead end
- GIVEN a leftover branch `deploy/DEMO-2-to-INT` remains from a prior promotion, and ticket `DEMO-2`'s real source branch also matches
- WHEN the user re-runs discovery for `DEMO-2`
- THEN the source resolves unambiguously with no spurious multi-candidate confirmation prompt
- AND the ticket's pending commits are listed (`OrderedCommits` is non-empty)

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

### Requirement: Single Source Branch Enforcement

The system SHALL require the user to choose exactly one source branch per run when multiple candidate branches exist; commits from different branches SHALL NOT be mixed in one run.

#### Scenario: Multiple candidates require a single choice
- GIVEN commits exist across more than one candidate branch
- WHEN the user continues past discovery
- THEN the user must select a single source branch for the run

### Requirement: Suggested Default Source For Environment Promotions

For promotions between two configured sandbox environments (e.g. `INT -> UAT`), the system SHALL suggest the previous/validated environment as the default source branch, not the feature branch, while still allowing the user to override the suggestion and always enforcing single-source-branch selection.

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

### Requirement: Known Source For Re-Promotion Bypasses Suggested Default Source

For a re-promotion (HU-016), the system SHALL use `origin/<rec.Target>` —
the prior run's own environment branch — as the discovery source directly,
bypassing the `Suggested Default Source For Environment Promotions` path,
since the source is already known from the selected prior run. The existing
`Suggested Default Source For Environment Promotions` requirement (RF-002)
remains unchanged for non-re-promotion promotions.

#### Scenario: Re-promotion source is the prior run's environment branch
- GIVEN a re-promotion started from a prior run targeting `<rec.Target>`
- WHEN commit discovery begins
- THEN the source is `origin/<rec.Target>`, and `SuggestDefaultSource` is not consulted (HU-016)

### Requirement: Next-Environment Branch Suggested As Default Target

The system SHALL provide `NextEnvironmentBranch`, mirroring
`SuggestDefaultSource`'s use of the configured environment pipeline order
but in the forward direction: given the prior run's environment, it SHALL
suggest the next environment in the pipeline as the default target on
`StateTargetSelection`, still user-overridable. When the prior environment
has no next environment in the pipeline, or falls outside the fixed
pipeline order (e.g. a `Release/*` glob target), the system SHALL NOT
auto-suggest a default and SHALL require the user to pick manually.

#### Scenario: Next environment is suggested as default target
- GIVEN a re-promotion from a prior run whose environment has a configured next environment in the pipeline
- WHEN target selection is shown
- THEN the next environment branch is suggested as the default target

#### Scenario: Suggested next-environment default can be overridden
- GIVEN the next-environment branch is suggested as the default target
- WHEN the user selects a different target instead
- THEN the override is accepted

#### Scenario: No next environment falls back to manual target choice
- GIVEN the prior run's environment is last in the pipeline order or matches a `Release/*` glob
- WHEN target selection is shown
- THEN no default target is auto-suggested and the user picks manually

## Design Notes

- The equivalence detection uses `git cherry` and `git cherry-pick --help` documented semantics (`-` indicates a patch with the same result already applied).
- For environment-to-environment promotions, the "previous environment" is determined by the promotion direction defined in configuration.
- For re-promotion discovery source, the prior run's target environment is used directly (no default suggestion fallback); when the prior environment has no next environment in the pipeline, the re-promotion degrades to manual flow.
