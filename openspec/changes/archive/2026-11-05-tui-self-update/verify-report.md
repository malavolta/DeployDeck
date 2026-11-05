# Verification Report: tui-self-update

**Mode**: Strict TDD, openspec
**Worktree**: branch `feat/tui-self-update`, based on `origin/main` (e0d4036), uncommitted
**Initial verdict**: FAIL (1 CRITICAL, 7 WARNING, 3 SUGGESTION)
**Verdict after orchestrator fix**: PASS WITH WARNINGS

## Commands

All of the following exited 0:

| Command | Result |
|---|---|
| `go vet ./...` | Clean |
| `gofmt -l .` | No files listed |
| `go build ./...` | Clean |
| `go test ./... -short -race` | All 15 packages ok |
| `go test ./internal/app/... ./internal/update/... ./cmd/... -count=1` | All ok |

`internal/app/boundary_test.go` is unchanged and passes.

## Size

Size excludes `openspec/`. The tracked diff is 147 lines and the 7 new files add 653 lines, for 800 at verify time. The CRITICAL-1 fix added one line, so the total is **801**: one line over the user's 800-line budget.

## Spec compliance

There are 35 scenarios across 13 requirements:

- 30 are compliant.
- 4 are partial (W-2, W-3 and two cases of W-4).
- 1 was untested (CRITICAL-1), and is now fixed.

The full scenario-to-test matrix was returned by the verify agent. The partial scenarios are:

| Scenario | Gap |
|---|---|
| u on another screen does nothing | Only a negative assertion (W-2) |
| Requested upgrade runs after quit | The requested branch of `postRun` and the stdio of `runInherited` are not tested (W-3) |
| Non-zero exit prints manual commands | The test checks stdout and stderr together, so "the command is on stderr" is not asserted strictly (W-4) |
| Command cannot start | Same as W-4 |

## Checklist

1. **Scenario-to-test mapping:** complete after the CRITICAL-1 fix.
2. **Commands:** all clean.
3. **Boundary:** unchanged and passing.
4. **Isolation:** no test runs a real package manager, uses the network, or touches the real cache dir. Tests use `t.TempDir()` and `httptest`, and never call `runInherited` or `defaultRunTUI`.
5. **Tag validation:** the go-install tag must match `^v?\d+\.\d+\.\d+$` (`internal/update/method.go:15`, checked at `:87`).
6. **`u` key scope:** works only on StateMainMenu, and is blocked with a notice while an operation is in progress (`internal/app/keys.go:585-594`).
7. **Windows:** prints the command and never runs it (`cmd/deploydeck/selfupdate.go:39-42`).
8. **Opt-out:** setting the opt-out leaves `Cache` and `Checker` unbuilt, so there is no network or cache access (`selfupdate.go:65`).
9. **README:** documents the `u` key, the Windows behavior, the 24h cache and `DEPLOYDECK_NO_UPDATE_CHECK` (`README.md:65-68`).

## Issues

### CRITICAL

- **CRITICAL-1 (FIXED):** the scenario "Pre-release tag never triggers a notice" had no test. The orchestrator added the row `{name: "pre-release latest never nags", current: "1.0.0", latest: "v2.0.0-rc1", want: false}` to `internal/update/hasnewer_test.go`. It passes; the behavior was already correct.

### WARNING

- **W-1 (FIXED):** design, proposal and state.yaml still described ADR-7 as in scope. The cut is now recorded as an amendment in `design.md` and `proposal.md` and in `state.yaml`.
- **W-2:** `internal/app/self_update_test.go:36-42` does not prove that `u` keeps its existing handling on other screens, such as typing into the ticket input.
- **W-3:** the requested branch of `postRun` (`cmd/deploydeck/selfupdate.go:96-99`) is untested, and the stdio inheritance of `runInherited` is checked only by reading the source.
- **W-4:** `cmd/deploydeck/selfupdate_test.go:60-62` checks stdout and stderr concatenated.
- **W-5:** the commit tasks in `tasks.md` are unchecked, because commits are deferred to the user.
- **W-6:** with ADR-7 cut, binaries installed with `go install` report `dev` and never show the banner. `README.md:65` overstates the feature for those users. This is a recorded follow-up.
- **W-7:** the size is 801 against the 800 budget.

### SUGGESTION

- **S-1:** on Windows, show the command in the banner instead of quitting the TUI.
- **S-2:** reuse the print-only wording for nil argv.
- **S-3:** in the opt-out case of `TestComposeUpdate`, the httptest server is never called and could be removed.

## Design coherence

ADR-4, ADR-5 and ADR-6 are implemented as designed. ADR-7 was cut deliberately. The deviations recorded in apply-progress are acceptable: the flat `composeUpdate` signature, the `postRun` helper, and the before-message without the version.
