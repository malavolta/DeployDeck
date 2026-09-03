# Archive Report: Wire `Config.Validate()` Into The Prerequisite Report

**Change**: `config-validation-wiring`  
**Status**: `pass` — All 7 spec scenarios covered; all 3 regression fixes proven genuine; 8 mutations across verify phase and addendum all killed by their intended guard.  
**Archived**: 2026-09-03

---

## What Shipped

### Primary Deliverable

`CheckConfig` as a **blocking prerequisite running FIRST**, wired into `prereq.Checker.Check()` before `CheckVersions`:

- **New file** `internal/prereq/checker_config.go`: `CheckConfig` method that wraps `config.Validate()`, reports blocking status with the error verbatim as `Detail`, and supplies a `FixCommand` naming the resolved config file.
- **ConfigPath plumbing**: Widened `roots struct` to include `ConfigDir`, returned by `resolveRoots`. `newChecker` composes `Checker.ConfigPath = filepath.Join(ConfigDir, config.FileName)`, so the fix instruction names the real file the upward search located (critical for nested layouts).
- **Registration**: Four lines in `checker_check.go` invoke `CheckConfig` FIRST; extended `Check()`'s doc list to include "config validity".
- **ADR-6 invariant**: Field doc on `Checker.Config` + doc on `CheckConfig` + two pinned regression tests enforce the precondition that `Config` MUST have been through `applyDefaults` (produced by `config.Load`), never a bare `config.Config{}` literal.

### Three Regression Fixes

The implementation surfaced three test breakages that proved genuine after fix-and-verify:

| # | Site | Why it broke | Fix | Proved by |
|---|---|---|---|---|
| **R1** | `cmd/deploydeck/doctor_e2e_test.go:216` — 10 doctorVariant cases | Bare `Config{}` literals failed poll-seconds rules; all 10 variants newly blocked | Added `baselineConfig()` helper mirroring `applyDefaults`; line 216 uses it | All 10 variants keep correct exit codes; no wantDoctorBlocked value changed |
| **R2** | `cmd/deploydeck/root_test.go:163` — TestNewRootCmd_Doctor_AllChecksPass_ExitsZero | Same poll-seconds rules fired; doctor exited non-zero instead of zero | Substituted `Config: baselineConfig()` | Assertion byte-identical; test passes |
| **R3** | `internal/prereq/checker_check_test.go:41` — TestChecker_Check_AllPrerequisitesPass_AllCriticalChecksOK | New `config file` check blocked; loop asserting "all checks OK" failed | Rebuilt Checker via `loadMinimalConfig(t, body)` (real YAML through `config.Load`); added `"config file"` to critical-names list | Assertion strengthened: critical-names list now includes `"config file"`. No sandbox content altered, only the route changed from struct literal to production path. Reverting all three files to pre-change state fails exactly these three sites and nothing else. |

All three changes were **mutation-verified**: reverting any one of them leaves exactly that one site failing, and no others.

---

## Why It Mattered

`Config.Validate()`'s 11 rules were dead code — no production path reached them. The hazard: `delta.outputDir` with an empty `sourceDirs` was silently accepted, yielding an always-empty delta package. This violated HU-001's acceptance criteria ("a malformed `deploydeck.yaml` blocks with a corrective action, the same pattern already used for missing `git`/`sf`/plugin/alias/lock").

