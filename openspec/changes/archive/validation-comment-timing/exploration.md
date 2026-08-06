# Exploration — validation-comment-timing

Bug fix (found by dogfooding v1.3.0 on agenforce-demo PR #7): with the deploy gate enabled, the
DeployDeck validation comment is never posted to the PR in the normal flow. Diagnosed against the
code, root cause is a trigger-ordering defect.

## The bug (root cause, verified)

The normal DeployDeck flow validates BEFORE the PR exists:

- State order (`internal/app/app.go:69-120`): `StatePackageReview` → `StateQueueReview`
  (HU-009, "real stop between package confirm and validation start") → `StateValidationStart` →
  `StateValidationPolling` → `StateSucceeded`. Push + PR creation (`StatePushPreparation`,
  `app.go:117-120`) is reached AFTERWARD via the explicit `p` key ("push the **validated**
  branch", `keys.go`).
- The validation comment is triggered at validation success:
  `onReportDone` calls `maybeTriggerValidationCommentCmd(msg.report.Status)`
  (`internal/app/update.go:1170`), which returns `postValidationCommentCmd` when the target env's
  gate is enabled and `requireValidationComment` is on (`update.go:1185-1194`).
- At that moment the PR does NOT exist yet: `postValidationCommentCmd` resolves the PR via
  `resolvePRURL` (`commands.go:1554-1567`) = `rec.PRUrl` → `PRForBranch(RenderBranchName(...))`.
  `rec.PRUrl` is empty (set only by `MarkPRCreated`, which runs later in `onPrCreated`), and the
  branch is not pushed yet, so `PRForBranch` finds nothing → `resolvePRURL` returns `ok=false` →
  the comment is skipped (best-effort, silent).
- The user then presses `p`, which pushes and creates the PR (`onPrCreated`, `update.go:1319`;
  `MarkPRCreated` at `:1330` sets `rec.PRUrl`). But the comment trigger already fired-and-skipped
  at validation time and is never retried. Result: the PR carries the provenance SIGNATURE (body,
  paso 6) but never the validation COMMENT.

So in the standard flow the validation comment is unreachable. Confirmed live: agenforce-demo
PR #7 has the signature in its body but zero comments, with `gates.INT` enabled.

## Why reviews missed it

The deploy-gate tests exercise `postValidationCommentCmd` with the PR already resolvable
(`rec.PRUrl` set) — they assert post-when-absent / skip-when-present, but never the real ordering
where validation completes BEFORE the PR exists. No test drove "validate, then create PR" end to
end. A classic assume-the-precondition gap that only surfaces running it for real.

## Seams for the fix

- `onPrCreated(msg prCreatedMsg)` (`update.go:1319`): on success it calls `MarkPRCreated`
  (`:1330`) — this is exactly the point where the PR first exists (and `rec.PRUrl` becomes set).
- `m.report salesforce.DeployReport` (`app.go:544`) is a Model field that PERSISTS after
  validation, so at `onPrCreated` time the validation result (`m.report.Status`,
  `NumberComponentErrors`, `NumberTestErrors`, `CodeCoverage`) is still available to compose the
  comment.
- `maybeTriggerValidationCommentCmd(status)` (`update.go:1185`) already encapsulates the
  gate-enabled + requireValidationComment-on decision; it can be reused verbatim from the new
  trigger point.
- `postValidationCommentCmd` (`commands.go:1714+`) already does skip-if-present via the hidden
  marker, so firing the comment from a SECOND point is idempotent — no double-post.

## Recommended approach (for propose/design)

Fire the validation comment from BOTH points, guarded by the existing gate check and made
idempotent by the existing skip-if-present:
1. Keep the existing `onReportDone` trigger (covers the reuse/incremental flow where the PR
   ALREADY exists at validation time — e.g. reuse-on-collision updating an open PR).
2. ADD a trigger in `onPrCreated` success: if a prior validation reached a terminal SUCCESS
   (`m.report.Status` is `Succeeded`/`SucceededPartial`) and the gate/toggle say so, fire
   `postValidationCommentCmd` right after `MarkPRCreated`. This covers the normal flow where the
   PR is created AFTER validation.

The dedup marker guarantees exactly one comment across both triggers. No new gh primitive; reuse
`maybeTriggerValidationCommentCmd` + `postValidationCommentCmd`.

## Risks / constraints

- Only fire the second trigger when a validation actually succeeded THIS session — guard on
  `m.report.Status` being a terminal success (don't post on a PR created without a prior
  successful CheckOnly).
- Exec boundary unchanged (gh stays in internal/github; app decides).
- Milestone: patch release v1.3.1. Small, behavior-fixing change under strict TDD; the key new
  test is the end-to-end ordering (validate-then-create-PR → comment present exactly once).

Artifact store: OpenSpec (Engram MCP unavailable).
