# Apply Progress: Incremental Promotion (reuse-on-collision)

## Status: COMPLETE — 26/26 tasks done, both slices green

## Slice 1 — git/github/runs primitives (11/11 tasks done)

| Task | File | Status |
|------|------|--------|
| 1.1/1.2 | `internal/git/trailers.go` + `trailers_test.go` | Done — `ParseCherryPickTrailers` |
| 1.3/1.4 | `internal/git/service_reuse.go` + `service_reuse_test.go` | Done — `FFResult` + `FastForwardBranch` (IsAncestor-layered, never force-push) |
| 1.5/1.6 | same files | Done — `FilterNotOnBranch` (ancestor → trailer → cherry, sourceRef=="" degrades) |
| 1.7/1.8 | `internal/github/client.go` + `pr_for_branch_test.go` | Done — `PRForBranch` |
| 1.9/1.10 | `internal/runs/finder.go` + `finder_test.go` | Done — `FindRunForBranch` |
| 1.11 | — | Done — `go build ./...` + `go test ./... -race` GREEN |

## Slice 2 — App Wiring (15/15 tasks done)

| Task | File | Status |
|------|------|--------|
| 2.1/2.2 | `internal/app/app.go`, `update.go`, `branch_collision_test.go` | Done — `StateBranchCollision` const, `reusing`/`prExisting` fields, `onBranchCreated` routing |
| 2.3/2.4 | `internal/app/keys.go` | Done — `keyBranchCollision` (r/d/c) |
| 2.5/2.6 | `internal/app/commands.go` | Done — `reuseBranchCmd`, `deleteAndRecreateBranchCmd`, `reuseReadyMsg` |
| 2.7/2.8 | `internal/app/update.go` | Done — `onReuseReady` (diverged/err → StateError; empty → notice + StatePushPreparation; remainder → FindRunForBranch mutate-in-place or fresh run) |
| 2.9/2.10 | `internal/app/commands.go`, `update.go` | Done — `preparePRCmd` PRForBranch gate (reusing && authed only); `onPrepDone` skip-create/fallthrough |
| 2.11/2.12 | `internal/app/view.go` | Done — `viewBranchCollision` screen + `viewPRData`'s "PR existente" branch |
| 2.13/2.14 | `internal/app/incremental_promotion_e2e_test.go` | Done — full real-temp-repo E2E; passed on first run (no wiring gaps) |
| 2.15 | — | Done — `go build ./...` + `go test ./... -race -count=1` GREEN across all 13 packages |

## TDD Cycle Evidence

