# Archive Report — deploy-error-detail

**Status:** archived · **Delivery:** single-PR (size:exception) · **Mode:** strict TDD · **Milestone:** v1.2.0

## Summary

Surfaces the failure detail the Salesforce CLI already emits but DeployDeck discarded — the 8 gaps
found by crossing the official CLI/Metadata API documentation against a full audit of the error
pipeline (both performed by parallel research agents, synthesized by the orchestrator):

1. Component failures now render `fileName:lineNumber[:columnNumber]` and label `problemType:
   Warning` entries distinctly (amber) from errors (red).
2. Test failures render a compact one-frame stack-trace excerpt (muted); the full trace lives in
   the persisted report whose path is now visible.
3. Failure/result screens show the latest persisted `report-NNN.json` path, replacing the
   "revisa el JSON crudo" dead-end that pointed at no reachable viewer.
4. `SucceededPartial` renders a distinct header + amber callout — never plain success
   (`terminalState` unchanged; quick-deploy eligibility unaffected).
5. Per-class `codeCoverage[]` below the 75% gate renders worst-first (max 10 rows, `+K más`
   overflow), with flow coverage warnings inline; the org-wide empty-name warning stays distinct.
6. Queue SOQL gains `StateDetail`/`ErrorMessage`/`ErrorStatusCode` (WHERE unchanged — the view is
   the ACTIVE queue); erroring in-queue entries show their error detail with StateDetail context.
7. Cancel/quick responses are decoded; an already-terminal cancel (`CannotCancelDeployPre`/
   `CannotCancelDeploy` envelope names) classifies via the `ErrCancelAlreadyTerminal` sentinel as
   a friendly outcome (run untouched, no companion); generic failures persist
   `cancel-error.json`/`quick-error.json`.
8. A failed validate LAUNCH persists its raw envelope (`validate.json` companion, or a
   `Create(Failed)` record when no runID exists) and surfaces the path (HU-010's original AC).

All parsing stays additive in `internal/salesforce` (JSON tags only on private envelope structs);
`internal/runs` gained `SaveRawCompanion` and `AppendReport` now returns the written path; the
view renders parsed data only. Docs-validated invariant kept: the report poll parses stdout
independently of the exit code (the CLI emits full failure results on stdout with exit != 0).

## SDD trail

exploration (orchestrator-synthesized from two parallel research passes: official sf CLI docs +
plugin-deploy-retrieve source, and a full repo error-pipeline audit) → propose → spec + design
(parallel) → fresh-context design validation (FAIL → reconciled) → tasks (45) → apply (45/45,
strict TDD, incl. two revert-to-prove-RED self-corrections) → verify (PASS 0C/0W) + full 4R
adversarial review (parallel) → consolidated remediation → archive. Artifact store: OpenSpec.

## Design-validation corrections (pre-apply)

The validator caught two CRITICAL spec↔design contradictions: the deploy-queue delta promised
error detail for terminal `Failed` jobs the active-queue SOQL can never return, and a
StateDetail-when-present scenario broader than the errored-only design gate. Both resolved by
rewording the scenarios to the reachable condition (erroring in-queue entries), keeping the WHERE
clause and the proposal's out-of-scope line (progress-UX) intact. A WARNING (whether the
already-terminal sentinel writes a companion — it does NOT) and a JSON-tag-convention note were
made explicit in the design.

## Review & remediation

Five parallel reviews: **sdd-verify PASS** (0C/0W; 29/29 scenarios mapped to named tests, 45/45
tasks, boundary clean); **resilience CLEAN** (primary error always set before companion writes;
failed report writes never break polling; the no-runID Failed record is not resumable; SOQL
failure degrades with retry); **risk 1 WARNING**; **readability 1 WARNING + 1 SUGGESTION**;
**reliability 1 WARNING + 1 SUGGESTION**. Consolidated remediation (strict TDD, RED→GREEN each):

- **Terminal-injection sanitization** (risk): org-sourced strings (another user's queue
  `ErrorMessage`/`StateDetail`, stack frames, component/test/coverage messages, `CreatedBy`) are
  now stripped of C0/C1 control characters (incl. ESC/BEL) via `sanitizeTerm`/`sanitizeTermLine`
  before rendering; single-line contexts also collapse newlines/tabs — a crafted deploy error can
  no longer inject fake queue rows or escape sequences into the operator's terminal.
- **Single `hasStructuredFailureDetail` predicate** (readability): the coverage render condition
  and the "sin detalle estructurado" fallback now derive from one predicate — a Failed report
  whose only detail is below-gate coverage or flow warnings can no longer render both the detail
  AND the contradictory fallback line.
- **`NumLocations==0` classes excluded** (reliability): a class with no executable locations is
  vacuously covered (N/A), no longer a false 0% culprit displacing genuine ones into `+K más`.
- **Code-only queue error formatting** (reliability): `(CODE)` renders without a dangling
  separator via `queueErrorDetail`, consistent with validationBody's org-error guard.

## Verification

- `go build ./...`, `go vet ./...`, `gofmt -l internal cmd` clean.
- `go test ./... -race -count=1` green across all 14 packages (independently re-run by the
  orchestrator after remediation).
- Exec boundary preserved; no new exec surface anywhere.

## Spec merge note

The six delta specs were merged ADDITIVELY into the living specs by the orchestrator (replace the
named MODIFIED requirement blocks with their supersets, append ADDED requirements; delta-only
`(Previously: …)` notes dropped): validation-progress 110→159, deploy-queue 59→74,
validation-cancel 57→76, quick-deploy 110→124, deploy-validation 55→60, run-persistence 213→245.
Diffstat +144/−10 (the −10 being replaced requirement sentences); no pre-existing requirement or
scenario lost; no delta markers leaked.

## Follow-up (accepted, non-blocking)

- `ValidateRequest.Tests` plumbing for `RunSpecifiedTests` remains unimplemented (latent,
  pre-existing, explicitly out of scope) — wire it or remove the option in a future change.
- `stateDetail` live-progress display during polling (nice-to-have UX, not an error gap).
- The CLI-added `files[]` (local source paths per failure) remains unparsed (nice-to-have).
