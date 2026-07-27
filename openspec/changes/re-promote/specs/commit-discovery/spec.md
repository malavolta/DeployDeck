# Delta for Commit Discovery

## ADDED Requirements

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
