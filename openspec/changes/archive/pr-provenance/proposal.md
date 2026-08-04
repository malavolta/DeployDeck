# Proposal: PR Provenance (verifiable DeployDeck-created marker)

## Intent

Give anyone looking at a pull request a cheap, verifiable way to tell it was created by
DeployDeck. Every DeployDeck-created PR body gains a visible footer ("Created with DeployDeck
v<version>") plus an invisible HTML-comment marker carrying an HMAC signature. A new
`deploydeck pr verify <url>` recomputes the HMAC and confirms authenticity. Success: a genuine
DeployDeck PR verifies green; a hand-copied or hand-forged marker fails; the check never crashes
when `gh` is missing. Milestone: next minor after v1.0.0.

## Problem / Motivation

DeployDeck-created PRs are today indistinguishable from hand-made ones — nothing in the PR body
signals origin, and the non-AI path sends an EMPTY body. Teams that standardize promotions on
DeployDeck have no lightweight signal of "this PR came from the tool." The goal is a *minimal*
restriction ("restricción mínima"): hard to replicate by hand, trivial to verify. This is
explicitly NOT a strong security control — see Threat Model.

## Threat Model (deliberately minimal — stated honestly)

Deters manual/casual forgery only. The HMAC secret is injected into the released binary and is
therefore extractable by a motivated attacker who inspects the binary; such an attacker can mint
valid markers. The strong alternative — a GitHub App attributing the PR to a bot identity GitHub
itself authenticates — is OUT of scope. We accept the weak guarantee in exchange for zero
infrastructure and a self-contained verify command.

## Scope

### In Scope
- New PURE package `internal/provenance`: `Sign`, `RenderFooter`/`RenderMarker`, `ParseMarker`,
  `Verify`; `var secret` package variable injected at RELEASE build only.
- Footer + invisible marker appended to the PR body at the single composition site
  (`createPRCmd`), for both AI and non-AI paths.
- `github.Client.PRDetails(ctx, url)` via `gh pr view <url> --json headRefName,body`, degrading
  non-crashing like `PRForBranch`.
- New cobra subcommand `deploydeck pr verify <url>`.
- Exported `owner/repo` helper over `internal/github`'s existing `parseOrigin`.
- goreleaser `-ldflags -X internal/provenance.secret=…` from a GitHub Actions secret.

### Out of Scope
- GitHub App / bot-identity attribution (the strong control).
- Marking the compare-URL manual flow — no PR object exists there to mark; the marker only
  applies where DeployDeck itself creates the PR via `gh`.
- Any re-sign / refresh flow (marker is written once, never rewritten).
- `gh` minimum-version prerequisite check beyond graceful degrade.
- Rotating or per-run secrets; secret storage hardening.

## Approach — Decisions

| # | Decision | Rationale |
|---|----------|-----------|
| D1 | `sig = HMAC-SHA256(secret, "owner/repo\|headBranch\|runID")` truncated to 16 hex chars. Payload deliberately EXCLUDES headSHA. | Repo+branch bind the marker to *this* PR (copy elsewhere fails verify); runID links to the local run record; excluding headSHA lets incremental-promotion appends land without invalidating the signature. |
| D2 | Marker = invisible `<!-- deploydeck: v1 run:<RunID> sig:<sig> -->`; footer = visible `Created with DeployDeck v<version>`. Marker carries a `v1` scheme tag. | `v1` leaves room to evolve the payload; footer is the human-visible signal, marker is the machine-verifiable one. Version sourced from `internal/version.Version`. |
| D3 | PURE `internal/provenance` (Sign/Render*/ParseMarker/Verify); `var secret` injected only at release via goreleaser `-X`; dev/snapshot builds keep the zero value. | Keeps crypto+string logic exec-free and unit-testable; secret never lands in source or dev builds. |
| D4 | **Empty-secret = dev build, NOT a real signature.** With `secret == ""`, `Sign` refuses to emit a normal HMAC and produces a distinguishable dev marker (`sig:dev`). `pr verify` reports "dev-signed / unverifiable (development build)" distinctly, exiting non-zero. | An empty HMAC key is *publicly known*, so an empty-key signature is trivially forgeable by any locally-built binary and would be indistinguishable from a release signature — a false-authenticity vector. Making dev builds explicitly unverifiable is the honest choice. (Open point 1.) |
| D5 | `createPRCmd` captures `m.runID`+`m.originURL` and calls pure provenance to append footer+marker to `effectiveDescription()`. On the non-AI path the body becomes footer+marker only — **accepted as the intended default UX**. | The visible footer IS meaningful body content ("this PR came from DeployDeck"); inventing an extra default line adds copy to maintain for no gain. The exec boundary (`gh.CreatePR`) is untouched. (Open point 2.) |
| D6 | `github.Client.PRDetails(ctx, url)` runs `gh pr view <url> --json headRefName,body`; exec stays in `internal/github`; missing `gh`/field mismatch degrades like `PRForBranch` (exit-code-as-data), never crashes. | Minimal, stable field set; consistent with existing degrade tolerance. |
| D7 | `deploydeck pr verify <url>` derives owner/repo from the URL ARGUMENT (not `gh` `headRepository*`, which reports the fork on cross-fork PRs), fetches headRefName+body via `PRDetails`, parses the marker, recomputes HMAC, exits 0 (verified) / non-zero (missing/invalid/dev). Wires its own `github.New(exec.NewOSRunner())`, mirroring `newRunsCmd`. | The marker is signed against the BASE repo; deriving owner/repo from the URL avoids fork mismatch. No `app.Deps` coupling. |
| D8 | Export a bare `owner/repo` helper over the existing unexported `parseOrigin` in `internal/github/compare_url.go` instead of duplicating the SSH/HTTPS regex in `internal/provenance`. | Origin parsing stays single-sourced; pure string logic, no exec. |
| D9 | Marker is written ONCE at PR creation and never refreshed; the incremental reuse-and-append path (open PR exists → `createPRCmd` skipped) leaves the original marker intact. | Sound because all three payload fields — repo, headBranch, runID — are stable across that lifecycle (same RunID asserted by the incremental-promotion tests). No refresh needed; design must not assume per-push body regeneration. (Open point 3.) |

## Capabilities

### New Capabilities
- `pr-provenance`: signature scheme + marker/footer format, `internal/provenance`
  Sign/Render/Parse/Verify contract, dev-build (empty-secret) behavior, `PRDetails` fetch, and the
  `deploydeck pr verify <url>` subcommand semantics (verified / invalid / dev-signed / degraded).

### Modified Capabilities
- `push-pr-preparation`: "PR Creation Requires Explicit Confirmation And Records The URL" — the
  submitted PR body now always ends with the visible footer + invisible marker (both AI and
  non-AI paths); footer-only body is the accepted default on the non-AI path.
- `release-pipeline`: NEW requirement (sibling of "Injected Version Matches Build ldflags") —
  release builds inject `internal/provenance.secret` from a CI secret via goreleaser `-ldflags
  -X`; snapshot/dev builds carry the zero-value secret and produce dev markers only.

## Affected Areas

| Area | Impact | Change |
|------|--------|--------|
| `internal/provenance` | New | Pure Sign/Render/Parse/Verify + injected `var secret`. |
| `internal/app/commands.go` (`createPRCmd`) | Modified | Capture runID+originURL; append footer+marker to body. |
| `internal/github/client.go` + `compare_url.go` | Modified | New `PRDetails`; exported `owner/repo` helper over `parseOrigin`. |
| `cmd/deploydeck` | New | `pr verify <url>` subcommand (runs-cmd wiring pattern). |
| `.goreleaser.yaml` + `.github/workflows/release.yml` | Modified | `-X provenance.secret` ldflag + CI secret env. |

## Risks

| Risk | Likelihood | Mitigation |
|------|-----------|------------|
| Secret extracted from released binary → forged markers | High (accepted) | Out of scope by design; documented threat model; GitHub App is the strong path. |
| Empty-key signature mistaken for real | Med | D4: dev builds emit `sig:dev`, never a normal HMAC; verify reports dev distinctly. |
| Touching stable `compare_url.go`/tests to export owner/repo | Med | D8: thin exported wrapper over existing `parseOrigin`; no regex duplication. |
| `gh pr view --json` field mismatch on old `gh` | Med | D6: degrade like `PRForBranch` (exit-code-as-data), never crash. |
| >400 changed lines (new pkg + cmd + wiring, strict-TDD) | High | Single PR, `size:exception` pre-approved. |

## Rollback Plan

Additive and gated at the composition site. Revert the change to stop appending footer+marker and
drop the `pr verify` subcommand; existing PRs keep their (now unverifiable) markers harmlessly.
Remove the goreleaser ldflag to stop secret injection. No schema change, no migration.

## Success Criteria

- [ ] A DeployDeck-created PR (release build) verifies green via `deploydeck pr verify <url>`.
- [ ] A marker copied to a different repo/branch fails verification.
- [ ] Dev/snapshot builds emit a `sig:dev` marker; verify reports "dev-signed", exits non-zero.
- [ ] Both AI and non-AI paths submit a body ending with footer+marker; footer-only body is
      accepted on the non-AI path.
- [ ] Incremental reuse-and-append leaves the original marker valid (RunID/branch/repo stable).
- [ ] `pr verify` degrades (clear message, non-zero) when `gh` is missing/unauthenticated.

## Delivery Note

Single PR, strict TDD. `size:exception` pre-approved (>800 lines allowed) given the new package,
subcommand, and CI wiring. Next recommended phases: `sdd-spec` and `sdd-design` (parallel).
