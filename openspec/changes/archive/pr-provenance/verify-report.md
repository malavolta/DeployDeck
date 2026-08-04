# Verification Report: PR Provenance

**Change**: `pr-provenance`
**Branch**: `feat/pr-provenance` (uncommitted working tree — CURRENT tree verified)
**Mode**: Strict TDD · Full artifacts (proposal + 3 delta specs + design + tasks + apply-progress)
**Verdict**: **PASS**
**Executive summary**: 0 CRITICAL · 0 WARNING · 2 SUGGESTION. All 6 requirements / 23 scenarios have covering implementation + passing tests; all 27 tasks done and match code; build/vet/gofmt clean; targeted `-race` suites green; both boundary invariants hold; the 3 documented deviations are legitimate test strategy, not contract drift.

## Completeness

| Dimension | Result |
|---|---|
| Tasks complete | 27/27 checked, all spot-checked against real symbols/files |
| Specs mapped | 3 delta specs → 6 requirements → 23 scenarios all covered |
| Design coherence | Implementation matches design.md (signatures, exit codes, payload, data flow) |
| Deviations audited | 3, all legitimate (see below) |

## Gate Evidence (run on current tree)

| Gate | Command | Result |
|---|---|---|
| Build | `go build ./...` | exit 0, clean |
| Vet | `go vet ./...` | exit 0, clean |
| Format | `gofmt -l internal cmd` | empty output |
| Targeted race | `go test ./internal/provenance/... ./internal/github/... ./cmd/deploydeck/... -race -count=1` | exit 0 — all 3 packages `ok` |
| App provenance+boundary | `go test ./internal/app/... -race -run "CreatePR\|Provenance\|NeverImports\|PushPreparation\|HU014_PushPR"` | exit 0 — incl. `TestApp_NeverImportsExecSeam` PASS |

Full-suite `-race` (14 packages) was run independently by the orchestrator (apply-progress final gates, all green) and not repeated here.

## Spec Compliance Matrix

### Capability: pr-provenance (4 requirements / 12 scenarios)

| Requirement / Scenario | Covering test(s) | Status |
|---|---|---|
| HMAC scheme — deterministic 16-hex lowercase | `provenance.TestSign_Deterministic` | PASS |
| HMAC scheme — owner/repo casing doesn't break verify | `provenance.TestSign_PayloadCanonicalization` (owner/repo lowered, branch/runID case-exact, `\|` separators) | PASS |
| HMAC scheme — appended commits (headSHA excluded) | Structural: `Sign(ownerRepo, headBranch, runID)` has no SHA param; asserted by payload-canon test | PASS |
| Marker parses with surrounding content | `provenance.TestMarker_RoundTrip` | PASS |
| Absent marker → not-found, no error | `provenance.TestParseMarker_AbsentAndForeign` (absent/empty/v2/garbage/short-sig) | PASS |
| Footer format vX.Y.Z + dev variant | `provenance.TestRenderFooter` | PASS |
| Dev sentinel — empty secret → `sig:dev` (never empty-key HMAC) | `provenance.TestSign_DevSentinel_EmptySecret` | PASS |
| Dev marker → dev-signed non-zero (not verified) | `provenance.TestVerify` (DevMarker) + `cmd.TestRunPRVerify_DevSignedMarker` (exit 3) | PASS |
| `pr verify` genuine → exit 0 | `cmd.TestRunPRVerify_Verified` (mapping) + `provenance.TestVerify` "verified" (real secret) | PASS |
| `pr verify` cross-repo/branch copy → exit 1 mismatch | `cmd.TestRunPRVerify_CrossRepoBranchCopiedMarker_Mismatch` + `provenance.TestVerify` wrong repo/branch/runID/tampered | PASS |
| `pr verify` missing marker → exit 2 | `cmd.TestRunPRVerify_NoMarker` | PASS |
| `pr verify` dev-built verifier on release marker → exit 3, **never** invalid/forged | `cmd.TestRunPRVerify_DevBuiltVerifier_GenuineMarker` (REAL provenance.Verify, no seam → DevVerifier) | PASS |
| `pr verify` gh degraded → exit 4, no crash | `cmd.TestRunPRVerify_Degraded` (gh missing / non-zero exit / malformed JSON / unparseable URL) | PASS |

### Delta: push-pr-preparation (1 modified requirement / 9 scenarios)

