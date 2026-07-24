# Apply Progress: Foundation + MVP Git (HU-001..HU-006)

**Mode**: Strict TDD
**Batch scope so far**: Phases 1-5 (Bootstrap, `internal/exec`, `internal/config`, `internal/git` core,
HU-001 `prereq-check`). Phases 6-12 (HU-002..HU-006, `internal/app` wiring, final verification) are
NOT started.

## Completed Tasks (Phases 1-4)

- [x] 1.1 Create `go.mod` (module `deploydeck`) at repo root.
- [x] 1.2 Create `.gitignore` including `.deploydeck/` + standard Go ignores.
- [x] 1.3 `[U]` RED: `cmd/deploydeck/root_test.go` — `newRootCmd(deps)` builds and registers a `doctor` subcommand.
- [x] 1.4 GREEN: `cmd/deploydeck/main.go` + `newRootCmd(deps)` — Cobra root, stub `doctor` (exit 0 placeholder; real behavior Phase 5).
- [x] 1.5 Verify `go build ./...` and `go test ./...` run clean (CI smoke check).
- [x] 2.1 `[U]` RED: `FakeRunner.Run` returns canned `CommandResult` for a matching Name+Args key; unmatched request errors explicitly.
- [x] 2.2 GREEN: `internal/exec/runner.go` — `Runner`, `CommandRequest{Dir,Name,Args,Env}`, `CommandResult{Stdout,Stderr,ExitCode,Duration}`, `FakeRunner`.
- [x] 2.3 `[I]` RED: `NewOSRunner` runs a real command in `t.TempDir()`, captures stdout/exit/duration (skip `-short`).
- [x] 2.4 GREEN: implement `NewOSRunner` wrapping `os/exec.CommandContext`.
- [x] 2.5 `[I]` RED: `NewOSRunner` merges env — req.Env override present AND inherited PATH still resolves.
- [x] 2.6 GREEN: implement env layering (`cmd.Env = append(os.Environ(), req.Env...)`).
- [x] 2.7 `[I]` RED: context timeout/cancel on `OSRunner` returns an error, no hang.
- [x] 2.8 GREEN: ctx-cancel/timeout handling — already satisfied by `os/exec.CommandContext` from 2.4 (see Deviations).
- [x] 2.9 `[I]` RED: non-zero exit yields nil `Runner` error + populated `ExitCode`, distinct from start failure and timeout.
- [x] 2.10 GREEN: implement exit-code extraction from `*exec.ExitError`.
- [x] 3.1 `[U]` RED: `Load(dir)` table-driven — defaults applied for omitted fields.
- [x] 3.2 GREEN: `Config` struct (`branches`,`sandboxes`,`ticketPatterns`,`branchFormat`,`minVersions`,`runs`) + `Load(dir)`.
- [x] 3.3 `[U]` RED: `Validate()` table-driven — invalid regex, missing alias, disallowed `branchFormat` token.
- [x] 3.4 GREEN: implement `Validate()` against shared `{{ticket}}`/`{{target}}` allow-list.
- [x] 3.5 `[U]` RED: `SandboxFor(branch)` table-driven — exact, glob, no-match.
- [x] 3.6 GREEN: implement `SandboxFor()`.
- [x] 4.1 Shared `newTempRepo(t)` test helper — real temp repo + bare origin remote wired and pushed.
- [x] 4.2 `[I]` RED: `Service` resolves repo root via `rev-parse --show-toplevel`; errors outside a repo.
- [x] 4.3 GREEN: `git.Service`, `New(Runner)`.
- [x] 4.4 `[U]` RED: every git request carries `GIT_EDITOR=true`,`GIT_TERMINAL_PROMPT=0`,`GIT_PAGER=cat`.
- [x] 4.5 GREEN: implement env injection in the shared git request builder (`newRequest`).
- [x] 4.6 `[I]` RED: branch listing returns local + remote branches.
- [x] 4.7 GREEN: implement `ListBranches`.
- [x] 4.8 `[I]` RED: working-tree status distinguishes clean vs dirty.
- [x] 4.9 GREEN: implement `Status` + partial `RepoState`.

