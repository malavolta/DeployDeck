# Design: Deploy Queue Visibility and Self-Service Cancel (HU-009 + HU-012)

## Technical Approach

Extend, never rewrite. `internal/salesforce` gains two read/write shim methods on the unchanged `New(runner)` + `decodeEnvelope` seam. `internal/app` turns the inert `StateQueueReview` into a real stop and adds one new state `StateCancelConfirm`, reusing existing `StateCanceled`, `cancelPoll()`, the `keyTicket` input idiom, and the stale-message guard. `internal/runs.Writer` gains `MarkCanceled`. `internal/app` still never execs directly — every subprocess flows through the injected `salesforce.Client` (boundary invariant, `ARQUITECTURA.md:84-91`). Maps to specs `deploy-queue`, `validation-cancel`, `validation-progress` (Δ key `c`), `run-persistence` (Δ `cancel.json`).

## Architecture Decisions

| Decision | Choice | Rejected | Rationale |
|---|---|---|---|
| Queue decode target | `{TotalSize,Done,Records[]}` struct via `decodeEnvelope` | bare `[]record` | Empirically confirmed 2026-07-27 vs `AM-DEV-EDITION`; `sf data query` wraps records. |
| Own-job identity source | Reuse `Client.Orgs()`→`FindByAlias(alias).Username`; match `CreatedBy.Username` | new `sf org display`/whoami call | Zero new subprocess; `username` is a stable unique key (display `Name` renames/collides). Requires projecting `CreatedBy.Username` additively in the SOQL (only enriches each record; envelope shape unchanged — the opt-in queue e2e re-confirms). |
| Permission-error branch | Typed sentinel `ErrQueuePermission` from a heuristic over the oclif `message` (`validateErrorMessage` precedent); unmatched → generic wrapped error | swallow all query errors | Non-blocking degrade only for permission denial; generic failure stays actionable, never silent (`ARQUITECTURA.md:429`). |
| QueueReview stop | Approach A manual-confirm: real cmd/msg/handler/view, no new state constant | auto-skip; new state | `StateQueueReview` already exists as scaffolding; mockup needs `r`/`Enter`/`Esc`. |
| Cancel entry | New `c` key → `StateCancelConfirm`; `q` untouched | repurpose `q` | `q` "exit-leaves-job-resumable" is a protected invariant (`validation-progress` spec). |
| Cancel gate | Typed `m.cancelInput == "CANCELAR"` (case-sensitive, no normalization), `keyTicket` idiom | y/n confirm | Mockup `:346-367`; deliberate friction for a destructive shared-queue action. |
| Cancel persistence | New `Writer.MarkCanceled(runID, cancelRaw)` → `cancel.json` + `run.json` Status=Canceled | `AppendReport` | `AppendReport` writes `report-NNN.json`, misrepresenting a cancel as a poll. Mirrors `Create`'s verbatim `validate.json`. |
| Two writers into `run.json` | Symmetric stale guard (`update.go:249-253` idiom) on the cancel handler + reuse existing `onReportDone` guard | lock/merge | A late `reportDoneMsg` after cancel is already dropped once `state != ValidationPolling`; guarding `onCancelDone` on `state != StateCancelConfirm` blocks a duplicate/late cancel from re-marking. |

## Data Flow

```
PackageReview --confirm--> StateQueueReview + queueCmd
   queueCmd = Orgs()->username  +  ListDeployQueue(alias)
   onQueueDone: ErrQueuePermission -> notice + StateValidationStart + validateCmd
                generic err        -> stay QueueReview (error, non-blocking)
                ok                 -> QueueReview list (own [propio] + approx pos)
   keyQueueReview: enter->ValidationStart+validate | r->re-queue | esc->PackageReview
                          |
                          v
ValidationPolling --c--> StateCancelConfirm (m.cancelInput)
   enter & =="CANCELAR" -> cancelCmd -> CancelDeploy(jobID,alias)
      ok  -> MarkCanceled(runID,raw) + cancelPoll() + StateCanceled
      err -> stay CancelConfirm + error (run NOT marked)   esc -> ValidationPolling
```

