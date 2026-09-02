# Capability: Directory Resolution

Source: corrective infrastructure spanning HU-001, HU-007, HU-010, HU-012, HU-015 (`docs/HISTORIAS.md`). Resolves three distinct roots — git root, SFDX project root, artifacts root — that the composition root previously collapsed onto a single `os.Getwd()` value.

## Purpose

Define the three named directory roots DeployDeck operates against, how each is resolved once at the composition root, and which operation class binds to which root. This is the anchor capability other capabilities' anchoring statements (prereq-check, delta-generation) point back to.

## Requirements

### Requirement: Three Named Roots

The system SHALL resolve exactly three roots at the composition root: the **git root** (`git rev-parse --show-toplevel`), the **SFDX project root** (`projectDir`, defaulting to the directory `deploydeck.yaml` was found in), and the **artifacts root** (equal to the SFDX project root, where `.deploydeck/` lives). For a flat repo (no `projectDir` configured, `deploydeck.yaml` at the git root) all three roots SHALL coincide.

#### Scenario: Flat repo collapses all three roots to one directory
- GIVEN a repo with `deploydeck.yaml` at the git root and no `projectDir` configured
- WHEN the three roots are resolved
- THEN git root, SFDX project root, and artifacts root are the identical directory

#### Scenario: Nested repo resolves three distinct roots
- GIVEN a repo whose `deploydeck.yaml`/`sfdx-project.json` live in a subdirectory of the git root
- WHEN the three roots are resolved
- THEN the SFDX project root and artifacts root equal that subdirectory, and the git root equals the true repository top level

### Requirement: Git Operations Bind To The Git Root

Every Git operation (status, branch, cherry-pick, config, hooks, `ChangedFiles`) SHALL run against the git root, either by receiving it directly or by self-resolving it via `RepoRoot`.

#### Scenario: Git commands run at the git root regardless of nesting
- GIVEN a nested repo where the SFDX project root is a subdirectory of the git root
- WHEN any Git-backed operation runs
- THEN it operates against the true git root, not the SFDX project root

### Requirement: `sf project` Commands Bind To The SFDX Project Root

Every `sf project deploy *` invocation (validate, report, quick, cancel) SHALL run with the SFDX project root as its working directory, because `sfdx-project.json` must be present in that command's cwd.

#### Scenario: sf project commands run at the SFDX project root
- GIVEN a nested repo whose SFDX project root differs from the git root
- WHEN `sf project deploy validate`, `report`, `quick`, or `cancel` is invoked
- THEN each runs with its working directory set to the SFDX project root, not the git root or raw process cwd

An omitted working directory SHALL be rejected before the command runs. `internal/exec` treats an
empty `CommandRequest.Dir` as "inherit the process cwd", so a caller that omits the directory would
silently reintroduce the very conflation this capability removes — on the four commands that
deploy, where it is most expensive. The shim SHALL therefore fail fast with a sentinel error
(`salesforce.ErrMissingProjectDir`, wrapped so `errors.Is` matches) and SHALL NOT start any process.

#### Scenario: An empty working directory is rejected before any command runs
- GIVEN any of the four `sf project deploy *` calls made with an empty directory
- WHEN the call is issued
- THEN it returns an error matching `ErrMissingProjectDir` and no external command is executed

#### Scenario: A real working directory still reaches the CLI unchanged
- GIVEN one of the four calls made with a non-empty SFDX project root
- WHEN the call is issued
- THEN the command runs with that directory as its working directory, and the directory never appears among the command's arguments

### Requirement: sgd Binds To The Git Root, Not The Project Root

`sf sgd source delta` SHALL run with its working directory set to the git root AND SHALL receive an explicit `--repo-dir <gitRoot>` flag, because sgd's `--repo-dir` defaults to `./` and does not walk up to find the repository the way `git` does. This binding MUST NOT be conflated with the SFDX-project-root binding used for `sf project` commands — it is the distinction most likely to be re-broken by a future change.