## Completed Tasks (Phase 5: HU-001 `prereq-check`)

- [x] 5.1 `[U]` RED: `sf --version`/`sf plugins --json`/`sf org list --json` JSON parsing, incl. malformed-JSON error case (canned `FakeRunner`).
- [x] 5.2 GREEN: `internal/salesforce/client.go` — `Client`, `New(Runner)`, `VersionInfo`/`Plugin`/`Org`/`OrgList`.
- [x] 5.3 `[U]` RED: lock acquire/refuse/stale-takeover table-driven with fake `ProcessProber.Alive(pid, startedAt)`.
- [x] 5.4 GREEN: `internal/prereq/lock.go` — `LockInfo{PID,PName,Host,StartedAt,CreatedAt}`, `ProcessProber`.
- [x] 5.5 `[U]`/`[I]` RED: stale-lock takeover race — two acquirers hit the same stale lock, exactly one wins.
- [x] 5.6 GREEN: implement stale takeover as remove-then-bounded-retry exclusive claim.
- [x] 5.7 `[U]` RED: `LockInfo` is never observable half-written.
- [x] 5.8 GREEN: implement atomic full-write-before-discoverable `LockInfo` persistence.
- [x] 5.9 `[U]` RED: git/sf/`sfdx-git-delta` version-vs-`minVersions` check.
- [x] 5.10 GREEN: implement version/plugin checks in `internal/prereq/checker_versions.go`.
- [x] 5.11 `[I]` RED: repo-membership + missing-origin blocks with corrective action.
- [x] 5.12 GREEN: implement repo/origin checks composing `git.Service`.
- [x] 5.13 `[I]` RED: dirty working tree blocks branch-modifying operations.
- [x] 5.14 GREEN: implement dirty-tree check.
- [x] 5.15 `[I]` RED: `.gitignore` missing `.deploydeck/` blocks + offers to add; present entry passes.
- [x] 5.16 GREEN: implement gitignore check + add-entry action.
- [x] 5.17 `[U]` RED: git hooks interference detected, informative non-blocking `PrereqCheck` (RNF-005).
- [x] 5.18 GREEN: implement git-hooks interference detection (informative-only, never blocking).
- [x] 5.19 `[U]` RED: `commit.gpgsign=true` produces a non-blocking doctor warning.
- [x] 5.20 GREEN: implement gpgsign warning check.
- [x] 5.21 `[U]` RED: configured alias absent from `sf org list --json` (all five categories) blocks that sandbox.
- [x] 5.22 GREEN: implement alias validation composing `salesforce.Client` + config.
- [x] 5.23 `[U]` RED: full report assembly — all-pass and missing-git-binary-blocks scenarios.
- [x] 5.24 GREEN: implement `Checker.Check()` aggregating `[]PrereqCheck{Status,Detail,FixCommand}`.
- [x] 5.25 `[U]`/`[I]` RED: `doctor` exits non-zero (distinct from success) on any blocker, zero when all pass.
- [x] 5.26 GREEN: wire `doctor` subcommand to `Checker` with distinct non-zero exit.

## Remaining Tasks (NOT in this batch)

- [ ] Phase 6: HU-002 `commit-discovery` (6.1-6.20)
- [ ] Phase 7: HU-003 `commit-selection` (7.1-7.16)
- [ ] Phase 8: HU-004 `target-selection` (8.1-8.16)
- [ ] Phase 9: HU-005 `promotion-branch` (9.1-9.15)
- [ ] Phase 10: HU-006 `cherry-pick` (10.1-10.38)
- [ ] Phase 11: `internal/app` wiring (11.1-11.6)
- [ ] Phase 12: Final verification (12.1-12.3)

## TDD Cycle Evidence (Phases 1-4 — carried forward from the prior batch)

