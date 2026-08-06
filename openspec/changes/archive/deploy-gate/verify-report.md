# Verification Report: Deploy Gate

- **Change**: `deploy-gate` (per-environment governance check before quick-deploy)
- **Branch**: `feat/deploy-gate` (uncommitted working tree — CURRENT tree verified)
- **Mode**: Full spec-driven verification (proposal + 3 delta specs + design + tasks), Strict TDD
- **Artifact store**: OpenSpec (Engram unavailable — no mem_* calls)
- **Runner**: `PATH=/usr/local/go/bin:$PATH go test ... -race`
- **Verdict**: **PASS WITH WARNINGS**

## Completeness

| Dimension | Result |
|---|---|
| Delta-spec scenarios (deploy-gate 23 + quick-deploy 3 + validation-progress 3) | 29/29 mapped to implementation AND a passing covering test |
| Tasks | 69/69 checked; spot-checked symbols/files all exist and match code state |
| Design coherence | D1–D11 realized; no undocumented deviation |

## Build / Test Evidence (this tree)

| Gate | Command | Result |
|---|---|---|
| build | `go build ./...` | exit 0 (clean) |
| vet | `go vet ./...` | exit 0 (clean) |
| gofmt | `gofmt -l internal cmd` | empty (clean) |
| targeted -race | `go test ./internal/config/... ./internal/github/... ./internal/gate/... ./internal/provenance/... ./cmd/deploydeck/... ./internal/app/... -race -count=1` | exit 0 — all 6 packages `ok` |

Per-package: config 1.36s · github 1.91s · gate 1.64s · provenance 2.16s · cmd/deploydeck 6.15s · app 34.83s — all GREEN. (Full suite run separately by orchestrator; not repeated.)

## Spec Compliance Matrix (29/29)

### Capability: Deploy Gate (23)

| # | Scenario | Covering test | Status |
|---|---|---|---|
| 1 | exact-match resolves over glob | `config.TestConfig_GateFor` (exact-wins subtest) | PASS |
| 2 | no matching entry → ungated | `config.TestConfig_GateFor` (no-match → false) | PASS |
| 3 | enabled gate w/ invalid approvals rejected (min<1, empty approvers) | `config.TestConfig_Validate_Gates` (min<1 + empty-approvers + nil-ok) | PASS |
| 4 | gated deploy proceeds when all conditions pass | `app.TestOnGateCheckDone_Passed_DispatchesQuickDeployCmd` + `gate.TestEvaluate_ToggleWiring` | PASS |
| 5 | gated deploy blocked when any condition fails | `app.TestOnGateCheckDone_Blocked_TransitionsWithoutDispatchingQuickDeploy` + `gate.TestEvaluate_ToggleWiring` (unresolved-thread blocks) | PASS |
| 6 | enough listed approvals satisfy (min=2) | `gate.TestEvaluate_Approvals` (explicit min=2 subtest) | PASS |
| 7 | non-listed approvals do not count | `gate.TestEvaluate_Approvals` (mallory subtest) | PASS |
| 8 | later CHANGES_REQUESTED negates earlier APPROVED | `gate.TestEvaluate_Approvals` (negation subtest) | PASS |
| 9 | all threads resolved passes | `gate.TestEvaluate_Threads` (count==0) + `github.TestClient_UnresolvedThreadCount_AllResolvedIsZero` | PASS |
| 10 | one unresolved thread blocks | `gate.TestEvaluate_Threads` (count==1) + `github.TestClient_UnresolvedThreadCount_CountsUnresolved` | PASS |
| 11 | thread status unavailable fails closed | `gate.TestEvaluate_Threads` (ThreadErr) + `github.TestClient_UnresolvedThreadCount_Degrades` | PASS |
| 12 | successful validation on gated env posts marker comment | `app.TestOnReportDone_TerminalSuccess_GateEnabledRequireCommentOn_TriggersPostComment` + `TestPostValidationCommentCmd_PostsWhenNoMarkerPresent` + `github.TestClient_PostComment_Success` | PASS |
| 13 | re-validation does not duplicate | `app.TestPostValidationCommentCmd_SkipsWhenMarkerAlreadyPresent` + `gate.TestHasValidationComment` | PASS |
| 14 | condition passes only when marker present | `gate.TestEvaluate_ValidationComment` (present passes / absent fails) | PASS |
| 15 | ungated env posts no comment | `app.TestOnReportDone_TerminalSuccess_UngatedTarget_DoesNotTriggerPostComment` | PASS |
| 16 | valid signature passes | `gate.TestEvaluate_Signature` (Verified) | PASS |
| 17 | missing/invalid signature fails | `gate.TestEvaluate_Signature` (Mismatch) + `provenance.TestBestResult_AnyOfClassification` (real secret) | PASS |
| 18 | dev-signed marker fails | `gate.TestEvaluate_Signature` (DevMarker/DevVerifier) | PASS |
| 19 | recorded PR URL resolves PR | `app.TestResolvePRURL` (PRUrl set) | PASS |
| 20 | empty PR URL → branch fallback | `app.TestResolvePRURL` (PRForBranch fallback) | PASS |
| 21 | no resolvable PR blocks | `app.TestResolvePRURL` (neither → false) + `gate.TestEvaluate_PRResolutionFailsClosed` + `app.TestGateCheckCmd_NoResolvablePR_SkipsAllFourReads` | PASS |
| 22 | all unmet conditions listed together | `app.TestViewDeployGateBlocked_ListsEveryUnmetCondition` | PASS |
| 23 | no override exists | `app.TestViewDeployGateBlocked_NoOverrideKeyOffered` + `app.TestKeyDeployGateBlocked_NoOverrideKeyBypassesTheBlock` | PASS |

