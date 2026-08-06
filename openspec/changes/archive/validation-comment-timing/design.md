# Design: Validation-Comment Timing (post the deploy-gate comment once the PR exists)

## Technical Approach

Add a SECOND, idempotent validation-comment trigger at the exact point the PR first
exists — `onPrCreated`'s success path — reusing the existing decision site
`maybeTriggerValidationCommentCmd` and the existing skip-if-present marker for exactly-once
delivery. The normal flow validates BEFORE the PR exists, so the current single
`onReportDone` trigger fires-and-skips (no PR to resolve) and never retries. Firing again
from `onPrCreated` closes that gap; the existing `onReportDone` trigger is KEPT verbatim for
the reuse/incremental flow (PR already open at validation). No new gh primitive, no new exec
surface, no new app State, no schema change. Maps to proposal D1–D4 and both amended spec
requirements (deploy-gate, validation-progress).

## Architecture Decisions

| Decision | Choice | Rejected | Rationale |
|---|---|---|---|
| D1 two triggers | Fire from BOTH `onReportDone` (validation success) and `onPrCreated` (PR creation) | single trigger relocated to `onPrCreated` only | relocating alone would break the reuse flow (PR open at validation); dual + dedup covers both orderings with zero new state |
| D2 reuse decision site | Call `m.maybeTriggerValidationCommentCmd(m.report.Status)` at the new point | inline a fresh gate/toggle/status check | one guard implementation; config-toggle logic stays in ONE place |
| D3 idempotency source | Rely on the EXISTING `gate.HasValidationComment` skip-if-present in `postValidationCommentCmd` (commands.go:1746) | new "already-posted" model flag / persisted state | marker-on-PR is the durable source of truth across both triggers and across process restarts; a model flag would not survive resume |
| D4 status guard | Pass `m.report.Status`; `maybeTrigger…` returns nil unless status ∈ {`Succeeded`,`SucceededPartial`} AND gate enabled AND `requireValidationComment` on (update.go:1186-1192) | post on any PR creation | never posts on a PR created without a prior terminal-success CheckOnly this session — guard reused verbatim |
| D5 cmd composition | `onPrCreated` returns `m, nil` today (update.go:1332); mirror `onReportDone`'s pattern: `if cmd := …; cmd != nil { return m, cmd }` | unconditional `tea.Batch` | no existing cmd to batch on this path; keep the byte-local mirror of the proven `onReportDone` seam (1170-1172). Batch only becomes necessary if a future refactor makes this path return a non-nil cmd |

## Data Flow

Normal flow (the bug — now fixed):

    validate → onReportDone(Succeeded)
        └─ maybeTrigger → postValidationCommentCmd → resolvePRURL: PRUrl="" & branch unpushed
           → ok=false → SKIP (no PR yet)                          [unchanged, expected]
    user presses p → push → g → gh pr create → prCreatedMsg
        onPrCreated: m.prURL = msg.url (:1327) → MarkPRCreated (:1330)
        └─ maybeTrigger(m.report.Status) → postValidationCommentCmd     [NEW]
           → resolvePRURL: rec.PRUrl=m.prURL → ok=true (no PRForBranch call)
           → PRComments: no marker → PostComment → COMMENT POSTED (exactly once)

Reuse flow (PR already open at validation) — unchanged: `onReportDone` resolves via
PRForBranch and posts; a later `onPrCreated` is not reached (no new PR created).

Second fire of either trigger → `PRComments` finds the marker → SKIP, never a duplicate.

## File Changes

| File | Action | Description |
|---|---|---|
| `internal/app/update.go` `onPrCreated` (:1326-1332) | Modify | after `MarkPRCreated` (:1330), before `return m, nil`: `if cmd := m.maybeTriggerValidationCommentCmd(m.report.Status); cmd != nil { return m, cmd }`. Success path only (inside `msg.err == nil`) |
| `internal/app/update.go` `onReportDone` (:1170) | None | existing terminal-success trigger kept verbatim |
| `internal/app/deploy_gate_test.go` (or new `validation_comment_timing_test.go`) | Modify/Create | new RED coverage for `onReportDone`→`onPrCreated` ordering (see Testing Strategy) |
| `openspec/specs/deploy-gate/spec.md` | Modify | amend posting requirement + validate-then-create-PR scenario |
| `openspec/specs/validation-progress/spec.md` | Modify | amend trigger requirement + validate-then-create-PR scenario |

