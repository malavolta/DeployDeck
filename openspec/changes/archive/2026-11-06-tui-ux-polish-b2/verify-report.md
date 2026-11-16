# Verification Report: tui-ux-polish-b2, round 1

**Verdict: FAIL.** 1 CRITICAL, 5 WARNING, 4 SUGGESTION.

## Summary

- **Commands:** `vet`, `gofmt`, `build`, `go test ./... -short -race` and `go test ./internal/app/... -count=1` are all green (482 PASS lines).
- **Size:** 1265 of the 1300-line budget.
- **Boundary:** `boundary_test.go` is unchanged. `go.mod` only promotes `x/ansi` to a direct dependency.
- **Spec coverage:** 58 scenarios. 54 compliant, 1 partial, 2 failing, 1 untested.
- **Footer diffs** are exactly the expected ones: the `? ayuda` suffix and the W2 disables. At size 0, the view bodies are byte-identical to e8d783a.
- **Spinner:** a leak hunt found no leaks. The single-flight guard holds.
- **Overlay:** the `?` overlay and the text-input behavior are correct.
- **Polling clock:** correct.
- **Regressions:** none.

## CRITICAL

- **C1.** `footer_parity_test.go:311` hard-codes `listed=true`, so the strict-parity requirement (W5) is not enforced.
  - `blockedNotice` and `isBlockedNotice` are dead code.
  - A mutation run shows six notice-only Enter keys that are enabled but not listed: quick deploy, package review empty, CommitSelection, TargetSelection, PickVerification and QuickDeploy.
  - Fix: wire in `isBlockedNotice`, then list or disable each of those keys. Give the quick-deploy fixture an eligible run.

## WARNING

- **W1.** `window.go:52/58`: with both overflow indicators visible, the view uses `height` newlines plus a trailing newline. bubbletea then drops the top line (the title or the banner). Reserve one more row.
- **W2.** The `isTyped == textInput()` cross-check is missing, even though the task is checked.
- **W3.** `isBatch` and `isTick` depend on `tea.Batch` and `tea.Tick` being inlined. `go test -gcflags=all=-l` fails at HEAD but passes at e8d783a. Negative assertions can pass silently when detection breaks.
- **W4.** Strict TDD evidence: in U2, U4 and U5 the RED step landed together with stubs. The evidence for tasks 1.3 and 1.4 is contradicted by C1.
- **W5.** W7/U7 was cut, so the shared-predicate gap remains.

## SUGGESTION

- **S1.** The `dd-pkg` temp dirs are never removed (`footer_parity_test.go:192`).
- **S2.** Add a golden pin for zero-size byte identity.
- **S3.** Reset `queueLoading` on Esc.
- **S4.** The polling tick runs at 120 ms; 1 s would be enough.

---

# Verification Report: round 2 (after corrective commit 5c9d708)

**Verdict: PASS WITH WARNINGS.** 0 CRITICAL, 3 WARNING, 5 SUGGESTION. All 58 of 58 scenarios are compliant.

## Checks

- **Commands:** vet, gofmt, build, `go test -short -race`, `go test ./internal/app -count=1` (976 PASS) and the `-gcflags=all=-l` run are all green.
- **Size:** 1379 changed lines, 79 over the 1300 ceiling. The overrun is the user's decision.

## Round-1 findings, all resolved

- **C1:** mutation probes M1 to M6 were run. An unlisted notice-only key fails, as does a stale entry or an acting entry.
- **W1:** a sweep of 12,760 renders found no overflow above the `max(3,…)` floor, and the height-0 output is byte-identical to e8d783a.
- **W2:** the `isTyped` / `textInput()` cross-check is now present.
- **W3:** the suite passes under `-gcflags=all=-l`, and the canary runs under any filter.
- **S1:** temp dirs are cleaned up.
- **S3:** Esc clears `queueLoading`.

There are no regressions: the footer pins are unchanged against ce296e3, and `boundary_test.go` is unchanged.

## Remaining warnings

- **W4:** TDD evidence ordering in U2, U4 and U5 is a recorded deviation.
- **W5:** U7 was cut, so the W7 shared-predicate gap remains.
- **W6:** the size overrun described above.

## Suggestions (follow-ups)

- **S2:** add a golden pin for zero-size output.
- **S4:** lower the polling tick from 120 ms to 1 s.
- **S5:** tighten the `rawBatch` prefilter so it excludes `tea.Every` and test closures whose names contain `Batch`.
- **S6:** decide how to handle the `max(3,…)` floor overflow on very short terminals.
- **S7:** clean up minor weaknesses in the parity test: a stale-entry check for keys that act and also show a notice, the unused-entry check sitting outside its subtest, and checking that the notice actually appears in `View()`.
