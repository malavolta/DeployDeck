# Proposal: Deploy Error Detail (surface the failure detail the CLI already emits)

## Intent

Close the 8 error-surfacing gaps found by crossing the official Salesforce CLI docs against
DeployDeck's deploy/validation pipeline. The `--json` envelope already carries per-component
`lineNumber`/`columnNumber`/`fileName`/`problemType`, per-test `stackTrace`, per-class
`codeCoverage[]`, queue `StateDetail`/`ErrorMessage`, and full cancel/validate error envelopes —
DeployDeck parses a subset and renders even less, so a failed run tells the user *that* it broke
but not *where* or *what to fix*. Success: every failure screen shows actionable detail (file:line,
a stack frame, failing coverage, queue error, friendly cancel outcome) and the on-disk report path
for full detail. Milestone: next minor (v1.2.0).

## Problem / Motivation

- `report.go` discards `lineNumber`/`columnNumber`/`fileName`/`problemType`, `stackTrace`,
  `codeCoverage[]`, `flowCoverageWarnings`, and `stateDetail`; `view.go:986` tells the user to
  "revisa el JSON crudo" while **no** screen shows raw JSON or its on-disk path.
- `SucceededPartial` (a DISTINCT terminal state, exit 68) folds into `StateSucceeded` and renders
  as full success apart from counters (`app.go:769-778`).
- Queue SOQL never selects `StateDetail`/`ErrorMessage`, so a failed queued job shows no reason.
- Cancel/quick responses are never decoded (`Raw` only); an already-terminal cancel surfaces as a
  raw error, and failed cancel/quick/validate-launch raw bodies are dropped, not persisted.
- The exit-code-independent report parsing (`report.go:176-201`) is docs-validated as CORRECT and
  is kept unchanged.

## Scope

### In Scope
- Parse + render componentFailures `lineNumber`/`columnNumber`/`fileName`/`problemType` (Warning
  entries labeled).
- Parse + render test-failure `stackTrace`, compactly (bounded frames).
- Surface the persisted `report-NNN.json` PATH on failure/result screens (replace the dead-end
  "revisa el JSON crudo" hint).
- `SucceededPartial` gets an explicit partial-success callout + distinct header — never plain
  success.
- Parse per-class `codeCoverage[]` + `flowCoverageWarnings[]`; render failing classes' %.
- Queue SOQL adds `StateDetail`/`ErrorMessage`/`ErrorStatusCode`; queue screens render failure
  detail.
- Decode cancel/quick responses; already-terminal cancel classified as a friendly outcome; raw
  persisted on failure.
- Failed validate-LAUNCH raw envelope persisted + surfaced (message + actionable detail, HU-010 AC).

### Out of Scope
- `RunSpecifiedTests --tests` plumbing (latent bug, `commands.go:981-987`) — separate change.
- A full raw-JSON viewer screen — the surfaced path suffices.
- Changing the exit-code-independent report parsing — docs-validated as correct.
- `stateDetail` live-progress display — nice-to-have, not an error gap.

## Approach — Decisions

| # | Decision | Rationale |
|---|----------|-----------|
| D1 | All parsing stays additive in `internal/salesforce` (`DeployReport`, queue, cancel, quick, validate structs grow new fields); `app`/`view` render parsed data only; NO new exec surface. | Preserves the exec boundary; additive struct growth loads old data zero-valued and keeps `os_runner` exit-code-as-data intact. |
| D2 | Stack traces render in COMPACT form (first N frame(s)); the full multi-line trace lives in the persisted report whose path is now shown (D5). | Multi-line traces would overflow the TUI; the path is the escape hatch to full detail. |
| D3 | Per-class coverage renders only classes below the 75% gate / the N worst, not the full `codeCoverage[]`; the org-wide gate warning (EMPTY `name`) stays distinguished from per-class ones. | `codeCoverage[]` can hold hundreds of classes; empty-name distinction is already handled. |
| D4 | `SucceededPartial` = callout + distinct header on the EXISTING success screen, NOT a new app state. | A new state would ripple through resume, persistence, and quick-deploy eligibility (which already treats `SucceededPartial`) for little gain; a callout is byte-local. |
| D5 | Failure/result screens surface the LATEST persisted `report-NNN.json` path (replacing "revisa el JSON crudo"); no raw-JSON viewer screen. | The path unlocks full detail with zero new UI; a viewer is out of scope. |
| D6 | Already-terminal cancel (`CannotCancelDeployPre`-shaped envelope `name`) is classified via a package SENTINEL (e.g. `ErrCancelAlreadyTerminal`), mirroring the existing `ErrQueuePermission` idiom; the decode still yields structured data for the success path; raw persisted on failure too. | Sentinel is the codebase's proven pattern for an expected non-fatal CLI outcome the app branches on via `errors.Is`; keeps classification in `salesforce` and the app branch trivial, and composes with the byte-compatible `MarkCanceled`/`MarkQuickDeployed` success paths. |
| D7 | Failed validate-LAUNCH raw envelope is persisted via the Writer COMPANION pattern (like `cancel.json`/`report-NNN.json`) + a one-line path on the error screen; message + actionable detail surfaced. | Mirrors the established writer pattern and fulfills HU-010's original AC; exact no-`jobId` location resolved in design (see Risks). |
| D8 | Queue SOQL adds `StateDetail`/`ErrorMessage`/`ErrorStatusCode`; `ListDeployQueue`/`QueueReview` render failure detail for failed/errored jobs. | Same Tooling-API SOQL DeployDeck already runs; additive columns only. |