Exploration executed all 11 rules against three real configs (user's live nested config, README example, ARQUITECTURA example). All three pass through `config.Load` (i.e. after `applyDefaults`); none fail the blocking rule set. False-positive risk is empirically zero.

---

## Five Secondary Items Closed

### W-5: Comment fix in `checker_aliases.go:32`

Updated comment: "config.Validate() already rejects a missing alias" is now TRUE under ADR-1 (wiring `CheckConfig` as a prerequisite). This note clarifies that `CheckConfig` reports the violation first; the `continue` in `CheckAliases` stays correct with no double-reporting. **Zero behavior change.**

### W-3: One nested-`Deps` test proving root usage

`internal/app/discover_roots_test.go` (new file), test `TestModel_DiscoverCmd_NestedDeps_RunsGitAtGitRoot`. Reuses existing `candidateFakeRunner` and `run` helpers; builds `Deps{GitRoot, ProjectDir=<GitRoot>/project, ArtifactsRoot}` with distinct values; drives `m.discoverCmd()`; asserts every recorded git call has `Dir == GitRoot`.

**Explicitly partial by design**: proves the pattern for 1 of ~13 git-backed sites. The failure it closes is one `roots_guard_test.go` cannot (that guard only proves `deps.Dir` is never read outside `normalizeRoots`, not that a call reading `ProjectDir` wouldn't pass it equally).

### S-2: Isolating `--repo-dir` e2e test

`TestSgd_RepoDirIndependentOfCwd_RealSgd` in `internal/delta/generate_e2e_test.go` (new test). Bypasses `delta.Service` (which cannot separate cwd from `--repo-dir`); calls `exec.NewOSRunner().Run` directly with `Dir=<gitRoot>/project` and explicit `--repo-dir <gitRoot>`, `--source-dir project/force-app`, absolute `--output-dir`. Asserts exit 0 and a non-empty `package.xml` with `AccountService`. Real `sf`/`sgd`, `-short`-skipped.

Proves the ADR-7 binding from `directory-resolution`: the child cwd and the `--repo-dir` flag come from one field, structurally inseparable at the Service level. S-2 is the only test that can distinguish them and prove `--repo-dir` is not silently dropped.

### S-4: Archived `project-dir-resolution/tasks.md` note

Appended 4-line note under `tasks.md:104` (item 7.2): recorded that the `--repo-dir` flag assertion lives in `service_test.go`, and that the cwd-vs-`--repo-dir` separation is covered by S-2's isolating e2e. The archived `- [x]` item's own text is byte-for-byte unchanged — append-only history, never rewrite.

### README Update section

Added `## Update` section after `## Install`, mirroring three-platform shape:  
- macOS: `brew upgrade --cask deploydeck`  
- Windows: `scoop update deploydeck`  
- Linux / any OS: `go install github.com/malavolta/DeployDeck/cmd/deploydeck@latest`

Explicitly flags that **DeployDeck is a cask, not a formula**, so dropping `--cask` is an easy habit error. Motivated by the existing non-blocking update banner, which tells users a new version exists but not how to get it.

---

## Living Spec Changed

**File**: `openspec/specs/prereq-check/spec.md`

**Changes**:
1. **Added** `### Requirement: Config Validity Check (Blocking)` at the head of the Requirements section, with 5 scenarios:
   - Malformed config blocks with first error and fix path
   - Config check runs before CheckVersions
   - Loaded (defaulted) minimal config passes
   - Bare struct literal is rejected (regression pin)
   - Multiple violations require multiple fix cycles

2. **Modified** `### Requirement: Prerequisite Check Execution And Reporting`: expanded check list to include `config validity` as first, documented that the config validity check MUST run first before all others (since version and alias checks consume `MinVersions`/`Sandboxes` from that config). Added explicit note that previously the check list omitted config validity and defined no first-check ordering constraint.

**Result**: Spec grew from 11 to 12 `### Requirement:` sections. All prior requirements preserved byte-for-byte; no removed or renamed requirements.

---

## Verification Evidence

### Test Suite Coverage

`go test ./... -count=1 -timeout 900s` exit **0**, 15/15 packages, run twice (before and after mutation testing). Real `git`, `sf`, and `sgd` exercised throughout.

**Spec compliance**: 2 requirements, 7 scenarios. **6 COVERED by unit + e2e tests**, **1 UNTESTED at proposal time** (scenario 5: "Multiple violations require multiple fix cycles") → **CLOSED by the addendum** (`TestChecker_CheckConfig_MultipleViolations_ReportsOneAtATime`).

### Regression Fixes — Highest Risk

All three made the fixture represent production reality:

- **R1** (`doctor_e2e_test.go`): Helper was diffed against `applyDefaults` and mirrors all five unconditional entries faithfully. No `wantDoctorBlocked` value changed.
- **R2** (`root_test.go`): Assertion byte-identical.
- **R3** (`checker_check_test.go`): Sandbox content preserved exactly; only the route changed (struct literal → real YAML through `config.Load`). The assertion was **strengthened**: `"config file"` was added to the critical-names list.

**Proof the inventory was complete**: reverting all three test files to pre-change state fails exactly these three sites and nothing else in any package.

### Mutation Verification — Eight Experiments

| # | Mutation | Caught by | Purpose |
|---|---|---|---|
| A | `CheckConfig` registered last instead of first | ordering test (`checks[0].Name == "config file"`) | Proves first-position requirement |
| B | `StatusBlocking` → `StatusWarning` | 3 CheckConfig unit tests | Proves blocking severity |
| C | `configFilePath()` always returns `""` | 3 CheckConfig unit tests | Proves path in FixCommand |
| D | `applyDefaults` drops `PollIntervalSeconds` | flagship all-pass test + 2 more | Proves defaults are load-bearing |
| E | `CheckConfig` applies defaults locally (ADR-6-forbidden) | `TestChecker_CheckConfig_BareLiteral_BlocksPerADR6` (addendum) | Proves defaults NOT applied inside CheckConfig |
| F | R1/R2/R3 fixtures reverted | exactly the 3 named sites | Proves regression fix necessity |
| G | `Detail: err.Error() + "\nseconda linea"` (multi-line) | `TestChecker_CheckConfig_MultipleViolations_ReportsOneAtATime` (addendum) | Proves single-line render contract |
| H | (implicit: scenario 5 never run) | `TestChecker_CheckConfig_MultipleViolations_ReportsOneAtATime` (addendum) | Proves N-cycle cost is real |

**No test in this change is decorative.** All eight mutations were caught by their intended guard.

### Coverage of New Production Code

- `CheckConfig`: 100%
- `configFilePath`: 100%
- `resolveRoots`: 100%
- `Validate`: 100%
- `newChecker`: 85.7%
- `Check`: 76.6%

(Uncovered lines are pre-existing error paths.)

### Code-Only Diff

**555 lines** against 1200 budget (46% utilization). Production (non-test) code is only ~103 of those 555.

---

## Open Items Carried Forward

### WARNING-2 — No `apply-progress` artifact

A traceability gap in the SDD pipeline, not a code gap. The substance (RED/GREEN per task, file, test) is inline in `tasks.md`, and the verify phase replaced self-reported evidence with six mutation experiments (A–F), which is stronger. Accepted as a known limitation of the current archiving process.

### SUGGESTION-1 — No end-to-end doctor variant on the config-blocking path

Mutation B proved that turning `CheckConfig` into a warning left `cmd/deploydeck` green. The exit-code path is generic (`anyBlocking`) and covered by 9 other variants; residual risk is low. ~15 lines would close the `Detail` + `fix:` render path end to end. Deferred for future work.

### SUGGESTION-2 — `TestResolveRoots_NestedRepo` does not assert `ConfigDir`

Plan-conformant omission; one line completes the triple. Deferred for future work.

### ADR-6 Residual — Compile-time enforcement of defaults precondition

Go cannot express "this `Config` went through `applyDefaults`" without a new type. The design accepted this risk explicitly:

- **Prevents**: Silent removal or regression of `applyDefaults`' poll entries (the `Load`-backed pin in R3 goes red).
- **Does NOT prevent**: A future caller writing `prereq.Checker{Config: config.Config{...}}` by hand from a non-`Load` source (none exists today; `resolveRoots` is the single choke point).

Manifestation is loud and immediate — `doctor` blocks on the first run with a `Detail` naming `pollIntervalSeconds` — not a silent wrong result. Accepted as residual risk with low likelihood and mitigated by doc + pinned tests.

---

## Closed Before Archive

**CRITICAL-1** — Spec scenario "Multiple violations require multiple fix cycles" was untested at proposal time; addendum added `TestChecker_CheckConfig_MultipleViolations_ReportsOneAtATime`, which builds a config violating two independent rules, fixes one, and asserts the second violation surfaces. Mutation-verified.

**WARNING-1** — The initial ordering test had a vacuous six-line condition guarantee that was readable as a second independent check. Addendum replaced it with a genuine `cfgIdx >= firstVersionIdx` comparison with proper guards.

**WARNING-3** — The ADR-6 negative pin had one guardian (`TestChecker_CheckConfig_ZeroValueConfigPath_DegradesToBareFileName`), whose declared subject was `FixCommand` degradation. Addendum added `TestChecker_CheckConfig_BareLiteral_BlocksPerADR6` with a name that declares the invariant it guards. Mutation E (`CheckConfig` applies defaults locally) is now caught by a properly named test.

---

## SDD Cycle Complete

The change has been fully planned (proposal), specified (delta), designed (design), implemented with three regression fixes (apply phase, commit `ddb4b83`), verified with all 7 spec scenarios covered and 8 mutations killed (verify phase + addendum), and archived with spec merge and secondary items closed.

**Readiness for the next change**: `sdd` phase pipeline is ready.

---

**Archived to**: `openspec/changes/archive/2026-09-03-config-validation-wiring/`
