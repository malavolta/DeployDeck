# Run Retention

## Purpose

HU-013 (`docs/HISTORIAS.md:814-894`) bounds `.deploydeck/runs/` growth with a configurable retention policy (default: last 30 runs or 90 days) and a CLI command to apply it. HU-013 owns this policy fully; other consumers (e.g. a future HU-017 branch cleanup) MUST reuse this config and mechanism rather than reimplementing retention selection.

## Requirements

### Requirement: A Run Is Kept If Recent-By-Count OR Recent-By-Age

A run SHALL be kept if it is within the most-recent `keepLast` runs by `CreatedAt`, OR its age is `<= keepDays`. A run SHALL be pruned only if it is outside BOTH conditions. This selection SHALL be implemented as a pure function over `([]Record, keepLast, keepDays, now)`.

#### Scenario: Kept by recency-count despite being older than keepDays
- GIVEN a run is among the most-recent `keepLast` runs by `CreatedAt` but older than `keepDays`
- WHEN retention selection runs
- THEN the run is kept

#### Scenario: Kept by age despite being outside the most-recent keepLast
- GIVEN a run is outside the most-recent `keepLast` runs but its age is `<= keepDays`
- WHEN retention selection runs
- THEN the run is kept

#### Scenario: Pruned when outside both conditions
- GIVEN a run is outside the most-recent `keepLast` runs AND older than `keepDays`
- WHEN retention selection runs
- THEN the run is selected for pruning

### Requirement: `deploydeck runs prune` Applies Retention

The system SHALL provide a `deploydeck runs prune` CLI command that applies the configured `keepLast`/`keepDays` retention to `.deploydeck/runs/`.

#### Scenario: Running the command prunes only outside-window runs
- GIVEN a runs directory with more than `keepLast` runs, some older than `keepDays`
- WHEN `deploydeck runs prune` runs
- THEN only runs outside both `keepLast` and `keepDays` are removed (HU-013 E2E `docs/HISTORIAS.md:892`)

### Requirement: KeepLast/KeepDays Config Bounds Are Validated

Config validation SHALL reject negative `runs.keepLast` or `runs.keepDays` values.

#### Scenario: Negative keepLast is rejected
- GIVEN a config with `runs.keepLast` set to a negative number
- WHEN config validation runs
- THEN validation fails with an error identifying the field

#### Scenario: Negative keepDays is rejected
- GIVEN a config with `runs.keepDays` set to a negative number
- WHEN config validation runs
- THEN validation fails with an error identifying the field
