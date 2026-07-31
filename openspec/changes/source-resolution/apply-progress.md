# Apply Progress: Source Resolution — Dedupe Candidates + Current-Branch Confirm

**Status**: All 22 tasks complete (Phases 1-5). Ready for `sdd-verify`.

## Final Verification

- `go build ./...` — clean
- `gofmt -l .` — clean (no output)
- `go vet ./...` — clean
- `go test ./... -race -short` — **ok** (all packages)
- `go test ./...` (full suite, real-git e2e included) — **ok** (all packages)
- Actual changed-line count (`internal/git` + `internal/app` diff): **769 insertions + 29 deletions = 798 lines** (within the 800-line session budget; no `size:exception` needed despite the tasks.md forecast flagging it as a possibility — final line count landed under budget after a trim pass).

## TDD Cycle Evidence (Strict TDD)

| Task | Behavior | RED (test written first, confirmed failing) | GREEN (implementation) | REFACTOR |
|------|----------|-----------------------------------------------|-------------------------|----------|
| 1.1 | `CandidateBranches` dedupes local+origin twin → 1 candidate | `TestService_CandidateBranches_DedupesLocalAndRemoteTrackingOfSameBranch` — ran RED, failed with `got 2` (both `feature/PROJ-7` and `origin/feature/PROJ-7` present) | `dedupeByLocalName` implemented + called at end of `CandidateBranches` (task 1.4/1.5) | Trimmed doc comments during budget pass; no logic change |
| 1.2 | Over-aggression guard: distinct names never collapsed | `TestService_CandidateBranches_DoesNotOverCollapseDistinctNames` — ran RED, failed with `got 3` (pre-dedupe) | Same `dedupeByLocalName` (compares on bare name only, never distinct names) | — |
| 1.3 | `ListBranches`/`branchesByPattern` stay undeduplicated | Pre-existing `TestService_ListBranches_ReturnsLocalAndRemoteBranches` (unmodified) already asserts local `main` + remote `origin/main` as separate rows — used as the guard per design; confirmed still green post-1.4/1.5 | N/A (no production change touches this path) | — |
| — | Regression: existing `TestService_CandidateBranches_ReturnsLocalAndRemoteMatchesByTicketName` broke post-dedupe (its `hotfix/PROJ-1` setup accidentally created a real local+origin twin) | Discovered via full-suite run after 1.4/1.5 GREEN | Fixed test setup to push `hotfix/PROJ-1` directly to a new remote ref (`HEAD:refs/heads/hotfix/PROJ-1`) so it stays genuinely remote-only, preserving the test's original intent | Verified `git push` auto-creates the local remote-tracking ref without a local branch (manual repro in `/tmp`) before applying |
| 2.1 | `resolveSource` tri-state table-driven coverage (8 cases) | `TestResolveSource` — ran RED, compile failure `undefined: resolveOutcome` (right reason: new API didn't exist yet) | `resolveOutcome`/`resolveDegrade`/`resolveReady`/`resolveNeedsConfirm` + 4-arg `resolveSource` (task 2.2) | Trimmed doc comment + inline comments during budget pass |
| 3.1/3.3/3.5/3.7/3.10 | `StateSourceConfirm` exists; `discoverCmd` pass-1 confirm message; `onDiscoverDone` confirm routing; `keySourceConfirm` s/n/N/enter/esc; `viewSourceConfirm` render | `source_confirm_test.go` (5 test funcs) — ran RED, `go vet` failure `not enough arguments in call to resolveSource` (commands.go's old call site, right reason: split not yet wired) | `StateSourceConfirm` (3.2), `discoverDoneMsg.confirm` + split `discoverCmd`/`confirmSourceCmd` (3.4/3.9), `onDiscoverDone` confirm branch (3.6), `keySourceConfirm` (3.8), `viewSourceConfirm` (3.11) — implemented together since 3.1-3.11 are mutually compile-dependent (documented as a single coherent GREEN batch, all RED tests confirmed failing together first) | Trimmed doc comments during budget pass; no logic change |
| 4.1 | Git-level e2e: repro (1 deduped candidate, non-empty range) + distinct-candidates via `SelectSingleSource` | New `TestHU002_Discover_E2E_SourceResolutionDedupe` (2 subtests) written against the already-GREEN Phase 1 implementation, run immediately GREEN (verifies Phase 1 end-to-end, not a fresh RED cycle per task wording "RED then GREEN verifying Phase 1+2+3 together") | N/A — verification of already-implemented behavior | — |
| 4.2 | App-level e2e: dedupe path (no confirm) + confirm accept/decline | New `TestHU_FullFlow_SourceResolutionDedupe_NoConfirmNeeded` + `TestHU_FullFlow_SourceConfirm_AcceptAndDecline` (2 subtests) against already-GREEN Phase 2/3, run immediately GREEN | N/A — verification of already-implemented behavior | Refactored `setupSourceConfirmRepo` to reuse `setupFlowRepo` instead of rebuilding the repo from scratch (budget pass) |
| 5.1/5.2 | Regression guard | N/A (verification only) | Ran the exact named suites (`TestStandaloneBranchesCmd_ComposesListBranches`, `standalone_modes_e2e_test.go`, `re_promote_e2e_test.go`, `original_branch_test.go`/`original_branch_e2e_test.go`, `boundary_test.go`) — all green | — |

## Design Interpretation Note (flagged, not a silent deviation)

Task 2.2's literal prose ("only when `SuggestDefaultSource` yields nothing AND `currentBranch` is eligible AND matches a candidate, return `resolveNeedsConfirm`") does not explicitly gate on candidate count. However, task 4.2 explicitly requires the dedupe-repro e2e to reach `StateCommitSelection` directly "(dedupe path, **no confirm needed**)" even when the user is checked out on the single deduped candidate. Implementing 2.2 literally (no count gate) would make a single deduped candidate ALSO trigger `resolveNeedsConfirm` whenever `currentBranch` matches it — contradicting 4.2's explicit "no confirm needed" requirement and the Technical Approach's framing ("fires only when discovery is genuinely ambiguous, 2+ distinct candidates"). Resolved by adding `len(candidates) > 1` to the needs-confirm gate in `resolveSource` — this is the only way to satisfy both 2.2's table (which never tests the single-candidate+match combination) and 4.2's explicit e2e assertion simultaneously. All 8 `TestResolveSource` table cases plus both e2e scenarios pass under this reading.

## Files Changed

| File | Action | What Was Done |
|------|--------|----------------|
| `internal/git/service_branches.go` | Modified | Added `dedupeByLocalName`; called at end of `CandidateBranches` only |
| `internal/git/service_branches_test.go` | Modified | 2 new dedupe unit tests (1.1/1.2); fixed one pre-existing test's setup to avoid an accidental local+origin twin; trimmed 1.3 guard note |
| `internal/git/discovery_e2e_test.go` | Modified | New `TestHU002_Discover_E2E_SourceResolutionDedupe` (repro + distinct-candidates subtests) |
| `internal/app/flow.go` | Modified | `resolveOutcome` type; `resolveSource` gains `currentBranch`, tri-state return |
| `internal/app/flow_test.go` | Created | Table-driven `TestResolveSource` (8 cases) |
| `internal/app/commands.go` | Modified | `discoverDoneMsg.confirm`; `discoverCmd` split into pass 1/pass 2 via `resolveOutcome` switch; new `confirmSourceCmd` |
| `internal/app/app.go` | Modified | New `StateSourceConfirm` (appended last) |
| `internal/app/keys.go` | Modified | `handleKey` routes `StateSourceConfirm`; new `keySourceConfirm` (s/n/N/enter/esc) |
| `internal/app/update.go` | Modified | `onDiscoverDone` routes `msg.confirm` to `StateSourceConfirm` before the existing selection-screen path |
| `internal/app/view.go` | Modified | `viewBody` routes `StateSourceConfirm`; new `viewSourceConfirm` |
| `internal/app/source_confirm_test.go` | Created | `TestStateSourceConfirm_Exists`, `TestDiscoverCmd_NeedsConfirm_ReturnsConfirmMsgWithoutRangedDiscover`, `TestOnDiscoverDone_Confirm_RoutesToStateSourceConfirm`, `TestKeySourceConfirm`, `TestViewSourceConfirm` |
| `internal/app/flow_e2e_test.go` | Modified | `setupSourceResolutionRepo`/`setupSourceConfirmRepo` helpers; `TestHU_FullFlow_SourceResolutionDedupe_NoConfirmNeeded`; `TestHU_FullFlow_SourceConfirm_AcceptAndDecline` |

## Work Unit Evidence

| Evidence | Value |
|---|---|
| Focused test command (git layer) | `go test ./internal/git/... -run CandidateBranches` and `-run TestHU002_Discover_E2E` — all pass |
| Focused test command (app layer) | `go test ./internal/app/... -run TestResolveSource` and `-run Confirm` — all pass |
| Runtime harness | `go test ./internal/app/... -run TestHU_FullFlow` (real temp-git e2e, no network) — pass; `go test ./...` full suite — pass |
| Rollback boundary | D1: revert `dedupeByLocalName` + its call site in `CandidateBranches` (leaves `ListBranches`/`branchesByPattern` untouched, already the case). D2: revert the confirm additions in `flow.go`/`app.go`/`commands.go`/`keys.go`/`update.go`/`view.go` — `discoverCmd`'s resolved/degrade paths are structurally unchanged so a partial revert of just D2 leaves D1's dedupe fix intact and functional |

## Regression Guard (Phase 5, explicit)

All named pre-existing suites confirmed green, unmodified in behavior:
- `TestStandaloneBranchesCmd_ComposesListBranches` (`internal/app/standalone_delta_test.go`) — PASS
- `standalone_modes_e2e_test.go` (`pickStandaloneBase` local+remote rows, `TestE2E_StandaloneDelta_*`) — PASS
- `re_promote_e2e_test.go` (`SuggestDefaultSource` priority over current-branch inference) — PASS
- `original_branch_test.go` / `original_branch_e2e_test.go` (`quitCmd`'s `m.originalBranch` guard) — PASS
- `boundary_test.go` (`TestApp_NeverImportsExecSeam`) — PASS, confirming no new git/exec call was introduced in `internal/app` by the confirm flow (it reuses the already-captured `m.originalBranch` and funnels through the unmodified `git.SelectSingleSource`)

## Deviations from Design

None in behavior. One clarifying interpretation was required (see "Design Interpretation Note" above) to reconcile task 2.2's prose with task 4.2's explicit e2e requirement — resolved in favor of the concrete, testable e2e assertion.

## Remaining Tasks

None. All 22 tasks across Phases 1-5 are `[x]` in `tasks.md`.