## Capabilities

### New Capabilities
- None — all 8 gaps extend existing capabilities additively.

### Modified Capabilities
- `validation-progress`: componentFailures detail (line/column/file/problemType + Warning
  labeling); compact test `stackTrace`; per-class `codeCoverage[]`/`flowCoverageWarnings`
  rendering (bounded); `SucceededPartial` partial-success callout; surface the persisted
  `report-NNN.json` path on failure/result screens.
- `deploy-queue`: SOQL selects `StateDetail`/`ErrorMessage`/`ErrorStatusCode`; queue screens
  render failure detail.
- `validation-cancel`: decode the cancel response; classify already-terminal via
  `ErrCancelAlreadyTerminal` sentinel (friendly outcome); persist raw on failure.
- `quick-deploy`: decode the quick response; persist raw on failure (mirrors cancel).
- `deploy-validation`: failed validate-LAUNCH raw envelope persisted + surfaced (fulfils the
  existing "CLI Error Surfaces Message And Raw JSON" requirement the code currently drops).
- `run-persistence`: Writer companion(s) persisting failed cancel/quick raw + the
  validate-launch-failure raw.

## Affected Areas

| Area | Impact | Change |
|------|--------|--------|
| `internal/salesforce/report.go` | Modified | Parse line/column/file/problemType, stackTrace/time/type, `codeCoverage[]`, `flowCoverageWarnings`, stateDetail, success/checkOnly (additive). |
| `internal/salesforce/queue.go` | Modified | SOQL + parse `StateDetail`/`ErrorMessage`/`ErrorStatusCode`. |
| `internal/salesforce/cancel.go` + `quick.go` | Modified | Decode response; `ErrCancelAlreadyTerminal` sentinel; return/persist raw on failure. |
| `internal/salesforce/validate.go` | Modified | Capture + return the raw envelope on launch failure. |
| `internal/app/view.go` | Modified | Render component detail, compact traces, per-class coverage, report path, partial callout, queue failure detail. |
| `internal/app/app.go` | Modified | `SucceededPartial` renders the partial callout instead of folding silently into full success. |
| `internal/app/update.go` + `commands.go` | Modified | Persist failed cancel/quick + validate-launch raw; wire report path onto failure screens. |
| `internal/runs/writer.go` | Modified | Companion writer(s) for failed cancel/quick + validate-launch-failure raw. |

## Risks

| Risk | Likelihood | Mitigation |
|------|-----------|------------|
| Extending render lines touches many `strings.Contains` assertions | High | Strict TDD; update assertions deliberately; additive parsing stays backward-compatible. |
| Failed validate-launch has no `jobId` yet → no run dir to hold the companion | Med | D7: reuse the no-`jobId` `validate.json` fallback or a minimal companion; resolve exact location in design. |
| Large stack traces / coverage arrays overflow the TUI | Med | Bounded compact rendering (D2/D3); full detail via the surfaced path (D5). |
| Cancel/quick decode must stay byte-compatible with asserted `MarkCanceled`/`MarkQuickDeployed` | Med | Decode is additive; sentinel only on the already-terminal branch; success paths unchanged. |
| >400 changed lines across salesforce + app + runs under strict TDD | High | Single PR, `size:exception` pre-approved. |

## Rollback Plan

Additive and render-gated. Parsing struct growth is backward-compatible (fields zero-valued when
absent), so persisted `report-NNN.json`/`run.json` files are unaffected — no schema bump, no
migration. Revert the `view`/`app` render additions and the Writer companion calls to restore the
prior screens; the CLI exec surface is untouched throughout.

## Success Criteria

- [ ] componentFailures render `file:line[:col]` + `problemType`; Warning-typed entries labeled distinctly.
- [ ] Test failures render a compact stack frame; the full trace is reachable via the surfaced report path.
- [ ] Failure/result screens show the latest `report-NNN.json` path (no "revisa el JSON crudo" dead-end).
- [ ] `SucceededPartial` renders a distinct partial-success callout + header, never plain success.
- [ ] Coverage failures render failing/below-75% classes' %, bounded; the org-wide gate warning stays distinct.
- [ ] Queue screens show `StateDetail`/`ErrorMessage` for failed/errored jobs.
- [ ] Already-terminal cancel renders a friendly outcome (not a raw error); failed cancel/quick raw persisted.
- [ ] Failed validate-launch surfaces message + actionable detail + a persisted-raw path; the flow does not crash.
- [ ] Existing byte-asserted success/cancel/quick behaviors remain green.

## Delivery Note

Single PR, strict TDD. `size:exception` pre-approved (>800 lines allowed) given the fan-out across
`salesforce` parsing, `app`/`view` rendering, and `runs` persistence. Milestone v1.2.0. Next
recommended phases: `sdd-spec` and `sdd-design` (parallel).
