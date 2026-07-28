# Release Pipeline Specification

## Purpose

CI validates every PR (build/vet/test) and validates the release
configuration (`goreleaser check` + `--snapshot` dry-run), producing
versioned, distributable artifacts. Real publication to package-manager
repositories is infra-gated and captured as a documented activation
checklist, not exercised by this change.

## ADDED Requirements

### Requirement: CI Build/Vet/Test on Every PR

CI MUST run a build, `go vet`, and the test suite on every pull request.

(AC1, docs/HISTORIAS.md:1179; Test E2E, docs/HISTORIAS.md:1193;
verification: CI job `ci.yml` — no local unit-test claim)

#### Scenario: PR opens and CI runs the core checks

- GIVEN a PR is opened against the repository
- WHEN CI runs
- THEN it executes build, `go vet`, and tests, and the job fails if any
  step fails

### Requirement: goreleaser Configuration Validity

The `.goreleaser.yaml` configuration MUST be valid as verified by
`goreleaser check`.

(Test E2E, docs/HISTORIAS.md:1193; verification: CI job — goreleaser is
not installed locally, per exploration.md; no local test)

#### Scenario: Config passes goreleaser check

- GIVEN the committed `.goreleaser.yaml`
- WHEN `goreleaser check` runs in CI
- THEN it reports the configuration valid with no errors

### Requirement: Snapshot Build Produces Versioned, Multi-Platform Artifacts

A snapshot/dry-run release build (`goreleaser release --snapshot --clean`)
MUST produce, under `dist/`, binaries for macOS (amd64 and arm64), Linux,
and Windows, plus a Homebrew formula and a Scoop manifest, WITHOUT
publishing to any remote tap, bucket, or GitHub Release.

(AC2, docs/HISTORIAS.md:1180; Test E2E, docs/HISTORIAS.md:1193;
verification: CI job — no local test)

#### Scenario: Snapshot dry-run produces dist artifacts without publishing

- GIVEN a snapshot build is triggered in CI
- WHEN `goreleaser release --snapshot --clean` completes
- THEN `dist/` contains macOS/Linux/Windows binaries, a Homebrew formula,
  and a Scoop manifest, and nothing is pushed to the tap/bucket repos

### Requirement: Injected Version Matches Build ldflags

Any binary produced by the pipeline (CI snapshot or real release build)
MUST report a `--version` matching the version injected via ldflags for
that build.

(AC4, docs/HISTORIAS.md:1182; Test E2E, docs/HISTORIAS.md:1193; cross-ref
`release-versioning`; verification: CI job)

#### Scenario: Snapshot binary version matches injected ldflags value

- GIVEN a snapshot build injects a version string via ldflags
- WHEN the resulting binary runs `--version`
- THEN the printed version matches the injected value

### Requirement: Real Publication Activation Prerequisites (Infra-Gated, Documented Only)

The pipeline REQUIRES, for real publication of Homebrew formula/Scoop
manifest updates and GitHub Releases artifacts, the following
prerequisites provided out-of-band and NOT delivered by this change:
(a) a decided real module/repo path (blocked today on no `origin`
remote); (b) the `homebrew-tap` and `scoop-bucket` auxiliary repositories;
(c) a write-scoped GitHub token wired into the release workflow as a
secret; (d) a public/private repository decision, which determines
whether `brew`/`scoop install` require a per-user token; (e) cutting the
first real semver tag. These MUST be captured in an activation checklist
and MUST NOT be represented as passing or exercised by this change's
tests or CI.

(AC2's "se actualizan la formula...manifiesto" clause, AC3, Nota decision
pendiente, Infraestructura necesaria — docs/HISTORIAS.md:1180,1181,1185,1187)

#### Scenario: Activation checklist documents remaining steps (not a passing test)

- GIVEN the documented activation checklist
- WHEN a maintainer completes items (a)-(e)
- THEN real publication, `brew install <org>/tap/deploydeck`, and
  `scoop install deploydeck` become executable
- AND this scenario is explicitly out of scope for this change's
  verification — no automated test or CI job asserts it