### Capability: Quick Deploy delta (3)

| # | Scenario | Covering test | Status |
|---|---|---|---|
| 24 | fully authorized execution runs & registers | `app.TestOnGateCheckDone_Passed_DispatchesQuickDeployCmd` (pass → quickDeployCmd) + `TestKeyQuickDeploy_Enter_UngatedTarget_StillDispatchesQuickDeployCmd` | PASS |
| 25 | enabled gate w/ unmet condition blocks dispatch (NO quick deploy) | `app.TestKeyQuickDeploy_Enter_GatedTarget_DispatchesGateCheckNotQuickDeploy` (asserts NO sf call) + `TestOnGateCheckDone_Blocked_...` | PASS |
| 26 | no enabled gate deploys unchanged | `app.TestKeyQuickDeploy_Enter_UngatedTarget_StillDispatchesQuickDeployCmd` | PASS |

### Capability: Validation Progress delta (3)

| # | Scenario | Covering test | Status |
|---|---|---|---|
| 27 | successful CheckOnly on gated env triggers comment | `app.TestOnReportDone_TerminalSuccess_GateEnabledRequireCommentOn_TriggersPostComment` | PASS |
| 28 | successful CheckOnly on ungated env posts nothing | `app.TestOnReportDone_TerminalSuccess_UngatedTarget_DoesNotTriggerPostComment` | PASS |
| 29 | non-successful terminal state does not trigger | `app.TestOnReportDone_NonSuccessfulTerminal_GatedTarget_DoesNotTriggerPostComment` | PASS |

Additional (beyond the 29): config parse `*bool`/`*int` nil-vs-explicit (`TestLoad_GatesSection_RequireTogglesNilVsExplicitFalse`, `..._MinApprovalsNilVsExplicit`); `github.ParsePRURLParts` + delegation guard; `gate.TestAggregateCoverage`, `TestValidationComment`, `TestNormalizeLogin`; `app.TestOnReportDone_TerminalSuccess_RequireCommentOff_DoesNotTrigger`, `TestOnPostCommentDone_Failure_DoesNotAlterTerminalSuccessState`.

## Boundary / Architecture Invariants (item 4)

| Invariant | Evidence | Status |
|---|---|---|
| `internal/gate` exec-free (deps only config+provenance) | `gate/boundary_test.go` (non-vacuous: requires config+provenance; forbids os/exec, internal/exec, net/http, internal/github) — GREEN | PASS |
| `internal/provenance` exec-free | `provenance/boundary_test.go` present, unchanged, GREEN | PASS |
| `internal/app` never imports exec | `app/boundary_test.go::TestApp_NeverImportsExecSeam` (non-vacuous: requires internal/git; forbids os/exec + internal/exec) — GREEN | PASS |
| all gh flows through `internal/github.Client` | new `PRReviews`/`UnresolvedThreadCount`/`PRComments`/`PostComment` live in `client.go`; gate pure; app calls via `m.deps.GH` | PASS |
| JSON tags on private envelope structs only | public `Review`/`Comment` carry NO json tags; wire structs `prReviewsView`/`reviewThreadsResponse`/`prCommentsView` are private + tagged | PASS |
| gh args discrete, never shell-joined | `client_gate_test.go` asserts exact arg slices per method (incl. graphql `-f`/`-F` discrete args) | PASS |

