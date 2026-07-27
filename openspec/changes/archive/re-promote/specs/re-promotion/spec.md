# Re-Promotion Specification

## Purpose

HU-016 (`docs/HISTORIAS.md:1038-1077`) turns run history from a passive log
into the source of the next promotion. From `StateRunHistory`, re-promoting a
ticket that already succeeded in one environment pre-seeds a new promotion
run toward the next environment: known source, patch-id-mapped commits,
explicit gaps, and a `SourceRunID` link. The pre-seeded run then proceeds
through the unmodified promotion states (target selection, plan preview,
branch creation, cherry-pick, delta, validate) — this spec covers only the
pre-seed behavior, not those downstream states.

## Requirements

### Requirement: Re-Promote Eligibility Gate

The system SHALL offer re-promotion for a `StateRunHistory` row only when the
prior run's status is `Succeeded` or `SucceededPartial`. Rows with `Failed`,
`Canceled`, or `Aborted` status SHALL NOT offer re-promotion.

#### Scenario: Successful prior run offers reuse
- GIVEN a prior run for a ticket completed with status `Succeeded` or `SucceededPartial`
- WHEN a promotion is started from that run's history row
- THEN the system offers to reuse its commits (HU-016 AC `docs/HISTORIAS.md:1062`)

#### Scenario: Failed prior run offers no pre-load
- GIVEN a prior run for a ticket completed with status `Failed`
- WHEN the user views that run's history row
- THEN no re-promotion or commit pre-load is offered (HU-016 `docs/HISTORIAS.md:1076`)

### Requirement: Re-Promotion Source Is The Prior Run's Environment Branch

The system SHALL use `origin/<rec.Target>` — the prior run's own environment
branch, not its temporary `PromotionBranch` — as the discovery source for a
re-promotion.

#### Scenario: Source resolves to the prior run's environment branch
- GIVEN a re-promotion started from a prior run with target `<rec.Target>`
- WHEN discovery begins
- THEN the source branch is `origin/<rec.Target>`

### Requirement: Prior-Run Commits Pre-Loaded And Editable

When the user accepts reuse, the system SHALL pre-load the commit selection
with the prior run's commits and SHALL keep the selection editable.

#### Scenario: Accepted reuse pre-loads an editable selection
- GIVEN the user accepts reusing a prior run's N commits
- WHEN the commit selection screen is shown
- THEN the N commits are pre-checked and remain editable (HU-016 AC `docs/HISTORIAS.md:1063`)

#### Scenario: User edits the pre-loaded selection before continuing
- GIVEN a pre-loaded, editable commit selection
- WHEN the user toggles a commit's selection
- THEN the change is accepted and the flow continues with the edited selection (HU-016 `docs/HISTORIAS.md:1076`)

### Requirement: Patch-ID Remap Of Prior Commits

The system SHALL map each prior-run commit SHA to its patch-id equivalent in
the new discovery range, and SHALL pre-select the mapped equivalent when
found.

#### Scenario: Equivalent SHA is mapped and pre-checked
- GIVEN a prior-run commit whose content exists under a different SHA in the new origin range
- WHEN the patch-id remap runs
- THEN the new-range equivalent is identified and pre-checked in the selection (HU-016 AC `docs/HISTORIAS.md:1063`)

### Requirement: Missing Commit Warned Explicitly

The system SHALL warn explicitly, and SHALL NOT silently omit, a prior-run
commit that has no patch-id equivalent in the new origin range.

#### Scenario: Commit absent from the new origin is warned, not dropped
- GIVEN a prior-run commit with no patch-id equivalent in the new origin range
- WHEN the patch-id remap runs
- THEN the system shows an explicit warning naming the missing commit rather than omitting it silently (HU-016 AC `docs/HISTORIAS.md:1064`)

### Requirement: SourceRunID Linkage On Completion

Upon completion of a re-promotion, the new run SHALL record `SourceRunID`
referencing the prior run it was re-promoted from.

#### Scenario: Completed re-promotion records SourceRunID
- GIVEN a re-promotion run reaches completion
- WHEN its record is persisted
- THEN `SourceRunID` holds the prior run's ID (HU-016 AC `docs/HISTORIAS.md:1065`)

### Requirement: No Prior Run Falls Back To Manual Flow

When a ticket has no prior run, the system SHALL fall back to the normal
manual promotion flow with no pre-load.

#### Scenario: Ticket without a prior run uses the manual flow
- GIVEN a ticket with no prior run recorded
- WHEN a promotion is started for that ticket
- THEN the manual flow proceeds with no commits pre-loaded (HU-016 `docs/HISTORIAS.md:1076`)
