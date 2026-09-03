# Verification Report — `config-validation-wiring`

**Branch**: `feat/config-validation-wiring` · **Verdict**: `partial` at verification time →
**upgraded to `pass`** after the orchestrator addendum below.

---

## Runtime evidence (executed by the verify phase)

`go test ./... -count=1 -timeout 900s` exit **0**, 15/15 packages, run **twice** (before and after
mutation testing, proving every restoration was faithful). Real `git`, `sf` and `sgd` exercised —
`internal/git` 60s, `internal/delta` 12.4s, `internal/app` 29.8s, nothing skipped, no `-short`.
`gofmt`, `go build`, `go vet` all clean.

## Spec compliance

2 requirements, 7 scenarios. **6 COVERED, 1 UNTESTED** (scenario 5, closed by the addendum).

## The three regression fixes — the highest-risk area, came back clean

All three made the fixture represent production reality. **Zero assertions weakened; one
strengthened.**

- **R1 `doctor_e2e_test.go`** — `config.Config{...}` → `baselineConfig()` + field assignment. The
  helper was diffed against `applyDefaults` and mirrors all five unconditional entries faithfully.
  No `wantDoctorBlocked` value changed.
- **R2 `root_test.go`** — one-line fixture substitution. Assertion byte-identical.
- **R3 `checker_check_test.go`** — sandbox content preserved exactly; only the *route* changed
  (struct literal → real YAML through `config.Load`). The assertion was **strengthened**:
  `"config file"` was added to the critical-names list. The global
  "fail if any check blocks" loop is untouched.

**Proof the inventory was complete**: reverting all three test files to their pre-change state fails
**exactly** the three named sites and nothing else in any package.

## Mutation verification — zero guards survived

| # | Mutation | Caught by |
|---|---|---|
| A | `CheckConfig` registered last instead of first | ordering test (`checks[0].Name`) |
| B | `StatusBlocking` → `StatusWarning` | 3 `CheckConfig` unit tests |
| C | `configFilePath()` always returns `""` | 3 `CheckConfig` unit tests |
| D | `applyDefaults` drops `PollIntervalSeconds` | flagship all-pass test + 2 more |
| E | `CheckConfig` applies defaults locally (ADR-6-forbidden) | 1 test — see WARNING-3 |
| F | R1/R2/R3 fixtures reverted | exactly the 3 named sites |

**No test in this change is decorative.**

## Mandate items confirmed

- `loadMinimalConfig` writes into a fresh `t.TempDir()`, never the repo under test — verified by
  reading, and empirically by the `working tree` check reporting OK at runtime.
- `CheckConfig` does **not** apply defaults internally.
- `Validate()` is **not** wired into `config.Load` or `resolveRoots`; `Load` still ends at
  `applyDefaults`.
- The four SAFE `Check()` call sites are untouched (`git diff --stat` empty on those files).
- W-3's test is labelled partial in four places; no implied full closure.
- S-4 is append-only: 10 added lines, **0 deleted**; the archived `- [x]` line is byte-unchanged.

## Coverage of new production code

`CheckConfig` 100% · `configFilePath` 100% · `resolveRoots` 100% · `Validate` 100% ·
`newChecker` 85.7% · `Check` 76.6% (uncovered lines are pre-existing error paths).

Code-only diff (excluding `openspec/`): **555 lines** against the 1200 budget — 46%. Production
(non-test) code is only ~103 of those 555.

## Design coherence

All six ADRs implemented as designed. **Zero design deviations.**

---

## Findings, and their resolution

### CRITICAL-1 — spec scenario "Multiple violations require multiple fix cycles" untested — **CLOSED**

The implementation was correct (`Validate()` returns one error; `Detail` copies it verbatim) but
nothing pinned it. Nothing prevented a future change from making `Detail` multi-line, which would
break the single-line render contract in **both** `cmd/deploydeck/main.go:187` and
`internal/app/view.go:158`.

### WARNING-1 — vacuous second loop in the ordering test — **CLOSED**

The `versionNames[c.Name] && i == 0` condition was unreachable: a fatal six lines above already
guaranteed `checks[0]` is the config file. It read as a second independent guarantee and was none.

### WARNING-3 — the ADR-6 negative pin had one guardian, under a misleading name — **CLOSED**

Mutation E was caught only by `TestChecker_CheckConfig_ZeroValueConfigPath_DegradesToBareFileName`,
whose declared subject is `FixCommand` degradation. A contributor tidying that test toward its stated
purpose would have silently destroyed the only guard against `CheckConfig` reporting OK for an
un-defaulted config.

