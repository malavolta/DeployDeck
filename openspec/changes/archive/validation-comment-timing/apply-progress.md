# Apply Progress: Validation-Comment Timing

**Change**: `validation-comment-timing`
**Branch**: `fix/validation-comment-timing`
**Mode**: Strict TDD (RED → GREEN → REFACTOR)
**Status**: 14/14 tasks complete. Ready for verify.

## Completed Tasks

### Phase 1 — RED
- [x] 1.1 `vcTimingModel(t, fr)` fixture builder
- [x] 1.2 `TestOnReportDone_TerminalSuccess_PRAbsent_PostsNothing`
- [x] 1.3 `TestOnPrCreated_AfterTerminalSuccess_PostsCommentExactlyOnce` (load-bearing)
- [x] 1.4 `TestOnPrCreated_MarkerAlreadyPresent_NoDoublePost`
- [x] 1.5 `TestOnPrCreated_NoTerminalSuccessThisSession_PostsNothing`
- [x] 1.6 `TestOnReportDone_ReuseFlow_PRAlreadyOpenAtValidation_PostsOnce`
- [x] 1.7 Verify only 1.3 failed pre-fix

### Phase 2 — GREEN
- [x] 2.1 Wire the second trigger in `onPrCreated`
- [x] 2.2 Verify all Phase 1 tests GREEN
- [x] 2.3 Verify no regression on `PostValidationCommentCmd`

### Phase 3 — Final Gates
- [x] 3.1 `go build ./...`
- [x] 3.2 `go vet ./...`
- [x] 3.3 `gofmt -l internal cmd`
- [x] 3.4 `go test ./... -race -count=1`

## TDD Cycle Evidence

| Task | RED (test written, run, seen failing) | GREEN (implementation, test passes) | REFACTOR |
|---|---|---|---|
| 1.1–1.6 fixtures + 5 tests | Wrote `internal/app/validation_comment_timing_test.go` (6 test funcs incl. 1 sub-table). Ran `go test ./internal/app/... -race -run '<all 5 names>' -v`: 4/5 PASS immediately (1.2, 1.4, 1.5, 1.6 — existing guard/dedup logic was already correct), 1/5 **FAILED** (1.3, the load-bearing ordering test) with `onPrCreated() cmd = nil, want the validation-comment cmd...` — confirms production still returns `(m, nil)` unconditionally. | Added the single guarded trigger call in `onPrCreated`'s success path (`update.go`). Re-ran the same 5-test filter: all 5 PASS, including 1.3. | No refactor needed — the fix reuses `maybeTriggerValidationCommentCmd`/`postValidationCommentCmd` verbatim (D2/D3 in design.md); no new helper needed production-side. Test-side: extracted `vcCommentBody()`/`primeVcPostComment()`/`countPrComments()` helpers during initial test authoring (not a later refactor pass) to avoid duplicating the probe-then-reprime pattern across 3 call sites. |
| 1.7 verify-only task | N/A (verification step) | `go test ./internal/app/... -race` full package run pre-fix: only `TestOnPrCreated_AfterTerminalSuccess_PostsCommentExactlyOnce` failed; every other test in the package (including 1.2/1.4/1.5/1.6 and all pre-existing tests) passed. | N/A |
| 2.1–2.3 | (RED already established in Phase 1) | `go test ./internal/app/... -race` full package: PASS. `go test ./internal/app/... -race -run PostValidationCommentCmd -v`: both `TestPostValidationCommentCmd_SkipsWhenMarkerAlreadyPresent` and `TestPostValidationCommentCmd_PostsWhenNoMarkerPresent` PASS unchanged. | None |
| 3.1–3.4 | N/A | `go build ./...` exit 0; `go vet ./...` exit 0; `gofmt -l internal cmd` empty; `go test ./... -race -count=1` — all 15 packages `ok`. | None |

## Work Unit Evidence

| Evidence | Value |
|---|---|
| Focused test command and exact result | `PATH=/usr/local/go/bin:$PATH go test ./internal/app/... -race` → `ok github.com/malavolta/DeployDeck/internal/app 30.521s` (all tests in package, including the 6 new funcs) |
| Runtime harness command/scenario and exact result | `PATH=/usr/local/go/bin:$PATH go test ./... -race -count=1` → all 15 packages `ok` (full-repo integration/e2e suite, real `git`-backed tests in `internal/git`/`internal/app` included) |
| Rollback boundary | Revert the single 14-line insertion in `internal/app/update.go`'s `onPrCreated` (the `if cmd := m.maybeTriggerValidationCommentCmd(...)` block) and delete `internal/app/validation_comment_timing_test.go`. `onReportDone` and every other file are untouched. |

## Exact Diff — `internal/app/update.go` (`onPrCreated`)

