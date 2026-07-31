# DeployDeck

DeployDeck is a Go terminal UI (TUI) that orchestrates Salesforce commit
promotion through controlled Git cherry-picks. It guides a developer through:
prerequisite checks (`git`, `sf`, `sfdx-git-delta`, `gh`), commit discovery on
a source branch, commit selection, target-branch/sandbox selection, promotion
branch creation, and cherry-picking with post-pick verification against a
Salesforce org — without hand-rolling the cherry-pick sequence or the
Salesforce delta/validation steps yourself.

Built with [Bubble Tea](https://github.com/charmbracelet/bubbletea),
[Bubbles](https://github.com/charmbracelet/bubbles),
[Lip Gloss](https://github.com/charmbracelet/lipgloss), and
[Cobra](https://github.com/spf13/cobra).

## Build from source

Requires Go 1.26.x (see `go.mod`).

```sh
go build ./cmd/deploydeck
./deploydeck --version
```

Or install straight into `$GOBIN` without a local checkout (the repo is
public and tagged, so this resolves):

```sh
go install github.com/malavolta/DeployDeck/cmd/deploydeck@latest
```

## Usage

```sh
deploydeck              # launches the TUI in the current Git repository
deploydeck doctor        # checks local prerequisites (git, sf, sfdx-git-delta, gh)
deploydeck runs prune     # prunes local run history under .deploydeck/runs/ per config
deploydeck --version      # prints the build's version/commit/date, or "dev"
```

## Version reporting

`deploydeck --version` reports the version baked in at build time via
`-ldflags -X github.com/malavolta/DeployDeck/internal/version.Version=...` (plus `Commit`/`Date`
when the build sets them). A binary built without those `-X` flags — e.g.
plain `go build`/`go run` — reports `dev`.

## Update notifications

On startup, DeployDeck makes one non-blocking, best-effort check (bounded to
3 seconds) against the GitHub Releases API for a newer tagged version. If a
newer version is found, a one-line banner is shown above the normal TUI
content; nothing else changes and nothing is blocked. If the check fails,
times out, or the current build is a `dev` build, DeployDeck stays silent —
it never shows an error and never nags. This check contacts no server
besides GitHub, sends no credentials, and can't hang the TUI.

## Installation

DeployDeck is published — see the
[latest release](https://github.com/malavolta/DeployDeck/releases/latest).
Pick whatever fits your platform:

```sh
# Homebrew (macOS) — installs the cask from the tap
brew install malavolta/tap/deploydeck

# Scoop (Windows)
scoop bucket add deploydeck https://github.com/malavolta/scoop-bucket
scoop install deploydeck

# Go (any OS)
go install github.com/malavolta/DeployDeck/cmd/deploydeck@latest
```

You can also download a prebuilt binary (macOS/Linux/Windows · amd64/arm64)
straight from the [Releases page](https://github.com/malavolta/DeployDeck/releases).

> **Homebrew note:** DeployDeck ships as a Homebrew **cask**, which is
> macOS-only. On Linux, use `go install` or the release tarball.

## Releasing

Releases are fully automated by [GoReleaser](https://goreleaser.com) through
`.github/workflows/release.yml`. To cut a new version, push a semver tag:

```sh
git tag v0.1.1
git push origin v0.1.1
```

That builds the macOS/Linux/Windows binaries, publishes a GitHub Release with
checksums, and pushes the updated Homebrew cask and Scoop manifest to
[`malavolta/homebrew-tap`](https://github.com/malavolta/homebrew-tap) and
[`malavolta/scoop-bucket`](https://github.com/malavolta/scoop-bucket). Every PR
also runs `goreleaser check` plus a `--snapshot` dry-run in CI
(`.github/workflows/ci.yml`) without publishing.

The full activation history (how the tap/bucket, tokens, and first tag were set
up) lives in [`docs/ACTIVATION-CHECKLIST.md`](docs/ACTIVATION-CHECKLIST.md).
