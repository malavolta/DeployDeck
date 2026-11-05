# Verification Report: tui-ux-consistency (change A), round 1

**Verdict: FAIL.** 7 CRITICAL, 9 WARNING, 3 SUGGESTION.

- **Commands:** vet, gofmt, build, `go test ./... -short -race` and `go test ./internal/app/... -count=1` (427 PASS) are all green.
- **Size:** 1034 lines, against fd7dac1 and excluding openspec. The user accepted about 970.
- **Boundary:** `boundary_test.go` is unchanged and there is no bubbles dependency.
- **No regressions** in resume, the `quitCmd()` restore, the DESPLEGAR/CANCELAR/BORRAR gates, or ctrl+c.
- **Conflict cursor:** verified at both entry sites.

## CRITICAL
- **C1.** The push-ready footer never lists `a` (`view.go:1018`, `:1040`), although the handler acts on it at `keys.go:443-457`. There is no test.
- **C2.** Footer parity is only checked for esc and q (`footer_parity_test.go:28-47`). Three real violations:
  1. C1 above.
  2. In the cleanup confirm and prune confirm views, `n` acts but is not listed (`view.go:1473`, `:1481`).
  3. **Regression.** On Succeeded or Aborted, the inline delete confirmation reached through the newly advertised `d` renders no prompt. The footer still offers p/d/Enter/q/Esc, but `q` and `esc` now cancel silently (`keys.go:87-91`, `:135-137`, `:171`).
- **C3.** Cleanup columns are narrower than their headers. "Antigüedad" is printed into `%-6s` (`view.go:1451`), and "pusheada"/"sin push" into `%-7s` (`:1461-1462`). The requirement's width clause was not removed when ADR-8 was cut.
- **C4.** English user-facing copy remains:
  - `mergedLabel` returns "unknown", "likely merged" and "abandoned" (`commands.go:440-445`);
  - the re-promote warning at `view.go:427`;
  - "Deploy"/"Validate" (`view.go:660`, `:662`);
  - "Components/Tests" (`view.go:664`);
  - "(timed out)" (`view.go:909`);
  - "title de arriba" (`view.go:1038`, `:1062`).
- **C5.** Untested scenarios:
  - the commit-selection footer;
  - the push-ready AI footer;
  - declining the resume offer with esc, quitting, then seeing it offered again on the next launch (task 3.8);
  - `r` on Canceled/Aborted, which is only partially covered.
- **C6.** `tasks.md` is out of sync. Tasks 6.2-6.4 are done but unchecked; the cut tasks are not marked as cut; 3.6 and 3.8 are not done.
- **C7.** `apply-progress.md` has no TDD Cycle Evidence table.

## WARNING
- **W1.** Size is +64 over the accepted exception.
- **W2.** design.md, proposal.md, tasks.md and state.yaml still describe ADR-8 and ADR-9 as live.
- **W3.** ADR-6 says unknown reasons fall back to raw text, but the implementation uses a Spanish fallback.
- **W4.** Footer casing is inconsistent: `esc/q cancelar` in some views, "Esc …" elsewhere.
- **W5.** Notices now clear on every key, so async notices vanish on the next key press.
- **W6.** Several tests are weak, for example states set by hand and missing `cmd == nil` assertions.
- **W7.** The parity fixtures skip sub-phases.
- **W8.** "Destino" overflows `%-6s` in the run history (`view.go:1309`).
- **W9.** Some pre-existing tests were edited beyond the pins the design listed.

## SUGGESTION
- **S1.** Extend footer parity to sub-phases, or restore a per-key probe.
- **S2.** Render a prompt and an honest footer for the inline delete confirmation.
- **S3.** Widen the cleanup columns to 10/8.

---

# Verification Report: round 2 (after the corrective apply)

**Verdict: PASS WITH WARNINGS.** 0 CRITICAL, 6 WARNING, 3 SUGGESTION.

## Checks

- **Commands:** vet, gofmt, build, `go test ./... -short -race -count=1` (15/15 ok) and `go test ./internal/app/... -count=1` (439 PASS, 0 FAIL) all pass.
- **Size:** 1505 lines (514 tracked plus 991 in new test files). This equals the user-approved exception.
- **Boundary:** `boundary_test.go` and go.mod/go.sum are unchanged; there is no bubbles dependency.
- **Round-1 CRITICAL findings:** C1 through C7 are all resolved.
- **C2.3 runtime probe:** the probe exercised the delete confirmation on Succeeded and on Aborted, in both normal and strong mode. The prompt renders, the footer is honest, and n/esc/q cancel. In strong mode the typed-q rule holds and the BORRAR gate is intact.
- **Spec coverage:** 74 scenarios. 73 are COMPLIANT. 1 is PARTIAL: the footer parity matrix is probed with every key only on the 12 sub-phases. The runtime probe found no real violation across 43 fixtures.
- **Regressions:** none in resume, the quitCmd restore, DESPLEGAR/CANCELAR/BORRAR, or ctrl+c.

## WARNING

- **W1 and W2:** stale bookkeeping in tasks.md, proposal.md, design.md, apply-progress.md and state.yaml. **Fixed by the orchestrator.**
- **W3:** the TDD evidence table covers only the corrective round and has no Safety Net column.
- **W4:** the footer parity scenario is only partly met (see Checks). Recorded as a follow-up; change B replaces footers with `key.Binding`.
- **W5:** on Succeeded/Aborted, a wrong word in the strong delete confirmation sets a notice (`keys.go:177`) that `deleteConfirmView` never renders (`view.go:1078`). This predates the change. Recorded as a follow-up for B.
- **W6:** the conflict continue-gate pending reasons are still in English (`internal/git/continue_gate.go:26,29`), as part of the ADR-9 cut. The requirement now records this as a known exception, and a follow-up is recorded.

## SUGGESTION

- **S1:** the conflict footer lists `c` while continue is disabled; there `c` only shows a notice.
- **S2:** the doc comment at `keys.go:229` still describes the old back target for q/esc.
- **S3:** a few tests are weak. The "none" outcome does not assert `cmd == nil`, and one notice test sets its state by hand. "Warning:" at `view.go:484` could become "Aviso:".