## Deviation Audit (item 5) — pr.go seam restructure

**Confirmed: zero deviation; the any-of proof was NOT weakened, it was strengthened.**

- `cmd/deploydeck/pr.go` replaced `verifyFn`/`bestVerifyResult`/`verifyResultRank` with `var bestResult = provenance.BestResult`; `runPRVerify` passes the full parsed-marker slice through unchanged.
- The sig-DISCRIMINATING any-of ranking proof genuinely lives in `internal/provenance/provenance_test.go::TestBestResult_AnyOfClassification`, run under a **real, non-dev secret** (`withSecret(t, "topsecret")`, verified via `Sign(...)`). Its cases (genuine-wins-over-prepended-bogus, genuine-wins-over-prepended-dev, mismatch-only, dev-only, dev-beats-mismatch, nil→zero) are a strict **superset** of the old per-marker `verifyFn` fake — and stronger, because the fake never exercised real signature computation.
- The restructured `pr_verify_test.go::TestRunPRVerify_AnyOfClassification` now proves pr.go's remaining responsibility only: ALL markers pass to `bestResult` in **document order** (exact `RunID` assertions) and Verified→exit 0.
- Exit-code mapping (Verified 0 / Mismatch 1 / Dev* 3 / unknown 4) preserved intact in `TestRunPRVerify_ExhaustiveResultMapping`.

## Regression Audit (item 6)

- `git diff --numstat` on all pre-existing modified `*_test.go`: **0 deletions** in config/validate/compare_url/keys/update/view/provenance tests (purely additive). Only `pr_verify_test.go` has deletions (the audited restructure above).
- Removed-assertion scan (grep of `-` lines for `t.Fatalf/Errorf/want/expected`): **empty** across all pre-existing test files.
- No `Contains`-for-exact weakening: restructured pr_verify assertions use exact equality (`gotMarkers[0].RunID != "bogus-run"`, exit-code `!=`).
- Quick-deploy ungated behavior intact: `TestKeyQuickDeploy_Enter_UngatedTarget_StillDispatchesQuickDeployCmd`; `keys.go` branches on `GateFor` and leaves the ungated `quickDeployCmd` dispatch untouched.

## Issues

### CRITICAL
- None.

### WARNING
- **W1 — Unresolved-threads pagination edge (`reviewThreads(first: 100)`).** A PR with more than 100 review threads truncates the unresolved count in `UnresolvedThreadCount`; if all unresolved threads fall beyond the first 100, the threads condition could PASS while unresolved threads remain — a latent gap against the requirement text "no unresolved review thread remains." This is a **documented, conscious deferral** (design.md Open Questions, unchecked: loop on `pageInfo.hasNextPage` only if a real PR is observed to exceed 100 threads). None of the 3 tested thread scenarios exercise >100 threads, and the path fails closed on any lookup error. Non-blocking; disclosed for awareness.

### SUGGESTION
- **S1 — Condition-3 cooperative caveat.** With `requireValidationComment:on` + `requireSignature:off`, the validation-comment condition has no anti-forgery backstop (marker presence is echoable by any commenter). Accepted per the cooperative trust model; design recommends pairing the two toggles in config docs. Consider surfacing that recommendation in user-facing config documentation.
- **S2 — Minor toggle-on duplication.** `commands.go::requireValidationCommentOn` reimplements the default-on `*bool` semantics rather than sharing gate's unexported `toggleOn`. Documented as a deliberate dispatch-gating decision; candidate for future consolidation if the pure package ever exports the helper.
- **S3 — Trivial helpers without dedicated unit tests.** `coverageTotals`/`coverageNotCovered`, `effectiveMinApprovals`, `resultRank` have no direct tests (covered indirectly by `TestPostValidationCommentCmd_*`, `TestEvaluate_Approvals`, `TestBestResult_AnyOfClassification`). Acceptable — no action required.

## Final Verdict

**PASS WITH WARNINGS.** All 29 delta-spec scenarios are implemented and covered by named, passing tests; 69/69 tasks match code reality; build/vet/gofmt/targeted -race all green on the current tree; architecture boundaries hold; the pr.go seam restructure preserved (and strengthened) the any-of signature proof with zero silent loss; no pre-existing test weakened; quick-deploy ungated behavior intact. The single WARNING (W1) is a documented, non-blocking deferral. No CRITICAL issues block archive.
