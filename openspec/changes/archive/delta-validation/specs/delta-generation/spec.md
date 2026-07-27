# Delta for Delta Generation

Source: HU-007 (`docs/HISTORIAS.md:440-504`). Wraps `sf sgd source delta` to produce a delta package after cherry-pick promotion.

## ADDED Requirements

### Requirement: Single Multi-Source-Dir Delta Invocation

The system SHALL generate the delta package with ONE `sf sgd source delta` invocation comparing `origin/<target>` to `HEAD`, passing every configured `delta.sourceDirs` entry as a repeated `--source-dir` flag (sgd spike resolved: repeatable flag supported, no per-dir merge fallback).

#### Scenario: Multiple source dirs produce one merged package
- GIVEN `delta.sourceDirs` configures more than one directory
- WHEN the delta is generated
- THEN a single `sf sgd source delta` call runs with one `--source-dir` flag per configured directory
- AND the resulting `package.xml` contains members from all configured directories

### Requirement: Delta Artifacts Written Under `.deploydeck/manifest/`

The system SHALL write generated delta artifacts under `.deploydeck/manifest/delta/<ticket>-to-<target>/`, producing `package.xml` for Salesforce changes, and SHALL produce `destructiveChanges.xml` when the diff includes deleted metadata.

#### Scenario: Salesforce changes produce package.xml
- GIVEN Salesforce metadata changes between `origin/<target>` and `HEAD`
- WHEN the delta is generated
- THEN `package.xml` is created under `.deploydeck/manifest/delta/<ticket>-to-<target>/`

#### Scenario: Deleted metadata produces destructiveChanges.xml
- GIVEN deleted metadata between `origin/<target>` and `HEAD`
- WHEN the delta is generated
- THEN `destructiveChanges.xml` is created alongside `package.xml`

### Requirement: Empty Package Blocks Validation Pending Override

The system SHALL show a clear warning when the generated package is empty, and SHALL block entry to Salesforce validation until the user gives an explicit confirmation override.

#### Scenario: Empty package warns and blocks validation
- GIVEN the delta produces an empty `package.xml`
- WHEN the delta generation completes
- THEN a warning is shown and validation is blocked
- AND validation only proceeds after the user explicitly confirms an override

### Requirement: sgd Failure Surfaces Output Without Running Validation

The system SHALL show the raw `sfdx-git-delta` stdout/stderr when the delta command fails, and SHALL NOT run Salesforce validation in that case.

#### Scenario: sgd failure blocks validation
- GIVEN `sf sgd source delta` exits with a failure
- WHEN the delta generation step runs
- THEN the stdout/stderr output is shown to the user
- AND no Salesforce validation is executed

### Requirement: Generated Paths Persisted On The Deployment Plan

The system SHALL save the generated `package.xml` and, when present, `destructiveChanges.xml` paths on the `DeploymentPlan`, and SHALL save the raw sgd output for diagnostics.

#### Scenario: Generated paths recorded on the plan
- GIVEN a successful delta generation
- WHEN the artifacts are written
- THEN their paths are saved on the `DeploymentPlan`
- AND the raw sgd output is saved for diagnostics

### Requirement: Working Tree Stays Clean

The system SHALL confine all delta artifacts to paths under `.deploydeck/`, and SHALL NOT modify any tracked working-tree file as part of delta generation.

#### Scenario: Delta generation leaves the working tree clean
- GIVEN a delta generation run, empty or not
- WHEN it completes
- THEN no tracked file outside `.deploydeck/` is modified

### Requirement: Entry Gated By Post-Pick Verification

Delta generation SHALL only run when the existing `cherry-pick` capability's "Failed Cherry-Pick Blocks Downstream Steps" gate allows it (see `openspec/specs/cherry-pick/spec.md`); this change does not restate or re-derive that gate.

#### Scenario: Failed cherry-pick prevents delta generation
- GIVEN a cherry-pick run that failed
- WHEN the flow evaluates whether to start delta generation
- THEN delta generation does not start, per the reused cherry-pick gate
