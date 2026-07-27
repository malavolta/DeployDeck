# Apply Progress: Deploy Queue Visibility and Self-Service Cancel (HU-009 + HU-012)

**Change**: `deploy-queue`
**Batch**: 1 (this batch) — Phase 1, Phase 2, Phase 7.1 (HU-009 only)
**Mode**: Strict TDD (RED → GREEN per task)
**Branch**: `deploy-queue`
**Delivery**: single-pr, `size:exception` granted (~1.7k line budget for the whole change)

## Scope Of This Batch

HU-009 (deploy queue visibility) end to end: `internal/salesforce.ListDeployQueue`,
the `internal/app` `QueueReview` real stop, and the opt-in real-org queue e2e.
HU-012 (self-service cancel, Phases 3-6) and Phase 8 (final verification across
both HUs) are explicitly OUT of this batch — do not start HU-012.

## Completed Tasks

### Phase 1: HU-009 — `internal/salesforce` Queue (Foundation)
- [x] 1.1 RED `internal/salesforce/queue_test.go`: FakeRunner canned multi-user incl. own, permission-error, generic-error, empty, CheckOnly cases (HU-009)[U]
- [x] 1.2 GREEN `internal/salesforce/queue.go` + `client.go`: `ListDeployQueue` added to `Client` interface, SOQL builder (fields incl. `CreatedBy.Username`), envelope decode `{TotalSize,Done,Records[]}`, `ErrQueuePermission` heuristic (HU-009)[U]

### Phase 2: HU-009 — `internal/app` QueueReview Real Stop (Integration)
- [x] 2.1 RED `internal/app/queue_review_test.go`: PackageReview→QueueReview→ValidationStart; permission auto-skip+notice; generic-error stays non-aborting; own-highlight+position via `Orgs()/FindByAlias.Username`; `r`/`enter`/`esc` (HU-009)[T]
- [x] 2.2 GREEN `internal/app/app.go`: add `queue`, `queueErr`, `identity` Model fields (HU-009)[U]
- [x] 2.3 GREEN `internal/app/commands.go`: `queueCmd`, `queueDoneMsg`, `queueCallTimeout` (HU-009)[U]
- [x] 2.4 GREEN `internal/app/update.go`: `onQueueDone` (permission/generic/success branches) wired into `Update` (HU-009)[U]
- [x] 2.5 GREEN `internal/app/keys.go`: `confirmPackageReview`→`StateQueueReview`+`queueCmd`; `keyQueueReview`; dispatcher case (HU-009)[U]
- [x] 2.6 GREEN `internal/app/view.go`: `viewQueueReview` replaces the shared PackageReview/QueueReview dead fallback (HU-009)[T]

### Phase 7: Real E2E (Verification) — HU-009 subset
- [x] 7.1 `[E2E-ORG]` queue query vs `AM-DEV-EDITION` — real `DeployRequest` records parse incl. `CreatedBy.Username`, own-job identified (HU-009). Verified as a NEW gated file `internal/salesforce/real_org_queue_e2e_test.go` rather than extending `real_org_e2e_test.go` (both explicitly permitted by the task) — keeps the queue e2e isolated from the existing validate+report e2e.

## Remaining Tasks (Out Of This Batch)

- [ ] 3.1-3.2 HU-012 `internal/salesforce.CancelDeploy` (Foundation)
- [ ] 4.1-4.2 HU-012 `internal/runs.Writer.MarkCanceled` (Foundation)
- [ ] 5.1-5.6 HU-012 `internal/app` Cancel Wiring (Integration)
- [ ] 6.1 Threat-Matrix Proof (Security) — covers both HU-009 (already satisfied by Phase 1 GREEN, per the task note) and HU-012's `CancelDeploy` arg composition
- [ ] 7.2 `[E2E-ORG]` optional/best-effort real cancel (HU-012)
- [ ] 8.1-8.3 Final Verification (Cleanup) — spans both HUs, run only after HU-012 lands

## Files Changed