#### Scenario: sgd carries an explicit --repo-dir pointing at the git root
- GIVEN a nested repo where the SFDX project root is a subdirectory of the git root
- WHEN the delta package is generated
- THEN the `sf sgd source delta` invocation includes `--repo-dir <gitRoot>` and runs with working directory equal to the git root

#### Scenario: sgd never binds to the SFDX project root
- GIVEN a nested repo
- WHEN the delta package is generated
- THEN the sgd invocation's working directory and `--repo-dir` value are NOT the SFDX project root

### Requirement: `projectDir` Configuration Key

`Config` SHALL carry an additive, top-level, zero-value-safe `projectDir` field (`yaml:"projectDir"`), interpreted relative to the git root. An absent or empty `projectDir` SHALL mean "the git root IS the SFDX project root" — today's flat-layout behavior.

A `projectDir` that is an absolute path, or that contains a `..` path segment, SHALL be rejected with an actionable error **on the resolution path that production actually executes** (`Config.ProjectRoot`), not only inside `Config.Validate()`.

> **Why this wording is explicit.** `Config.Validate()` is currently dead code in production: it is
> invoked only from `internal/config/config_test.go` and `internal/config/validate_test.go`; neither
> `config.Load` nor `cmd/deploydeck` ever calls it (verified). A criterion worded solely against
> `Validate()` would therefore have zero runtime effect, and a config with an absolute `projectDir`
> would escape the guard entirely. The same rule MAY additionally be surfaced through `Validate()`
> for consistency, but `ProjectRoot` is the binding enforcement point. Repairing the dead
> `Validate()` call path itself is a separate, deferred concern — see the deferred follow-up
> recorded in this change's proposal.

#### Scenario: Absent projectDir defaults to the config file's directory
- GIVEN a config with no `projectDir` key
- WHEN the SFDX project root is resolved
- THEN it equals the directory `deploydeck.yaml` was found in, which in a flat
  layout is the git root

> Worded against the config file's directory, not unconditionally the git root, to stay consistent
> with the "projectDir Defaults To The Config File's Directory" requirement below. The two are the
> same directory in a flat layout and deliberately differ in a nested one; stating it as "the git
> root" would encode a rule the implementation intentionally does not follow.

#### Scenario: Absolute projectDir is rejected on the resolution path
- GIVEN a config with `projectDir` set to an absolute path
- WHEN the SFDX project root is resolved via `Config.ProjectRoot`
- THEN resolution fails with an actionable error naming the offending value

#### Scenario: projectDir containing a parent-traversal segment is rejected on the resolution path
- GIVEN a config with `projectDir` containing a `..` segment
- WHEN the SFDX project root is resolved via `Config.ProjectRoot`
- THEN resolution fails with an actionable error naming the offending value

### Requirement: projectDir Defaults To The Config File's Directory

WHEN `projectDir` is unset, the SFDX project root SHALL default to the directory in which `deploydeck.yaml` was found (via `config.Locate`) — not unconditionally the git root and not an auto-detected `sfdx-project.json` location.

#### Scenario: Nested config file's directory becomes the default project root
- GIVEN `deploydeck.yaml` is found in a subdirectory of the git root, with no `projectDir` configured
- WHEN the SFDX project root is resolved
- THEN it equals the directory the config file was found in

### Requirement: Config Discovery Bounded At The Git Root

`config.Locate(startDir, stopDir)` SHALL search upward from `startDir`, probing `startDir` first and then each ancestor directory up to and including `stopDir` (the resolved git root), returning the first directory containing `deploydeck.yaml`. Taking the bound as a parameter keeps `internal/config` a pure leaf with no git dependency (design ADR-1). The starting directory SHALL be canonicalised (symlinks resolved) before the search, so the lexical bound check compares two paths in the same path space — a symlinked checkout otherwise collapses the search to `startDir` alone and silently disables it. `Load(dir)` SHALL remain unchanged. WHEN `startDir` is not inside a Git repository, the upward search SHALL be skipped and the system SHALL fall back to `Load(startDir)` directly, so the "not a git repository" prerequisite diagnostic remains primary rather than being masked by a "config not found" error.