| Task | Test File | Layer | Safety Net | RED | GREEN | TRIANGULATE | REFACTOR |
|---|---|---|---|---|---|---|---|
| 1.1-1.2 | `internal/git/trailers_test.go` | Unit | N/A (new) | ✅ Written (compile fail) | ✅ Passed | ✅ 4 cases (single/none/multiline-mixed/malformed) | ➖ None needed |
| 1.3-1.4 | `internal/git/service_reuse_test.go` | Integration (real temp repo) | N/A (new) | ✅ Written | ✅ Passed | ✅ 5 FFResult cases incl. diverged-never-touches-either-tip | ➖ None needed |
| 1.5-1.6 | `internal/git/service_reuse_test.go` | Integration (real temp repo) | N/A (new) | ✅ Written | ✅ Passed | ✅ 2 cases (layered classification + empty-sourceRef degrade) | ➖ None needed |
| 1.7-1.8 | `internal/github/pr_for_branch_test.go` | Unit (FakeRunner) | N/A (new) | ✅ Written | ✅ Passed | ✅ 5 cases (open/closed/merged/none/runner-error) | ➖ None needed |
| 1.9-1.10 | `internal/runs/finder_test.go` | Unit | N/A (new) | ✅ Written | ✅ Passed | ✅ 3 cases (match/no-match/newest-of-many) | ➖ None needed |
| 2.1-2.2 | `internal/app/branch_collision_test.go` | Unit | ✅ existing `onBranchCreated` tests re-run clean | ✅ Written (compile fail: `StateBranchCollision` undefined) | ✅ Passed | ✅ 2 cases (collision-route / other-error-unchanged) | ➖ None needed |
| 2.3-2.4 | same file | Unit | ✅ n/a (new key handler) | ✅ Written (behavioral fail: unrouted state) | ✅ Passed | ✅ 4 sub-cases (r/d/c/esc) | ➖ None needed |
| 2.5-2.6 | same file | Integration (FakeRunner + real `git.Service`, call-order assertion) | N/A (new) | ✅ Written (compile fail: `reuseBranchCmd` undefined) | ✅ Passed | ➖ Single scenario (call-order is the property under test) | ➖ None needed |
| 2.7-2.8 | same file | Unit (`runs.Writer` on real temp dir) | N/A (new) | ✅ Written (behavioral fail: all 5 sub-cases) | ✅ Passed | ✅ 5 cases (diverged/err/empty/match/no-match) | ➖ None needed |
| 2.9-2.10 | same file | Unit (FakeRunner) | ✅ existing push-preparation suite re-run clean | ✅ Written (compile fail: `prOpen`/`prURL` undefined on `prepDoneMsg`) | ✅ Passed | ✅ 3 cases (open-skip / closed-fallthrough / not-reusing-never-calls) | ➖ None needed |
| 2.11-2.12 | same file | Unit | N/A (new) | ✅ Written (behavioral fail: empty view) | ✅ Passed | ➖ Single scenario (structural render) | ➖ None needed |
| 2.13-2.14 | `internal/app/incremental_promotion_e2e_test.go` | E2E (real temp git repo, fake `gh`) | ✅ full suite re-run clean throughout | ✅ Written | ✅ Passed on first run (proves 2.1-2.12's units wired correctly — no GREEN gap to close) | ➖ Single end-to-end scenario covering 3 properties | ➖ None needed |

### Test Summary

- **Total new/extended test files**: 8 (`trailers_test.go`, `service_reuse_test.go`, `pr_for_branch_test.go`, `finder_test.go`, `branch_collision_test.go`, `incremental_promotion_e2e_test.go`, plus extensions to no other pre-existing files' logic)
- **Total tests written**: 30 top-level `Test*` functions (several table-driven with multiple sub-cases)
- **Layers used**: Unit (many), Integration/real-temp-repo (`internal/git`, the reuse call-order test, the E2E), FakeRunner-backed (github, app push-prep gate)
- **Pure functions created**: `ParseCherryPickTrailers`, `FindRunForBranch` (both zero I/O, trivially unit-tested)

## Files Changed

| File | Action | What Was Done |
|------|--------|----------------|
| `internal/git/trailers.go` | Created | `ParseCherryPickTrailers(gitLog []byte) map[string]bool` |
| `internal/git/trailers_test.go` | Created | RED tests for the above |
| `internal/git/service_reuse.go` | Created | `FFResult` enum + `Service.FastForwardBranch` + `Service.FilterNotOnBranch` + private `cherryPickTrailersOnBranch` |
| `internal/git/service_reuse_test.go` | Created | Real-temp-repo RED tests for both |
| `internal/github/client.go` | Modified | Added `PRForBranch` to `Client` interface + `client` impl (`gh pr view --json url,state`) |
| `internal/github/pr_for_branch_test.go` | Created | RED/GREEN tests (FakeRunner) |
| `internal/runs/finder.go` | Created | `FindRunForBranch(records, format, branchName) (Record, bool)` |
| `internal/runs/finder_test.go` | Created | RED tests |
| `internal/app/app.go` | Modified | `StateBranchCollision` const; `reusing`/`prExisting` `Model` fields |
| `internal/app/update.go` | Modified | `onBranchCreated` collision routing; new `onReuseReady`; `onPrepDone` PR-open gate |
| `internal/app/commands.go` | Modified | New `reuseReadyMsg`, `reuseBranchCmd`, `deleteAndRecreateBranchCmd`; `prepDoneMsg` gained `prURL`/`prOpen`; `preparePRCmd` gained the reuse-gated `PRForBranch` call |
| `internal/app/keys.go` | Modified | `keyBranchCollision`; `StateBranchCollision` routed in `handleKey`; `keySucceeded`'s `p` resets `prExisting` |
| `internal/app/view.go` | Modified | `viewBranchCollision`; `viewPRData` gained the "PR existente" branch (`prExisting`) |
| `internal/app/branch_collision_test.go` | Created | All Slice-2 unit/integration RED tests |
| `internal/app/incremental_promotion_e2e_test.go` | Created | Full E2E (task 2.13) |

## Deviations from Design

None — implementation matches design.md's Interfaces/Contracts, Data Flow, and Threat Matrix exactly. Two minor, in-scope clarifications made where design.md/tasks.md left implementation details open:

1. `reuseBranchCmd`'s `sourceRef` parameter uses `m.source.Name` (task wording said "`m.source.SHA`", but `git.Branch` has no `SHA` field — only `Name`/`Remote`). `Name` is a valid, directly resolvable ref for `Cherry`'s `head` argument, and degrades to `""` identically when unresolved.
2. Added a `prExisting bool` Model field (not in design.md) purely for `viewPRData` wording: it renders "PR existente" instead of "PR creado" when the PR was found via `PRForBranch` rather than just created — a display-only refinement matching the spec's "the existing PR URL is shown" wording. No behavioral/test-contract impact.

## Issues Found

None.

## Workload / PR Boundary

- Mode: single PR (`size:exception` pre-accepted per tasks.md's Review Workload Forecast), 2 sequential internal commits (Slice 1, Slice 2) — matches the forecast exactly.
- Both slices are green independently and together; the single PR's diff is the natural rollback unit (revert Slice 2 alone leaves Slice 1's zero-caller primitives inert; revert both to fully back out).

## Status

26/26 tasks complete. `go build ./...` and `PATH=/usr/local/go/bin:$PATH go test ./... -race -count=1` are GREEN across all 13 packages (cmd/deploydeck, internal/{ai,app,config,delta,exec,git,github,prereq,runs,salesforce,update,version}). Ready for `sdd-verify`.
