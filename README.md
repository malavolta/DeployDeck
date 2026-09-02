# DeployDeck

A terminal UI that promotes Salesforce commits between environments through
controlled Git cherry-picks. It wraps `git`, `sf`, `sfdx-git-delta` and `gh`, so
you run the whole promotion — discover commits → branch → cherry-pick → delta →
CheckOnly validation → PR → deploy — from one screen instead of scripting it by
hand. Nothing runs through a shell.

Built with [Bubble Tea](https://github.com/charmbracelet/bubbletea), Bubbles,
[Lip Gloss](https://github.com/charmbracelet/lipgloss) and
[Cobra](https://github.com/spf13/cobra).

## The flow

Each step is its own screen:

1. **Doctor** — checks `git`, `sf`, `sfdx-git-delta`, `gh`.
2. **Select** — suggests the ticket from your branch, discovers the commits in
   range, you pick which ones and the target environment.
3. **Branch** — creates `deploy/<ticket>-to-<target>`, or reuses an existing one
   and appends only the new commits.
4. **Cherry-pick** — interactive conflict resolution, then post-pick verification.
5. **Delta** — builds the package with `sfdx-git-delta`.
6. **PR** — pushes and opens the PR (optional AI title/description + a verifiable
   signature).
7. **Validate** — CheckOnly against the org, with real failure detail: component
   `file:line`, test stack traces, per-class coverage below the gate.
8. **Deploy** — promotes the validated job, optionally behind a per-environment
   approval gate.

## Requirements

| Tool | For | macOS | Linux |
|------|-----|-------|-------|
| **git** ≥ 2 | cherry-picks | `brew install git` | `apt install git` / `dnf install git` |
| **Salesforce CLI (`sf`)** ≥ 2 | validate / deploy | `brew install sf` | `npm i -g @salesforce/cli` |
| **sfdx-git-delta** ≥ 5 | delta package | `sf plugins install sfdx-git-delta` | same |
| **GitHub CLI (`gh`)** | PRs, reviews | `brew install gh` | [cli.github.com](https://cli.github.com/manual/installation) |

Then authenticate what you use — `sf org login web --alias <ALIAS>` per org,
`gh auth login` — and run `deploydeck doctor` to see what's still missing.

## Install

```sh
brew install malavolta/tap/deploydeck                              # macOS (cask)
go install github.com/malavolta/DeployDeck/cmd/deploydeck@latest   # Linux / any OS (Go 1.26)
```

Windows: `scoop bucket add deploydeck https://github.com/malavolta/scoop-bucket && scoop install deploydeck`.
Or grab a prebuilt binary from [Releases](https://github.com/malavolta/DeployDeck/releases).
On Linux the Homebrew cask is unavailable — use `go install` or the tarball.

## Configure

Put `deploydeck.yaml` at your Salesforce project root (next to
`sfdx-project.json`) and run `deploydeck` there — it resolves the git root, the
SFDX project root and the `.deploydeck/` artifacts root separately, so the
project can sit in a subdirectory of the git repo (a monorepo, for example)
with no other config change. `deploydeck.yaml` itself can also be found by
walking up from where you run `deploydeck`, bounded at the git root.

```yaml
# OPTIONAL. The SFDX project's location relative to the GIT ROOT. Omit it
# entirely for a flat repo (deploydeck.yaml at the git root — the default,
# and every install that predates this key): the SFDX project root then
# defaults to the directory deploydeck.yaml was found in. Set it only when
# the SFDX project lives in a subdirectory of the git repo. Rejected if
# absolute or containing a ".." segment.
#
# Setting this does NOT change how delta.sourceDirs is read — see the note
# there. It also moves .deploydeck/ (runs, manifests, lock) to the project
# directory, so set it before you accumulate run history you care about.
# projectDir: up_saln0001_giss_salesforce

# Pipeline. Keys MUST be integration/uat/production (read by name to order the
# promotion); values are your branch names.
branches:
  integration: INT
  uat: UAT
  production: main

# Which org each target validates against. Key by branch name or a glob.
sandboxes:
  INT:
    alias: MY_INT_SANDBOX
    testLevel: RunLocalTests
  "Release/*":                 # any Release/... branch
    alias: MY_PREPROD_SANDBOX
    testLevel: RunLocalTests
  main:
    alias: MY_PROD_ORG
    testLevel: RunLocalTests

# Tickets in branch/commit names. `-?` = optional dash (PROJ-123 or PROJ123).
ticketPatterns:
  - "PROJ-?[0-9]+"

branchFormat: "deploy/{{ticket}}-to-{{target}}"   # tokens: {{ticket}}, {{target}}

# Delta package. sourceDirs is relative to the GIT ROOT — ALWAYS, and even
# when projectDir is set. Setting projectDir does not make these paths
# project-relative: if your project lives in a subdirectory, the subdirectory
# still belongs here. Getting this wrong does not raise an error — sgd simply
# matches nothing and produces an EMPTY package that validates green and
# deploys nothing. Copy the path from `git ls-files` output, not from
# sfdx-project.json.
#
#   flat repo:    sourceDirs: [force-app]
#   nested repo:  sourceDirs: [up_saln0001_giss_salesforce/force-app]
delta:
  sourceDirs:
    - force-app
  outputDir: .deploydeck/manifest/delta

# --- optional below ---

# Deploy gate: before the real deploy to a gated target, the run's PR must be
# approved by the listed reviewers, have no unresolved threads, carry
# DeployDeck's validation comment, and have a valid signature — or it's blocked.
gates:
  INT:
    enabled: true
    approvers: [alice, bob, carol]   # GitHub logins
    minApprovals: 1

# Run the real deploy. false (default) only shows the command.
quickDeploy:
  allowExecution: false

# Local AI to draft the PR title/description. Optional: omit the block (or set
# enabled: false) and DeployDeck runs the same. If the model is unreachable it
# degrades silently — it never blocks the flow. Any OpenAI-compatible server
# (Ollama, LM Studio, llama.cpp).
ai:
  enabled: false
  endpoint: "http://localhost:11434"
  model: "qwen2.5-coder:3b"
```

`branches`, `sandboxes`, `ticketPatterns`, `branchFormat` and `delta` are
required; `projectDir`, `gates`, `quickDeploy` and `ai` are optional and
default to off (`projectDir` defaults to a flat layout — see above).

## Use

Run `deploydeck` from your project directory and follow the [flow](#the-flow)
above. To deploy: from the run history, press `x` on a validated run to open the
deploy screen, then type `DESPLEGAR`. On a gated target, DeployDeck checks
approvals, resolved threads, the validation comment and the signature first, and
blocks with the list of what's missing before touching the org.

```sh
deploydeck                    # launch the TUI
deploydeck doctor             # check prerequisites
deploydeck pr verify <url>    # confirm a PR was created by DeployDeck (exit 0 = ok)
deploydeck runs prune         # trim local run history
deploydeck --version
```

`pr verify` uses distinct exit codes (0 verified · 1 mismatch · 2 no marker ·
3 dev · 4 degraded), so it fits straight into CI.

## Notes

- **Provenance.** Every DeployDeck PR carries a signed marker; release builds
  inject the signing secret at build time, `dev` builds sign a `dev` marker.
  `deploydeck pr verify` re-checks it.
- **Deploy gate** fails **closed**: if the PR can't be resolved or GitHub can't
  be reached, the deploy is blocked, never allowed.
- **Version.** `--version` reports the value baked in via `-ldflags`; a plain
  `go build`/`go run` reports `dev`. On startup DeployDeck does one 3s
  best-effort check for a newer release and shows a one-line banner if there is
  one — silent otherwise, no credentials sent.

## Releasing

Push a semver tag; [GoReleaser](https://goreleaser.com) builds the binaries and
publishes the GitHub Release, Homebrew cask and Scoop manifest:

```sh
git tag v1.3.2 && git push origin v1.3.2
```

Every PR runs `goreleaser check` + a `--snapshot` dry-run in CI. Setup history is
in [`docs/ACTIVATION-CHECKLIST.md`](docs/ACTIVATION-CHECKLIST.md).
