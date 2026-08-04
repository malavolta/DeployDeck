# Tasks: PR Provenance (verifiable DeployDeck-created marker)

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | ~1100-1250 (2 new packages + 1 new cmd file + 4 modified files + strict-TDD RED coverage for each; design risk: High) |
| 400-line budget risk | High |
| Chained PRs recommended | No |
| Suggested split | Single PR, 3 sequential internal work units (pure leaf → app wiring → cmd+CI wiring) |
| Delivery strategy | exception-ok |
| Chain strategy | size-exception |

Decision needed before apply: No
Chained PRs recommended: No
Chain strategy: size-exception
400-line budget risk: High

Rationale: `size:exception` is pre-approved for this change (>800 lines allowed).
Unit 2 (app wiring) hard-depends on Unit 1's `provenance.Compose`/`github.OwnerRepo`
compiling first, and Unit 3 (`pr verify`) depends on Unit 1's `provenance.Verify`/
`github.PRDetails`+`ParsePRURL`. Splitting into separate PRs would add review-order
friction without shrinking the reviewer's real surface (one new capability, one
composition site, one new subcommand). The 3 units stay the rollback/commit
boundary inside the one PR.

### Suggested Work Units (sequential commits inside the single PR)

| Unit | Goal | Focused test command | Runtime harness | Rollback boundary |
|------|------|----------------------|-----------------|-------------------|
| 1 | Pure `internal/provenance` + `internal/github` primitives (`OwnerRepo`/`ParsePRURL`/`PRDetails`) | `go test ./internal/provenance/... ./internal/github/... -race -run "Sign\|Marker\|Verify\|Compose\|OwnerRepo\|ParsePRURL\|PRDetails"` | N/A — pure logic + `FakeRunner`-backed github tests; no real `gh`/network call needed | Revert `internal/provenance/` (whole new package) and the `OwnerRepo`/`ParsePRURL`/`PRDetails` additions to `internal/github` — zero callers yet |
| 2 | `internal/app` `createPRCmd` composes footer+marker via `provenance.Compose` | `go test ./internal/app/... -race -run "CreatePR\|Provenance"` | N/A — `FakeRunner`-backed `github.Client`, same pattern as existing `push_preparation_test.go`; no real `gh`/network needed | Revert the `createPRCmd` `ownerRepo`/`Compose` call (~15 lines) — `gh.CreatePR` and the exec boundary stay untouched, body reverts to pre-provenance content |
| 3 | `cmd/deploydeck pr verify <url>` + goreleaser/release.yml/docs wiring | `go test ./cmd/deploydeck/... -race -run "PRVerify"` | N/A for tests (`FakeRunner`-backed cobra `Execute`); `goreleaser check` locally if installed, else CI-validated on next PR | Delete `cmd/deploydeck/pr.go`, revert `main.go`'s registration/exit-branch, and the 2-line `.goreleaser.yaml`/`release.yml` ldflag/env additions — independent of Units 1-2, existing commands unaffected |

## Phase 1: `internal/provenance` (pure leaf)

- [x] 1.1 RED: `internal/provenance/provenance_test.go` — `Sign` determinism, `hex[:16]` lowercase truncation, payload canon (`owner/repo` lowered ONLY, `headBranch`/`runID` case-exact, `|` separators), dev sentinel (`secret==""` → `Sign` returns literal `"dev"`). Compile fails (package/`Sign` undefined).
- [x] 1.2 RED: same file — `RenderMarker`↔`ParseMarker` roundtrip; absent marker → `ok=false`; foreign/`v2`/garbage → `ok=false`.
- [x] 1.3 RED: same file — `RenderFooter` (injected `internal/version.Version` → `Created with DeployDeck vX.Y.Z`; default `"dev"` → dev-build variant) + `Verify` (`Verified` / `Mismatch` ×4: tampered sig, wrong repo, wrong branch, wrong runID / `DevMarker` / `DevVerifier`) + `Compose` (body+footer+marker; empty body→footer+marker; `ownerRepo==""`→footer ONLY, no marker; marker count==1 via `ParseMarker`).
- [x] 1.4 RED: `internal/provenance/boundary_test.go` — `TestProvenance_NeverImportsExecNetGithub` via `build.ImportDir` forbids `os/exec`, `internal/exec`, `net/http`, `internal/github`. Compile fails (package undefined).
- [x] 1.5 GREEN: `internal/provenance/provenance.go` — `var secret`; `Sign`/`RenderFooter`/`RenderMarker`/`ParseMarker`/`Verify`/`Compose` + `Result` enum (`Verified`/`Mismatch`/`DevMarker`/`DevVerifier`); satisfies 1.1-1.4.
- [x] 1.6 Verification: `PATH=/usr/local/go/bin:$PATH go test ./internal/provenance/... -race -v` — GREEN, all cases pass.

