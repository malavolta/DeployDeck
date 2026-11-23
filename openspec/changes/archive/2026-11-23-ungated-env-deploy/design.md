# Design: Ungated Environment Deploy

## Technical Approach

Add a deploy offer (`x`) to `StateSucceeded`. It reuses HU-015's `StateQuickDeploy` without changes to its Enter path. When the validation reaches terminal success, `onReportDone` loads the run from disk and upserts it into `m.runs`. A pure predicate classifies the offer as hidden, unavailable (with a reason) or available. Pressing `x` points `m.runsCursor` at that row, so `quickDeployCmd` (`commands.go:1336`) still reads `m.runs[m.runsCursor]`. A bool origin flag sends Esc/q back to the result screen. The change adds no new state, imports or subprocesses.

## Verified Facts

| Fact | Evidence |
|---|---|
| Target, Alias and TestLevel are persisted at branch creation | `update.go:660-672`, `update.go:742-754` |
| JobID and Status=Queued are persisted when validation launches | `commands.go:1065-1073` |
| Terminal Status is persisted synchronously before the state flips | `update.go:1210-1215` (AppendReport), then `update.go:1238` |
| The standalone validate record has `Target:""`, so the target is classified as production (offer hidden) | `keys.go:885-893`, `source_suggestion.go:122` |
| The no-runID fallback `Create` omits TestLevel, so the offer is unavailable with "no se ejecutaron los tests requeridos" | `commands.go:1085-1094` |
| History reload replaces `m.runs` and resets the cursor, so an upsert never leaks into history | `update.go:407-408` |
| `Deps.Runs` is a concrete `*runs.Writer` with `Load` | `app.go:296`, `writer.go:192` |

## Architecture Decisions

| Decision | Options | Tradeoff | Choice |
|---|---|---|---|
| Where the run is loaded | (a) `Load` in `onReportDone` on terminal success, upserted into `m.runs`; (b) `Load` on the `x` press; (c) a separate `m.resultRun` field | (b) leaves View with no record for the footer/line, and doing IO in View is forbidden. (c) needs `quickDeployCmd` and `onQuickDeployDone`'s H-2 mirror (`update.go:1164`) to change. | **(a)**. It follows the existing sync-IO precedent (AppendReport) and keeps the H-2 in-memory mark working by RunID. |
| Upsert placement | Replace the row with the same RunID, otherwise prepend | Prepending keeps the list newest-first. Resumed runs are replaced in place. | Replace or prepend. `m.runsCursor` is untouched until `x`. |
| Status source | Disk only, or disk plus the live `msg.report.Status` | A best-effort AppendReport failure would leave `Queued`. | Overlay the live Status in memory. Salesforce is authoritative. |
| Offer when `allowExecution:false` | Open the suggest-only screen, or show "unavailable" | The proposal requires "unavailable, never executes". | Unavailable, with a reason line. `x` is a strict no-op. |
| Origin | `quickFromResult bool`, or `quickReturn State` | The zero State is `StatePrereqCheck`, which makes it error-prone. | `quickFromResult bool` |
| `x` when unavailable | Notice, or strict no-op | A notice would break the footer-parity "advertised iff acts" invariant. The reason is already on screen. | Strict no-op, with a disabled hint. |

## Offer Predicate (pure)

```go
type deployOffer int
const (offerHidden deployOffer = iota; offerUnavailable; offerAvailable)

// resultDeployOffer: gated -> hidden; IsProductionTarget -> hidden;
// !QuickDeployEligible -> unavailable(quickDeployReasonES(reason));
// !quickDeployExecAllowed(cfg,false) -> unavailable("la ejecución está deshabilitada (quickDeploy.allowExecution)");
// else available.
func resultDeployOffer(cfg config.Config, rec runs.Record, now time.Time) (deployOffer, string)
```

`m.resultOffer()` returns hidden unless `m.state == StateSucceeded` and a row exists with `RunID == m.runID`. Both the footer and `keySucceeded` call it.

## Data Flow

```
onReportDone(terminal success) ── Runs.Load(runID) ──> upsert m.runs (Status=live)
StateSucceeded ── View: resultOffer() ──> line + footer hint
   └─ x (available) ── runsCursor=idx, quickFromResult=true ──> StateQuickDeploy
StateQuickDeploy ── Enter: unchanged guards + GateFor (keys.go:1548-1609) ──> quickDeployCmd
   └─ Esc/q(empty) ── quickFromResult ? StateSucceeded : StateRunHistory; flag=false
onQuickDeployDone ── marks row QuickDeployedAt ──> offer becomes "ya se desplegó…"
```

