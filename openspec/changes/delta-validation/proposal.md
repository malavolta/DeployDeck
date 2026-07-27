# Proposal: Delta + Validation (HU-007, 008, 010, 011)

**Phase**: Fase 2 Delta + Fase 3 Validación. Builds on archived `foundation-mvp-git` (HU-001..006). All open decisions were pre-resolved in `exploration.md` (SPIKE RESOLVED); this proposal states them.

## Intent

After promotion (cherry-picks + post-pick verification) the flow stops at `StatePickVerification`; users still generate the delta package and run `sf` validation by hand — slow and error-prone. This change continues the flow: auto-generate the delta, summarize it for review, then run and live-track an async CheckOnly validation — no manual `sgd`/`sf` steps.

## Scope

**In**: HU-007 delta via one `sf sgd source delta` call → `package.xml` (+`destructiveChanges/`); HU-008 summarize (per-type counts, sensitive-type warnings); HU-010 `sf project deploy validate --async --json` → jobId; HU-011 poll `sf project deploy report --json` to a terminal state.

**Out**: HU-009 queue/`DeployRequest`, HU-012 cancel, HU-014 push/PR, HU-015 quick deploy; full run history/list/retention/resume-by-jobId (HU-013).

## Capabilities

**New**:
- `delta-generation`: run sgd, locate artifacts, empty-package block, sgd-failure handling; entry gated by existing `DeltaAndValidationAllowed`.
- `package-summary`: per-type counts, destructive shown separately, sensitive-type warnings.
- `deploy-validation`: async validate → jobId; `--post-destructive-changes`/`--tests`; CLI error + raw JSON.
- `validation-progress`: poll report; live components/tests + errors; terminal-state detection; job stays active on exit.
- `run-persistence`: MINIMAL writer — on jobId create `.deploydeck/runs/<run-id>/` with jobId + status + raw JSON; history/retention/resume deferred to HU-013 (extends, not rewrites).

**Modified**: None (`cherry-pick`'s "Failed Cherry-Pick Blocks Downstream Steps" gate reused unchanged).

## Approach

Extend, don't rewrite (see Affected Areas). Resolved decisions:
- sgd multi-dir = SINGLE call, repeated `--source-dir`; artifacts `<out>/package/package.xml` (+`destructiveChanges/`).
- States `DeltaGeneration → PackageReview → QueueReview(inert pass-through) → ValidationStart → ValidationPolling → Succeeded/Failed/Canceled`; `QueueReview` kept for HU-009 forward-compat.
- Empty package → warning + validation blocked on `PackageReview`, overridable only by explicit user confirm.
- Polling: fixed `pollIntervalSeconds` (default 10) + hard timeout + `context` cancel; transient errors retry within timeout.
- Build order HU-007 → HU-008 (parallelizable on fixtures) → HU-010 → HU-011.

## Affected Areas

| Area | Impact | Change |
|------|--------|--------|
| `internal/delta` | New | sgd runner + package/destructive parse + per-type summary |
| `internal/salesforce` | Modified | add `validate.go`/`report.go` (same `Client`, `decodeEnvelope`, `New(runner)` unchanged) |
| `internal/config` | Modified | `delta.{outputDir,sourceDirs,ignoreFile,ignoreDestructiveFile}`, `pollIntervalSeconds` + `Validate()` |
| `internal/git` | Modified | `DeploymentPlan.{PackageXMLPath,DestructiveChangesPath}` |
| `internal/app` | Modified | states past `StatePickVerification`; `Deps.Delta` |
| `internal/runs` | New | minimal jobId+status+raw-JSON writer |

## Risks

| Risk | Lk | Mitigation |
|------|----|-----------|
| Run-persistence scope creep | Med | jobId+status+raw JSON only; history/resume → HU-013 |
| Polling never terminates | Med | hard timeout + terminal set + ctx cancel |
| e2e touches live sandbox queue | Med | CheckOnly, opt-in `DEPLOYDECK_E2E_ORG`, personal alias only |

## Testing (strict TDD)

Pure unit: package/destructive parse, validate/report JSON envelopes, poll-state transitions, config load+validate. Integration: real sgd on temp repo (`-short`-skippable, plugin installed). Opt-in real-org e2e: non-destructive CheckOnly validate/report vs `AM-DEV-EDITION` via `DEPLOYDECK_E2E_ORG`, polling to terminal.

## Rollback Plan

Additive on branch `delta-validation`; revert the feature commits — foundation flow (≤`StatePickVerification`) is untouched. `.deploydeck/runs/` is local + gitignored (delete the dir). No shared Git/sandbox state mutated (validation is CheckOnly).

## Dependencies

`sfdx-git-delta@6.45.1` installed (CI needs `sf plugins install sfdx-git-delta`); foundation modules `exec.Runner`/`git`/`config`/`salesforce` all built.

## Success Criteria

- [ ] Post-verification flow reaches a terminal validation state with no manual `sgd`/`sf` calls.
- [ ] Delta artifacts + summary shown; empty package blocks validation pending explicit confirm.
- [ ] jobId + status + raw JSON persisted under `.deploydeck/runs/<run-id>/`.
- [ ] Unit + temp-repo-sgd integration green; opt-in real-org e2e polls to terminal.
