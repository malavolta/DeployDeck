# Proposal: Validation-Comment Timing (post the deploy-gate comment once the PR exists)

## Intent

With the deploy gate enabled, DeployDeck's validation comment never reaches the PR in the
normal flow. It fires at validation success (`onReportDone`), but the normal flow validates
BEFORE the PR exists (the user presses `p` afterward → `onPrCreated`). At validation time
`resolvePRURL` finds no PR, so the comment is silently skipped and never retried once the PR
exists. Result: the PR carries the provenance SIGNATURE in its body but zero validation
COMMENT — and the deploy-gate validation-comment condition can never pass in this flow.
Confirmed live on agenforce-demo PR #7 (`gates.INT` enabled). Fix so the comment ships exactly
once, as soon as the PR exists. Milestone: patch v1.3.1.

## Problem / Motivation

- Trigger-ordering defect: `onReportDone` → `maybeTriggerValidationCommentCmd` →
  `postValidationCommentCmd` (`update.go:1170`, `commands.go:1725`) runs while `rec.PRUrl` is
  empty and the branch is unpushed → `resolvePRURL` returns `ok=false` → best-effort silent
  skip, never retried.
- The PR first exists later in `onPrCreated` (`MarkPRCreated`, `update.go:1330`, sets `PRUrl`),
  but no comment trigger runs there.
- Tests only drove `postValidationCommentCmd` with the PR already resolvable; none exercised
  the real validate-then-create-PR ordering.

## Scope

### In Scope
- ADD a second validation-comment trigger in `onPrCreated` success (right after `MarkPRCreated`),
  guarded on a terminal-success validation THIS session.
- KEEP the existing `onReportDone` trigger (covers reuse/incremental flow where the PR already
  exists at validation time — e.g. reuse-on-collision on an open PR).
- Reuse existing `maybeTriggerValidationCommentCmd` + `postValidationCommentCmd`; rely on the
  EXISTING hidden-marker skip-if-present for idempotency across both triggers.
- Amend the two spec requirements that encode the old single-trigger assumption.

### Out of Scope
- Changing the validate → push/PR flow order.
- Any new gh method or exec-boundary change.
- The gate's other conditions (approval, threads, signature) and the PR-body signature/provenance
  — unaffected; this fixes only the missing COMMENT.

## Approach — Decisions

| # | Decision | Rationale |
|---|----------|-----------|
| D1 | Fire from BOTH `onReportDone` and `onPrCreated`; the EXISTING hidden dedup marker (`gate.HasValidationComment` skip-if-present) guarantees exactly one comment. | Second trigger is idempotent by construction — no double-post, no new state. |
| D2 | Reuse `maybeTriggerValidationCommentCmd(m.report.Status)` at the new point; `m.report` persists post-validation, so status + error counts + coverage stay available at PR-creation. | Same gate/toggle guard, one decision site; no duplicated config logic. |
| D3 | Fire the second trigger ONLY on an actual terminal-success validation this session (`m.report.Status` ∈ {`Succeeded`,`SucceededPartial`}); never post on a PR created without a prior successful CheckOnly. | Guard already lives inside `maybeTrigger…`; reused verbatim. |
| D4 | No new gh primitive, no exec-boundary change; `onPrCreated` sets `m.prURL` (`:1327`) before the trigger, so `resolvePRURL` now resolves the just-created PR. | Byte-local seam; exec boundary intact (gh stays in `internal/github`). |

## Capabilities

### New Capabilities
- None.

### Modified Capabilities
- `deploy-gate` — "Validation-Comment Condition And Posting": clarify the comment is posted once
  the run's PR EXISTS, from whichever of validation-success or PR-creation occurs WITH the PR
  present (not strictly at validation completion). Add a scenario: validation succeeds before the
  PR exists → the comment is posted at PR creation, exactly once.
- `validation-progress` — "Terminal Successful CheckOnly Triggers The Deploy-Gate Validation
  Comment": the trigger ALSO fires at PR creation when a prior terminal-success validation
  occurred this session and the PR did not yet exist at validation time. Add a validate-then-
  create-PR scenario.

## Affected Areas

| Area | Impact | Change |
|------|--------|--------|
| `internal/app/update.go` `onPrCreated` (:1319-1333) | Modified | After `MarkPRCreated` (:1330), call `maybeTriggerValidationCommentCmd(m.report.Status)`; return its cmd when non-nil. |
| `internal/app/update.go` `onReportDone` (:1170) | Unchanged | Existing terminal-success trigger kept. |
| `openspec/specs/deploy-gate/spec.md` | Modified | Amend posting requirement + add scenario. |
| `openspec/specs/validation-progress/spec.md` | Modified | Amend trigger requirement + add scenario. |

## Risks

| Risk | Likelihood | Mitigation |
|------|-----------|------------|
| Double-post across the two triggers | Low | Existing skip-if-present marker; idempotent by design (D1). |
| Posting on a PR with no prior successful validation | Low | D3 status guard reused verbatim from the existing trigger. |
| `onPrCreated`/`onReportDone` lack covering tests | Med | Strict TDD; key new test drives validate-then-create-PR → comment present exactly once. |

## Rollback Plan

Behavior is additive and gate-scoped. Revert the single `onPrCreated` trigger call to restore
prior behavior; no persisted-data or schema change, no gh/exec change. Runs and existing PRs are
unaffected. Does not touch shared Git state (branches/cherry-picks) or Salesforce sandboxes.

## Success Criteria

- [ ] Normal flow (validate → `p` → PR created): the validation comment is present on the PR exactly once.
- [ ] Reuse/incremental flow (PR already open at validation): still exactly one comment, no double-post.
- [ ] A PR created without a prior terminal-success validation this session posts NO comment.
- [ ] The deploy-gate validation-comment condition can now pass in the normal flow.
- [ ] No new gh method; exec boundary unchanged; the PR-body signature/provenance is unaffected.

## Delivery Note

Single small PR, strict TDD, well under the 400-line budget. Milestone v1.3.1 (patch bug fix).
Delivery phase: Fase 5 Productizacion (deploy gate). Next recommended: `sdd-spec` and `sdd-design`
(parallel).