## Phase 2: `internal/github` (OwnerRepo / ParsePRURL / PRDetails)

- [x] 2.1 RED: `internal/github/compare_url_test.go` — `OwnerRepo` cases (ssh/https/ssh-url → `"owner/repo"`; junk → `ok=false`) + `ParsePRURL` cases (enterprise host, trailing slash, non-PR path → `ok=false`). Compile fails (`OwnerRepo`/`ParsePRURL` undefined).
- [x] 2.2 GREEN: `internal/github/compare_url.go` — export `OwnerRepo(originURL)` wrapping `parseOrigin`; add `ParsePRURL(prURL)` + anchored PR-URL regex (`https://<host>/<owner>/<repo>/pull/<n>`).
- [x] 2.3 RED: `internal/github/pr_details_test.go` — `PRDetails` via `FakeRunner`: success (`headRefName`+`body` parsed); `gh` missing (Runner err) → `err`; exit≠0 → `err`; malformed JSON → `err` — all non-panic. Compile fails (`PRDetails` undefined on `Client`).
- [x] 2.4 GREEN: `internal/github/client.go` — add `PRDetails(ctx, url)` to `Client` interface + `client` impl (`gh pr view <url> --json headRefName,body`) + `prDetails` JSON struct.
- [x] 2.5 Verification: `PATH=/usr/local/go/bin:$PATH go test ./internal/github/... -race -v` — GREEN.

## Phase 3: `internal/app` (createPRCmd composition)

