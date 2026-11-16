# Verify Report: TUI Bubbles Adoption (change B, core)

**Change**: `tui-bubbles-adoption`
**Scope**: U1 (keymap, footers, handlers, structural parity), U2 (wrong-word notice), U5 (AdaptiveColor). U3 and U4 were CUT to B2 by user decision (2026-11-05).
**Commits**: `d0ebf0b` (U2), `16f577f` (U5), `14da889` (U1), `360d0d6` (verify fixes).
**Verdict**: **PASS WITH WARNINGS**. Round-1 CRITICAL C1 is FIXED by `360d0d6`. W3 is FIXED by `360d0d6`. No CRITICAL issues remain.

> Source note: this report was persisted from the verifier's summary. The verification commands were reported green by the orchestrator. The archive executor did not re-run them (no shell access).

---

## Findings Resolved

| ID | Severity | Finding | Resolution |
|----|----------|---------|------------|
| C1 | CRITICAL | The queue-review error footer said `r refrescar` instead of `r reintentar`. This was an unintended footer change. | FIXED in `360d0d6` under TDD. Added pin row `queue failed` in `footer_pins_test.go`. |
| W3 | WARNING | On an empty cleanup list, `p` opened a prune confirmation that was not rendered. `y` then pruned run dirs. | FIXED in `360d0d6`. The prompt and its y/n footer now render. Test: `TestCleanup_EmptyListPruneConfirmIsVisible`. |

## Spec Coverage

- All 19 delta scenarios have passing tests. The first run had a FAIL, which was fixed (see the Findings above).
- The scenario mapping follows `tasks.md` "Scenario Coverage Index" for U1, U2 and U5. U3 and U4 scenarios are CUT to B2.

## Commands

| Command | Result |
|---------|--------|
| `go vet ./...` | green |
| `gofmt -l` | clean |
| `go build ./...` | green |
| `go test ./... -short -race` | green |
| `go test ./internal/app/... -count=1` | green |

## Dependency Check

- `go.mod` adds only `github.com/charmbracelet/bubbles v0.21.0`.
- `bubbletea v1.3.10` and `lipgloss v1.1.0` are unchanged.
- Only `bubbles/key` and `bubbles/help` are imported (non-test code).
- `boundary_test.go` is unchanged.

## Footer Byte Parity

- 60 of 61 footers are byte-identical to the pre-change output.
- Intentional change: the conflict footer drops `c` while continue is disabled (proposal S1 fix).
- Unintended change: C1 (`r refrescar`). Fixed in `360d0d6`.

## Regression Check

- No regressions in A's notices, the typed gates (CANCELAR, DESPLEGAR, BORRAR), ctrl+c, resume, or the `quitCmd()` restore.

## Coverage (internal/app, from verifier)

| File | Coverage |
|------|----------|
| `keymap.go` | 98.9% |
| `keys.go` | 96.9% |
| `view.go` | 94.4% |
| `style.go` | 100% |

## Warnings (remaining)

- **W1**: stale artifacts after the U3/U4 cut. FIXED in the archive step (state.yaml, proposal, design, tasks, apply-progress and the tui-presentation delta header were amended).
- **W2**: these keys are not caught by the parity fixtures:
  - history `Enter reanudar` on terminal runs (`keymap.go:180`);
  - push-ready `a` acts after the PR exists while the footer shows only `q salir` (`keymap.go:222-223`, `keys.go:451-466`).
  Follow-up: B2 (with fixtures for the W2/W3 cases, S4).
- **W4**: the ticket `b` is a guarded printable binding. This conflicts with the "no printable bindings in text-input states" MUST. Follow-up: B2.
- **W5**: the lenient rule "a notice counts as acting" is load-bearing (9 fixtures). Follow-up: B2.
- **W6**: sub-phase coverage relies on a hand-picked fixture list. Follow-up: B2.
- **W7**: enablement predicates are duplicated between `keymap.go` and `keys.go`. Follow-up: B2.
- **W8**: tasks 7.1 and 7.2 were open at verify time. Ticked in the archive step with the evidence above.

## Suggestions

- **S1**: the coverage check is partly tautological. Add a `stateCount` sentinel. (B2)
- **S2**: make parity strict with valid buffers. (B2)
- **S3**: speed up the slow quit-detector tests. (B2)
- **S4**: add fixtures for the W2/W3 cases. (B2)

## Verdict

PASS WITH WARNINGS. 0 CRITICAL. C1 and W3 are fixed by `360d0d6`. Remaining warnings W2, W4 to W8 and suggestions S1 to S4 are carried to the B2 follow-up (see `state.yaml` `follow_up_changes`).
