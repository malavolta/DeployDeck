# Release Pipeline Activation Checklist (HU-019)

This checklist captures the infrastructure prerequisites for **real
publication** of DeployDeck releases (Homebrew formula / Scoop manifest
updates pushed to their repos, and installable `brew`/`scoop`/`go install`
paths). None of it is delivered by the `release` change — it is deferred
on purpose, per design ADR-5 ("Ships-now vs infra-gated split") and the
`release-pipeline` spec's "Real Publication Activation Prerequisites"
requirement.

**Explicitly infra-gated / documented-not-exercised by this change:**

- **AC2's "se actualizan la formula...manifiesto" clause** — i.e. that a
  real release actually updates the Homebrew formula and Scoop manifest in
  their respective repos. CI only validates that `goreleaser check` passes
  and that a `--snapshot --clean` dry-run *renders* a formula/manifest
  locally under `dist/` without publishing anywhere (see
  `.github/workflows/ci.yml`'s `goreleaser-check` job). No test or CI job
  asserts a real tap/bucket repo was updated.
- **AC3 in full** (`brew install <org>/tap/deploydeck` and
  `scoop install deploydeck` actually working) — both require steps (b),
  (c), and (d) below, none of which exist yet.

These are never represented as passing, done, or exercised by this change's
tests or CI. Do not close them by inspection of `.goreleaser.yaml` alone —
they require the steps below to actually happen.

## Steps

### (a) Decide the real module path

Blocked today on no `origin` remote — `go.mod` currently reads
`module deploydeck`. Once a real GitHub repo exists:

1. `go.mod`: `module deploydeck` → `module github.com/<owner>/deploydeck`.
2. Update every internal import path accordingly (`deploydeck/internal/...`
   → `github.com/<owner>/deploydeck/internal/...`).
3. `.goreleaser.yaml`'s `ldflags` block: update the `-X` symbol paths from
   `deploydeck/internal/version.*` to
   `github.com/<owner>/deploydeck/internal/version.*`.
4. README's `go install`/`go build` examples: update the module path.

### (b) Create the auxiliary repositories

- `homebrew-tap` — a GitHub repo named `homebrew-tap` under the real owner,
  holding generated `.rb` Homebrew formulas (goreleaser's `brews:` pipe
  pushes here on a real release).
- `scoop-bucket` — a GitHub repo holding generated `.json` Scoop manifests
  (goreleaser's `scoops:` pipe pushes here on a real release).

Until these exist, `.goreleaser.yaml`'s `brews:`/`scoops:` `repository:`
blocks point at the placeholder owner `<OWNER>`.

### (c) Provision and wire the write-scoped token

goreleaser needs a token with push access to `homebrew-tap` and
`scoop-bucket` to open the formula/manifest update commit/PR. Provision a
fine-grained (or classic, contents:write-scoped) GitHub token and add it as
repository secrets:

- `HOMEBREW_TAP_TOKEN` — consumed by `.goreleaser.yaml`'s `brews[].token`
  and referenced in `.github/workflows/release.yml`.
- `SCOOP_TOKEN` — consumed by `.goreleaser.yaml`'s `scoops[].token`, same
  workflow.

The default `GITHUB_TOKEN` (already wired in `release.yml`) is sufficient
for the GitHub Release itself, but not for pushing to the separate tap/bucket
repos — that needs the dedicated token(s) above.

### (d) Public vs. private repository decision

This is the story's pending/open decision (see design.md "Open Questions"
and the `release-pipeline` spec's Real Publication Activation Prerequisites
requirement, item (d)):

- **Public repo**: `brew`/`scoop install` and `go install` work with no
  per-user token for end users; the update-check's GitHub API call also
  needs no auth.
- **Private repo**: end users need a personal GitHub token configured for
  `brew`/`scoop`/`go install` to authenticate against GitHub, and the
  install docs (README's "Installation (once released)" section) need an
  explicit per-user token setup step. `internal/update.Checker.Latest`
  already treats a 401 (unauthorized) identically to any other failure —
  silent skip, no nag — so a private repo does not require app-code changes,
  only documentation.

Update README's install instructions once this is decided.

### (e) Cut the first real semver tag

Once (a)-(d) are done:

```sh
git tag v0.1.0
git push origin v0.1.0
```

This triggers `.github/workflows/release.yml`, which runs
`goreleaser release --clean` — building, archiving, checksumming, creating
the GitHub Release, and (now that (b)/(c) are wired) pushing the updated
Homebrew formula and Scoop manifest to their repos.

## What already works without this checklist

- `deploydeck --version` (ldflags-injected `internal/version`).
- The non-blocking update-notification banner (`internal/update`), which
  degrades silently on any failure — including a 401 from a private repo
  before step (d) is decided.
- `goreleaser check` and `goreleaser release --snapshot --clean` in CI
  (`.github/workflows/ci.yml`), validating the whole pipeline shape and
  producing local `dist/` artifacts on every PR, without publishing.
- `go build ./cmd/deploydeck` / `go install deploydeck/cmd/deploydeck@latest`
  against the current placeholder module path (works today; only the
  *convenience* of a short `github.com/<owner>/...` path and a tagged
  release are gated on this checklist).
