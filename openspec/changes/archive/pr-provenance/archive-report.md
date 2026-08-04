# Archive Report — pr-provenance

**Status:** archived · **Delivery:** single-PR (size:exception) · **Mode:** strict TDD · **Milestone:** v1.1.0

## Summary

Verifiable provenance for DeployDeck-created PRs. Every PR body DeployDeck submits now ends with a
visible footer (`Created with DeployDeck v<version>`) plus an invisible HTML-comment marker
`<!-- deploydeck: v1 run:<RunID> sig:<sig> -->`, where
`sig = HMAC-SHA256(secret, lower(host/owner/repo) + "\n" + headBranch + "\n" + runID)[:16 hex]`.
A new `deploydeck pr verify <url>` subcommand fetches the PR (`gh pr view --json headRefName,body`),
recomputes the signature, and reports distinct exit codes: 0 verified · 1 mismatch · 2 no marker ·
3 dev-unverifiable · 4 degraded/indeterminate. The secret is injected only into release builds via
goreleaser `-ldflags -X` from the `PROVENANCE_SECRET` Actions secret (`envOrDefault` — snapshot/dev
builds carry the empty secret and emit `sig:dev`, never an empty-key HMAC). Threat model is
deliberately minimal and documented: deters casual/manual forgery; the secret is extractable from
the released binary; a GitHub App is the strong (out-of-scope) alternative.

New capability `pr-provenance` (4 requirements); MODIFIED `push-pr-preparation` (body always ends
with footer+marker, sanitize-on-compose, footer-only degrade exception) and ADDED requirement in
`release-pipeline` (secret injection). New pure package `internal/provenance` (Sign/RenderFooter/
RenderMarker/ParseMarkers/Verify/Compose, own import-boundary test); `internal/github` gained
host-qualified `OwnerRepo`/`ParsePRURL` + `PRDetails`; `cmd/deploydeck` gained `pr verify`
(runs-cmd wiring pattern, testable `runPRVerify` core). The exec boundary holds — `internal/app`
only imports the pure provenance package.

## SDD trail

explore → propose → spec + design (parallel) → fresh-context design validation (FAIL → corrected)
→ tasks (27) → apply (27/27, strict TDD) → verify (PASS, 0 CRITICAL/0 WARNING) + full 4R
adversarial review (risk/reliability/resilience/readability, parallel) → consolidated remediation
→ archive. Artifact store: OpenSpec (Engram MCP unavailable).

## Design-validation corrections (pre-apply)

A fresh-context validator caught two spec↔design contradictions before implementation:
1. The spec's unconditional "always footer+marker" vs the design's footer-only degrade when
   `owner/repo` cannot be derived — resolved by spec'ing the exception explicitly (never write a
   marker whose signature could not be bound to the repository).
2. The signature formula did not state the payload's owner/repo lowercasing (GitHub is
   case-insensitive; creation-time remote casing can differ from verify-time URL casing) —
   formula text amended, scenario added.

## Review & remediation

Five parallel reviews (sdd-verify PASS 0C/0W; resilience CLEAN; readability CLEAN+1S; risk 2S;
reliability 1W+1S). All actionable findings were fixed in one consolidated strict-TDD pass —
possible without breakage because the v1 scheme was still unreleased:

- **Host-bound payload** (risk): the payload omitted the GitHub host, so a marker signed for
  `github.com/org/repo` would verify on an Enterprise host with the same org/repo/branch/runID.
  `OwnerRepo`/`ParsePRURL` now return host-qualified `host/owner/repo` and the payload signs it.
- **Injective separator** (reliability): fields were `|`-joined, but `|` is legal in git branch
  names (`Sign(o,"a|b","c") == Sign(o,"a","b|c")`), and the guard test exercised `/`, not `|`.
  Fields are now newline-joined — `\n` cannot occur in a git ref or the marker's `\S+` runID —
  with a true separator-injectivity test.
- **Sanitize-on-compose + any-of verification** (reliability WARNING + risk): an AI description
  echoing a prior marker would ship TWO markers with the stale one first, and first-match parsing
  let anyone who can edit a PR body prepend a fake marker to discredit the genuine one (false
  negative; never false positive). `Compose` now strips every pre-existing marker (exactly-one is
  unconditional) and `pr verify` classifies EVERY parsed marker, best outcome wins
  (verified > dev > mismatch) — a genuine marker can no longer be shadowed.
- **Exhaustive exit-code switch** (readability): explicit `case Mismatch:`; the `default:` now
  guards an unrecognized future `Result` to exit 4 (indeterminate), never a forgery accusation.

## Follow-up (accepted, non-blocking)

- Silent footer-only degrade on unparseable origin has no operator-visible notice (resilience
  SUGGESTION); spec'd as accepted — a one-line `m.notice` would be cheap if it ever matters.
- The goreleaser ldflag has no local executable test (goreleaser not installed); CI's
  `goreleaser check` + snapshot job validates it on every PR.
- `PROVENANCE_SECRET` must be set in the repo's Actions secrets before the next release for
  release binaries to sign verifiably; unset is fail-soft (dev markers), documented in
  `docs/ACTIVATION-CHECKLIST.md`.

## Spec merge note

Merged ADDITIVELY: NEW `pr-provenance` living spec (209 lines); `push-pr-preparation` requirement
block replaced with its superset (174→219, all 5 pre-existing scenarios preserved + 5 new);
`release-pipeline` new requirement appended after its ldflags sibling (95→119). Delta-only
`(Previously: …)` note dropped so living specs state current truth. No pre-existing requirement
or scenario was lost (verified by diffstat: +70/−1, the −1 being the replaced requirement
sentence).

## Verification

- `go build ./...`, `go vet ./...`, `gofmt -l internal cmd` clean.
- `go test ./... -race -count=1` green across all 14 packages (independently re-run by the
  orchestrator after remediation).
- Purity boundaries: `internal/provenance` imports no exec/net/github (own boundary test);
  `TestApp_NeverImportsExecSeam` still green.
- The dev-verifier-on-release-marker safety case (exit 3, never a forgery accusation) is proven
  against the REAL `provenance.Verify` with no test seam.
