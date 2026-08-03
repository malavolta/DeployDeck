# Delta for Commit Discovery

## MODIFIED Requirements

### Requirement: Candidate Branch Search By Name

The system SHALL find local and remote branches whose name contains the ticket, EXCLUDING branches that match the tool's own promotion-branch shape derived from `config.BranchFormat` (the literal prefix up to its first template token, e.g. `deploy/`), in both bare (`deploy/<ticket>-to-<env>`) and `origin/`-prefixed (`origin/deploy/<ticket>-to-<env>`) form. When a local branch and its remote-tracking counterpart (e.g. `X` and `origin/X`) both match the ticket and represent the same logical branch, the system MUST treat them as a single candidate; genuinely distinct branches (different logical names) MUST NOT be collapsed.
(Previously: candidate search matched any branch containing the ticket with no exclusion, so a leftover `deploy/<ticket>-to-<env>` branch from a prior promotion was returned as a second, spurious source candidate.)

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

#### Scenario: Leftover promotion branch no longer causes a false multi-candidate dead end
- GIVEN a leftover branch `deploy/DEMO-2-to-INT` remains from a prior promotion, and ticket `DEMO-2`'s real source branch also matches
- WHEN the user re-runs discovery for `DEMO-2`
- THEN the source resolves unambiguously with no spurious multi-candidate confirmation prompt
- AND the ticket's pending commits are listed (`OrderedCommits` is non-empty)