| Scenario | Covering test(s) | Status |
|---|---|---|
| AI-accepted body gains footer + marker | `app.TestCreatePRCmd_Provenance_AIAcceptedPath` (exact-match vs independent `Compose`) | PASS |
| Non-AI path → footer-and-marker-only (no longer empty) | `app.TestCreatePRCmd_Provenance_NonAIPath` | PASS |
| runID threaded into marker | `app.TestCreatePRCmd_Provenance_RunIDWiring` (2 runIDs) | PASS |
| Unparseable origin → footer only, NO marker, create still proceeds | `app.TestCreatePRCmd_Provenance_UnparseableOrigin_FooterOnly` + `provenance.TestCompose` (ownerRepo=="") | PASS |
| Marker written exactly once | `TestCompose` / `TestCreatePRCmd_Provenance_*` (`strings.Count(...)==1`) | PASS |
| Reuse-and-append leaves original marker intact (no refresh) | Structural: `Compose` lives only in `createPRCmd`, skipped on open PR; `app.TestPreparePRCmd_Reusing_SkipsCreateWhenPROpen`, `TestIncrementalPromotion_E2E_ReuseAppendsOnlyNewCommits_SkipsCreateWhenPROpen` (still green) | PASS (no regression) |
| Pre-existing create/confirm/AI-title/open-PR/degrade scenarios | Pre-existing `TestModel_PushPreparation_*` (body-literal assertions updated to footer shape) all green | PASS |

### Delta: release-pipeline (1 added requirement / 2 scenarios)

| Scenario | Covering evidence | Status |
|---|---|---|
| Release build signs verifiably (secret via ldflags) | `.goreleaser.yaml` `builds[0].ldflags` += `-X …/internal/provenance.secret={{ envOrDefault "PROVENANCE_SECRET" "" }}`; `.github/workflows/release.yml` goreleaser step env += `PROVENANCE_SECRET` | PASS (config inspection; goreleaser not installed locally — CI `goreleaser check` validates on next run) |
| Snapshot build emits dev marker (empty secret) | `envOrDefault` defaults to `""` → `secret==""` → `sig:dev`; behaviorally proven by `provenance.TestSign_DevSentinel_EmptySecret` | PASS |
| Activation doc | `docs/ACTIVATION-CHECKLIST.md` documents `PROVENANCE_SECRET` fail-soft prerequisite | PASS |

## Boundary / Architecture Invariants

| Check | Result |
|---|---|
| `internal/provenance` imports no exec/net/github | PASS — `TestProvenance_NeverImportsExecNetGithub` green; imports = crypto/hmac, crypto/sha256, encoding/hex, fmt, regexp, strings, internal/version |
| `TestApp_NeverImportsExecSeam` still green after provenance import | PASS |
| No new exec outside internal/git\|github | PASS — `pr.go` uses `exec.NewOSRunner()` at the cmd composition root (same pre-existing pattern as `main.go:204/277`); real command exec confined to `internal/github` (`PRDetails` `gh pr view`) |

## Deviations Audit

1. **Same-package `secret` mutation (`withSecret`) for Verified/Mismatch in `provenance_test.go`** — LEGITIMATE. Internal (`package provenance`) test with `t.Cleanup` restore, standard Go pattern for a build-injected package var (modeled on `internal/ai`'s `doctorProbeTimeout`). Production ldflags-injection behavior unchanged. Not drift.
2. **`verifyFn = provenance.Verify` seam in `cmd/deploydeck/pr.go`** — LEGITIMATE (planned in design.md + tasks.md). `go test` never links the release secret, so `Verified`/`Mismatch` are architecturally unreachable through the real crypto path in this binary; the seam tests ONLY the Result→exit-code mapping, while the actual classification is proven with a real secret in `provenance.TestVerify`. `DevMarker`/`DevVerifier`/no-marker/degraded cases use the REAL functions (no seam), including the security-critical dev-verifier-on-release-marker→3 case. Public signature/behavior of `runPRVerify` unchanged. Not drift.
3. **3 pre-existing app test files updated (`push_preparation_test.go`, `push_preparation_ai_test.go`, `push_pr_e2e_test.go`)** — LEGITIMATE. Required consequence of the MODIFIED push-pr-preparation requirement ("body SHALL always end with the visible footer"). These tests set no `m.originURL`, so they correctly degrade to footer-only / description+footer. Leaving them would leave the full suite red. Not scope creep.

## Findings

- **CRITICAL**: none.
- **WARNING**: none.
- **SUGGESTION 1**: The cmd-level `Verified→0` / `Mismatch→1` exit mappings are exercised through the `verifyFn` seam rather than the real crypto path (unavoidable without a linked release secret). Combined coverage is complete (Result classification proven at provenance level with a real secret; mapping proven at cmd level), but a future e2e that builds with a real test secret and drives the full cobra path would close the last synthetic gap. Non-blocking.
- **SUGGESTION 2**: Phase 5 (goreleaser ldflag / release.yml env) has no executable test locally (goreleaser not installed); it relies on CI `goreleaser check` on the next PR. Consider a lightweight CI assertion that the built binary reports a non-`dev` signature on release to guard the ldflag path end-to-end. Non-blocking.

## Verdict

**PASS** — ready for archive.