## File Changes

| File | Action | Description |
|---|---|---|
| `internal/salesforce/queue.go` | Create | `ListDeployQueue`, record/envelope structs, SOQL builder, `ErrQueuePermission` + heuristic. |
| `internal/salesforce/cancel.go` | Create | `CancelDeploy` + `CancelResult`, Raw/error handling. |
| `internal/runs/writer.go` | Modify | `MarkCanceled(runID, cancelRaw)` + `cancel.json`. |
| `internal/app/app.go` | Modify | `StateCancelConfirm` const; `cancelInput`, `queue`, `queueErr`, `identity` fields. |
| `internal/app/commands.go` | Modify | `queueCmd`/`cancelCmd`, `queueDoneMsg`/`cancelDoneMsg`, `queueCallTimeout`. |
| `internal/app/update.go` | Modify | `onQueueDone`, `onCancelDone` (+ stale guard); wire into `Update`. |
| `internal/app/keys.go` | Modify | `confirmPackageReview`→QueueReview; `keyQueueReview`; `c` in `keyValidationPolling`; `keyCancelConfirm`; dispatcher cases. |
| `internal/app/view.go` | Modify | `viewQueueReview` (replace dead fallback), `viewCancelConfirm`; dispatcher cases. |

## Interfaces / Contracts

```go
// salesforce
func (c *client) ListDeployQueue(ctx, targetOrg string) ([]DeployQueueEntry, error) // ErrQueuePermission sentinel
func (c *client) CancelDeploy(ctx, jobID, targetOrg string) (CancelResult, error)     // CancelResult{Raw string}
// runs — cancel.json = verbatim cancel stdout+stderr; run.json Status="Canceled", UpdatedAt=now
func (w *Writer) MarkCanceled(runID string, cancelRaw []byte) error
```

`DeployQueueEntry`: JobID, Status, CreatedBy, Username, CheckOnly, CreatedDate, StartDate, Components{Total,Deployed,Errors}, Tests{Total,Completed,Errors}. Own = `Username == identity`; position = 1-based index in CreatedDate order.

## Testing Strategy

| Layer | What | Approach |
|---|---|---|
| Unit | SOQL builder; envelope/records parse; `CheckOnly`; empty state; permission vs generic branch; `CancelResult`/Raw | Table-driven + `FakeRunner` canned JSON. |
| Unit | `Model.Update`: PackageReview→QueueReview→ValidationStart; permission auto-skip; ValidationPolling→CancelConfirm; typed-`CANCELAR` gate (wrong/backspace/esc); cancel ok→Canceled+persist / err→unmarked; stale guard | Direct `Update(msg)`, no `teatest`. |
| Unit | `MarkCanceled` → `cancel.json` + `run.json` Status | `t.TempDir()`. |
| E2E | Queue query record shape (read-only) | Opt-in `DEPLOYDECK_E2E_ORG` vs `AM-DEV-EDITION`, `real_org_e2e_test.go` convention. |
| E2E | Cancel | Timing-hard → **fake primary/required**; real best-effort (assert CLI runs, not terminal status). |

## Threat Matrix

| Boundary | Applicability | Design response | RED test |
|---|---|---|---|
| Documentation-like paths | N/A — no file classification/execution. | — | — |
| Git repository selection | N/A — no git; `--target-org` alias + `Dir` via Runner. | — | — |
| Commit / Push state | N/A — no VCS mutation. | — | — |
| PR / argument composition | Applicable — new `sf data query --query "<SOQL>"` and `sf project deploy cancel --job-id <id>`. | Args as slice through existing `exec.Runner` (never a shell); SOQL filter static; jobID = current run's own only (never user-typed); alias = configured. | SOQL builder arg-slice assertion; `CancelDeploy` targets exactly `m.jobID`. |

## Migration / Rollout

No migration. Additive/revertible: revert wiring → QueueReview inert, `c`/`StateCancelConfirm` removed; `New(runner)` unchanged; `cancel.json` is local, gitignored.

## Open Questions

- [ ] Permission-error text unverified (no restricted profile) — heuristic + documented generic fallback; unmatched never swallowed.
