# Delta for Run Retention

## ADDED Requirements

### Requirement: Non-Terminal Or Resumable Runs Are Never Pruned

Deferred follow-up from HU-017's adversarial review (`docs/HISTORIAS.md:814-894`). This requirement EXTENDS — it does NOT replace — the existing count/age rule ("A Run Is Kept If Recent-By-Count OR Recent-By-Age").

A run SHALL be protected from pruning, regardless of the count/age selection outcome, when it is NON-TERMINAL / still resumable: it has a `jobId` and a non-terminal `Status` (a validation in flight), OR it is mid-cherry-pick (an unfinished `Phase`). Retention selection SHALL skip a protected run even when it falls outside both `keepLast` and `keepDays`. A terminal run outside the window SHALL still be pruned as before.

#### Scenario: Non-terminal in-flight run outside the window is kept
- GIVEN a run has a `jobId` and a non-terminal `Status`
- AND it falls outside both `keepLast` and `keepDays`
- WHEN retention selection runs
- THEN the run is kept, not selected for pruning

#### Scenario: Unfinished cherry-pick run outside the window is kept
- GIVEN a run is mid-cherry-pick (an unfinished `Phase`)
- AND it falls outside both `keepLast` and `keepDays`
- WHEN retention selection runs
- THEN the run is kept, not selected for pruning

#### Scenario: Terminal old run outside the window is still pruned
- GIVEN a run has a terminal `Status` and no unfinished `Phase`
- AND it falls outside both `keepLast` and `keepDays`
- WHEN retention selection runs
- THEN the run is selected for pruning (existing behavior preserved)

#### Scenario: Recent or count-kept runs remain kept as before
- GIVEN a run is within the most-recent `keepLast` runs by `CreatedAt`, or its age is `<= keepDays`
- WHEN retention selection runs
- THEN the run is kept, regardless of its terminal/non-terminal status