| File | Action | What Was Done |
|------|--------|----------------|
| `internal/salesforce/client.go` | Modified | Added `ListDeployQueue(ctx, targetOrg) ([]DeployQueueEntry, error)` to the `Client` interface. |
| `internal/salesforce/queue.go` | Created | `ListDeployQueue` implementation: SOQL builder (query is ONE slice arg), `{totalSize,done,records[]}` envelope decode into `DeployQueueEntry`/`QueueComponentProgress`/`QueueTestProgress`, `ErrQueuePermission` typed sentinel + heuristic classification, best-effort SF-datetime parsing. |
| `internal/salesforce/queue_test.go` | Created | Table/scenario tests: SOQL-as-one-arg, multi-user ordering incl. own, empty queue, `CheckOnly` true/false, permission-error → `ErrQueuePermission`, generic-error not swallowed/misclassified, runner-error survives. |
| `internal/salesforce/real_org_queue_e2e_test.go` | Created | `TestE2ERealOrg_Queue`: opt-in (`DEPLOYDECK_E2E_ORG`), `NewOSRunner`, read-only. Verified locally against `AM-DEV-EDITION`. |
| `internal/app/app.go` | Modified | `StateQueueReview` doc comment updated (real stop, not inert); added `queue []salesforce.DeployQueueEntry`, `queueErr error`, `identity string` Model fields. |
| `internal/app/commands.go` | Modified | Added `queueCallTimeout` (30s), `queueDoneMsg`, `queueCmd()` (identity via `Orgs()`/`FindByAlias`, best-effort; then `ListDeployQueue`). |
| `internal/app/update.go` | Modified | Added `errors` import, `onQueueDone` (permission-skip / generic-stay / success branches), wired `queueDoneMsg` into `Update`. |
| `internal/app/keys.go` | Modified | `confirmPackageReview` now enters `StateQueueReview` + fires `queueCmd` (was: direct to `StateValidationStart`); added `keyQueueReview` (`enter`/`r`/`esc`); dispatcher case in `handleKey`. |
| `internal/app/view.go` | Modified | Added `salesforce` import, `viewQueueReview` (own-highlight `[propio]` + approximate position, empty state, error state), `queueElapsed` helper; split the `StatePackageReview, StateQueueReview` shared dispatcher case into two. |
| `internal/app/queue_review_test.go` | Created | `Model.Update`-driven tests for every AC in `specs/deploy-queue/spec.md`: confirm→QueueReview+queueCmd, identity best-effort, success w/ own-highlight+position, own-absent, empty-state, permission auto-skip+notice, generic-error non-aborting, `enter`/`r`/`esc`. |
| `internal/app/delta_validation_test.go` | Modified | Updated 2 pre-existing tests (`TestModel_PackageReview_Confirm_PassesThroughQueueReview` → doc-only, superseded; `TestModel_PackageReview_EmptyBlocksUntilOverride`) that hardcoded the old inert-pass-through assumption (confirm → `StateValidationStart` directly) to the new `StateQueueReview` real-stop behavior. |
| `openspec/changes/deploy-queue/design.md`, `exploration.md`, `proposal.md`, `specs/**/spec.md`, `tasks.md` | Committed (pre-existing, uncommitted from earlier SDD phases) | Docs commit at the start of this batch — no content authored by apply, just persisted to git alongside the code that implements them (single-PR delivery). |

## TDD Cycle Evidence

| Task | RED | GREEN | REFACTOR |
|---|---|---|---|
| 1.1/1.2 `ListDeployQueue` | `queue_test.go` written first; `go vet` failed with `ListDeployQueue undefined` (compile-time RED, no `Client` method yet) | `client.go` interface + `queue.go` implementation; `go test ./internal/salesforce/... -run Queue` → 7/7 PASS | `gofmt -w` reformatted the `deployQueueResult` struct tag alignment; no logic change |
| 2.1-2.6 QueueReview real stop | `queue_review_test.go` written first; `go vet ./internal/app/...` failed with `undefined: queueDoneMsg` (compile-time RED) | `app.go`/`commands.go`/`update.go`/`keys.go`/`view.go` implemented together (a single cohesive state-machine change — Model fields, cmd, msg, handler, keys, view all needed simultaneously for the package to compile); `go test ./internal/app/... -run Queue -v` → 11/11 PASS | Discovered 2 stale pre-existing tests asserting the old inert-pass-through behavior; updated them (see Deviations) — this is the REFACTOR step surfacing a design consequence, not scope creep |
| 7.1 real-org queue e2e | N/A — an e2e confirmation test, not a red/green unit; written directly against the already-GREEN `ListDeployQueue` | Ran with `DEPLOYDECK_E2E_ORG=AM-DEV-EDITION` → PASS (0 active jobs at test time, identity resolved to `amalavolta.93dfa3278fb9@agentforce.com`, shape assertions all held) | N/A |

## Work Unit Evidence

### Unit 1 — HU-009 queue visibility (this batch, complete)

