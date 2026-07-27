# Tasks: Deploy Queue Visibility and Self-Service Cancel (HU-009 + HU-012)

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | ~1500-1900 (incl. tests) |
| Session review budget | 40000 (session-preflight override; skill default 400) |
| 400-line budget risk | Low (well under session budget; would read High vs. the unmodified 400 default) |
| Chained PRs recommended | No |
| Suggested split | Single PR, 2 work-unit commits (HU-009, HU-012) |
| Delivery strategy | single-pr |
| Chain strategy | pending |

Decision needed before apply: No
Chained PRs recommended: No
Chain strategy: pending
400-line budget risk: Low

### Suggested Work Units

| Unit | Goal | Likely PR | Focused test command | Runtime harness | Rollback boundary |
|------|------|-----------|----------------------|-----------------|-------------------|
| 1 | HU-009 queue visibility | PR 1 (single) | `go test ./internal/salesforce/... ./internal/app/... -run Queue` | `DEPLOYDECK_E2E_ORG=AM-DEV-EDITION go test ./internal/salesforce/... -run E2ERealOrg_Queue` | Revert `queue.go`, `client.go` interface add, QueueReview wiring in app; PackageReview→ValidationStart direct restored |
| 2 | HU-012 self-service cancel | PR 1 (single) | `go test ./internal/salesforce/... ./internal/runs/... ./internal/app/... -run Cancel` | N/A — cancel real-org e2e is timing-hard (design); fake is primary/required | Revert `cancel.go`, `MarkCanceled`, `StateCancelConfirm`/`c` key wiring; ValidationPolling `q` path untouched |

## Phase 1: HU-009 — `internal/salesforce` Queue (Foundation)

- [x] 1.1 RED `internal/salesforce/queue_test.go`: FakeRunner canned multi-user incl. own, permission-error, generic-error, empty, CheckOnly cases (HU-009)[U]
- [x] 1.2 GREEN `internal/salesforce/queue.go` + `client.go`: `ListDeployQueue` added to `Client` interface, SOQL builder (fields incl. `CreatedBy.Username`), envelope decode `{TotalSize,Done,Records[]}`, `ErrQueuePermission` heuristic (HU-009)[U]

## Phase 2: HU-009 — `internal/app` QueueReview Real Stop (Integration)

- [ ] 2.1 RED `internal/app/queue_review_test.go`: PackageReview→QueueReview→ValidationStart; permission auto-skip+notice; generic-error stays non-aborting; own-highlight+position via `Orgs()/FindByAlias.Username`; `r`/`enter`/`esc` (HU-009)[T]
- [ ] 2.2 GREEN `internal/app/app.go`: add `queue`, `queueErr`, `identity` Model fields (HU-009)[U]
- [ ] 2.3 GREEN `internal/app/commands.go`: `queueCmd`, `queueDoneMsg`, `queueCallTimeout` (HU-009)[U]
- [ ] 2.4 GREEN `internal/app/update.go`: `onQueueDone` (permission/generic/success branches) wired into `Update` (HU-009)[U]
- [ ] 2.5 GREEN `internal/app/keys.go`: `confirmPackageReview`→`StateQueueReview`+`queueCmd`; `keyQueueReview`; dispatcher case (HU-009)[U]
- [ ] 2.6 GREEN `internal/app/view.go`: `viewQueueReview` replaces the shared PackageReview/QueueReview dead fallback (HU-009)[T]

## Phase 3: HU-012 — `internal/salesforce` CancelDeploy (Foundation)

- [ ] 3.1 RED `internal/salesforce/cancel_test.go`: success/failure + Raw cases (HU-012)[U]
- [ ] 3.2 GREEN `internal/salesforce/cancel.go` + `client.go`: `CancelDeploy` added to `Client` interface, arg-slice `sf project deploy cancel --job-id <id> --target-org <alias>` (HU-012)[U]

## Phase 4: HU-012 — `internal/runs` MarkCanceled (Foundation)

- [ ] 4.1 RED `internal/runs/writer_test.go`: `MarkCanceled` writes `cancel.json` companion + sets `run.json` Status=Canceled; does not consume `report-<NNN>.json` numbering (HU-012)[U]
- [ ] 4.2 GREEN `internal/runs/writer.go`: `Writer.MarkCanceled(runID, cancelRaw)` (HU-012)[U]

## Phase 5: HU-012 — `internal/app` Cancel Wiring (Integration)

- [ ] 5.1 RED `internal/app/cancel_confirm_test.go`: ValidationPolling→CancelConfirm via `c`; typed `CANCELAR` gate wrong/backspace/esc; success→Canceled+persist; failure→stay+error unmarked; stale-message guard (HU-012)[T]
- [ ] 5.2 GREEN `internal/app/app.go`: `StateCancelConfirm` const + `cancelInput` field (HU-012)[U]
- [ ] 5.3 GREEN `internal/app/commands.go`: `cancelCmd`, `cancelDoneMsg`; targets exactly `m.jobID`, never `m.cancelInput` (HU-012)[U]
- [ ] 5.4 GREEN `internal/app/update.go`: `onCancelDone` — success→`cancelPoll()`+`MarkCanceled`+`StateCanceled`; failure→stay+error; guard `state != StateCancelConfirm`; wired into `Update` (HU-012)[U]
- [ ] 5.5 GREEN `internal/app/keys.go`: `c` case in `keyValidationPolling`→`StateCancelConfirm`; `keyCancelConfirm` (`keyTicket` idiom) gated on exact literal `CANCELAR`; dispatcher case; `q` path untouched (HU-012)[U]
- [ ] 5.6 GREEN `internal/app/view.go`: `viewCancelConfirm`; dispatcher case (HU-012)[T]

## Phase 6: Threat-Matrix Proof (Security)

- [ ] 6.1 `internal/salesforce/argcomposition_test.go`: assert `FakeRunner.Calls` carries the SOQL as one slice arg for `ListDeployQueue`, and `jobID` as a plain slice arg for `CancelDeploy` — never shell-joined/interpolated; confirms Phase 1/3 GREEN already satisfies it [U]

## Phase 7: Real E2E (Verification)

- [ ] 7.1 `[E2E-ORG]` extend `internal/salesforce/real_org_e2e_test.go`: queue query vs `AM-DEV-EDITION` — real `DeployRequest` records parse incl. `CreatedBy.Username`, own-job identified (HU-009)
- [ ] 7.2 `[E2E-ORG]` optional/best-effort: real cancel vs a real jobId — assert the CLI call runs, do NOT assert terminal status (timing-hard, documented, not required) (HU-012)

## Phase 8: Final Verification (Cleanup)

- [ ] 8.1 `go test -race ./...`, `go vet ./...`, `gofmt -l .` clean
- [ ] 8.2 Confirm `internal/app/boundary_test.go` still passes (no direct `os/exec`/`internal/exec` import)
- [ ] 8.3 Check off `proposal.md` Success Criteria items

## Notes

- `q` exit keeps the job-active/resumable invariant unchanged (validation-progress delta); cancel is only the separate `c`→`CANCELAR` path (Phase 5).
- OUT of scope, untouched: HU-013 history/resume, HU-014 push/PR, HU-015 quick deploy.
