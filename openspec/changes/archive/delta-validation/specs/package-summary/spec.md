# Delta for Package Summary

Source: HU-008 (`docs/HISTORIAS.md:506-569`). Summarizes `package.xml`/`destructiveChanges.xml` for review before validation.

## ADDED Requirements

### Requirement: Per-Type Member Counts

The system SHALL parse `package.xml` and SHALL show a member count per metadata type.

#### Scenario: Package with multiple types shows a count per type
- GIVEN a `package.xml` with several metadata types
- WHEN the summary is shown
- THEN each type displays its member count

### Requirement: Empty Package Clearly Warned

The system SHALL clearly warn the user when the parsed package has no members.

#### Scenario: Empty package is warned
- GIVEN an empty `package.xml`
- WHEN the summary is shown
- THEN a clear warning is displayed

### Requirement: Destructive Changes Shown Separately

The system SHALL parse `destructiveChanges.xml` when present and SHALL show its per-type member counts separately from additive `package.xml` types; destructive counts SHALL NOT be mixed into the additive per-type counts.

#### Scenario: Destructive changes appear separated and highlighted
- GIVEN a delta with destructive changes
- WHEN the summary is shown
- THEN destructive types and counts appear separately from and are highlighted apart from additive types

#### Scenario: Destructive members never inflate additive counts
- GIVEN a delta with the same metadata type present in both `package.xml` and `destructiveChanges.xml`
- WHEN per-type counts are computed
- THEN the additive count for that type excludes destructive members

### Requirement: Sensitive Metadata Type Warning

The system SHALL flag `Profile`, `PermissionSet`, `Flow`, `CustomObject`, and `CustomField` as sensitive types and SHALL show a warning before the user continues when any are present.

#### Scenario: Sensitive metadata triggers a warning
- GIVEN a package containing `Profile`, `PermissionSet`, `Flow`, `CustomObject`, or `CustomField` members
- WHEN the summary is shown
- THEN a warning identifies the sensitive types before the user continues

### Requirement: Files Outside Configured Source Dirs Listed

The system SHALL list changed files that fall outside the configured `sourceDirs` when they appear in the diff.

#### Scenario: File outside sourceDirs is listed
- GIVEN a changed file outside the configured `delta.sourceDirs`
- WHEN the summary is shown
- THEN that file is listed under files outside configured source dirs
