# Proposal: Wire `Config.Validate()` Into The Prerequisite Report

**Delivery phase**: post-epic corrective work. EPICA's Plan De Entrega is complete; this touches its Fase 1 (prerequisites, HU-001) and Fase 5 (Config YAML) surfaces without being a new phase item.

## Intent

`Config.Validate()`'s 11 rules are dead code — no `cmd/deploydeck` path reaches them, so `delta.outputDir` with an empty `sourceDirs` is silently accepted and yields an always-empty delta package. Exploration executed all 11 rules against three real configs: all pass through `Load`, all fail without `applyDefaults`. The rules are safe; the wiring order is the hazard.

## Scope

### In Scope
- `CheckConfig`: new `prereq.Checker` check wrapping `Validate()`, registered **first** in `Check()`.
- `roots.ConfigDir` → `Checker.ConfigPath`, so `FixCommand` names the real file (zero value falls back to `deploydeck.yaml`).
- `doctor_e2e_test.go`: defaulted baseline for `doctorVariant` — its 10 variants build bare `Config{}` literals and would newly fail.
- Correct `Validate()`'s now-false "NOT called in production" note; document the defaults-first precondition on `Checker.Config`.
- `prereq-check` spec delta.
- Follow-ups, separate commits: W-5 comment fix; W-3 one nested-`Deps` git-root test (**partial by design**: 1 of ~13 sites); S-2 isolating `--repo-dir` e2e (real `sf`, `-short`-skipped); S-4 note appended to archived `tasks.md` 7.2; README Update section.

### Out of Scope
- Wiring into `Load`/`resolveRoots` — a bare error there preempts `renderPrereqChecks`, destroying the structured report for every check.
- W-4 `apply-progress` artifact (retroactive fabrication); full W-3 closure; `runs prune` validation (status quo).

## Capabilities

### New Capabilities
None.

### Modified Capabilities
- `prereq-check`: adds a blocking config-validity prerequisite; the execution/reporting requirement's check list gains it.

## Approach

| Decision | Choice | Rationale |
|---|---|---|
| Name / Detail | `config file` / `Validate()`'s error verbatim | matches `git version` naming; the error already names rule and key |
| `FixCommand` | `edit <resolved path>` | non-runnable fixes already exist (`https://git-scm.com/downloads`); nested layouts make a bare filename ambiguous |
| Granularity | first error only | deterministic (gates sorted); leaves a function with 5 test sites untouched; multi-line Detail breaks the one-line render contract. Accepted cost: N problems, N cycles |
| Order | before `CheckVersions` (revises exploration) | config feeds `MinVersions`/`Sandboxes`; no process spawn; tests look up by name, so reordering is free |
| Invariant | doc + pinned tests | field doc on `Checker.Config`; tests that a `Load`ed minimal YAML passes and a bare literal blocks |
| Severity | `StatusBlocking` | every rule describes downstream wrong behavior; 3/3 real configs pass, so false-positive risk is empirically zero |

## Affected Areas

| Area | Impact | Description |
|---|---|---|
| `internal/prereq/checker_config.go` | New | `CheckConfig` |
| `internal/prereq/checker.go`, `checker_check.go` | Modified | `ConfigPath` field, registration |
| `cmd/deploydeck/roots.go`, `doctor_e2e_test.go` | Modified | `ConfigDir`; defaulted baseline |
| `internal/config/validate.go` | Modified | doc comment only |
| `openspec/specs/prereq-check/` | Modified | delta |

## Risks

| Risk | Likelihood | Mitigation |
|---|---|---|
| A field config newly blocks | Low | 3/3 real configs verified passing by execution |
| Future bare-`Config{}` caller fails spuriously | Med | doc plus pinned regression test |
| Follow-ups dilute review | Low | separate, independently readable commits |

## Rollback Plan

No shared Git state, Salesforce sandbox, or persisted artifact is touched; the check is read-only and in-memory. Revert the single commit — nothing to unwind. A wrongly blocked user's interim escape hatch is fixing the reported key.

## Dependencies

None. S-2 needs real `sf` locally; `-short`-skipped elsewhere.

## Success Criteria

- [ ] `doctor` reports `config file` first, OK for all three real configs.
- [ ] `delta.outputDir` with empty `sourceDirs` blocks, naming rule and file path.
- [ ] `go build ./...` and `go test ./...` pass.
- [ ] `Validate()`'s doc no longer claims it is unused.