## Interfaces / Contracts

No signature changes. Reused as-is:

```go
// update.go:1185 — unchanged guard, reused from the new call site
func (m Model) maybeTriggerValidationCommentCmd(status string) tea.Cmd
// nil unless status ∈ {"Succeeded","SucceededPartial"} && gated && requireValidationCommentOn

// commands.go:1725 — unchanged; reads m.prURL at construction (:1728) so
// resolvePRURL short-circuits on rec.PRUrl (:1555) for the just-created PR;
// PRComments + gate.HasValidationComment (:1746) guarantee skip-if-present.
func (m Model) postValidationCommentCmd() tea.Cmd
```

`m.report` (`salesforce.DeployReport`, app.go:544) is written ONLY at `onReportDone` (:1153);
verified no reset on the `StatePushPreparation` transition, so `Status`/error-counts/coverage
persist to `onPrCreated` time. The value-receiver `Model` threads it through unchanged.

## Testing Strategy (strict TDD — RED first)

`onPrCreated` and `onReportDone` have NO covering tests today — new message-driven `Update`
scaffolding is required (message → `Update` → assert `fr.Calls`), following the fake-`gh`
`FakeRunner` idiom already in `deploy_gate_test.go` and the `reportDoneMsg`-driven tests in
`update_test.go`.

| Layer | What | Approach |
|---|---|---|
| Integration (RED, load-bearing) | (b) `onPrCreated` success after a terminal-success validation on a gate-enabled env → `gh pr comment` posted exactly once | drive `prCreatedMsg{url}` through `Update`; assert one `pr comment` call; `m.prURL` resolves via `rec.PRUrl` (no `pr view <branch>`) |
| Integration | (a) terminal-success validation while PR absent → NO comment yet | drive `reportDoneMsg{Succeeded}` with `m.prURL=""` & PRForBranch→none; assert zero `pr comment` |
| Integration | (c) second `onPrCreated`/`onReportDone` with marker present → NO double-post | seed `PRComments` with the marker; assert zero `pr comment` |
| Integration | (d) `onPrCreated` with `m.report.Status` empty/`Failed` → posts nothing | assert `maybeTrigger` returns nil; zero gh comment calls |
| Integration | (e) reuse flow (PR open at validation) → still posts once at validation | existing `onReportDone` path; PRForBranch resolves; one `pr comment` |
| Churn | none expected — existing `postValidationCommentCmd` skip/post tests (6.17) stay green | re-run |

## Threat Matrix

| Boundary | Applicability | Design response | RED test |
|---|---|---|---|
| VCS/PR automation (comment post) | Applicable — second trigger point | reuses existing `PRComments`/`PostComment`; comment body unchanged (org data already handled by `gate.ValidationComment`); no new content path | scenarios (b)(c)(d) |
| Idempotency / double-post | Applicable — two fire sites | existing marker skip-if-present (commands.go:1746) is the single dedup authority | scenario (c) |
| New exec / new gh method | N/A | no new subprocess, no new `github.Client` method; gh stays in `internal/github` | boundary_test.go holds |
| Routing / executable-file classification | N/A | no routing or file-classification change | — |

## Migration / Rollout

No migration, no schema bump, no feature flag. Behavior is additive and gate-scoped: an
ungated target, a toggled-off condition, or a non-success status is a no-op. Rollback = revert
the single `onPrCreated` trigger line; runs and existing PRs are unaffected; shared Git state
and Salesforce sandboxes untouched. Single small PR, well under the 400-line budget, milestone
v1.3.1.

## Open Questions

- [ ] None blocking. Confirmed: `m.report` is not reset between validation and PR creation;
  `m.prURL` is set (:1327) before the new trigger; `onPrCreated` returns nil today so no
  `tea.Batch` is needed (D5).
