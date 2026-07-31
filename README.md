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

Or install straight into `$GOBIN` without a local checkout. This needs a
published tag (see the activation checklist) and a public repository:

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

## Installation (once released) — ACTIVATION-GATED

The install paths below are documented for when the project has cut a real
release; they are **not usable yet**. See "Activation checklist" for why.

```sh
# Homebrew (macOS/Linux) — once the tap exists:
brew install malavolta/tap/deploydeck

# Scoop (Windows) — once the bucket exists:
scoop bucket add deploydeck https://github.com/malavolta/scoop-bucket
scoop install deploydeck

# go install against a real tagged release:
go install github.com/malavolta/DeployDeck/cmd/deploydeck@vX.Y.Z
```

Every release is built by `.goreleaser.yaml` and validated in CI
(`.github/workflows/ci.yml` runs `goreleaser check` and a
`--snapshot --clean` dry-run on every PR, asserting `dist/` contains the
macOS/Linux/Windows binaries, checksums, a Homebrew formula, and a Scoop
manifest — without publishing anything). Real publication is driven by
`.github/workflows/release.yml` on a `v*` tag push, but that workflow is
inert until the checklist below is complete.

## Activation checklist

Real publication (Homebrew formula / Scoop manifest actually pushed, and
`brew`/`scoop install` actually working) needs infrastructure this change
does **not** provide. It is deferred and documented, not silently marked
done:

1. ~~**Real module path**~~ — **DONE.** `go.mod` is
   `module github.com/malavolta/DeployDeck` and `.goreleaser.yaml`'s ldflags
   use `-X github.com/malavolta/DeployDeck/internal/version...`.
2. **Create the `homebrew-tap` and `scoop-bucket` repositories** under the
   real GitHub owner — `.goreleaser.yaml`'s `brews:`/`scoops:` stanzas
   currently point at a placeholder `<OWNER>`.
3. **Provision and wire a write-scoped GitHub token** as the
   `HOMEBREW_TAP_TOKEN`/`SCOOP_TOKEN` repository secrets consumed by
   `.github/workflows/release.yml`.
4. **Decide public vs. private repository** — this determines whether
   `brew`/`scoop install` (and `go install`) need a per-user token, and the
   install docs above must be updated once decided (currently pending, per
   the story's open question).
5. **Cut the first real `vX.Y.Z` tag** — this is what actually triggers
   `.github/workflows/release.yml` and, once steps 1-4 are done, produces a
   real GitHub Release plus tap/bucket updates.

See `docs/ACTIVATION-CHECKLIST.md` for the full checklist with rationale and
cross-references to the `release-pipeline` spec's infra-gated requirement.