#### Scenario: Config found immediately at cwd
- GIVEN `deploydeck.yaml` exists in the current working directory
- WHEN `Locate` searches from that directory
- THEN it returns that directory on the first probe

#### Scenario: Config found after walking up to the git root
- GIVEN `deploydeck.yaml` exists at the git root and cwd is a subdirectory with no config file
- WHEN `Locate` searches upward from cwd
- THEN it returns the git root

#### Scenario: No config found anywhere up to the git root
- GIVEN no `deploydeck.yaml` exists between cwd and the git root, inclusive
- WHEN `Locate` searches upward
- THEN it reports not found, the same failure mode as today's single-directory `Load`

#### Scenario: cwd is outside any Git repository
- GIVEN the current working directory is not inside a Git repository
- WHEN resolution runs
- THEN the upward search is skipped and `Load(cwd)` is attempted directly, preserving "not a git repository" as the primary diagnostic

### Requirement: Gitignore Check Anchored At The Artifacts Root

The `.deploydeck/` gitignore enforcement check (and its fix) SHALL read and write the `.gitignore` governing the artifacts root, not unconditionally the git root's `.gitignore`. WHEN the artifacts root and the git root coincide (flat layout), this SHALL be observably identical to today's behavior.

#### Scenario: Nested layout checks the artifacts-root gitignore
- GIVEN a nested repo where the artifacts root differs from the git root
- WHEN the gitignore enforcement check runs
- THEN it reads the `.gitignore` governing the artifacts root, not the git-root `.gitignore`

#### Scenario: Flat layout behaves identically to today
- GIVEN a flat repo where the artifacts root equals the git root
- WHEN the gitignore enforcement check runs
- THEN it reads the same `.gitignore` file it would have read before this change

### Requirement: `delta.sourceDirs` Stays Repo-Root-Relative (INVARIANT)

`delta.sourceDirs` SHALL remain interpreted relative to the git root, unaffected by `projectDir`. Rationale: `Summarize` and `buildArgs`'s `--source-dir` compare `sourceDirs` entries against `changedFiles` (from `git diff --name-only`, always git-root-relative) via raw string-prefix matching; both sides of that comparison MUST share the same base. Reinterpreting `sourceDirs` as project-relative would double-prefix nested-layout paths, causing sgd to match nothing — an empty-but-successful package that reports every changed file as "outside sourceDirs." This requirement is a regression guard: a future change MUST NOT reinterpret `sourceDirs`'s anchor without introducing a distinct, separately-keyed configuration field.

#### Scenario: sourceDirs entries stay git-root-relative under a configured projectDir
- GIVEN a nested repo with `projectDir` configured and `sourceDirs` containing a git-root-relative path
- WHEN the delta package is generated
- THEN `sourceDirs` is matched against `changedFiles` using the git root as the shared base, unaffected by `projectDir`

#### Scenario: Configuring projectDir does not require editing sourceDirs
- GIVEN an existing nested-repo config with `sourceDirs` already set correctly, relative to the git root
- WHEN `projectDir` is added to that same config
- THEN the delta package generated is unchanged from before `projectDir` was added

### Requirement: Backward Compatibility For Flat Repos

WHEN `projectDir` is absent and the repo is flat (git root, SFDX project root, and artifacts root already coincide), every directory, command working directory, and artifact location SHALL be identical to the system's behavior before this change.

#### Scenario: Flat repo behavior is unchanged
- GIVEN a flat repo with `projectDir` absent from configuration
- WHEN the full flow runs (prereq check, delta generation, validation, quick deploy, cancel)
- THEN every resolved path and command working directory matches this change's absence exactly