Reset points for `quickFromResult`:
- It is set true in `keySucceeded` on `x`.
- It is set false in the run-history `x` (`keys.go:1433`) and on every Esc/q exit (`keys.go:1621-1631`).
- `keyDeployGateBlocked` is unreachable from this origin, because the target is ungated and the config is static. It stays unchanged.

The in-flight guards `quickDeployingRunID`/`gateCheckingRunID` and the gated check at `keys.go:1597` are unchanged.

## File Changes

| File | Action | Description |
|---|---|---|
| `internal/app/result_offer.go` | Create | `deployOffer`, `resultDeployOffer`, `m.resultOffer()`, `m.upsertResultRun()` |
| `internal/app/update.go` | Modify | `onReportDone`: call `upsertResultRun` when `terminalState == StateSucceeded` (~1238) |
| `internal/app/keys.go` | Modify | `keySucceeded` `x` case. Run-history `x` resets the flag. Esc/q in `keyQuickDeploy` returns to the origin. |
| `internal/app/app.go` | Modify | `quickFromResult bool` field |
| `internal/app/keymap.go` | Modify | `StateSucceeded` with offer not hidden: append `hint(keys.Quick,"x","desplegar",available)` before `exit` (`keymap.go:191-192`). Other terminals stay unchanged, so the overlay shows nothing new for them. |
| `internal/app/view.go` | Modify | `viewValidationResult` (`view.go:932`), before `noticeLine`: available shows `mark("i") Entorno sin gate: x despliega esta validación (quick deploy, requiere DESPLEGAR)`; unavailable shows `mark("i") Despliegue no disponible: <reason>` |
| `README.md` | Modify | "Deploying to a test environment": ungated default, `branches` mapping footgun, the `allowExecution` opt-in |
| `docs/HISTORIAS.md` | Modify | HU-020 after HU-019 (~1151), e2e row (~1287), refresh HU-015 wording (~987) |
| `docs/MOCKUPS_TUI.md` | Modify | Result screen with the offer line and footer near the quick-deploy mockup (~420) |

Boundary: `internal/app` gains no imports. `config`, `git` and `runs` are already imported, and there is no `os/exec`, `net/http`, `internal/version` or `internal/update`. The deploy still flows only through `Deps.SF` and then `internal/exec`.

## Testing Strategy (Strict TDD)

| Layer | What | Approach |
|---|---|---|
| Unit | `resultDeployOffer` | Table cases: gated, disabled-gate, production, unmapped, empty target, each ineligible reason, allowExecution false, available |
| Unit | `upsertResultRun` | Real `runs.NewWriter(t.TempDir())`: replace vs prepend, live Status overlay, nil Runs or Load error leaves the offer hidden |
| Unit | Keys | `x` available opens StateQuickDeploy with the right cursor. Unavailable, hidden and gated are no-ops. Esc/q(empty) returns to StateSucceeded. History `x` still returns to history. |
| Matrix | `esc_q_matrix_test.go:103` | Add a "DESPLEGAR from result" row with `back: StateSucceeded` |
| Parity | `footer_parity_test.go` | Fixtures for succeeded offer available and unavailable, plus a pinned `x desplegar` fragment. Add a `keymap_states_test.go:82` case. |
| E2E | `quick_deploy_e2e_test.go` style | Real writer and fake SF: poll to Succeeded, `x`, type DESPLEGAR, Enter, then QuickDeploy is called with the run's JobID/Alias and Esc returns to the result |

## Threat Matrix

All rows are N/A. The change adds no new subprocess, routing or VCS/PR automation. It reuses the existing `sf project deploy quick` dispatch behind unchanged guards. Documentation-like paths, Git repo selection, commit state, push state and PR commands are all N/A.

## Line Budget (~650 approved)

| Area | Lines |
|---|---|
| Production code | ~110 |
| Tests | ~280 |
| Docs (README, HISTORIAS, MOCKUPS) | ~150 |
| **Total** | **~540** (about 110 lines of headroom) |

## Migration / Rollout

No migration is required. There is no schema change, because `Record` is untouched.

## Open Questions

None blocking.
