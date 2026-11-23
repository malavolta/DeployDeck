# Verification Report: ungated-env-deploy

**Verdict:** PASS WITH WARNINGS. There are 0 critical findings, 3 warnings and 3 suggestions.

## Evidence

| Check | Result |
|---|---|
| Tasks | 19 of 19 checked |
| Spec coverage | 6 requirements and 17 scenarios. 16 are compliant and 1 (DG2) is partial |
| `go vet ./...` | clean |
| `gofmt -l .` | empty |
| `go test ./...` | all 15 packages ok |

Size: 10 tracked files changed (+159/-13) plus 3 new files (87, 190 and 118 lines), about 567 changed lines in total.

## Safety checks

All six safety checks passed:

- (a) Gated, production and unmapped targets never get the offer, and no new path skips the gate check at `keys.go:1612`.
- (b) `x` does nothing when the offer is unavailable.
- (c) `allowExecution=false` never executes.
- (d) The origin flag is reset in `quickDeployBack` and in the history `x` key.
- (e) The `upsertResultRun` RunID-keyed replace or prepend works, and H-2 marking is intact.
- (f) No new imports were added and `stateCount` is unchanged.

## Warnings

- **W1:** DG2 has no app-level test that drives a disabled gate (`enabled: false`) through Enter and confirms it reaches quickDeployCmd without a gate check. It is covered only at the predicate level and by the GateFor unit tests.
- **W2:** The README's "Deploying to a test environment" section does not say that `NoTestRun` and `RunSpecifiedTests` sandboxes are ineligible. The whitelist is in `internal/runs/quick_deploy.go:14-17`.
- **W3:** Task 4.1 asks for a change to `keymap_states_test.go`, but that file was not edited. The case is covered by the footer parity fixtures instead.

## Suggestions

- **S1:** Add a negative gated-target case to the view and footer tests.
- **S2:** Make the README warning more specific: "listed under any `branches` key except `production`; `main` is always production".
- **S3:** Cosmetic: the prepend can shift the stale history cursor by one row until the history is reloaded.

## Fix round

See the "Fix round" section in apply-progress.md.
