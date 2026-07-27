# Capability: Commit Selection

## Overview

Commit selection allows the user to choose which discovered commits to include in the promotion. It renders commit metadata, enforces constraints (already-applied, merge commits), computes file dependencies, and generates a preliminary deployment plan on confirmation.

## Requirements

### Requirement: Selectable Commit List Rendering

The system SHALL render each discovered commit as a selectable row showing short SHA, message, author, date, and status flags.

#### Scenario: Row shows identifying data
- GIVEN a list of discovered commits
- WHEN the selection screen renders
- THEN each row shows short SHA, message, author, date, and flags

### Requirement: Already-Applied Commit Selection Block

The system SHALL disable selection of commits already present in the target (by SHA or equivalence), showing a `Reason`.

#### Scenario: Already-applied commit cannot be selected
- GIVEN a commit already present in the target branch
- WHEN the user attempts to select it
- THEN selection is prevented and shows a blocking `Reason`

### Requirement: Merge Commit Selection Block

The system SHALL disable selection of merge commits, since merge cherry-pick (`-m`) is unsupported in MVP.

#### Scenario: Merge commit cannot be selected
- GIVEN a merge commit in the list
- WHEN the list renders
- THEN it appears `Disabled` and is not selectable

### Requirement: Multi-Ticket Commit Warning

The system SHALL show an additional-tickets notice when a commit message references tickets other than the one searched.

#### Scenario: Commit with multiple tickets shows a notice
- GIVEN a commit message mentions more than one ticket
- WHEN it is shown in the list
- THEN it displays a notice of additional tickets

### Requirement: Per-File Intermediate-Commit Dependency Warning

The system SHALL compute, for each file touched by the current selection, unselected intermediate commits in `origin/<target>..<source>` that also touch that file, and SHALL show a dependency warning before the user confirms.

#### Scenario: Unselected intermediate commit on a shared file triggers a warning
- GIVEN a selected commit touches a file also touched by an unselected intermediate commit from another ticket
- WHEN the user confirms the selection
- THEN a per-file dependency warning is shown before continuing

### Requirement: Empty Selection Block

The system SHALL block advancing when the user confirms with zero commits selected.

#### Scenario: Confirming with no selection is blocked
- GIVEN no commits are selected
- WHEN the user confirms
- THEN advancing is blocked

### Requirement: Advanced-Mode Reordering Warning

The system SHALL allow manual reordering of selected commits only in advanced mode, and SHALL show a warning that altering topological order increases conflict risk.

#### Scenario: Reordering in advanced mode shows a risk warning
- GIVEN advanced mode is enabled
- WHEN the user reorders selected commits
- THEN a warning about increased conflict risk is shown

### Requirement: Selection Confirmation Produces Preliminary Deployment Plan

The system SHALL generate a preliminary `DeploymentPlan` when the user confirms a valid, non-empty selection.

#### Scenario: Valid confirmation generates a preliminary plan
- GIVEN a valid non-empty commit selection
- WHEN the user confirms
- THEN a preliminary `DeploymentPlan` is generated

## Design Notes

- Commit selection operates in a table-driven UI where each row represents a discovered commit with a checkbox for selection.
- The dependency warning is computed by cross-referencing file paths touched by selected commits against unselected intermediate commits in the same range.
- Advanced mode is an opt-in configuration for power users; default mode prevents manual reordering to minimize merge conflicts.
