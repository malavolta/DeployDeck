# Apply Progress: Foundation + MVP Git (HU-001..HU-006)

**Mode**: Strict TDD
**Batch scope so far**: Phases 1-8 (Bootstrap, `internal/exec`, `internal/config`, `internal/git` core,
HU-001 `prereq-check`, HU-002 `commit-discovery`, HU-003 `commit-selection`, HU-004 `target-selection`).
Phases 9-12 (HU-005..HU-006, `internal/app` wiring, final verification) are NOT started.

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

## Completed Tasks (Phase 7: HU-003 `commit-selection`)

- [x] 7.1 `[U]` RED: `CommitSelectionItem` row shows short SHA/message/author/date/flags.
- [x] 7.2 GREEN: `internal/git/selection.go` — `CommitSelectionItem{DiscoveredCommit,Selected,Disabled,Reason,MultiTicketNotice,OtherTickets}` embedding `DiscoveredCommit` (which embeds `Commit`), promoting SHA/ShortSHA/Author/Date/Subject/Merge/Equivalence directly onto the row.
- [x] 7.3 `[U]` RED: already-applied commit → `Disabled=true` + `Reason`; select attempt is a no-op.
- [x] 7.4 GREEN: `alreadyAppliedReason(EquivalenceStatus)` renders a signal-specific Reason (identical SHA / git cherry / patch-id); `NewCommitSelectionItems` wires it.
- [x] 7.5 `[U]` RED: merge commit → `Disabled=true` (cherry-pick `-m` unsupported in MVP).
- [x] 7.6 GREEN: `ReasonMergeCommit` constant + merge-commit branch in `NewCommitSelectionItems`.
- [x] 7.7 `[U]` RED: commit message referencing other tickets sets a multi-ticket notice.
- [x] 7.8 GREEN: `OtherTicketMentions(subject, searchedTicket)` (pure, ticket-token regex) + `MultiTicketNotice`/`OtherTickets` wiring; `ToggleSelection` also added here (Disabled-aware no-op toggle, needed by 7.3's "select attempt is a no-op" AC).
- [x] 7.9 `[I]` RED: real `git diff --name-only` — selected commit's file also touched by an unselected intermediate commit triggers a dependency warning.
- [x] 7.10 GREEN: `internal/git/dependency_warning.go` — `Service.FilesTouchedByCommit` (real `git diff --name-only <sha>^ <sha>`), `DependencyWarning{File,SelectedSHA,UnselectedSHA}`, pure `ComputeDependencyWarnings` (EARLIER-unselected-commit-touches-same-file rule), and `Service.DependencyWarnings` composing both.
- [x] 7.11 `[U]` RED: confirming with zero selected commits is blocked.
- [x] 7.12 GREEN: `ErrEmptySelection` + `ValidateSelection(items)`.
- [x] 7.13 `[U]` RED: advanced-mode reorder shows conflict-risk warning; non-advanced reorder unavailable.
- [x] 7.14 GREEN: `ErrReorderRequiresAdvancedMode`, `ReorderConflictRiskWarning`, `ReorderSelection(items, from, to, advancedMode)`.
- [x] 7.15 `[U]` RED: valid non-empty confirmed selection generates a preliminary `DeploymentPlan`.
- [x] 7.16 GREEN: `internal/git/deployment_plan.go` — `DeploymentPlan{Ticket,SelectedCommits}` + `GenerateDeploymentPlan(ticket, items)` (reuses `ValidateSelection`).

## Completed Tasks (Phase 8: HU-004 `target-selection`)

- [x] 8.1 `[U]` RED: destination list from `config.Config.branches` shows associated sandbox.
- [x] 8.2 GREEN: `internal/git/target_selection.go` — `Destination{Environment,Branch,Sandbox,SandboxResolved}` + `ListDestinations(cfg)` composing `config.SandboxFor`, sorted by environment key for deterministic output.
- [x] 8.3 `[U]`/`[I]` RED: nonexistent local/remote destination branch blocks continuing.
- [x] 8.4 GREEN: `internal/git/service_target.go` — `Service.BranchExists` via a shared `revParseVerify` helper (`git rev-parse --verify --quiet <ref>`, exit 0/1 as data per the established contract), checking local then `origin/<branch>`.
- [x] 8.5 `[U]` RED: `Release/*` resolves sandbox via configured glob; unmapped `Release/*` blocks with actionable message.
- [x] 8.6 GREEN: `ResolveSandbox(cfg, branch)` wraps `config.SandboxFor` (Phase 3), turning its error into an actionable, branch-naming block message.
- [x] 8.7 `[I]` RED: `git rev-parse origin/<target>` remote HEAD is displayed.
- [x] 8.8 GREEN: `Service.RemoteHead(ctx, dir, target)` via the same `revParseVerify` helper as `BranchExists`.
- [x] 8.9 `[U]` RED (sf-fake): sandbox alias absent from `sf org list --json`, scanned across all five categories, shows a non-blocking warning before validation.
- [x] 8.10 GREEN: `UnauthenticatedSandboxWarning(orgs, alias)` (pure, `salesforce.OrgList.FindByAlias`) + `SandboxAuthWarning(ctx, sf, alias)` composing `salesforce.Client.Orgs`.
- [x] 8.11 `[U]` RED: selecting `main` shows a production-environment warning.
- [x] 8.12 GREEN: `ProductionBranchName`/`IsProductionBranch(branch)` (pure, literal `"main"` per the spec's exact wording, not config-driven).
- [x] 8.13 `[I]` RED: custom branch name — rejected when absent locally/remotely, accepted when present.
- [x] 8.14 GREEN: reaches GREEN immediately, reusing `Service.BranchExists` from 8.3/8.4 — no new production code (see Deviations).
- [x] 8.15 `[U]` RED: confirming a valid destination+sandbox saves branch/alias/testLevel into `DeploymentPlan`.
- [x] 8.16 GREEN: `DeploymentPlan` extended with `TargetBranch`/`SandboxAlias`/`TestLevel` fields + `ConfirmTargetSelection(plan, branch, alias, testLevel)` (pure, preserves every other field already on the plan).

## Remaining Tasks (NOT in this batch)

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
- Current work unit: Unit 2 of 4 suggested units — "HU-001..HU-005 (Phases 5-9)" — Phases 5-7 (`prereq-check`, `commit-discovery`, `commit-selection`) are now complete; Phases 8-9 remain for this unit
- Boundary: starts from Phase 1-4's `internal/exec`/`internal/config`/`internal/git` core (all green, no HU-level behavior); ends with `internal/salesforce`, `internal/prereq`, `deploydeck doctor` (Phase 5), `internal/git`'s discovery surface (Phase 6), and Phase 7's selection surface — `CommitSelectionItem`, `NewCommitSelectionItems`, `ToggleSelection`, `ValidateSelection`, `ReorderSelection`, `Service.DependencyWarnings`/`FilesTouchedByCommit`, `DeploymentPlan`/`GenerateDeploymentPlan` — fully wired and independently tested/green
- Estimated review budget impact: 5 commits this batch (~1,880 changed lines: selection model + disable reasons, per-file dependency warnings, empty-selection guard + reorder, `DeploymentPlan` generation, consolidated HU-003 E2E test); tracked against the session's explicit `review_budget_lines=40000` budget per the accepted `size:exception`

## TDD Cycle Evidence (Phase 7 — this batch)

| Task | Test File | Layer | Safety Net | RED | GREEN | TRIANGULATE | REFACTOR |
|------|-----------|-------|------------|-----|-------|-------------|----------|
| 7.1-7.8 | `internal/git/selection_test.go` (`TestNewCommitSelectionItems_TableDriven`, `TestToggleSelection_TableDriven`) | Unit (pure) | ✅ 91/91 (whole module, pre-Phase-7) | ✅ Written (`undefined: git.CommitSelectionItem`/`NewCommitSelectionItems`) | ✅ Passed | ✅ 4 table cases (plain/already-applied/merge/multi-ticket) + 3 toggle cases | ➖ None needed |
| 7.9-7.10 | `internal/git/dependency_warning_test.go` (`TestService_DependencyWarnings_Integration_RealDiff`, `TestComputeDependencyWarnings_TableDriven`) | Integration (`newTempRepo`, real `git diff --name-only`) + Unit (pure) | ✅ (`internal/git` green pre-task) | ✅ Written (`svc.DependencyWarnings undefined`) | ✅ Passed | ✅ 4 pure table cases (overlap/no-overlap/later-unselected-not-flagged/both-selected-no-warning) + 1 real-git scenario | ➖ None needed |
| 7.11-7.14 | `internal/git/selection_test.go` (`TestValidateSelection_TableDriven`, `TestReorderSelection_TableDriven`) | Unit (pure) | ✅ (green pre-task) | ✅ Written (`undefined: git.ErrEmptySelection`) | ✅ Passed | ✅ 3 empty-selection cases + 2 reorder cases (rejected/advanced) | ➖ None needed |
| 7.15-7.16 | `internal/git/deployment_plan_test.go` (`TestGenerateDeploymentPlan_TableDriven`) | Unit (pure) | ✅ (green pre-task) | ✅ Written (`undefined: git.DeploymentPlan`) | ✅ Passed | ✅ 3 cases (full selection/partial selection/empty-blocked) | ➖ None needed |
| HU-003 Test E2E (required deliverable) | `internal/git/selection_e2e_test.go` (`TestHU003_CommitSelection_E2E`) | Integration (`newTempRepo`, real git, full flow) | ✅ (green pre-task) | ➖ N/A — composes only already-GREEN production code; first real-git run itself is the empirical proof (topo order, equivalence classification, dependency detection all asserted against real output) | ✅ Passed on first run | ✅ 2 named sub-variants (empty-selection block, advanced-mode reorder) plus the primary scenario's 6 distinct assertions (merge disabled, already-applied disabled, multi-ticket notice, dependency warning, plan generation, order) | ➖ None needed |

### Test Summary (Phase 7)

- **Total new top-level test functions this batch**: 8 (`TestNewCommitSelectionItems_TableDriven`, `TestToggleSelection_TableDriven`, `TestService_DependencyWarnings_Integration_RealDiff`, `TestComputeDependencyWarnings_TableDriven`, `TestValidateSelection_TableDriven`, `TestReorderSelection_TableDriven`, `TestGenerateDeploymentPlan_TableDriven`, `TestHU003_CommitSelection_E2E`); 99 total top-level test functions passing in the whole module (`go test ./... -v`), up from 91 at the end of the Phase 6 batch, including all table-driven sub-cases.
- **`go test -race ./...`**: all packages `ok`, no data races.
- **`go test -short ./...`**: all packages `ok`; the new real-git integration tests (`TestService_DependencyWarnings_Integration_RealDiff`, `TestHU003_CommitSelection_E2E`) correctly skip via the shared `newTempRepo` harness's `testing.Short()` guard.
- **Layers used**: Unit (selection model construction, disabled/reason logic, multi-ticket detection, toggle no-op, empty-selection guard, reorder gating, dependency-warning pure decision logic, DeploymentPlan generation), Integration (real `git diff --name-only` per-commit file listing, the full consolidated HU-003 E2E scenario) — all via the shared `newTempRepo`/`writeAndCommit`/`runGit` helpers, no new helpers needed.
- **Approval tests** (refactoring): None — Phase 7 is exclusively new code composing HU-002's existing `Discover`/`DiscoveredCommit`/`Commit` surface; nothing existing was refactored.
- **Pure functions/types created**: `CommitSelectionItem`, `NewCommitSelectionItems`, `OtherTicketMentions`, `ToggleSelection`, `ValidateSelection`, `ReorderSelection`, `ComputeDependencyWarnings`, `DependencyWarning`, `GenerateDeploymentPlan`, `DeploymentPlan`, `alreadyAppliedReason` (unexported).

## Work Unit Evidence (Phase 7)

| Evidence | Value |
|---|---|
| Focused test command and exact result | `go test ./internal/git/... -run 'TestNewCommitSelectionItems_TableDriven|TestToggleSelection_TableDriven|TestService_DependencyWarnings_Integration_RealDiff|TestComputeDependencyWarnings_TableDriven|TestValidateSelection_TableDriven|TestReorderSelection_TableDriven|TestGenerateDeploymentPlan_TableDriven|TestHU003_CommitSelection_E2E' -v` → all PASS. |
| Runtime harness command/scenario and exact result | `go test ./internal/git/... -run TestHU003_CommitSelection_E2E -v` → PASS: real temp git repo (`newTempRepo`) seeded with an interleaved other-ticket commit and a later same-file commit (dependency warning), a content-equivalent already-applied commit, a dual-ticket commit, and a merge commit; asserts `NewCommitSelectionItems` (composed with `Service.Discover`'s real output) correctly disables the merge and already-applied rows with reasons, flags the multi-ticket notice, and that `Service.DependencyWarnings` (real `git diff --name-only`) detects the shared-file dependency after the user deselects the earlier commit; also asserts the empty-selection block and the advanced-mode-only reorder + warning as named sub-variants. `-short` correctly skips it (shells out to real git). |
| Rollback boundary | Revert commits `3ef86d2` (selection model + disable reasons), `759b82b` (per-file dependency warnings), `f04dca0` (empty-selection guard + reorder), `91d4dd2` (`DeploymentPlan` generation), `035a232` (consolidated HU-003 E2E test), plus this batch's `tasks.md`/`apply-progress.md` mark commit. Phases 1-6 (`internal/exec`, `internal/config`, `internal/git` HU-001/HU-002 surface, `internal/prereq`, `internal/salesforce`, `deploydeck doctor`) remain fully buildable/testable alone; nothing in Phase 8+ exists yet to depend on Phase 7. |

## Deviations from Design (Phase 7)

1. **`DeploymentPlan` lives in `internal/git`, not `internal/app`**: design.md's Decision prose says "`Model` holds `state`, `DeploymentPlan`, service deps", which could read as `DeploymentPlan` belonging to `internal/app`. But Phase 7 (HU-003) runs before `internal/app` exists (Phase 11), and `internal/git` must never depend on `internal/app` (the dependency runs the other way — `internal/app` composes `internal/git`, per the package-layout table). Placing `DeploymentPlan` in `internal/git`, alongside the other cross-HU business logic this design already puts there (`SelectSingleSource`, `SuggestDefaultSource`, `Service.CreatePromotionBranch` named in task 9.3), lets HU-003/004/005 build and extend the SAME value without a package cycle; `internal/app`'s `Model` (Phase 11) will simply hold a `git.DeploymentPlan` field. `DeploymentPlan` currently has only `Ticket`/`SelectedCommits` (HU-003's own fields); Phase 8/9/10 tasks add `TargetBranch`/`SandboxAlias`/`TestLevel`/`PromotionBranch`/delta-path fields to the SAME struct without breaking this phase's field-name-keyed literals.
2. **`CommitSelectionItem` embeds `DiscoveredCommit` (not a bare `Commit` field named literally "Commit")**: docs/HISTORIAS.md's suggested model (`CommitSelectionItem{Commit,Selected,Disabled,Reason}`, written before HU-002 existed) has no way to know a commit is a merge or already-applied — that classification is `DiscoveredCommit`'s (`Merge bool`, `Equivalence EquivalenceStatus`), the exact HU-002 output HU-003 is instructed to build on. Embedding `DiscoveredCommit` (which itself embeds `Commit`) promotes every field HISTORIAS.md's model and design.md's `CommitSelectionItem{ Commit; Selected; Disabled; Reason }` contract both name (SHA/ShortSHA/Author/Date/Subject) up to the top level by Go's normal embedding rules, so callers read `item.ShortSHA`/`item.Subject`/etc. exactly as either doc implies, while `NewCommitSelectionItems` also has direct access to `Merge`/`Equivalence` to compute `Disabled`/`Reason` without a second lookup or duplicated fields.
3. **Multi-ticket detection uses a package-local ticket-token regex, not `config.Config.TicketPatterns`**: `config.TicketPatterns` is scoped to ticket SEARCH (matching a repo's own ticket ID convention against a *known* ticket string, consumed by Phase 3's `Validate()`/future search wiring) and `internal/git` does not import `internal/config` anywhere in the existing Phase 4-6 code (no precedent to wire it in). HU-003's requirement is different in kind: given an ALREADY-discovered commit's Subject, find OTHER ticket-shaped tokens besides the one searched — a self-contained scan that doesn't need a per-repo pattern, since it only needs to recognize the same generic `KEY-123` shape the project's own fixtures/docs use throughout (HISTORIAS.md, HU-002's tests). Wiring `config.Config` into this one function for a single generic pattern was judged out of proportion for MVP; flagged here for confirmation if a future HU needs per-repo-customized ticket shapes for this specific notice.
4. **Dependency-warning detection only looks EARLIER in topo order, not both directions**: HISTORIAS.md's Spanish wording ("commits intermedios no seleccionados... que tocan el mismo fichero") doesn't explicitly say "earlier"; design.md's Interfaces/Contracts section is similarly general ("per-file dependency warning from intermediate unselected commits touching same file"). The batch's explicit task instructions, however, state the exact mechanics: "if the user selects commit N but skips an earlier commit M that also modified a file N touches, warn." This is also the semantically correct direction: cherry-picking a later commit alone, when an EARLIER commit to the same file was skipped, is the case where the file's assumed prior state is missing from the target — a later unselected commit touching the same file does not create this specific risk (the selected commit doesn't depend on content that comes AFTER it). Implemented and tested exactly as instructed; documented here in case a future HU wants bidirectional detection.

## Issues Found (Phase 7)

1. No blocking issues. The consolidated `TestHU003_CommitSelection_E2E` scenario passed on its first real-git run with no adjustment needed to the seeded topo order, equivalence classification, or dependency detection — all empirically verified rather than assumed.
2. All Phase 7 acceptance criteria (`specs/commit-selection/spec.md` under `openspec/changes/foundation-mvp-git/specs/commit-selection/spec.md`, and `docs/HISTORIAS.md` HU-003, including its consolidated `### Test E2E` scenario and the advanced-mode-reorder variant) are implemented and green.

## TDD Cycle Evidence (Phase 8 — this batch)

| Task | Test File | Layer | Safety Net | RED | GREEN | TRIANGULATE | REFACTOR |
|------|-----------|-------|------------|-----|-------|-------------|----------|
| 8.1/8.2 | `internal/git/target_selection_test.go` (`TestListDestinations_TableDriven`) | Unit (pure) | ✅ 99/99 (whole module, pre-Phase-8) | ✅ Written (`undefined: git.Destination`/`ListDestinations`) | ✅ Passed | ✅ 2 table cases (sandbox resolved / unmapped-but-listed) | ➖ None needed |
| 8.3/8.4 | `internal/git/service_target_test.go` (`TestService_BranchExists_TableDriven`) | Integration (`newTempRepo`, real git) | ✅ (green pre-task) | ✅ Written (`svc.BranchExists undefined`) | ✅ Passed | ✅ 3 cases (local+remote / remote-only / neither) | ➖ None needed |
| 8.5/8.6 | `internal/git/target_selection_test.go` (`TestResolveSandbox_TableDriven`) | Unit (pure) | ✅ (green pre-task) | ✅ Written (`undefined: git.ResolveSandbox`) | ✅ Passed | ✅ 3 cases (exact / Release/\* glob / unmapped block) | ➖ None needed |
| 8.7/8.8 | `internal/git/service_target_test.go` (`TestService_RemoteHead_TableDriven`) | Integration (`newTempRepo`, real git) | ✅ (green pre-task) | ✅ Written (`svc.RemoteHead undefined`) | ✅ Passed | ✅ 3 cases (UAT / main / absent target) | ➖ None needed |
| 8.9/8.10 | `internal/git/target_selection_test.go` (`TestUnauthenticatedSandboxWarning_TableDriven`, `TestSandboxAuthWarning_ComposesSalesforceClient`) | Unit (pure) + Unit (`FakeRunner` sf-fake) | ✅ (green pre-task) | ✅ Written (`undefined: git.UnauthenticatedSandboxWarning`/`SandboxAuthWarning`) | ✅ Passed | ✅ 4 pure cases (found in default category / found in non-default category / absent / empty alias) + 2 composed cases (warned / not warned) | ➖ None needed |
| 8.11/8.12 | `internal/git/target_selection_test.go` (`TestIsProductionBranch_TableDriven`) | Unit (pure) | ✅ (green pre-task) | ✅ Written (`undefined: git.IsProductionBranch`) | ✅ Passed | ✅ 3 cases (`main` / `UAT` / a branch merely containing "main") | ➖ None needed |
| 8.13/8.14 | `internal/git/service_target_test.go` (`TestService_BranchExists_CustomBranchName`) | Integration (`newTempRepo`, real git) | ✅ (green pre-task) | ➖ N/A — composes only already-GREEN `BranchExists` from 8.3/8.4; the first run itself is the empirical proof this generic primitive also works for an arbitrary custom (non-config) branch name | ✅ Passed on first run | ✅ 2 cases (custom branch present / custom branch never created) within the same test | ➖ None needed |
| 8.15/8.16 | `internal/git/target_selection_test.go` (`TestConfirmTargetSelection_TableDriven`) | Unit (pure) | ✅ (green pre-task) | ✅ Written (`undefined: git.ConfirmTargetSelection`; `DeploymentPlan` had no `TargetBranch`/`SandboxAlias`/`TestLevel` fields) | ✅ Passed | ✅ 2 cases (UAT/RunLocalTests, Release/NoTestRun) + an explicit prior-field-preservation assertion (Ticket/SelectedCommits untouched) | ➖ None needed |
| HU-004 Test E2E (required deliverable) | `internal/git/target_selection_e2e_test.go` (`TestHU004_TargetSelection_E2E`) | Integration (`newTempRepo`, real git) + fs (config fixture) + sf-fake (`FakeRunner`) | ✅ (green pre-task) | ➖ N/A — composes only already-GREEN production code; the first real-git/real-fs run is the empirical proof (destination/sandbox resolution, `Release/*` pattern, remote HEAD, production warning, `DeploymentPlan` persistence all asserted against real output) | ✅ Passed on first run | ✅ 2 named sub-variants (nonexistent destination branch, unauthenticated sandbox) plus the primary scenario's 6 distinct assertions | ➖ None needed |

### Test Summary (Phase 8)

- **Total new top-level test functions this batch**: 10 (`TestListDestinations_TableDriven`, `TestService_BranchExists_TableDriven`, `TestResolveSandbox_TableDriven`, `TestService_RemoteHead_TableDriven`, `TestUnauthenticatedSandboxWarning_TableDriven`, `TestSandboxAuthWarning_ComposesSalesforceClient`, `TestIsProductionBranch_TableDriven`, `TestService_BranchExists_CustomBranchName`, `TestConfirmTargetSelection_TableDriven`, `TestHU004_TargetSelection_E2E`); 109 total top-level test functions passing in the whole module (`go test ./... -v`), up from 99 at the end of the Phase 7 batch, including all table-driven sub-cases.
- **`go test -race ./...`**: all packages `ok`, no data races.
- **`go test -short ./...`**: all packages `ok`; the new real-git integration tests (`TestService_BranchExists_TableDriven`, `TestService_RemoteHead_TableDriven`, `TestService_BranchExists_CustomBranchName`, `TestHU004_TargetSelection_E2E`) correctly skip via the shared `newTempRepo` harness's `testing.Short()` guard.
- **Layers used**: Unit (destination listing, sandbox resolution message, production-branch rule, unauthenticated-sandbox pure decision), Unit/sf-fake (`FakeRunner`-backed `salesforce.Client` composition), Integration (real `git rev-parse --verify` for branch existence and remote HEAD, via `newTempRepo`), fs (a real `deploydeck.yaml` fixture written to the temp repo root and loaded through `config.Load`) — the consolidated E2E test is the only scenario in this batch (or any prior HU) combining all three: fs + temp git + sf-fake in one flow, as HU-004's own harness description requires.
- **Approval tests** (refactoring): None — Phase 8 is exclusively new code; `DeploymentPlan` was additively extended (three new fields), not refactored, and its own existing HU-003 test (`TestGenerateDeploymentPlan_TableDriven`) stayed green untouched throughout.
- **Pure functions/types created**: `Destination`, `ListDestinations`, `ResolveSandbox`, `ProductionBranchName`/`IsProductionBranch`, `UnauthenticatedSandboxWarning`, `ConfirmTargetSelection`; composed (Runner/Salesforce-backed): `Service.revParseVerify` (unexported), `Service.RemoteHead`, `Service.BranchExists`, `SandboxAuthWarning`.

## Work Unit Evidence (Phase 8)

| Evidence | Value |
|---|---|
| Focused test command and exact result | `go test ./internal/git/... -run 'TestListDestinations_TableDriven|TestService_BranchExists_TableDriven|TestResolveSandbox_TableDriven|TestService_RemoteHead_TableDriven|TestUnauthenticatedSandboxWarning_TableDriven|TestSandboxAuthWarning_ComposesSalesforceClient|TestIsProductionBranch_TableDriven|TestService_BranchExists_CustomBranchName|TestConfirmTargetSelection_TableDriven|TestHU004_TargetSelection_E2E' -v` → all PASS. |
| Runtime harness command/scenario and exact result | `go test ./internal/git/... -run TestHU004_TargetSelection_E2E -v` → PASS: a real `deploydeck.yaml` fixture written to a real temp git repo's root (fs), loaded via `config.Load`/`Validate`; real branches `UAT` (seeded + pushed, known remote HEAD captured), `INT` (exists but deliberately has no sandbox mapping), and `Release/Julio2026` (resolves its sandbox via the configured glob pattern); a FakeRunner-canned `sf org list --json` (sf-fake) proving the unauthenticated-sandbox warning. Asserts `ListDestinations`, `ResolveSandbox`, `Service.RemoteHead`, `IsProductionBranch`, and `ConfirmTargetSelection` end-to-end, plus the nonexistent-destination-branch and unauthenticated-sandbox named variants from `docs/HISTORIAS.md`. `-short` correctly skips it (shells out to real git). |
| Rollback boundary | Revert commits `5667b92` (`BranchExists`/`RemoteHead`), `3c29260` (`ListDestinations`/`ResolveSandbox`/warnings/`ConfirmTargetSelection` + `DeploymentPlan` field extension), `433bcaf` (consolidated HU-004 E2E test), plus this batch's `tasks.md`/`apply-progress.md` mark commit. Phases 1-7 (`internal/exec`, `internal/config`, `internal/git` HU-001/HU-002/HU-003 surface, `internal/prereq`, `internal/salesforce`, `deploydeck doctor`) remain fully buildable/testable alone; nothing in Phase 9+ exists yet to depend on Phase 8. |

## Deviations from Design (Phase 8)

1. **`internal/git` now imports `internal/config` and `internal/salesforce`, a first for the package**: Phase 7's own Deviation #3 flagged that `internal/git` had no precedent importing `internal/config`; design.md's package-layout table already anticipates this for Phase 9's `branchFormat` renderer, and HU-004's own spec (`specs/target-selection/spec.md`) requires composing both `config.Config` (branches/sandboxes) and `salesforce.Client` (`sf org list --json`) directly in the target-selection flow. No import cycle exists (`internal/config` and `internal/salesforce` both depend only on `internal/exec`/stdlib), and `DeploymentPlan` already lives in `internal/git` per Phase 7's own precedent for HU-003/004/005 shared state — so composing HU-004's whole domain here (pure config/salesforce composition in `target_selection.go`, real-git primitives in `service_target.go`) keeps the same "one domain, one package, grows without a cycle" shape the whole design already commits to, rather than introducing a new package for a six-function domain.
2. **`SandboxAuthWarning`/`ConfirmTargetSelection` are free functions, not `Service` methods**: every existing `Service` method wraps only the `Runner` `Service` already carries; adding a `salesforce.Client` parameter to a `Service` method would be the first departure from that shape. Kept as free functions taking their dependency explicitly, matching `SuggestDefaultSource(candidates, cfg, target)`'s existing precedent (Phase 6) for composing `config.Config` without being a `Service` method.
3. **8.13/8.14 (custom-branch validation) reached GREEN immediately, no separate production code**: `Service.BranchExists` (built for 8.3/8.4) is already fully generic — it validates ANY branch name, whether it came from `config.Config.Branches` or was typed freehand by the user; there is no "custom" code path to distinguish. This mirrors Phase 6's 6.13/6.14 precedent (merge-commit detection reached GREEN immediately via already-unified parsing): the dedicated test in this batch exists to LOCK IN and document that a custom branch name is correctly accepted/rejected too, not to drive new logic.
4. **`ProductionBranchName`/`IsProductionBranch` is a literal `"main"` check, not config-driven**: `config.Config.Branches["production"]` maps a logical key to whatever branch name a repo actually configures (the example config happens to use `"main"`, but nothing requires it). `docs/HISTORIAS.md`'s AC and `specs/target-selection/spec.md`'s "Production Branch Warning" requirement both name `main` literally ("Dado que el usuario selecciona `main`..."), not "whatever the configured production branch is" — implemented exactly as the spec's literal wording states. Flagged here in case a future HU wants this warning driven by `cfg.Branches["production"]` instead.

## Issues Found (Phase 8)

1. **Pre-existing, untouched gofmt nit discovered, not introduced by this batch**: `gofmt -l .` flags `internal/git/dependency_warning_test.go` (a Phase 7 file, commit `759b82b`, not modified in this batch — confirmed via `git status`/`git log` showing zero pending changes to it) for a trailing-comment alignment difference. Not fixed here to keep this batch's diff scoped to Phase 8 only; flagged for a follow-up `gofmt -w` pass before Phase 12's `gofmt -l .` clean-check task.
2. No other blocking issues. All Phase 8 acceptance criteria (`specs/target-selection/spec.md` under `openspec/changes/foundation-mvp-git/specs/target-selection/spec.md`, and `docs/HISTORIAS.md` HU-004, including its consolidated `### Test E2E` scenario and both named variants) are implemented and green.

## Status

108/108 tasks in scope (Phases 1-8) complete (170 total tasks in `tasks.md`; 62 remain across Phases 9-12).
Phase 8 (HU-004 `target-selection`) complete under strict TDD: config-driven destination listing with
resolved sandbox alias/test level (`ListDestinations`), `Release/*` glob-pattern sandbox resolution with an
actionable block message for any unmapped destination (`ResolveSandbox`), real destination-branch existence
validation covering both configured and custom branch names (`Service.BranchExists`), real remote-HEAD
display (`Service.RemoteHead`), the non-blocking unauthenticated-sandbox warning composing
`salesforce.Client` (`UnauthenticatedSandboxWarning`/`SandboxAuthWarning`), the literal `main`
production-environment warning (`IsProductionBranch`), and selection persistence extending `DeploymentPlan`
with `TargetBranch`/`SandboxAlias`/`TestLevel` (`ConfirmTargetSelection`) — all proved end-to-end via the
consolidated HU-004 Test E2E scenario from `docs/HISTORIAS.md`, run through the documented
fs + temp-git + sf-fake harness, plus its nonexistent-branch and unauthenticated-sandbox variants.
`go build ./...`, `go vet ./...`, `go test -race ./...` all green; `go test -short ./...` correctly skips
the real-git integration tests. Ready for next batch (Phase 9: HU-005 `promotion-branch`).
