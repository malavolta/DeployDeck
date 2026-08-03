# Delta for Cherry-Pick

## ADDED Requirements

### Requirement: Cherry-Pick Source Provenance Trailer

The system SHALL invoke `git cherry-pick` with the `-x` flag for every applied commit, so each promoted commit's message carries a `(cherry picked from commit <sha>)` trailer identifying its source SHA.

#### Scenario: Promoted commit message carries the source-SHA trailer
- GIVEN a commit is selected and cherry-picked onto the promotion branch
- WHEN the pick completes
- THEN the resulting commit's message contains `(cherry picked from commit <sha>)`, where `<sha>` is the source commit's SHA

#### Scenario: Trailer is present regardless of conflict resolution path
- GIVEN a commit cherry-picks cleanly, or is applied via conflict resolution followed by `--continue`
- WHEN the resulting commit is inspected
- THEN it carries the `-x` provenance trailer in both cases
