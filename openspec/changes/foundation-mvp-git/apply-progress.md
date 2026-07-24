# Apply Progress: Foundation + MVP Git (HU-001..HU-006)

**Mode**: Strict TDD
**Batch scope so far**: Phases 1-6 (Bootstrap, `internal/exec`, `internal/config`, `internal/git` core,
HU-001 `prereq-check`, HU-002 `commit-discovery`). Phases 7-12 (HU-003..HU-006, `internal/app` wiring,
final verification) are NOT started.

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

## Completed Tasks (Phase 6: HU-002 `commit-discovery`)

- [x] 6.1 `[U]` RED: ticket-grep parsing from canned `git log --grep` output; commit with two tickets matches either.
- [x] 6.2 GREEN: `Commit`/`parseCommitLog`/`Service.SearchCommits` in `internal/git`.
- [x] 6.3 `[I]` RED: seeded commits — ticket found in messages lists related commits (real git).
- [x] 6.4 GREEN: wire real `git log --grep` into `Service.SearchCommits`.
- [x] 6.5 `[I]` RED: candidate local+remote branches whose name contains the ticket are listed.
- [x] 6.6 GREEN: `Service.CandidateBranches` via `git branch --list '*<ticket>*'` (refactored `branchNames` to accept a pattern, reused by `ListBranches`).
- [x] 6.7 `[U]` RED: single-source-branch enforcement blocks continuing when >1 candidate branch exists.
- [x] 6.8 GREEN: `SelectSingleSource(candidates, selected)` (pure).
- [x] 6.9 `[U]` RED (RF-002): env-to-env promotion suggests the previous/validated environment as default source; override still enforces single-source.
- [x] 6.10 GREEN: `SuggestDefaultSource(candidates, cfg, target)` composing `config.Config` (pure) via a fixed `integration→uat→production` pipeline order.
- [x] 6.11 `[I]` RED: seeded commits with inverted author-date vs topo order — final order is topological.
- [x] 6.12 GREEN: `Service.CommitsInRange` via `git rev-list --reverse --topo-order --no-commit-header origin/<target>..origin/<source>`.
- [x] 6.13 `[U]` RED: merge-commit detection (parent count>1) flags + blocks.
- [x] 6.14 GREEN: `Commit.IsMerge()` (already correct from the shared `%P` parsing in 6.2; this task added dedicated table-driven + real multi-parent-line coverage).
- [x] 6.15 `[U]` RED: equivalence classification — canned `git cherry` `-`, `merge-base --is-ancestor`, `patch-id --stable` match → already-applied/equivalent.
- [x] 6.16 GREEN: `ClassifyEquivalence`/`ParseCherryOutput`/`ParsePatchID` (pure) + `Service.IsAncestor`/`Service.Cherry`/`Service.PatchID` (real git wiring, proved via `newTempRepo`).
- [x] 6.17 `[I]` RED: deleted source branch narrows search to grep-only with warning.
- [x] 6.18 GREEN: `DeletedSourceBranchWarning(matched, candidates)` (pure, derived from existing search results — no extra git call).
- [x] 6.19 `[U]` RED: squash-merge-history warning trigger + no-results alternatives.
- [x] 6.20 GREEN: `SquashMergeWarning`, `NoResultsAlternatives`, `DiscoveredCommit`, and `Service.Discover` — the composed HU-002 discovery entry point — assembled in `internal/git/discovery.go`.

### Preliminary work unit: `internal/exec` Stdin support (needed for `git patch-id --stable`)

Not a Phase 6 task by number, but a required, minimal, backward-compatible extension of the Phase 2
seam: `git show <sha> | git patch-id --stable` is a shell pipeline, and `internal/exec` never shells
out. Added `CommandRequest.Stdin []byte` (nil-safe, zero-value-compatible) wired through `OSRunner` via
`cmd.Stdin`, under its own RED (compile failure)/GREEN (real `cat`-echo integration test) cycle,
confirmed against the full Phase 1-5 safety net before and after.