### WARNING-2 — no `apply-progress` artifact — **OPEN, accepted**

A traceability gap in the SDD pipeline, not a code gap: the substance (RED/GREEN per task, file,
test) is inline in `tasks.md`, and the verify phase replaced self-reported evidence with six
mutation experiments, which is stronger.

### SUGGESTION-1 — no end-to-end doctor variant exercises the config-blocking path — **OPEN**

Demonstrated by mutation B: turning `CheckConfig` into a warning left `cmd/deploydeck` green. The
exit-code path is generic (`anyBlocking`) and covered by 9 other variants, so residual risk is low.
~15 lines would close the `Detail` + `fix:` render path end to end.

### SUGGESTION-2 — `TestResolveRoots_NestedRepo` does not assert `ConfigDir` — **OPEN**

Plan-conformant omission; one line completes the triple.

### NOTE-1 — `baselineConfig()` is a hand-mirror and provably does not track `applyDefaults`

Mutation D left `cmd/deploydeck` green. The design disclosed this and pointed at the `Load`-backed R3
pin as the real guard — and that pin fired. Verified rather than merely asserted.

### NOTE-2 — actual diff 555 lines vs the ~320 forecast (+73%), driven by tests (452 of 555)

Recorded for forecast calibration. Still 46% of budget.

---

## Orchestrator addendum — CRITICAL-1, WARNING-1 and WARNING-3 closed

Commit `ddb4b83`, test-only.

**CRITICAL-1** — `TestChecker_CheckConfig_MultipleViolations_ReportsOneAtATime` builds a config
violating two independent rules (a sandbox with no alias AND a delta block with no `sourceDirs`),
asserts `Detail` names one of them and contains no newline, then fixes that one and asserts the
**second** violation surfaces rather than silence. That second half is what makes the spec's "N
problems, N cycles" cost real rather than merely asserted.

**WARNING-1** — the vacuous loop is replaced by a genuine index comparison (`cfgIdx >=
firstVersionIdx` fails), with guards so it cannot pass vacuously if either check is missing from the
report.

**WARNING-3** — `TestChecker_CheckConfig_BareLiteral_BlocksPerADR6` now exists for the invariant and
says so in its name; the original fixture's comment is bound to it so a future tidy-up is warned.

**The three additions were mutation-verified by the orchestrator**, because a test that passes
without catching anything is worse than no test:

| Mutation | Result |
|---|---|
| `Detail: err.Error() + "\nsegunda linea"` | `MultipleViolations` FAILS — *"Detail must stay single-line"* |
| `CheckConfig` applies defaults to a local copy | `BareLiteral_BlocksPerADR6` FAILS — *"ADR-6 violated: … reported Status:OK"* |

`checker_config.go` restored byte-identical after both (`git diff` empty).

**Re-verification after the addendum**: `gofmt` clean · `go build` clean · `go vet` clean ·
`go test ./... -count=1 -timeout 900s` **15/15 green**, real e2e included.

---

## Result contract

- **status**: `pass` (was `partial`; CRITICAL-1, WARNING-1 and WARNING-3 closed by the addendum)
- **executive_summary**: All 7 spec scenarios now covered by passing tests. The three regression
  fixes were proven genuine under adversarial scrutiny — none weakened an assertion, one strengthened
  its own, and reverting all three fails exactly the three predicted sites and nothing else. Eight
  mutations across the verify phase and the addendum were all killed by their intended guard.
  Code-only diff 555 lines, 46% of budget, of which only ~103 are production code.
- **artifacts**: `openspec/changes/config-validation-wiring/verify-report.md`
- **next_recommended**: `sdd-archive` — which must also merge the delta into
  `openspec/specs/prereq-check/spec.md` (correctly still pending).
- **risks**: WARNING-2 (no `apply-progress` artifact — traceability, not code), SUGGESTION-1 (no
  end-to-end doctor variant on the config-blocking path), SUGGESTION-2 (one missing `ConfigDir`
  assertion), and the accepted ADR-6 residual: Go cannot express "this `Config` went through
  `applyDefaults`" at compile time, so a future hand-built `Checker{Config: ...}` from a non-`Load`
  source remains possible — its manifestation is loud (doctor blocks on first run), not silent.
- **skill_resolution**: paths-injected — go-testing, plus phase, shared-protocol and strict-TDD
  verify skills.