| Evidence | Value |
|---|---|
| Focused test command and exact result | `go test ./internal/salesforce/... ./internal/app/... -run Queue -v` → all `TestClient_ListDeployQueue_*` (7) and `TestModel_*Queue*`/`TestModel_KeyQueueReview_*`/`TestModel_OnQueueDone_*` (11) PASS |
| Runtime harness command/scenario and exact result | `DEPLOYDECK_E2E_ORG=AM-DEV-EDITION go test ./internal/salesforce/... -run E2ERealOrg_Queue -v` → PASS (`real_org_queue_e2e_test.go:47/65/93`): 0 active jobs, identity resolved, shape assertions held against the live org |
| Rollback boundary | Revert `internal/salesforce/queue.go`, `queue_test.go`, `real_org_queue_e2e_test.go`, the `ListDeployQueue` interface addition in `client.go`; in `internal/app`, revert the `queue`/`queueErr`/`identity` fields, `queueCmd`/`queueDoneMsg`/`queueCallTimeout`, `onQueueDone`, the `confirmPackageReview`/`keyQueueReview` changes, and `viewQueueReview` — this restores `confirmPackageReview` to its original direct `PackageReview → ValidationStart` transition and `StateQueueReview` to its original inert-fallback state, without touching any HU-010/011 (already-archived) code. `delta_validation_test.go`'s two updated assertions would need to revert to their prior expectations in lockstep. |

## Full Suite Verification

Ran verbatim as requested:

```
export PATH="/usr/local/go/bin:$PATH" && go build ./... && go vet ./... && gofmt -l . && go test -race ./...
```

Result: all packages `ok` (`cmd/deploydeck`, `internal/app`, `internal/config`, `internal/delta`,
`internal/exec`, `internal/git`, `internal/prereq`, `internal/runs`, `internal/salesforce`),
`gofmt -l .` produced no output (clean), `go vet ./...` clean.

Plus the real-org queue e2e (`DEPLOYDECK_E2E_ORG=AM-DEV-EDITION`): PASS.

`internal/app/boundary_test.go`'s `TestApp_NeverImportsExecSeam` re-verified explicitly: PASS
— `internal/app` still reaches `ListDeployQueue`/`Orgs` only through the injected
`m.deps.SF salesforce.Client`, never `os/exec`/`internal/exec` directly.

## Deviations From Design

None in the implementation shape — `ListDeployQueue`, `DeployQueueEntry`,
`QueueComponentProgress`/`QueueTestProgress`, `ErrQueuePermission`, `queueCmd`/
`queueDoneMsg`/`onQueueDone`/`keyQueueReview`/`viewQueueReview` all match
design.md's Interfaces/Contracts and Data Flow sections.

Two pre-existing tests in `internal/app/delta_validation_test.go`
(`TestModel_PackageReview_Confirm_PassesThroughQueueReview` and
`TestModel_PackageReview_EmptyBlocksUntilOverride`) hardcoded the OLD
inert-pass-through assumption from the archived `delta-validation` slice
(confirm → `StateValidationStart` directly). Both were updated in lockstep with
the Phase 2 GREEN — this is a necessary, expected consequence of design.md's
explicit decision to turn `StateQueueReview` into a real stop, not a deviation
from the plan. Task 7.1 was implemented as a NEW gated file
(`real_org_queue_e2e_test.go`) rather than extending `real_org_e2e_test.go`,
per the task's own "or a new gated file" option — chosen to keep the queue e2e
isolated from the existing (larger) validate+report e2e.

## Issues Found

None. `AM-DEV-EDITION`'s current queue was empty at test time, so the real-org
e2e's "own job found in queue" branch was exercised as a log-only, non-fatal
path (by design — a live queue's contents aren't controllable/deterministic);
the shape/identity assertions (the actual regression-catchers) all ran and
passed.

## Workload / PR Boundary

- Mode: single PR, `size:exception` granted for the whole `deploy-queue` change (~1.7k line forecast within the 40000 session budget)
- Current work unit: Unit 1 — HU-009 queue visibility (complete)
- Boundary: this batch starts from the archived `foundation-mvp-git` + `delta-validation` baseline and ends with HU-009 fully implemented, tested (unit + real-org e2e), and merged into 4 commits on `deploy-queue`. HU-012 (Unit 2) is untouched.
- Estimated review budget impact: ~1035 changed lines (1035 insertions + minor deletions) across 11 files for this batch — see `git diff --stat f8b9103^..HEAD -- internal/`. Well within the granted exception.

## Commits (this batch)

1. `f8b9103` — `docs(deploy-queue): add SDD planning artifacts for HU-009/HU-012`
2. `aa158c9` — `feat(salesforce): list deploy queue via tooling api`
3. `bda5f6d` — `feat(app): real queue-review stop before validation`
4. `0953f85` — `test(salesforce): real-org e2e for the deploy queue (opt-in)`

## Status

9/24 total change tasks complete (Phase 1: 2/2, Phase 2: 6/6, Phase 7: 1/2 —
7.1 done, 7.2 is HU-012). HU-009 is fully implemented and verified end to end.
Ready for the next `sdd-apply` batch to implement HU-012 (Phases 3-6, then
Phase 7.2 and Phase 8). Do NOT run `sdd-verify` yet — Phase 8's final
verification explicitly spans both HUs.