## Remaining Tasks (NOT in this batch)

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

## TDD Cycle Evidence (Phase 6 — this batch)

| Task | Test File | Layer | Safety Net | RED | GREEN | TRIANGULATE | REFACTOR |
|------|-----------|-------|------------|-----|-------|-------------|----------|
| exec Stdin (preliminary) | `internal/exec/os_runner_stdin_test.go` | Integration | ✅ 8/8 (all prior `internal/exec` tests) | ✅ Written (`unknown field Stdin`) | ✅ Passed | ✅ 2 cases (stdin fed / no-stdin no-hang) | ➖ None needed |
| 6.1/6.2 | `internal/git/service_search_test.go` | Unit (`FakeRunner`) | ✅ 13/13 (`internal/git` pre-Phase-6) | ✅ Written (`svc.SearchCommits undefined`) | ✅ Passed | ✅ 3 cases (single match, dual-ticket-matches-either, zero-match) | ➖ None needed |
| 6.3/6.4 | `internal/git/service_search_integration_test.go` | Integration (`newTempRepo`) | ✅ 16/16 | ✅ Written (new scenario against already-passing production code) | ✅ Passed | ✅ 3 assertions (2 tickets found, dual-ticket via 2nd ticket, unmatched ticket empty) | ➖ None needed |
| 6.5/6.6 | `internal/git/service_branches_test.go` | Integration (`newTempRepo`) | ✅ 1/1 (`ListBranches`, safety net before refactoring `branchNames`) | ✅ Written (`svc.CandidateBranches undefined`) | ✅ Passed | ✅ local+remote+excluded-unrelated in one scenario | ➖ None needed |
| 6.7/6.8 | `internal/git/source_selection_test.go` | Unit (pure) | N/A (new file) | ✅ Written (`undefined: git.SelectSingleSource` + 3 error vars) | ✅ Passed | ✅ 5 table cases | ➖ None needed |
| 6.9/6.10 | `internal/git/source_suggestion_test.go` | Unit (pure) | N/A (new file) | ✅ Written (`undefined: git.SuggestDefaultSource`) | ✅ Passed | ✅ 4 table cases + 1 dedicated override/enforcement scenario | ➖ None needed |
| 6.11/6.12 | `internal/git/service_range_test.go` | Integration (`newTempRepo`, explicit `--date`) | ✅ 18/18 | ✅ Written (`svc.CommitsInRange undefined`) | ✅ Passed | ➖ Single scenario (the inversion itself is the triangulation axis, plus a setup-sanity assertion proving the inversion is real) | ➖ None needed |
| 6.13/6.14 | `internal/git/commit_merge_test.go` | Unit (pure `IsMerge` + `FakeRunner` real multi-parent line) | ✅ 21/21 | ✅ Written (behavior already satisfied by 6.1/6.2's shared `%P` parsing — see Deviations) | ✅ Passed immediately | ✅ 4 table cases (0/1/2/3 parents) + 1 real merge-line parse | ➖ None needed |
| 6.15/6.16 | `internal/git/equivalence_test.go`, `internal/git/service_equivalence_test.go` | Unit (pure) + Integration (`newTempRepo`) | ✅ 22/22 | ✅ Written (`undefined: git.EquivalenceStatus` etc.; `svc.IsAncestor undefined` etc.) | ✅ Passed | ✅ 6 `ClassifyEquivalence` cases + 2 `ParsePatchID` cases + 1 real ancestor/cherry/patch-id integration scenario | ➖ None needed |
| 6.17/6.18 | `internal/git/discovery_deleted_branch_test.go` | Integration (`newTempRepo`) + Unit (pure) | ✅ 27/27 | ✅ Written (`undefined: git.DeletedSourceBranchWarning`) | ✅ Passed | ✅ 3 table cases + 1 real deleted-branch scenario | ➖ None needed |
| 6.19/6.20 | `internal/git/discovery_diagnostics_test.go`, `internal/git/discovery_e2e_test.go` | Unit (pure) + Integration (`newTempRepo`, full E2E) | ✅ 29/29 | ✅ Written (`undefined: git.DiscoveredCommit`/`SquashMergeWarning`/`NoResultsAlternatives`) | ✅ Passed | ✅ 4 + 3 table cases, plus the full consolidated HU-002 E2E scenario and its 2 named variants | ➖ None needed |

### Test Summary (Phase 6)

- **Total new top-level test functions this batch**: 20 (across `internal/exec` + `internal/git`); 91 total top-level test functions passing in the whole module (`go test ./... -v`), up from 57+ at the end of the Phase 5 batch, including all table-driven sub-cases.
- **`go test -race ./...`**: all packages `ok`, no data races.
- **`go test -short ./...`**: all packages `ok`; real-git integration tests correctly skip (`internal/git` drops from ~7s to ~0.8s under `-short`).
- **Layers used**: Unit (log/porcelain-style parsing, single-source enforcement, RF-002 suggestion, merge detection, equivalence classification, diagnostics), Integration (ticket search, candidate branches, topo order with a real author-date inversion, real `merge-base`/`cherry`/`patch-id`, deleted-branch, the full HU-002 E2E scenario) — all via the shared `newTempRepo` harness plus `writeAndCommit`/`commitWithAuthorDate` helpers added in this batch.
- **Approval tests** (refactoring): None — `branchNames` was extended (added a `pattern` parameter) under `ListBranches`' existing passing test as a safety net, then `CandidateBranches`' new failing test drove the extension; behavior for `ListBranches`' existing callers is unchanged (empty pattern = list everything, exactly as before).
- **Pure functions/types created**: `Commit.IsMerge`, `parseCommitLog`, `SelectSingleSource`, `SuggestDefaultSource`, `ClassifyEquivalence`, `ParseCherryOutput`, `ParsePatchID`, `DeletedSourceBranchWarning`, `SquashMergeWarning`, `NoResultsAlternatives`, `DiscoveredCommit.AlreadyApplied`/`SelectableByDefault`.

## Work Unit Evidence (Phase 6)

| Evidence | Value |
|---|---|
| Focused test command and exact result | `go test ./internal/git/... ./internal/exec/...` → both `ok`. `go test ./internal/git/... -run TestHU002_Discover_E2E -v` → PASS (primary scenario + 2 variants). |
| Runtime harness command/scenario and exact result | `go test ./internal/git/... -run TestHU002_Discover_E2E$ -v` → PASS: real temp git repo (`newTempRepo`) seeded with an author-date-inverted commit pair, a commit merged into target by identical SHA, a content-duplicated commit (different SHA), a merge commit, a dual-ticket commit, and a second candidate branch; asserts `Service.Discover` returns the delta in topological order, flags the merge commit as not-selectable, classifies the content-equivalent commit via `git cherry`, and that `IsAncestor`+`ClassifyEquivalence` correctly detect the identical-SHA case (proven directly, since it is structurally excluded from the topo-ordered delta by git's own range semantics — documented in the test and in Deviations below). `-short` correctly skips all of it (shells out to real git). |
| Rollback boundary | Revert commits `adb8061` (exec Stdin), `1bf4972` (search+candidate branches), `745ebd2` (single-source enforcement + RF-002), `e11f816` (topo order), `58ff3e8` (merge-detection coverage), `14c3700` (equivalence detection), `1eb132f` (diagnostics + discovery entry point), `7168731` (tasks.md marks). Phases 1-5 (`internal/exec` sans Stdin, `internal/config`, `internal/git` core, `internal/prereq`, `internal/salesforce`, `doctor` CLI) remain fully buildable/testable alone; nothing in Phase 7+ exists yet to depend on Phase 6. |

## Deviations from Design (Phase 6)

1. **`git log --grep` does not exit 1 on zero matches, contrary to design.md's blanket exit-code-as-data list**: design.md's Decision block lists `log --grep (1 = no match)` alongside `merge-base --is-ancestor`/`cherry-pick`/`diff --quiet`. Verified empirically against git 2.50.1: `git log --all --grep <no-match> --format=...` exits **0** with empty stdout, regardless of match count — only `merge-base --is-ancestor` (of this batch's commands) genuinely uses a non-zero exit as data. `SearchCommits`/`CommitsInRange` therefore treat any non-zero exit as a real error (consistent with `ListBranches`/`Status`'s existing pattern), and an empty result set is read from empty stdout, not from exit code 1.
2. **`internal/exec.CommandRequest` gained a `Stdin []byte` field (Phase 2 seam extension, not a Phase 6 task)**: `git show <sha> | git patch-id --stable` (HU-002's own documented command) is a shell pipeline; `internal/exec` never shells out. Rather than reimplementing the patch-id algorithm in Go, the first command's `Stdout` is passed as the second `CommandRequest.Stdin`. Nil-safe/zero-value-compatible — every pre-existing request is unaffected. Proved under its own RED/GREEN cycle with the full Phase 1-5 suite green before and after.
3. **`SuggestDefaultSource`'s environment pipeline order (`integration→uat→production`) is a fixed constant, not derived from config**: neither `config.Config` nor the docs define an explicit ordering for the `branches` map (a Go map has no order). `docs/ARQUITECTURA.md`'s example config and `docs/HISTORIAS.md` HU-016's "INT -> UAT -> Release -> main" description are the only source for this sequence; encoded as `environmentPipelineOrder`, documented in code, and scoped to exactly the three keys the example config uses. `Release/*` and custom branches are explicitly out of this pipeline (HU-004's sandbox glob owns them) and are never suggested as a default source. Flagged for confirmation if a future HU redefines the pipeline (e.g. adds a `Release` stage between `uat` and `production`).
4. **`Service.Discover`'s `OrderedCommits` cannot ever contain an `AlreadyAppliedBySHA` classification, by construction**: `git rev-list origin/<target>..origin/<source>` mechanically EXCLUDES any commit that is already an ancestor of target — that is the literal definition of the range operator. A commit merged into target by identical SHA is therefore correctly absent from the topo-ordered delta; there is nothing contradictory about this once documented (a commit that's already fully in target isn't "new" and has nothing to disambiguate for selection). Same-SHA already-applied detection is proven directly via `IsAncestor`/`ClassifyEquivalence`, and separately via `SearchCommits`'s `--all`-scoped message search (which is NOT range-limited and does surface such commits) — both documented and asserted in `TestHU002_Discover_E2E`. `EquivalentByCherry` (content equivalence, different SHA) is unaffected by this range exclusion and IS proven inside `OrderedCommits`, since a cherry-picked-elsewhere commit gets a new SHA and is never excluded by ancestry.
5. **`SquashMergeWarning` is an explicit, disclosed best-effort heuristic, not a real detector**: `docs/HISTORIAS.md` itself states squash-merge content equivalence "no es detectable estaticamente" and defers to HU-006's empty-pick `--skip` as the real safety net (DEC-001). No algorithm is specified anywhere. Implemented as: a ticket's classified commits showing a MIX of already-applied and still-pending (excluding merge commits from the check) is atypical for this tool's normal all-pending-or-all-promoted-together flow, and is used as the courtesy trigger. Documented in-code as best-effort, not authoritative.
6. **6.13/6.14 (merge-commit detection) reached GREEN immediately, no separate production change**: `Commit.IsMerge()` and `parseCommitLog`'s `%P` parsing (built for 6.1/6.2) already handled multi-parent lines correctly as a side effect of unifying commit parsing on one shared log format, rather than using a separate `git log --parents` call as HISTORIAS.md's technical info suggests. This task added dedicated, explicitly-named table-driven (0/1/2/3 parents) and real-log-line coverage to lock the behavior in per its own acceptance criterion, rather than leaving it only incidentally covered by 6.1/6.2's tests.

## Issues Found (Phase 6)

1. **Shell backtick interpolation broke one commit message**: the `feat(git): order discovered commits topologically` commit's `-m` message contained inline backtick-quoted git commands inside a double-quoted heredoc, which the shell interpreted as command substitution, producing an empty/garbled message body. Fixed via `git commit --amend` (no other commits were stacked on top yet, so no history was rewritten out from under later work) using a single-quoted heredoc delimiter (`<<'COMMITMSG'`) for every commit message from that point on, which prevents all shell expansion.
2. No other blocking issues. All Phase 6 acceptance criteria (`specs/commit-discovery/spec.md` under `openspec/changes/foundation-mvp-git/specs/commit-discovery/spec.md`, and `docs/HISTORIAS.md` HU-002) are implemented and green, including the RF-002 suggested-default-source requirement and the consolidated Test E2E scenario.

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

## HU-001 Remediation Batch (adversarial review + sdd-verify)

Scope: fix confirmed bugs and close named test gaps in HU-001 `prereq-check`, under strict TDD
(RED reproducing the real defect first, then minimal GREEN). The already-sound machinery — the
`os.Link`-based exclusive acquire and the content-gated stale-takeover — was NOT changed.

### Remediation TDD Cycle Evidence

| Item | Test File | RED (reproduces defect) | GREEN (minimal fix) | Commit |
|------|-----------|-------------------------|---------------------|--------|
| H1 — prober fails OPEN on localized/unparseable `ps` | `internal/prereq/process_prober_test.go` | ✅ localized & garbage lines returned `Alive=false` (=> takeover); no `LC_ALL=C` on the call | ✅ pin `LC_ALL=C`/`LANG=C`; non-empty-unparseable → `Alive=true` (fail-safe); empty/non-zero-exit → `false`; route via `exec.Runner` seam | `16c94f4` |
| H2 — stored `StartedAt` was Acquire wall-clock, not OS start-time | `internal/prereq/lock_test.go` (`…StampsStartedAtFromProber`, `…LiveOwnerWithSlowStartupToLock_NotTakenOver`) | ✅ persisted `StartedAt` == `time.Now()` not the prober start-time; a live owner with >2s startup-to-lock latency was taken over | ✅ `StartTimeReader` seam + `Lock.currentStartTime()` stamp `StartedAt` from the same `ps -o lstart=` source (`os.Getpid()`) | `7fec6c4` |
| H3 — corrupt/zero-length lock → misleading generic error, no FixCommand | `internal/prereq/lock_test.go` (`…CorruptLock_ReturnsErrLockCorrupt`), `internal/prereq/checker_lock_test.go` | ✅ garbage/zero-length/truncated lock returned `*errors.errorString` "contended stale takeover"; `CheckLock` propagated a hard error | ✅ typed `*ErrLockCorrupt` (file preserved, NOT auto-deleted — safer); `CheckLock` maps it to a `StatusBlocking` check with a `rm <path>` FixCommand | `fc99a82` |
| GAP A — HU-001 consolidated `### Test E2E` (HISTORIAS.md:86-92) | `cmd/deploydeck/doctor_e2e_test.go` | ✅ scenario did not exist | ✅ 10 variants (origin, gitignore, delta present/absent/below-min, git/sf below-min, dirty tree, alias missing, lock taken) asserted through `Checker.Check()` AND `deploydeck doctor` exit code | `9584e59` |
| GAP B — `CheckLock` blocking branch direct unit test | `internal/prereq/checker_lock_test.go` | ✅ no direct `checker_lock` unit test existed | ✅ nil-skip, live-owner blocks naming pid/pname, corrupt blocks with remove FixCommand, free lock acquires OK | `fc99a82` |
| Missing `OSProcessProber` test | `internal/prereq/process_prober_test.go` | ✅ prober had no unit test (prior Deviation 4) | ✅ covered by the H1 Runner-seam table (localized/garbage/empty/non-zero/matching/mismatch) + locale-pin assertion | `16c94f4` |

Design choice justified in code: a corrupt/zero-length lock is **surfaced** as an actionable blocking
check rather than auto-cleaned. Rationale (in `lock.go` / `checker_lock.go` comments): normal Acquire
publishes a COMPLETE record atomically via `os.Link`, so a corrupt lock was never produced by a live
instance's normal path — but deleting a file we cannot interpret is the less-safe option, so a human
decides.

### Remediation Work Unit Evidence

| Evidence | Value |
|---|---|
| Focused test command and exact result | `go test -race ./internal/prereq/... ./cmd/...` → both `ok` (H1/H2/H3 + GAP A/B green, `-race` clean) |
| Runtime harness command/scenario and exact result | `go test ./cmd/deploydeck/... -run TestHU001_Doctor_E2E -v` → PASS all 10 variants (real temp git repo + fake `sf`); each asserts `Checker.Check()` Status AND `deploydeck doctor` exit code (0 vs non-zero). `-short` correctly SKIPS it (shells out to real git). |
| Full verification | `go build ./... && go vet ./... && go test -race ./...` → all packages `ok`; `go test -short ./...` → all `ok` (integration skipped) |
| Rollback boundary | Revert commits `16c94f4` (H1 prober fail-safe/locale), `7fec6c4` (H2 start-time in lock), `fc99a82` (H3 corrupt lock + GAP B), `9584e59` (GAP A E2E). The `os.Link` acquire and content-gated stale-takeover are untouched. No Phase 6+ scope affected. |

## Workload / PR Boundary

- Mode: single PR (`size:exception` GRANTED by maintainer per tasks.md Delivery Decision, recorded 2026-07-24)
- Current work unit: Unit 2 of 4 suggested units — "HU-001..HU-005 (Phases 5-9)" — Phases 5-6 (`prereq-check`, `commit-discovery`) are now complete; Phases 7-9 remain for this unit
- Boundary: starts from Phase 1-4's `internal/exec`/`internal/config`/`internal/git` core (all green, no HU-level behavior); ends with `internal/salesforce`, `internal/prereq`, `deploydeck doctor` (Phase 5) and `internal/git`'s discovery surface — `SearchCommits`, `CandidateBranches`, `SelectSingleSource`, `SuggestDefaultSource`, `CommitsInRange`, `ClassifyEquivalence`/`IsAncestor`/`Cherry`/`PatchID`, `Discover` (Phase 6) — fully wired and independently tested/green
- Estimated review budget impact: 8 commits this batch (~2,140 changed lines: `internal/exec` Stdin extension, ticket/branch search, single-source enforcement + RF-002, topo order, merge-detection coverage, content-equivalence, diagnostics + discovery entry point, tasks.md marks); tracked against the session's explicit `review_budget_lines=40000` budget per the accepted `size:exception`

## Status

76/76 tasks in scope (Phases 1-6) complete (170 total tasks in `tasks.md`; 94 remain across Phases 7-12).
Phase 6 (HU-002 `commit-discovery`) complete under strict TDD: ticket message/branch-name search,
single-source-branch enforcement, RF-002 suggested default source, topological ordering (proven against
a real seeded author-date inversion), merge-commit flagging, content-equivalence detection (`git cherry`
+ `merge-base --is-ancestor`, with `patch-id --stable` available as a separately tested fallback
primitive), and search diagnostics (deleted-branch warning, best-effort squash-merge courtesy warning,
no-results alternatives) — all composed into `Service.Discover`, the single discovery entry point, and
proved end-to-end via the consolidated HU-002 Test E2E scenario from `docs/HISTORIAS.md` plus its
deleted-branch and no-results variants. `go build ./...`, `go vet ./...`, `go test -race ./...` all
green; `go test -short ./...` correctly skips the real-git integration tests. Ready for next batch
(Phase 7: HU-003 `commit-selection`).