| Task | Test File | Layer | Safety Net | RED | GREEN | TRIANGULATE | REFACTOR |
|------|-----------|-------|------------|-----|-------|-------------|----------|
| 1.3/1.4 | `cmd/deploydeck/root_test.go` | Unit | N/A (new) | Written | Passed | 2 cases | None needed |
| 2.1/2.2 | `internal/exec/fake_runner_test.go` | Unit | N/A (new) | Written | Passed | 2 cases | None needed |
| 2.3/2.4 | `internal/exec/os_runner_test.go` | Integration | N/A (new) | Written | Passed | Single | None needed |
| 2.5/2.6 | `internal/exec/os_runner_env_test.go` | Integration | 4/4 | Written | Passed | Single | None needed |
| 2.7/2.8 | `internal/exec/os_runner_timeout_test.go` | Integration | 5/5 | Written | Passed (behavior already provided by `CommandContext`) | 2 cases | None needed |
| 2.9/2.10 | `internal/exec/os_runner_exitcode_test.go` | Integration | 7/7 | Written | Passed | 2 cases | None needed |
| 3.1/3.2 | `internal/config/load_test.go` | Unit | N/A (new) | Written | Passed | 2 table cases | None needed |
| 3.3/3.4 | `internal/config/validate_test.go` | Unit | 3/3 | Written | Passed | 4 table cases | None needed |
| 3.5/3.6 | `internal/config/sandbox_for_test.go` | Unit | 7/7 | Written | Passed | 3 table cases | None needed |
| 4.1-4.3 | `internal/git/service_root_test.go` | Integration | N/A (new) | Written | Passed | 2 cases | None needed |
| 4.4/4.5 | `internal/git/service_env_test.go` | Unit (`FakeRunner`) | 2/2 | Written | Passed | Single | extracted `newRequest` |
| 4.6/4.7 | `internal/git/service_branches_test.go` | Integration | 3/3 | Written | Passed | 3 assertions | None needed |
| 4.8/4.9 | `internal/git/service_status_test.go` | Integration | 5/5 | Written | Passed | 2 cases | None needed |

## TDD Cycle Evidence (Phase 5 — this batch)

