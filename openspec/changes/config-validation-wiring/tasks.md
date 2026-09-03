# Tasks: Wire `Config.Validate()` Into The Prerequisite Report

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | ~350-420 (design forecast: ~320) |
| Applied budget (session override) | 1200 lines |
| 400-line budget risk | Low |
| Chained PRs recommended | No |
| Suggested split | Single PR, 6 commits (1 primary + 5 secondary) |
| Delivery strategy | single-pr-default |
| Chain strategy | pending (not needed — well under 1200) |

Decision needed before apply: No
Chained PRs recommended: No
Chain strategy: pending
400-line budget risk: Low

### Suggested Work Units

| Unit | Goal | Likely PR | Focused test command | Runtime harness | Rollback boundary |
|------|------|-----------|----------------------|-----------------|-------------------|
| 1 | CheckConfig + registration + ConfigPath plumbing + R1/R2/R3 + ADR-6 pin (Phases 1-6) | PR 1, commit 1 | `go test ./internal/prereq/... ./internal/config/... ./cmd/deploydeck/...` | `doctor_e2e_test.go` harness / `deploydeck doctor` against a real config | Revert commit 1 — read-only, in-memory, no shared state |
| 2 | W-5 comment fix (Phase 7) | PR 1, commit 2 | `go build ./internal/prereq/...` | N/A — comment only | Revert commit 2 |
| 3 | W-3 partial nested-Deps test (Phase 8) | PR 1, commit 3 | `go test ./internal/app/... -run TestModel_DiscoverCmd_NestedDeps` | N/A — FakeRunner, no real git | Revert/delete `discover_roots_test.go` |
| 4 | S-2 isolating `--repo-dir` e2e (Phase 9) | PR 1, commit 4 | `go test ./internal/delta/... -run TestSgd_RepoDirIndependentOfCwd_RealSgd` | Real `sf`/`sgd` via `seedNestedSgdRepo`, `-short`-skipped | Revert commit 4 — additive test only |
| 5 | S-4 archived tasks.md note (Phase 10) | PR 1, commit 5 | N/A — doc append | N/A | Revert commit 5 |
| 6 | README Update section (Phase 11) | PR 1, commit 6 | N/A — doc only | N/A | Revert commit 6 |

## Phase 1: Config path plumbing

- [x] 1.1 RED (`roots_test.go`): `resolveRoots` returns `ConfigDir` for flat + nested-with-`projectDir` layouts; `newChecker` sets `ConfigPath == ConfigDir/deploydeck.yaml`.
- [x] 1.2 GREEN: `roots.go` widens `roots{...ConfigDir}`, returns it, `newChecker` sets `ConfigPath`; `checker.go` adds `ConfigPath` field + ADR-6 precondition doc.

## Phase 2: `CheckConfig` behavior

- [x] 2.1 Add `loadMinimalConfig(t, body)` to `helpers_test.go` — writes `deploydeck.yaml` into a fresh `t.TempDir()` (never the repo under test), returns loaded `Config` + path.
- [x] 2.2 RED (`checker_config_test.go`, new): blocking (Detail verbatim, `FixCommand == "edit "+ConfigPath`); OK via `loadMinimalConfig`; zero-value `ConfigPath` degrades to `"edit deploydeck.yaml"`; `Delta{OutputDir set, SourceDirs nil}` blocks naming `sourceDirs`.
- [x] 2.3 GREEN: create `checker_config.go` — `nameConfigFile`, `CheckConfig`, `configFilePath`.

## Phase 3: Registration and ordering

- [x] 3.1 RED (`checker_check_test.go`): `Check()` returns `"config file"` at index 0, before any `CheckVersions` entry.
- [x] 3.2 GREEN: `checker_check.go` registers `CheckConfig` FIRST (4 lines); extend `Check`'s doc list with "config validity".

## Phase 4: Regression fixes (R1, R2, R3)

