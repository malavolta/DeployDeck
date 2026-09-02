# Delta for Prerequisite Check

Source: HU-001 (`docs/HISTORIAS.md:11-93`), corrected by the `project-dir-resolution` infrastructure change. Anchoring is now defined once in the `directory-resolution` capability; this delta updates only the requirement whose anchor was wrong.

## MODIFIED Requirements

### Requirement: `.deploydeck/` Gitignore Enforcement (Blocking)

The system SHALL block the flow when `.deploydeck/` is not present in the `.gitignore` governing the artifacts root, and SHALL offer to add the entry there. WHEN the artifacts root and the git root coincide (flat layout, the default), this is observably the repository's top-level `.gitignore`, unchanged from before.

(Previously: stated as "the repository's `.gitignore`", which the checker resolved unconditionally at the git root — wrong when the artifacts root is a nested SFDX project subdirectory, causing a false block and an untracked git-root `.gitignore` from the offered fix.)

#### Scenario: Ungitignored .deploydeck blocks the flow
- GIVEN `.gitignore` governing the artifacts root does not contain a `.deploydeck/` entry
- WHEN the environment is validated
- THEN the flow is blocked and the user is offered to add the entry

#### Scenario: Nested layout checks the artifacts-root gitignore, not the git root's
- GIVEN a nested repo where the artifacts root differs from the git root, and `.deploydeck/` is already gitignored at the artifacts root but not at the git root
- WHEN the environment is validated
- THEN the check passes, because it reads the `.gitignore` governing the artifacts root

#### Scenario: The offered fix writes at the artifacts root, not the git root
- GIVEN a nested repo missing the `.deploydeck/` entry at the artifacts root
- WHEN the user accepts the offered fix
- THEN the entry is added to the `.gitignore` governing the artifacts root, and no untracked `.gitignore` is created at the git root