| Task | Test File | Layer | Safety Net | RED | GREEN | TRIANGULATE | REFACTOR |
|------|-----------|-------|------------|-----|-------|-------------|----------|
| 5.1/5.2 | `internal/salesforce/client_test.go` | Unit (`FakeRunner`) | N/A (new pkg) | ✅ Written (package didn't compile: `no non-test Go files`) | ✅ Passed (8/8) | ✅ multiple cases per call (Version x2, Plugins x3 incl. empty + malformed, Orgs x2, FindByAlias) | ➖ None needed |
| 5.3/5.4 | `internal/prereq/lock_test.go` (`TestLock_Acquire_TableDriven`, `TestLock_Acquire_SameHostAliveOwner_ErrorIsTypedErrLockHeld`) | Unit (fake `ProcessProber`, real `t.TempDir()` files) | N/A (new pkg) | ✅ Written (package didn't compile) | ✅ Passed | ✅ 5 table cases (no lock / same-host alive / dead owner / pid-reused / different-host) | ➖ None needed |
| 5.5/5.6 | `internal/prereq/lock_test.go` (`TestLock_StaleTakeoverRace_ExactlyOneWinner`) | Unit/race (2 real goroutines racing real file syscalls) | ✅ 6/6 (prior lock tests) | ✅ Written | ✅ Passed, incl. `-race` clean, repeated 30x with no flake | ➖ Single scenario (the race itself is the triangulation axis) | ➖ None needed |
| 5.7/5.8 | `internal/prereq/lock_test.go` (`TestLock_Acquire_NeverObservablePartiallyWritten`) | Unit/concurrency (goroutine reader racing `Acquire`) | ✅ 7/7 | ✅ Written | ✅ Passed (0 violations observed) | ➖ Single (the concurrent-reader invariant is the whole test) | ➖ None needed |
| 5.9/5.10 | `internal/prereq/version_compare_test.go`, `internal/prereq/checker_versions_test.go` | Unit (`FakeRunner`) | N/A (new files) | ✅ Written (`compareVersions` undefined) | ✅ Passed | ✅ 6 compareVersions table cases + 4 checker scenarios (all-OK, git-below-min, delta-below-min, delta-missing) | ➖ None needed |
| 5.11/5.12 | `internal/prereq/checker_repository_test.go` | Integration (`newTempRepo`, real git) | ✅ 12/12 | ✅ Written (`CheckRepository` undefined) | ✅ Passed | ✅ 3 cases (outside repo / missing origin / with origin) | ➖ None needed |
| 5.13/5.14 | `internal/prereq/checker_workingtree_test.go` | Integration (real git) | ✅ 15/15 | ✅ Written (`CheckWorkingTree` undefined) | ✅ Passed | ✅ 2 cases (clean / dirty) | ➖ None needed |
| 5.15/5.16 | `internal/prereq/checker_gitignore_test.go` | Integration (real git + real fs) | ✅ 17/17 | ✅ Written (`CheckGitignore` undefined) | ✅ Passed | ✅ 4 cases (missing file / present-without-entry / entry-present / add-then-recheck) | ➖ None needed |
| 5.17/5.18 | `internal/prereq/checker_hooks_test.go` | Unit (`FakeRunner` root + real `t.TempDir()` hooks dir) | ✅ 21/21 | ✅ Written (`CheckHooks`/`DetectInterferingHooks` undefined) | ✅ Passed | ✅ 4 cases (no dir / only `.sample` / one interfering hook / multiple hooks incl. non-hook + sample exclusion) | ➖ None needed |
| 5.19/5.20 | `internal/prereq/checker_gpgsign_test.go` | Unit (`FakeRunner`) | ✅ 25/25 | ✅ Written (`CheckGpgSign` undefined) | ✅ Passed | ✅ 3 cases (unset / false / true-warns) | ➖ None needed |
| 5.21/5.22 | `internal/prereq/checker_aliases_test.go` | Unit (`FakeRunner`) | ✅ 28/28 | ✅ Written (`CheckAliases` undefined) | ✅ Passed | ✅ 2 cases (missing alias blocks / alias found in non-default `devHubs` category) | ➖ None needed |
| 5.23/5.24 | `internal/prereq/checker_check_test.go` | Integration (real git via `newTempRepo` + `FakeRunner` sf) | ✅ 30/30 | ✅ Written (`Checker.Check`/`Checker.Lock` field undefined) | ✅ Passed after fixing an uncommitted-`.gitignore`-makes-tree-dirty test bug (see Issues) | ✅ 2 scenarios (all-pass / missing-git-binary-blocks) — drove `CheckVersions` to degrade binary-launch failures into blocking checks instead of hard errors | ➖ None needed |
| 5.25/5.26 | `cmd/deploydeck/root_test.go` (`TestNewRootCmd_Doctor_AllChecksPass_ExitsZero`, `TestNewRootCmd_Doctor_BlockingCheck_ExitsNonZero`) | Unit (`FakeRunner`, injected `Deps.NewChecker`) | ✅ 2/2 (pre-existing `newRootCmd` tests) | ✅ Written (`Deps.NewChecker` field undefined) — replaces the Phase-1 stub-behavior test, an intentional design change per Phase 5's task list, not a regression | ✅ Passed after seeding a real `.gitignore` file at the FakeRunner's canned repo-root path (see Issues) | ✅ 2 scenarios (all-OK exits 0 / blocking exits non-zero) | ➖ None needed |

### Test Summary (Phase 5)

- **Total new top-level test functions**: 38 (57 total in the whole module vs. 19 at the end of the Phase 1-4 batch)
- **Total tests passing**: 57/57 (`go test ./...`), including all table-driven sub-cases; `-race` clean; `-short` correctly skips 20 real-git/real-process integration tests
- **Layers used**: Unit (salesforce parsing, lock table-driven + concurrency, version-compare, versions/hooks/gpgsign/aliases checks), Integration (repository/origin, working tree, gitignore, full-report assembly — all via real `git` through `newTempRepo`), CLI (doctor exit-code wiring)
- **Approval tests** (refactoring): None — `checker_versions.go`'s CheckVersions was extended (missing-binary degrades to a blocking check instead of a hard error) under a new failing test, not refactored behind an approval test, since the old "hard error" behavior had no prior spec-mandated contract to preserve
- **Pure functions created**: `compareVersions`/`splitVersion` (dotted numeric version comparison), `hasDeploydeckIgnoreEntry`, `DetectInterferingHooks`, `insideRepo`, `versionCheck`/`deltaPluginCheck`, `OrgList.FindByAlias`, `parseVersionOutput`, `decodeEnvelope`

## Work Unit Evidence (Phase 5)

| Evidence | Value |
|---|---|
| Focused test command and exact result | `go test ./internal/salesforce/... ./internal/prereq/... ./internal/git/... ./cmd/...` → all packages `ok` |
| Runtime harness command/scenario and exact result | `go test ./internal/prereq/... -run TestChecker_Check_AllPrerequisitesPass_AllCriticalChecksOK -v` → PASS: real git repo via `newTempRepo`, every HU-001 check (versions via `FakeRunner` sf + real git, repo/origin, working tree, gitignore, hooks, gpgsign, alias, lock) reports OK end-to-end. Also `go test ./internal/prereq/... -run TestLock_StaleTakeoverRace_ExactlyOneWinner -race -count=30` → PASS all 30, no data race, exactly one winner every run. |
| Rollback boundary | Revert commits `e358b4e` (salesforce shim), `66cc1de` (prereq checks + lock + git plumbing), `d2f8abf` (doctor CLI wiring), `21294d0` (tasks.md marks). Phases 1-4 (`internal/exec`, `internal/config`, `internal/git` core) remain fully buildable/testable alone; nothing in Phase 6+ exists yet to depend on Phase 5. |

## Deviations from Design

1. **`CheckVersions` degrades tool-launch failures to blocking checks, not Go errors** (discovered while writing 5.23's RED test): the design's `[]PrereqCheck` report contract implies `Check()` should always render a full report, including naming which specific tool is missing with a `FixCommand` — but the initial 5.9/5.10 implementation propagated `c.Git.Version(ctx)`/`c.SF.Version(ctx)` errors as hard Go errors, which would abort the entire report before any other check ran. Fixed under a new failing test (`TestChecker_Check_MissingGitBinary_BlocksWithFixCommand`) so a missing/unlaunchable binary is reported the same way a below-`minVersions` binary is: a blocking `PrereqCheck` with `FixCommand`, never a report-aborting error. `CheckAliases` was intentionally left propagating a hard error on `sf org list --json` failure (not extended for resilience) since no test in this batch's scope exercises that path — noted here rather than silently generalized beyond what a failing test drove.
2. **Signal-based lock release (SIGINT/SIGTERM) not implemented**: design.md's Decision prose mentions releasing the lock "also on SIGINT/SIGTERM", but no Phase 5 task (5.1-5.26) names this as a distinct RED/GREEN item, and Go's `signal.Notify`-based delivery is not reliably unit-testable within this batch's harness. `Lock.Release()` exists and is safe to call (idempotent, PID-checked) but `doctor`'s current wiring does not yet register an OS signal handler to call it on interrupt — the lock simply times out via the owning process's actual death (which `ProcessProber.Alive` then detects) if `doctor` is killed. Flagged as a gap for Phase 11 (`internal/app` TUI wiring), which is a longer-running process where this matters more than in the short-lived `doctor` CLI invocation.
3. **`internal/salesforce.Client.Version` parses plain text, not JSON**: design.md's decision block comments `sf --version` (no `--json`), consistent with the real `sf` CLI (`sf --version` has no JSON mode) — `Plugins`/`Orgs` decode the `{"status":0,"result":...}` envelope per design; `Version` extracts the CLI version via regex from the plain-text banner. Task 5.1's wording bundles all three calls under one "JSON parsing" RED task; the malformed-input triangulation case therefore lives on `Plugins`/`Orgs`, and `Version`'s triangulation is a second differently-versioned banner instead.
4. **`OSProcessProber` (real darwin/POSIX `ps`-backed prober) implemented without a preceding RED test**: task 5.4 explicitly states "the darwin OS start-time reader (`sysctl KERN_PROC` / `ps -o lstart`) is integration/manual, not fake-covered" — a deliberate carve-out in the task list itself. `internal/prereq/process_prober.go` implements this via `ps -p <pid> -o lstart=` run through the shared `exec.Runner` boundary (keeping the "one execution seam" architecture rule rather than raw `sysctl`/`/proc` calls), with no unit test, per that explicit exception. It IS exercised implicitly any time `deploydeck doctor` runs for real (not part of this batch's automated `go test` coverage).
5. **`newTempRepo`/`runGit` duplicated in `internal/prereq`'s test helpers**: `internal/git`'s `newTempRepo` lives in a `_test.go` file scoped to `package git_test`, which Go does not allow importing from another package's tests. `internal/prereq/helpers_test.go` reimplements an equivalent helper (parameterized with an `withOrigin bool` to cover both the Phase 4 git-core shape and the origin-less variant HU-001's origin check needs) rather than extracting a shared `internal/testutil` package, to avoid widening this batch's scope.

## Issues Found

1. **Test bug, fixed within this batch**: the first draft of `TestChecker_Check_AllPrerequisitesPass_AllCriticalChecksOK` wrote a `.gitignore` file into an already-committed `newTempRepo` without committing it, which made the working tree dirty and failed the `working tree` check — not a production bug, a test-setup oversight; fixed by committing the seeded `.gitignore`.
2. **Test bug, fixed within this batch**: `TestNewRootCmd_Doctor_AllChecksPass_ExitsZero` initially used a literal non-existent path (`/repo`) as the `FakeRunner`-canned `rev-parse --show-toplevel` result; `CheckGitignore`/`CheckHooks` read the filesystem directly at that resolved root (by design — they are not git subcommands), so the canned root must be a real directory. Fixed by using a real `t.TempDir()` as the canned root with a real seeded `.gitignore`.
3. No other blocking issues. All Phase 5 acceptance criteria (`specs/prereq-check/spec.md`) are implemented and green, including the informative/non-blocking `gh` CLI check's explicit exclusion (documented in that spec file itself as deferred to HU-014, out of this change's scope).

## Workload / PR Boundary

- Mode: single PR (`size:exception` GRANTED by maintainer per tasks.md Delivery Decision, recorded 2026-07-24)
- Current work unit: Unit 2 of 4 suggested units — "HU-001..HU-005 (Phases 5-9)" — Phase 5 (`prereq-check`) is now complete; Phases 6-9 remain for this unit
- Boundary: starts from Phase 1-4's `internal/exec`/`internal/config`/`internal/git` core (all green, no HU-level behavior); ends with `internal/salesforce`, `internal/prereq` (all HU-001 checks + single-instance lock) and `deploydeck doctor` fully wired and independently tested/green
- Estimated review budget impact: 4 commits this batch (~2,700 changed lines: salesforce shim, prereq checks + lock + git plumbing, doctor CLI wiring, tasks.md marks); tracked against the session's explicit `review_budget_lines=40000` budget per the accepted `size:exception`

## Status

56/56 tasks in scope (Phases 1-5) complete (170 total tasks in `tasks.md`; 114 remain across Phases 6-12). Ready for next batch (Phase 6: HU-002 `commit-discovery`).