- [x] 4.1 RED confirm: after 3.2, exactly 3 sites break — `doctor_e2e_test.go` (10 variants), `root_test.go:163`, `checker_check_test.go:46`. No others. Confirmed by running the full (non-short) suite: exactly `TestHU001_Doctor_E2E_ConsolidatedVariants` (only its `all_prerequisites_pass` subtest — the other 9 already expected `wantDoctorBlocked=true`), `TestNewRootCmd_Doctor_AllChecksPass_ExitsZero`, and `TestChecker_Check_AllPrerequisitesPass_AllCriticalChecksOK` fail. No other test in either package failed.
- [x] 4.2 GREEN (R1): `doctor_e2e_test.go` — add `baselineConfig()` (mirrors `baselineMinVersions`/`baselineSandboxes`); line 216 uses it.
- [x] 4.3 GREEN (R2): `root_test.go:163` — `Config: config.Config{}` → `Config: baselineConfig()`.
- [x] 4.4 GREEN (R3): `checker_check_test.go` — rebuild `Checker` via `loadMinimalConfig`; add `"config file"` to the critical-names list.
- [x] 4.5 Verify: `go test ./cmd/deploydeck/... ./internal/prereq/...` green; SAFE sites (`checker_check_test.go:85`, `checker_ai_test.go:116`, `checker_gh_test.go:103`, `real_org_e2e_test.go:117`) untouched — confirmed via `git diff --stat` on those four files: only `checker_check_test.go` changed, and only at the R3 site and the new phase-3 ordering test.

## Phase 5: ADR-6 defaults-order pin

- [x] 5.1 PIN (`validate_test.go`): bare `config.Config{}` errors naming `pollIntervalSeconds` (already-true behavior, guarded against regression).
- [x] 5.2 Doc-only: `validate.go` — remove the false "not called in production" note; state the `applyDefaults`-first precondition, name `CheckConfig` as production caller.

## Phase 6: Primary verification

- [x] 6.1 `go test ./...` — full suite green.
- [x] 6.2 `go build ./...`; runtime check via `doctor_e2e_test.go` harness confirms `config file` reports OK and first.

## Phase 7: Secondary — W-5 comment (separate commit)

- [x] 7.1 `checker_aliases.go:32` — update comment (now true under ADR-1); zero behavior change, no test.

## Phase 8: Secondary — W-3 partial coverage (separate commit)

- [x] 8.1 RED: new `internal/app/discover_roots_test.go` — `TestModel_DiscoverCmd_NestedDeps_RunsGitAtGitRoot`; reuse `candidateFakeRunner`/`run`; assert every `fr.Calls[i].Dir == gitRoot`. Label explicitly partial (1 of ~13 git-backed sites).
- [x] 8.2 Verify: passes against existing `discoverCmd` (no prod change expected) — confirmed green on first run, as design predicted; `TestNoDirReadsOutsideNormalizeRoots` and `TestApp_NeverImportsExecSeam` stay green too.

## Phase 9: Secondary — S-2 isolating e2e (separate commit)

- [ ] 9.1 RED: append `TestSgd_RepoDirIndependentOfCwd_RealSgd` to `generate_e2e_test.go` — bypass `delta.Service`, call `exec.NewOSRunner().Run` with `Dir=<gitRoot>/project` and explicit `--repo-dir <gitRoot>`; assert exit 0, `package.xml` has `<types>` and `AccountService`.
- [ ] 9.2 Verify: real `sf`/`sgd` locally; confirm `-short`-skip elsewhere.

## Phase 10: Secondary — S-4 archived note (separate commit)

- [ ] 10.1 Append a `> Note (config-validation-wiring):` block under `openspec/changes/archive/project-dir-resolution/tasks.md:104`. Do NOT edit the existing `- [x]` item text.

## Phase 11: Secondary — README Update section (separate commit)

- [ ] 11.1 `README.md` — add `## Update` after `## Install`: `brew upgrade --cask deploydeck` / `scoop update deploydeck` / `go install github.com/malavolta/DeployDeck/cmd/deploydeck@latest`; flag that DeployDeck is a cask, not a formula.

---

Note: `openspec/specs/prereq-check/spec.md` merge is `sdd-archive` scope, not this phase.
