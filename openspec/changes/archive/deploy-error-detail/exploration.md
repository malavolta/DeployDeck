# Exploration — deploy-error-detail

Close the 8 error-surfacing gaps found by crossing the official Salesforce CLI documentation
against DeployDeck's current deploy/validation error pipeline. Exploration was performed by two
parallel research passes (official docs + plugin-deploy-retrieve source; full repo audit with
file:line evidence) and synthesized by the orchestrator. User scope decision: apply ALL of gaps
1–8 (the four high-priority + four medium-priority findings).

## Grounding: what the CLI guarantees (docs reference)

- In `--json` mode EVERYTHING — including a failed deploy's full result and command-level error
  envelopes — is emitted on STDOUT, even with exit != 0. DeployDeck's exit-code-independent
  report parsing (`report.go:176-201`) is CORRECT per docs; keep it.
- `result.details.componentFailures[]` (Metadata API DeployMessage) ALWAYS carries `problem`,
  `problemType` (`Error`|`Warning`), `fullName`, `componentType`, `fileName` (zip path),
  `lineNumber`, `columnNumber` (code/XML errors). No flag needed.
- `result.details.runTestResult.failures[]` ALWAYS carries `message`, `stackTrace`, `methodName`,
  `name`, `namespace`, `time`, `type`. Per-test stack traces need NO extra flag.
- `runTestResult.codeCoverage[]` (per-class CodeCoverageResult) and `flowCoverageWarnings[]`
  exist in the same JSON. The org-wide 75% gate warning has EMPTY `name`; per-class warnings
  carry the class name.
- Exit codes: 0 Succeeded · 1 Failed/Canceled · 68 SucceededPartial · 69 InProgress/wait-timeout.
  `SucceededPartial` is a DISTINCT terminal state per the CLI.
- `sf project deploy cancel` returns a full DeployResultJson (`canceledBy(Name)` populated);
  typed errors: `CannotCancelDeployPre` (job already terminal), `CannotCancelDeploy` (cancel
  raced a finishing deploy; `INVALID_ID_FIELD` normalized to it).
- Tooling API `DeployRequest` exposes `StateDetail`, `ErrorMessage`, `ErrorStatusCode` —
  queryable via the SOQL DeployDeck already runs for the queue.
- The CLI normalizes single-element arrays via `ensureArray` before `--json` — object-vs-array
  quirk does NOT affect CLI consumers.
- Command-level error envelope shape: `{name, message, exitCode, status, code, context,
  commandName, stack, actions, data}` on stdout.

## Current state (audit, file:line)

- `internal/exec/os_runner.go:74-79`: exit != 0 is data, not a Go error. All sf calls flow here.
- `internal/salesforce/report.go:89-129`: DeployReport parses status, 6 counters, errorMessage,
  errorStatusCode, canceledByName, componentFailures{fullName,componentType,problem},
  runTestResult.failures{name,methodName,message}, codeCoverageWarnings{name,namespace,message}.
  DISCARDS: lineNumber/columnNumber/fileName/problemType; stackTrace/time/type; codeCoverage[];
  flowCoverageWarnings; stateDetail; success/checkOnly/dates.
- `internal/salesforce/queue.go:18`: queue SOQL selects Id/Status/CheckOnly/dates/CreatedBy/6
  counters — never StateDetail/ErrorMessage.
- `internal/salesforce/cancel.go:15-19` + `quick.go:15-19`: response NEVER decoded (Raw only);
  on failure the raw body is neither shown beyond err.Error() nor persisted
  (`update.go:995-998`, `1037-1040`).
- `internal/salesforce/validate.go:63-65`: only `result.id` decoded; on launch failure the Raw
  envelope is dropped by `onValidateDone` (`update.go:948-962`) — HU-010 asked for message + raw.
- `internal/app/view.go:942-987`: failure lists render ALL entries untruncated but WITHOUT
  line/column/file/problemType (942-947), WITHOUT stack traces (948-953), coverage warnings
  without per-class % (957-966; parsed Namespace never rendered). `view.go:986` says "revisa el
  JSON crudo" but no screen shows raw JSON or the on-disk path.
- `internal/app/app.go:769-778`: `SucceededPartial` folds into StateSucceeded — renders as full
  success apart from counters.
- Persistence (`internal/runs/writer.go`): every poll writes `report-NNN.json` (219-243) — the
  path exists on disk but is never surfaced. Failed cancel/quick raw responses are lost;
  `validate.json` written only on the no-runID fallback path (`commands.go:1016-1025`).
- Latent bug (out of the 8 but adjacent): `ValidateRequest.Tests` has no plumbing
  (`commands.go:981-987`) — RunSpecifiedTests would fail with a generic error. NOT in scope.

## The 8 gaps to fix (user-approved scope)

1. Parse + render componentFailures `lineNumber`/`columnNumber`/`fileName`/`problemType`
   (e.g. `- Comp (Type): problem — fichero:línea[:col]`, Warning-typed entries labeled).
2. Parse + render test-failure `stackTrace` (rendered compactly — e.g. first frame(s) — the full
   trace is in the persisted report JSON).
3. Surface the persisted `report-NNN.json` PATH on failure screens (replace the dead-end "revisa
   el JSON crudo" hint with the actual latest report path).
4. `SucceededPartial` gets an explicit partial-success callout (never renders as full success).
5. Parse `codeCoverage[]` per-class coverage + `flowCoverageWarnings[]`; render per-class % on
   coverage failures (org-wide gate warning distinguished by its empty name — already handled).
6. Queue SOQL adds `StateDetail`, `ErrorMessage` (+ `ErrorStatusCode`); render failure detail in
   the queue screens.
7. Decode cancel/quick responses; classify "job already terminal" (CannotCancelDeployPre-shaped)
   as a friendly outcome instead of a raw error; persist the raw response on failure too.
8. Persist + surface the raw envelope of a failed validate LAUNCH (store on model/run companion,
   show message + actionable detail per HU-010's original AC).

## Risks / constraints

- Exec boundary: all sf exec stays in `internal/salesforce`; app/view only render parsed data.
- Backward compat: `DeployReport` etc. grow additively; existing tests assert exact rendered
  strings — extending render lines will touch many `strings.Contains` assertions deliberately.
- Stack traces can be multi-line and long — rendering must be bounded (compact form) to keep the
  TUI readable; full detail lives in the persisted JSON whose path is now shown (gap 3).
- Per-class `codeCoverage[]` can be large (hundreds of classes) — render only failing/below-75%
  classes or the N worst, not the full table.
- The report poll loop's exit-code-independent parsing MUST stay as-is (docs-validated).
- Cancel/quick decode must keep the existing success behaviors (MarkCanceled/MarkQuickDeployed)
  byte-compatible where asserted.

Artifact store: OpenSpec (Engram MCP unavailable).