- [x] 3.1 RED: `internal/app/create_pr_provenance_test.go` — AI-accepted path (`aiDescription`+footer+marker); non-AI path (footer+marker only, no longer empty); marker appears exactly once (`ParseMarker` count); sig bound to `owner/repo|head|runID` (`provenance.Verify` returns `Verified` against `m.originURL`-derived `ownerRepo`, `m.plan.PromotionBranch`, `m.runID`). Fails against the unmodified `createPRCmd` (still submits bare title/body). **Deviation** (documented in apply-progress/return summary): the "sig bound...returns Verified" sub-case is proven at the `internal/provenance` unit level (Phase 1's `TestVerify`, real injected secret) rather than inside this app-level test, because `provenance.secret` is package-private and app-level test binaries always run with the dev-sentinel `secret==""` (ldflags-only injection) — under that sentinel `Verify` always short-circuits to `DevMarker`/`DevVerifier` before ever reaching a byte comparison, so "Verified" is not reachable from outside the `provenance` package. This test instead proves wiring correctness (exact-match against an independently-computed `provenance.Compose(...)` call, plus a dedicated `RunIDWiring` sub-test varying `m.runID`).
- [x] 3.2 RED: same file — unparseable origin (an `originURL` `OwnerRepo` rejects) degrades to footer-only body, NO marker, `gh.CreatePR` still called normally.
- [x] 3.3 GREEN: `internal/app/commands.go` — `createPRCmd` captures `m.runID`+`m.originURL`, derives `ownerRepo` via `github.OwnerRepo(m.originURL)`, calls `provenance.Compose(body, ownerRepo, head, m.runID)` before `gh.CreatePR`. Also updated pre-existing body-literal assertions in `push_preparation_test.go`, `push_preparation_ai_test.go`, and `push_pr_e2e_test.go` (none set `m.originURL`, so all degrade to the footer-only/description+footer body) — required by the push-pr-preparation spec's modified behavior, not a scope violation.
- [x] 3.4 Verification: `PATH=/usr/local/go/bin:$PATH go test ./internal/app/... -race -run "CreatePR|Provenance"` GREEN, then full `PATH=/usr/local/go/bin:$PATH go test ./internal/app/... -race` confirming `TestApp_NeverImportsExecSeam` still passes (provenance import adds no exec/net/github).

## Phase 4: `cmd/deploydeck` (`pr verify` subcommand)

- [x] 4.1 RED: `cmd/deploydeck/pr_verify_test.go` — cobra `Execute("pr","verify",<url>)` + `runPRVerify(w, gh, url)` core via `FakeRunner`-backed `github.Client`: verified→0; forged/mismatch→1; no marker→2; dev-signed marker→3; dev-BUILT verifier against a genuine release marker→3 (never reported invalid/forged); `gh` degraded (missing/unauth, `PRDetails` err, unparseable URL)→4; cross-repo/branch copied marker→1 (mismatch, not verified). Compile fails (`newPRCmd`/`runPRVerify` undefined). **Deviation** (documented, same root cause as 3.1): `go test` never links the release ldflags secret, so the real `provenance.Verify` can only return `DevMarker`/`DevVerifier` in this binary — `Verified`/`Mismatch` are architecturally unreachable without a real secret (by design: a dev-built verifier must never report forged). Added a package-private `verifyFn = provenance.Verify` seam in `pr.go` so `Verified`/`Mismatch` cases are tested deterministically via injection; `DevMarker`/`DevVerifier`/no-marker/degraded cases use the real `provenance`/`github` functions with no seam.
- [x] 4.2 GREEN: `cmd/deploydeck/pr.go` — `newPRCmd` (parent) + `newPRVerifyCmd` (wires `github.New(exec.NewOSRunner())`) + `runPRVerify(w io.Writer, gh github.Client, url string) int` implementing `ParsePRURL`→`PRDetails`→`ParseMarker`→`Verify`→exit-code mapping + `exitError` type for message formatting.
- [x] 4.3 GREEN: `cmd/deploydeck/main.go` — register `newPRCmd()` in `newRootCmd`; add exit-code-aware branch in `main()` (`exitCodeFor` helper) so `runPRVerify`'s distinct non-zero codes (1-4) propagate via `os.Exit` instead of the generic `err != nil → exit 1` path masking them.
- [x] 4.4 Verification: `PATH=/usr/local/go/bin:$PATH go test ./cmd/deploydeck/... -race -v` — GREEN, all 5 exit-code paths covered.

## Phase 5: Build/CI wiring (no Go tests)

- [x] 5.1 Modify `.goreleaser.yaml` — add `-X github.com/malavolta/DeployDeck/internal/provenance.secret={{ envOrDefault "PROVENANCE_SECRET" "" }}` to `builds[0].ldflags`, beside the existing version ldflags.
- [x] 5.2 Modify `.github/workflows/release.yml` — add `PROVENANCE_SECRET: ${{ secrets.PROVENANCE_SECRET }}` to the goreleaser step's `env:` block.
- [x] 5.3 Modify `docs/ACTIVATION-CHECKLIST.md` — document `PROVENANCE_SECRET` as an additional release-secret prerequisite (fail-soft: an unset secret yields dev-signed release markers, never a build crash).
- [x] 5.4 Verification: `goreleaser` is NOT installed locally in this environment (`goreleaser check` unavailable) — both modified YAML files were validated for well-formedness (`ruby -ryaml`, since `python3` here has no `yaml` module), and `ci.yml`'s existing `goreleaser check`/`--snapshot` job will validate the new ldflag template on the next PR/CI run — no local Go test covers this phase.

## Phase 6: Final Gates

- [x] 6.1 `PATH=/usr/local/go/bin:$PATH go build ./...` — clean, exit 0.
- [x] 6.2 `PATH=/usr/local/go/bin:$PATH go vet ./...` — clean, exit 0.
- [x] 6.3 `gofmt -l internal cmd` — empty output.
- [x] 6.4 `PATH=/usr/local/go/bin:$PATH go test ./... -race -count=1` — full suite GREEN (14 packages), including `TestApp_NeverImportsExecSeam` and `TestProvenance_NeverImportsExecNetGithub`.
