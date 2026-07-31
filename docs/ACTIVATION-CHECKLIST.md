# Release Pipeline Activation Checklist (HU-019)

**STATUS: COMPLETE — v0.1.0 shipped 2026-07-31.**
[Release](https://github.com/malavolta/DeployDeck/releases/tag/v0.1.0) ·
[homebrew-tap](https://github.com/malavolta/homebrew-tap) ·
[scoop-bucket](https://github.com/malavolta/scoop-bucket)

This checklist captured the infrastructure prerequisites for **real
publication** of DeployDeck releases (Homebrew cask / Scoop manifest pushed to
their repos, and installable `brew`/`scoop`/`go install` paths). The `release`
change deferred them on purpose, per design ADR-5 ("Ships-now vs infra-gated
split") and the `release-pipeline` spec's "Real Publication Activation
Prerequisites" requirement. Everything below is now done; this file is kept as
the record of what was set up.

## Steps (all complete)

### (a) Real module path — DONE

`origin` is `git@github.com:malavolta/DeployDeck.git` and the module was renamed
to `github.com/malavolta/DeployDeck` across `go.mod`, every internal import, the
`.goreleaser.yaml` ldflags, and the README/docs.

### (b) Auxiliary repositories — DONE

Two public repos hold the generated install recipes (goreleaser pushes to them
on every release; they hold no source code):

- [`malavolta/homebrew-tap`](https://github.com/malavolta/homebrew-tap) →
  `Casks/deploydeck.rb`.
- [`malavolta/scoop-bucket`](https://github.com/malavolta/scoop-bucket) →
  `deploydeck.json`.

### (c) Write-scoped token — DONE

A fine-grained PAT with **Contents: Read and write** on both repos is wired as
the `HOMEBREW_TAP_TOKEN` and `SCOOP_TOKEN` repository secrets (a single token
serves both, same owner). The default `GITHUB_TOKEN` creates the GitHub Release
but cannot push to the separate tap/bucket repos — hence the dedicated token.

### (d) Public vs. private — DECIDED: public

`malavolta/DeployDeck` and both auxiliary repos are **public**, so
`brew`/`scoop`/`go install` and the update-check need no per-user token.

### (e) First tag — DONE

`v0.1.0` was tagged and pushed, triggering `.github/workflows/release.yml`
(`goreleaser release --clean`): six binaries + checksums in the GitHub Release,
plus the cask and manifest pushed to the tap/bucket. The repository variable
`RELEASE_ACTIVATED=true` un-gates the workflow.

Future releases need only a new tag:

```sh
git tag vX.Y.Z
git push origin vX.Y.Z
```

## Fixes made during activation

The release config had never actually run in CI (no remote existed), so three
latent issues surfaced on the first real run and were fixed:

- **`brews` → `homebrew_casks`.** goreleaser removed `brews` (formula
  generation) in v2.16. The cask is macOS-only and carries a Gatekeeper
  quarantine `postflight` hook because the binary is unsigned.
- **`release.yml` `permissions: contents: write`.** Without it the default
  `GITHUB_TOKEN` is read-only and goreleaser cannot create the Release (it
  failed before publishing anything).
- **CI `-short` + case-insensitive artifact assertion.** `ci.yml` now runs
  `go test ./... -race -short` (integration e2e self-skip on the runner) and
  matches the `DeployDeck_*` archives with `find -iname` (the Linux runner
  filesystem is case-sensitive).

## What the binary does on its own

- `deploydeck --version` — ldflags-injected `internal/version`.
- The non-blocking update-notification banner (`internal/update`) checks the
  GitHub Releases API and degrades silently on any failure.
