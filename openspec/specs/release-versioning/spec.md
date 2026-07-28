# Release Versioning Specification

## Purpose

Embed a build-time version identifier in the DeployDeck binary and expose it
via `deploydeck --version`, replacing Cobra's default inert `--version`.

### Requirement: Version Variables Package

The system MUST provide an `internal/version` package exposing `Version`
(and `Commit`/`Date`) as package-level variables. `Version` MUST default to
`"dev"` when the binary is built without linker-flag injection.

(HU-019 Tareas, docs/HISTORIAS.md:1173)

#### Scenario: Built without ldflags

- GIVEN the binary is built without any `-ldflags` version injection
- WHEN `deploydeck --version` runs
- THEN it prints `"dev"`

#### Scenario: Built with ldflags

- GIVEN the binary is built with `-X deploydeck/internal/version.Version=1.2.3`
- WHEN `deploydeck --version` runs
- THEN it prints `1.2.3`

### Requirement: CLI Version Exposure

The `deploydeck` root command MUST report `internal/version.Version` through
Cobra's `--version` flag.

(AC4, docs/HISTORIAS.md:1182)

#### Scenario: Installed binary reports its version

- GIVEN an installed `deploydeck` binary built with an injected version
- WHEN a user runs `deploydeck --version`
- THEN the output matches the injected version, not a hardcoded or empty
  string

### Requirement: Release Build Version Injection

The release build process MUST inject the triggering semver tag as
`internal/version.Version` via ldflags, so the built artifact's `--version`
output matches the tag that produced it.

(Test E2E, docs/HISTORIAS.md:1193; verification: CI/goreleaser job — see
`release-pipeline` capability — not a local unit test)

#### Scenario: Tag-triggered build injects matching version

- GIVEN a release build is produced for semver tag `v1.2.3`
- WHEN the resulting binary runs `--version`
- THEN it prints the value matching tag `v1.2.3`, verified by the
  CI/goreleaser job
