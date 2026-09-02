# Delta for Delta Generation

Source: HU-007 (`docs/HISTORIAS.md:440-504`), corrected by the `project-dir-resolution` infrastructure change. The anchor for `--repo-dir` and `sourceDirs` is now defined once in the `directory-resolution` capability; this delta updates the invocation requirement to make the anchor explicit and testable.

## MODIFIED Requirements

### Requirement: Single Multi-Source-Dir Delta Invocation

The system SHALL generate the delta package with ONE `sf sgd source delta` invocation comparing `origin/<target>` to `HEAD`, passing every configured `delta.sourceDirs` entry — interpreted relative to the git root (see `directory-resolution`'s `sourceDirs` invariant) — as a repeated `--source-dir` flag (sgd spike resolved: repeatable flag supported, no per-dir merge fallback). The invocation SHALL include an explicit `--repo-dir <gitRoot>` flag and SHALL run with its working directory set to the git root, because sgd's `--repo-dir` defaults to `./` and does not walk up to find the repository.

(Previously: did not state the invocation's working directory or `--repo-dir` explicitly, silently relying on the caller's `Dir` already being the git root — broken when the SFDX project root, not the git root, was passed as `Dir`.)

#### Scenario: Multiple source dirs produce one merged package
- GIVEN `delta.sourceDirs` configures more than one directory
- WHEN the delta is generated
- THEN a single `sf sgd source delta` call runs with one `--source-dir` flag per configured directory
- AND the resulting `package.xml` contains members from all configured directories

#### Scenario: Invocation carries an explicit --repo-dir and runs at the git root
- GIVEN a nested repo where the SFDX project root is a subdirectory of the git root
- WHEN the delta is generated
- THEN the `sf sgd source delta` invocation includes `--repo-dir <gitRoot>` and its working directory equals the git root, regardless of where `deploydeck.yaml` or the SFDX project root live

#### Scenario: Flat repo produces the same invocation as before
- GIVEN a flat repo where the git root and the SFDX project root coincide
- WHEN the delta is generated
- THEN the invocation's working directory and `--repo-dir` value are the same directory the invocation used before this change