```diff
@@ -1329,6 +1329,20 @@ func (m Model) onPrCreated(msg prCreatedMsg) (tea.Model, tea.Cmd) {
 	if m.deps.Runs != nil && m.runID != "" {
 		_ = m.deps.Runs.MarkPRCreated(m.runID, msg.url)
 	}
+
+	// validation-comment-timing: the normal flow validates BEFORE the PR
+	// exists, so onReportDone's own trigger (below) fires-and-skips with no
+	// PR to resolve and never retries. Re-attempt here, at the exact point
+	// the PR first exists — reusing the same guard (gate-enabled +
+	// requireValidationComment-on + a terminal-success status this session)
+	// and the same skip-if-present marker for exactly-once delivery
+	// (deploy-gate: "posts at PR creation" / "exactly once across both
+	// trigger points"; validation-progress: "PR creation (re)triggers the
+	// comment"). A non-success/empty m.report.Status, an ungated target, or
+	// an already-posted marker all no-op inside the returned cmd.
+	if cmd := m.maybeTriggerValidationCommentCmd(m.report.Status); cmd != nil {
+		return m, cmd
+	}
 	return m, nil
 }
```

`onReportDone` was NOT touched (design D1: kept verbatim for the reuse flow).

## Final Gates

| Gate | Command | Result |
|---|---|---|
| Build | `go build ./...` | clean, exit 0 |
| Vet | `go vet ./...` | clean, exit 0 |
| Format | `gofmt -l internal cmd` | empty output |
| Full suite | `go test ./... -race -count=1` | all 15 packages `ok` |

## Files Changed

| File | Action | What Was Done |
|---|---|---|
| `internal/app/update.go` | Modified | `onPrCreated` success path now (re)triggers `maybeTriggerValidationCommentCmd(m.report.Status)` after `MarkPRCreated`, before the final `return m, nil`. `onReportDone` untouched. |
| `internal/app/validation_comment_timing_test.go` | Created | 6 test funcs (incl. 3-case table) covering the ordering fix: `vcTimingModel` fixture, RED/GREEN coverage for both trigger points, dedup-on-marker, non-success no-op, and the unchanged reuse-flow pin. |
| `openspec/changes/validation-comment-timing/tasks.md` | Modified | All 14 tasks marked `[x]`. |
| `openspec/changes/validation-comment-timing/apply-progress.md` | Created | This artifact. |

## Delta-Spec Scenario Coverage

`deploy-gate/spec.md` (6 scenarios):
1. Successful validation on a gated env posts the marker comment — pinned by 1.6 (new) + pre-existing `TestPostValidationCommentCmd_PostsWhenNoMarkerPresent` (unchanged).
2. Re-validation does not duplicate — pinned by 1.4 (new) + pre-existing `TestPostValidationCommentCmd_SkipsWhenMarkerAlreadyPresent` (unchanged).
3. Condition passes only when marker present — pre-existing `internal/gate` coverage (`HasValidationComment`), unaffected by this change.
4. Ungated environment posts no comment — pre-existing `maybeTriggerValidationCommentCmd` guard (status/gate check), unaffected by this change; no new dedicated RED test was in Phase 1's scope for this branch since the guard logic itself is untouched.
5. Validation completes before PR exists, comment posts at PR creation — pinned by 1.2 + 1.3 (new, load-bearing).
6. Comment posted exactly once across both trigger points — pinned by 1.2 (zero at validation) + 1.3 (exactly one at PR creation) together (new).

`validation-progress/spec.md` (4 scenarios):
1. Successful CheckOnly on gate-enabled env triggers the comment — pinned by 1.6 (new, reuse-flow variant).
2. Successful CheckOnly on ungated env posts nothing — pre-existing guard, unaffected by this change.
3. Non-successful terminal state does not trigger — pinned by 1.5 (new, at the `onPrCreated` re-check point).
4. Validation completes before PR exists; PR creation (re)triggers — pinned by 1.3 (new, load-bearing).

Scenarios 3 (deploy-gate) and 2 (validation-progress) are pre-existing, unchanged guard behavior — this change did not add dedicated RED tests for them since neither the guard logic nor its existing coverage needed to change; they are called out here for completeness per task 3.4's instruction to confirm all 10 scenarios are covered by the sum of Phase 1 + pre-existing tests.

## Deviations from Design

None — implementation matches design.md exactly (D1–D5), including the single-line insertion point and the exact guard-reuse call site.

## Issues Found

None.

## Remaining Tasks

None — all 14 tasks complete.

## Workload / PR Boundary

- Mode: single PR (Low budget risk per tasks.md forecast; no decision required)
- Current work unit: Unit 1 (RED tests + GREEN trigger wiring, single insertion point)
- Boundary: starts at the RED test file, ends at Phase 3's final gates — the complete, self-contained fix
- Estimated review budget impact: well under 400 changed lines (`git diff --stat`: `update.go` +14/-0; new test file ~250 lines; both comfortably within a single small PR)
