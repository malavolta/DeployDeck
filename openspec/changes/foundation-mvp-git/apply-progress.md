# Apply Progress: Foundation + MVP Git (HU-001..HU-006)

**Mode**: Strict TDD
**Batch scope so far**: Phases 1-9 (Bootstrap, `internal/exec`, `internal/config`, `internal/git` core,
HU-001 `prereq-check`, HU-002 `commit-discovery`, HU-003 `commit-selection`, HU-004 `target-selection`,
HU-005 `promotion-branch`). Phases 10-12 (HU-006 `cherry-pick`, `internal/app` wiring, final
verification) are NOT started.

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

## Completed Tasks (Phase 9: HU-005 `promotion-branch`)

- [x] 9.1 Extend shared helper with `newTempRepoWithRemote(t)` — local repo + local bare "remote", clone, then advance the bare remote independently.
- [x] 9.2 `[I]` RED: branch creation runs `git fetch origin` before creating the branch (assert ordering) — via `callRecordingRunner`, a spy wrapping the real `OSRunner`.
- [x] 9.3 GREEN: `internal/git/service_promotion.go` — `Service.CreatePromotionBranch` (fetch-then-checkout sequencing) + `Service.fetchOrigin` (unexported).
- [x] 9.4 `[I]` RED: after remote advances post-clone (via `newTempRepoWithRemote`'s `seedDir`, never touching `localDir`), branch is created exactly from post-fetch `origin/UAT` HEAD, not the stale local ref.
- [x] 9.5 GREEN: reaches GREEN immediately — `CreatePromotionBranch` already resolves the base ref as the literal `"origin/"+target` string and lets git resolve it AT CHECKOUT TIME (never a SHA captured pre-fetch), so the freshly-fetched ref is used by construction (see Deviations).
- [x] 9.6 `[U]` RED: `RenderBranchName` table-driven — default `deploy/{{ticket}}-to-{{target}}`, a custom reordered format, and a literal user-edited override with no tokens.
- [x] 9.7 GREEN: `internal/git/promotion_branch.go` — `RenderBranchName(format, ticket, target)` via `strings.NewReplacer` over the same `{{ticket}}`/`{{target}}` tokens as `config.AllowedBranchFormatTokens`.
- [x] 9.8 `[I]` RED: existing local branch and existing origin-only branch (discovered via the mandatory fetch) with the same name each block creation.
- [x] 9.9 GREEN: `ErrPromotionBranchExists` + a `Service.BranchExists` check (reusing the Phase 8 primitive) in `CreatePromotionBranch`, run AFTER the fetch so an origin-only collision pushed since the last fetch is still caught.
- [x] 9.10 `[I]` RED: user on a protected branch (`main`, per `config.Config.Branches`) — starting the flow does not modify that branch directly, proven by comparing its ref before/after a real `CreatePromotionBranch` call.
- [x] 9.11 GREEN: `ProtectedBranches(cfg)`/`IsProtectedBranch(cfg, branch)` (pure, config-driven — every branch name in `cfg.Branches`), documented as the caller-facing detection predicate; non-modification itself is structural (the new branch always bases on `origin/<target>`, never the current HEAD).
- [x] 9.12 `[I]` RED: a REAL `git fetch origin` failure (`origin` remote URL pointed at a nonexistent path) stops the flow; current branch and the (non-created) promotion branch are asserted unchanged.
- [x] 9.13 GREEN: reaches GREEN immediately — the fetch-error early return already in `CreatePromotionBranch` since 9.3 satisfies this (see Deviations).
- [x] 9.14 `[U]` RED: `RegisterPromotionBranch` table-driven — two branch names, asserting prior plan fields (`Ticket`/`TargetBranch`/`SandboxAlias`/`TestLevel`) are preserved.
- [x] 9.15 GREEN: `DeploymentPlan.PromotionBranch` field + `RegisterPromotionBranch(plan, branchName)` (pure, same gate-then-persist shape as `ConfirmTargetSelection`).
- [x] HU-005 Test E2E (required deliverable): `internal/git/promotion_branch_e2e_test.go` — `TestHU005_PromotionBranch_E2E` on the documented two-repo bare-remote harness, plus its three named variants (existing branch locally+remotely, protected-branch, fetch-failure) as subtests.

## Remaining Tasks (NOT in this batch)

- [x] Phase 10: HU-006 `cherry-pick` engine (10.1-10.18, 10.21-10.36) — DONE
- [x] Phase 10: 10.19/10.20 (`tea.Tick` re-poll) + 10.37/10.38 (dependency-boundary test) — DONE in the Phase 11 wiring batch (they require `internal/app`)
- [x] Phase 11: `internal/app` wiring (11.1-11.6) — DONE this batch
- [x] Phase 12: 12.1 (`go test ./...`) + 12.2 (`go vet` / `gofmt -l .`) run green this batch; 12.3 (proposal Success-Criteria check-off) left for the orchestrator's dedicated final-verification pass

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

## TDD Cycle Evidence (Phase 9 — this batch)

| Task | Test File | Layer | Safety Net | RED | GREEN | TRIANGULATE | REFACTOR |
|------|-----------|-------|------------|-----|-------|-------------|----------|
| 9.1 | `internal/git/helpers_test.go` (`newTempRepoWithRemote`, `callRecordingRunner`) | Test infra (no RED/GREEN pair — scaffolding, mirrors Phase 4's 4.1 `newTempRepo`) | N/A (additive to existing helper file) | N/A | N/A | N/A | N/A |
| 9.2/9.3 | `internal/git/service_promotion_test.go` (`TestService_CreatePromotionBranch_FetchRunsBeforeCheckout`) | Integration (`callRecordingRunner` spying on real `OSRunner`, `newTempRepoWithRemote`) | ✅ 109/109 (whole module, pre-Phase-9) | ✅ Written (`svc.CreatePromotionBranch undefined`) | ✅ Passed (real `git fetch origin` call index < real `git checkout -b` call index) | ➖ Single scenario (ordering is a binary property; the two-repo advanced-remote test in 9.4 is this task's real triangulation axis) | ➖ None needed |
| 9.4/9.5 | `internal/git/service_promotion_test.go` (`TestService_CreatePromotionBranch_FollowsAdvancedRemoteNotStaleLocalRef`) | Integration (`newTempRepoWithRemote`, real git, real remote advance via `seedDir`) | ✅ (green pre-task) | ➖ N/A — composes only already-GREEN 9.3 production code; the real-git run against an ADVANCED remote is the empirical proof | ✅ Passed on first run | ➖ Single scenario, but **verified as a REAL assertion**: production code was deliberately mutated to resolve `origin/<target>` to a SHA captured BEFORE the fetch (simulating the stale-ref bug this HU exists to prevent) — the test FAILED with the exact expected/got mismatch, then the mutation was reverted and the test passed again. This is stronger evidence than triangulation alone. | ➖ None needed |
| 9.6/9.7 | `internal/git/promotion_branch_test.go` (`TestRenderBranchName_TableDriven`) | Unit (pure) | ✅ (green pre-task) | ✅ Written (`undefined: git.RenderBranchName`) | ✅ Passed | ✅ 3 table cases (default format, reordered custom format, literal override with zero tokens) | ➖ None needed |
| 9.8/9.9 | `internal/git/service_promotion_test.go` (`TestService_CreatePromotionBranch_ExistingBranchCollision_TableDriven`) | Integration (`newTempRepoWithRemote`, real git) | ✅ (green pre-task) | ✅ Written (`undefined: git.ErrPromotionBranchExists`) | ✅ Passed | ✅ 3 table cases (local collision, origin-only collision discovered post-fetch, no collision creates normally) | ➖ None needed |
| 9.10/9.11 | `internal/git/promotion_branch_test.go` (`TestIsProtectedBranch_TableDriven`) + `internal/git/service_promotion_test.go` (`TestService_CreatePromotionBranch_ProtectedBranchNotModifiedDirectly`) | Unit (pure) + Integration (real git) | ✅ (green pre-task) | ✅ Written (`undefined: git.IsProtectedBranch`, both files) | ✅ Passed | ✅ 3 pure table cases (two configured environment branches, one unconfigured feature branch) + 1 real-git non-modification scenario (protected branch ref unchanged, HEAD moves to the new promotion branch) | ➖ None needed |
| 9.12/9.13 | `internal/git/service_promotion_test.go` (`TestService_CreatePromotionBranch_FetchFailure_StopsWithoutBranchChange`) | Integration (real git — `origin` URL repointed at a nonexistent path so the fetch genuinely fails) | ✅ (green pre-task) | ➖ N/A — composes only already-GREEN 9.3 short-circuit; the real fetch-failure run is the empirical proof | ✅ Passed on first run | ➖ Single scenario, but **verified as a REAL assertion** the same way as 9.4/9.5: production code was mutated to ignore the fetch error and continue anyway — the test FAILED (no error returned), then the mutation was reverted and the test passed again. | ➖ None needed |
| 9.14/9.15 | `internal/git/promotion_branch_test.go` (`TestRegisterPromotionBranch_TableDriven`) | Unit (pure) | ✅ (green pre-task) | ✅ Written (`undefined: git.RegisterPromotionBranch`) | ✅ Passed | ✅ 2 cases (default-format name, custom-format name), plus an explicit prior-field-preservation assertion (Ticket/TargetBranch/SandboxAlias/TestLevel untouched) | ➖ None needed |
| HU-005 Test E2E (required deliverable) | `internal/git/promotion_branch_e2e_test.go` (`TestHU005_PromotionBranch_E2E`) | Integration (`newTempRepoWithRemote`, real git, two-repo bare-remote) | ✅ (green pre-task) | ➖ N/A — composes only already-GREEN production code; the first real-git run against an advanced remote is the empirical proof | ✅ Passed on first run | ✅ 3 named subtests (existing branch locally+remotely, protected-branch, fetch-failure) plus the primary scenario's 4 distinct assertions (fetch-before-checkout ordering, advanced-tip base ref, plan registration, rendered branch name) | ➖ None needed |

### Test Summary (Phase 9)

- **Total new top-level test functions this batch**: 9 (`TestService_CreatePromotionBranch_FetchRunsBeforeCheckout`, `TestService_CreatePromotionBranch_FollowsAdvancedRemoteNotStaleLocalRef`, `TestService_CreatePromotionBranch_ExistingBranchCollision_TableDriven`, `TestService_CreatePromotionBranch_ProtectedBranchNotModifiedDirectly`, `TestService_CreatePromotionBranch_FetchFailure_StopsWithoutBranchChange`, `TestRenderBranchName_TableDriven`, `TestIsProtectedBranch_TableDriven`, `TestRegisterPromotionBranch_TableDriven`, `TestHU005_PromotionBranch_E2E`); 118 total top-level test functions passing in the whole module (`go test ./... -v`), up from 109 at the end of the Phase 8 batch, including all table-driven sub-cases.
- **`go test -race ./...`**: all packages `ok`, no data races.
- **`go test -short ./...`**: all packages `ok`; every new test in this batch is a real-git integration test and correctly skips via `newTempRepoWithRemote`'s `testing.Short()` guard (it delegates to the same guard `newTempRepo` uses).
- **Layers used**: Unit (branch-name rendering, protected-branch predicate, plan registration), Integration (fetch-then-checkout ordering via a call-recording spy on real git, the advanced-remote-not-stale-ref correctness property, local/remote collision detection, protected-branch non-modification, real fetch failure, the consolidated E2E scenario) — all via the NEW `newTempRepoWithRemote`/`callRecordingRunner` helpers plus the existing `runGit`/`writeAndCommit`/`trimNewline` helpers.
- **Approval tests** (refactoring): None — Phase 9 is exclusively new code (`service_promotion.go`, `promotion_branch.go`); `DeploymentPlan` was additively extended (one new field, `PromotionBranch`), not refactored, and its existing Phase 7/8 tests (`TestGenerateDeploymentPlan_TableDriven`, `TestConfirmTargetSelection_TableDriven`) stayed green untouched throughout.
- **Pure functions/types created**: `RenderBranchName`, `ProtectedBranches`, `IsProtectedBranch`, `RegisterPromotionBranch`; composed (Runner-backed): `Service.CreatePromotionBranch`, `Service.fetchOrigin` (unexported); error value `ErrPromotionBranchExists`.
- **Mutation-verified assertions**: 2 of this batch's tests (9.4/9.5's stale-ref test, 9.12/9.13's fetch-failure test) were explicitly confirmed to FAIL against a deliberately reintroduced bug, then reverted to pass again — the strongest available evidence that a test exercising already-satisfied production code is a REAL assertion, not a vacuous one (per the Assertion Quality Rules' "Incomplete TDD Cycle" guidance).

## Work Unit Evidence (Phase 9)

| Evidence | Value |
|---|---|
| Focused test command and exact result | `go test ./internal/git/... -run 'TestHU005_PromotionBranch_E2E\|TestService_CreatePromotionBranch\|TestRenderBranchName_TableDriven\|TestIsProtectedBranch_TableDriven\|TestRegisterPromotionBranch_TableDriven' -v` → all PASS. |
| Runtime harness command/scenario and exact result | `go test ./internal/git/... -run TestHU005_PromotionBranch_E2E -v` → PASS: the two-repo bare-remote harness (`newTempRepoWithRemote`) with `origin/UAT` seeded at an initial HEAD, then advanced to a NEW HEAD directly on the bare remote via `seedDir` (never touching the local clone under test) after the local clone already exists; asserts `git fetch origin` runs before `git checkout -b` (via `callRecordingRunner`), the created branch starts EXACTLY from the post-fetch advanced `origin/UAT` HEAD (not the stale clone-time ref), and the `DeploymentPlan` records the final rendered branch name — plus the existing-branch-collision (local and remote-only), protected-branch, and real-fetch-failure variants as subtests. `-short` correctly skips it (shells out to real git). |
| Rollback boundary | Revert commits `32b7e30` (`Service.CreatePromotionBranch`/`RenderBranchName`/`IsProtectedBranch`/`RegisterPromotionBranch` + `newTempRepoWithRemote`/`callRecordingRunner` helpers), `05a53a8` (consolidated HU-005 E2E test), plus this batch's `tasks.md`/`apply-progress.md` mark commit. Phases 1-8 (`internal/exec`, `internal/config`, `internal/git` HU-001/HU-002/HU-003/HU-004 surface, `internal/prereq`, `internal/salesforce`, `deploydeck doctor`) remain fully buildable/testable alone; nothing in Phase 10+ exists yet to depend on Phase 9. |

## Deviations from Design (Phase 9)

1. **9.5 and 9.13 reached GREEN immediately, no separate production code beyond 9.3's minimal implementation**: `CreatePromotionBranch`'s minimal 9.3 GREEN already (a) resolves the base ref as the literal `"origin/"+target` STRING rather than a pre-resolved SHA, letting git itself resolve that ref at `checkout -b` execution time — always AFTER the mandatory fetch — and (b) returns early on a fetch error before any checkout runs. Both are the CORRECT shape by construction, not something that needed additional code once 9.3 was written correctly. This mirrors the established precedent from Phase 6 (6.13/6.14) and Phase 8 (8.13/8.14): a dedicated RED/GREEN pair exists to LOCK IN and empirically PROVE the property with a real test, not to drive new logic. Unlike those precedents, this batch went one step further for both: the production code was DELIBERATELY, TEMPORARILY mutated to reintroduce the exact bug each test guards against (a pre-fetch-captured stale SHA for 9.4/9.5; an ignored fetch error for 9.12/9.13), confirmed the test FAILS with a clear diagnostic, then reverted — directly addressing the Assertion Quality Rules' concern that a GREEN test against already-passing code might not be exercising the real code path at all.
2. **Collision detection (9.9) runs AFTER the fetch, not before**: `docs/HISTORIAS.md`'s task list ("Validar si la rama temporal ya existe local o remota") doesn't state an explicit order relative to the fetch, but the Test E2E section's own seed data implies it: a colliding branch can exist "en origin" and must be discovered — `origin/<branchName>` as a resolvable ref only exists locally AFTER a fetch retrieves it. Running the collision check after `fetchOrigin` (reusing the exact `Service.BranchExists` primitive HU-004 already proved) means a collision pushed to origin since the caller's last fetch is still caught, not just a collision that existed at clone time.
3. **The protected-branch guard (9.11) is a caller-facing pure predicate (`IsProtectedBranch`), not an internal refusal inside `CreatePromotionBranch`**: the spec's own scenario only requires the protected branch to NOT be modified directly — it does not require refusing to START the flow while on one (a user starting HU-005 while on `main` is normal: they are ABOUT to leave it for a new promotion branch). `CreatePromotionBranch` structurally satisfies non-modification already (base ref is always the explicit `origin/<target>`, never the caller's current HEAD — `git checkout -b` never writes to the branch it starts FROM). `IsProtectedBranch`/`ProtectedBranches` give `internal/app` (Phase 11) the config-driven detection to INFORM the user before they proceed, matching the shape `SandboxAuthWarning`/`UnauthenticatedSandboxWarning` established in Phase 8 (a warning-surfacing primitive, not a hard block, for a condition that is not itself unsafe).
4. **`ProtectedBranches`/`IsProtectedBranch` treat every `cfg.Branches` value as protected, not a separate dedicated config key**: neither `config.Config` nor `docs/ARQUITECTURA.md`'s example config define a distinct "protected branches" list — `cfg.Branches` (the environment→branch map HU-004's `ListDestinations`/`IsProductionBranch` already read) is the only existing config surface naming the branches this tool treats as promotion DESTINATIONS, which are exactly the branches that must never be modified directly (always promoted into via a temp branch). This mirrors HU-004's own `IsProductionBranch` precedent (Phase 8 Deviation #4) of deriving a policy predicate from `cfg.Branches` rather than inventing a new config field. Flagged here in case a future HU wants a dedicated `protectedBranches` config list independent of the environment map (e.g. to protect a branch that is not itself a promotion destination).
5. **`newTempRepoWithRemote` fixes "UAT" as its seeded advancing branch, rather than taking a branch name parameter**: every Phase 9 scenario (per `docs/HISTORIAS.md`'s own Test E2E wording, "origin/UAT con un HEAD inicial... el remoto avanza UAT") needs the exact same shape — a real `git clone` (not `newTempRepo`'s init+push) of a bare remote pre-seeded with `main` AND `UAT`, so the local clone's `origin/UAT` is a REAL, resolvable, but soon-to-be-stale ref at clone time. Parameterizing the branch name would add complexity with no batch scenario needing a different one; every test that needs OTHER branches (protected-branch's `main`, collision tests' custom names) creates them directly via `runGit`/`seedDir`, exactly like `newTempRepo`'s existing callers already do for branches beyond its own fixed `main`.

## Issues Found (Phase 9)

1. No blocking issues. Both mutation-verification exercises (stale-ref bug for 9.4/9.5, ignored-fetch-error bug for 9.12/9.13) failed exactly as expected on the first attempt and passed again immediately after reverting — no test-setup bugs discovered, unlike some prior batches (Phase 5's `.gitignore` dirty-tree oversight, Phase 8's `-run` pattern debugging).
2. All Phase 9 acceptance criteria (`specs/promotion-branch/spec.md` under `openspec/changes/foundation-mvp-git/specs/promotion-branch/spec.md`, and `docs/HISTORIAS.md` HU-005, including its consolidated `### Test E2E` scenario and all three named variants) are implemented and green.

## Completed Tasks (Phase 10: HU-006 `cherry-pick`)

- [x] 10.1 `[U]` RED: porcelain XY + numstat classification (`DU`/`UD`→ModifyDelete, `DD`→BothDeleted, binary `-`→Binary, else Text).
- [x] 10.2 GREEN: `internal/git/conflict.go` — `ConflictKind`, `ConflictFile{Path,Kind}`, `classifyConflictKind`, `ClassifyConflicts` (pure).
- [x] 10.3 `[U]` RED: `git status --porcelain -z` NUL-delimited parsing — paths with spaces/unicode split safely; rename source path consumed.
- [x] 10.4 GREEN: `parsePorcelainZ` (bytes.Split on NUL, XY+space+path, rename-skip) + `numstatIsBinary`.
- [x] 10.5 `[I]` RED: `RepoState{InProgress,CurrentSHA,Unmerged,SequencerRemaining}` assembled during a REAL multi-commit pick (populated `.git/sequencer/todo`).
- [x] 10.6 GREEN: `internal/git/service_repostate.go` — `Service.RepoState()` reconciled from `CHERRY_PICK_HEAD` + `.git/sequencer/todo` + `status --porcelain -z`; extended `RepoState` struct.
- [x] 10.7 `[I]` RED (AC1): contiguous selection applies in topo order via ONE sequencer-driven invocation; content == source; single cherry-pick process proven via `callRecordingRunner`.
- [x] 10.8 GREEN: `CherryPickRevisions` (range `<first>^..<last>` vs explicit ordered list) + `IsContiguousSelection` + `Service.CherryPick` (single invocation, never a Go loop).
- [x] 10.9 `[U]` RED (M2): cherry-pick + `--continue` carry `-c commit.gpgsign=false` (asserted via capturing runner + arg-order check).
- [x] 10.10 GREEN: `cherryPickArgs`/`continueArgs`/`skipArgs` prepend `gpgSignOff` before the subcommand.
- [x] 10.11 `[I]` RED (AC2): a conflicting commit stops the flow; conflicting files shown classified by type.
- [x] 10.12 GREEN: `CherryPick`/`pickOutcome` reconcile `RepoState` with classified `Unmerged` on conflict (exit 1 as DATA).
- [x] 10.13 `[I]` RED: modify/delete conflict offers keep (`git add`) or delete (`git rm`).
- [x] 10.14 GREEN: `Service.KeepConflictFile`/`DeleteConflictFile` (`internal/git/service_conflict_resolution.go`).
- [x] 10.15 `[I]` RED: binary conflict offers `git checkout --theirs`/`--ours`.
- [x] 10.16 GREEN: `Service.ResolveBinaryConflict(side)` (checkout `--theirs/--ours` then stage) + `ResolutionSide`/`SideTheirs`/`SideOurs`.
- [x] 10.17 `[U]` RED (AC3): unmerged/unstaged → disabled with pending detail; staged `<<<<<<<` → blocked naming the file; clean+no markers → enabled.
- [x] 10.18 GREEN: pure `EvaluateContinueGate(RepoState, markerFiles)` + `Service.StagedConflictMarkers` (`git diff --cached --check` parse) in `continue_gate.go`.
- [ ] 10.19 `[T]` RED (AC4 live re-poll) — DEFERRED to Phase 11 (`internal/app`/`tea.Tick`; engine substance covered by 10.23/10.24 reconciliation).
- [ ] 10.20 GREEN (`tea.Tick` re-poll in `internal/app`) — DEFERRED to Phase 11.
- [x] 10.21 `[I]` RED (AC5): once the gate passes, `git cherry-pick --continue` runs non-interactively (no hang).
- [x] 10.22 GREEN: `Service.ContinueCherryPick` wires gate → `ErrContinueBlocked` when unresolved, else runs `--continue`.
- [x] 10.23 `[I]` RED (AC6): external `--continue`/`--abort` during a real sequencer run is detected on re-read.
- [x] 10.24 GREEN: `RepoState()` re-reads from repo on EVERY call (no cached state), so external actions reconcile by construction.
- [x] 10.25 `[I]` RED (AC7a): confirmed abort runs `git cherry-pick --abort`.
- [x] 10.26 GREEN: `Service.AbortCherryPick` (`internal/git/service_abort.go`).
- [x] 10.27 `[I]` RED (AC7b): abort after partial picks offers temp-branch cleanup.
- [x] 10.28 GREEN: `Service.AppliedPickCount` (`rev-list --count base..HEAD`) + pure `OfferPartialBranchCleanup`.
- [x] 10.29 `[I]` RED (AC8): content already present under a different SHA → empty pick detected, `--skip` offered with a message.
- [x] 10.30 GREEN: `isEmptyPickMessage`/`emptyPickExplanation` (`empty.go`) + `Service.SkipCherryPick`; `pickOutcome` surfaces `Empty`+`EmptyMessage`.
- [x] 10.31 `[I]` RED (AC9): `git rerere` auto-resolves a repeat conflict — labeled auto-resolved-from-prior, still requires confirmation (unmerged until staged).
- [x] 10.32 GREEN: `rerereResolvedPaths`/`Service.RerereEnabled`/`SuggestEnableRerere` (`rerere.go`); `pickOutcome` sets `RerereResolved`.
- [x] 10.33 `[I]` RED (AC10): a touched file differing from source after all picks → per-file partial-promotion warning.
- [x] 10.34 GREEN: `Service.VerifyPromotedContent` (`git diff --name-only -z HEAD <source> -- <files>`) + `PickVerification` (`pick_verification.go`).
- [x] 10.35 `[U]` RED (AC11): a failed/in-progress/aborted run blocks delta + Salesforce validation.
- [x] 10.36 GREEN: pure `DeltaAndValidationAllowed(state, aborted)`.
- [ ] 10.37 `[U]` RED (`internal/app` never imports `internal/exec`, `go list -deps` boundary) — DEFERRED to Phase 11 (requires `internal/app`).
- [ ] 10.38 GREEN (route all `internal/app` git/sf calls through `git.Service`/`salesforce.Client`) — DEFERRED to Phase 11.
- [x] HU-006 Test E2E (required deliverable): `internal/git/cherry_pick_e2e_test.go` — `TestHU006_CherryPick_E2E` on `newTempRepo` (real git, no FakeRunner for the pick): clean promotion (both commits, content==source, topo order) plus 7 named variants (text/binary/modify-delete conflict lifecycle with external-resolve reconciliation, empty-pick `--skip`, mid-sequence abort with partial-branch cleanup, external-abort reconciliation, post-pick partial-promotion verification).

## TDD Cycle Evidence (Phase 10 — this batch)

| Task | Test File | Layer | Safety Net | RED | GREEN | TRIANGULATE | REFACTOR |
|------|-----------|-------|------------|-----|-------|-------------|----------|
| 10.1-10.4 | `internal/git/conflict_test.go` | Unit (pure) | ✅ 118/118 (whole module, pre-Phase-10) | ✅ Written (`undefined: ConflictFile`/`ConflictText` …) | ✅ Passed | ✅ 8 classify cases + spaced/unicode + rename-skip + 3 numstat cases | ➖ None needed |
| 10.5/10.6, 10.23/10.24 | `internal/git/service_repostate_test.go` | Integration (`newTempRepo`, real multi-commit pick) | ✅ (green pre-task) | ✅ Written (`svc.RepoState undefined`) | ✅ Passed | ✅ 4 scenarios (mid-pick assembly, external abort, external continue, clean repo) | ➖ None needed |
| 10.7/10.8 | `internal/git/service_cherrypick_test.go` | Integration (`callRecordingRunner`) + Unit (pure) | ✅ (green pre-task) | ✅ Written (`svc.CherryPick undefined`) | ✅ Passed | ✅ single-invocation + range-form + 4 `CherryPickRevisions` + 5 `IsContiguousSelection` cases | ➖ None needed |
| 10.9/10.10 | `internal/git/service_cherrypick_test.go` | Unit (`capturingRunner`) | ✅ (green pre-task) | ✅ Written (`-c commit.gpgsign=false` absent) | ✅ Passed | ✅ cherry-pick + `--continue` + arg-order-before-subcommand | ➖ None needed |
| 10.11-10.16 | `internal/git/service_conflict_resolution_test.go` | Integration (`newTempRepo`, real conflicts) | ✅ (green pre-task) | ✅ Written (`svc.KeepConflictFile`/`ResolveBinaryConflict` undefined) | ✅ Passed | ✅ text stop+classify, modify/delete keep AND delete, binary theirs (content-verified) | ➖ None needed |
| 10.17/10.18, 10.21/10.22 | `internal/git/continue_gate_test.go`, `service_continue_test.go` | Unit (pure) + Integration | ✅ (green pre-task) | ✅ Written (`undefined: EvaluateContinueGate`; `ErrContinueBlocked`) | ✅ Passed | ✅ 4 gate cases + marker parse (spaced path) + full blocked→marker-blocked→continue lifecycle | ➖ None needed |
| 10.25-10.30 | `internal/git/service_abort_test.go`, `empty_test.go` | Integration + Unit (pure) | ✅ (green pre-task) | ✅ Written (`svc.AbortCherryPick`/`SkipCherryPick`/`AppliedPickCount` undefined) | ✅ Passed | ✅ abort-to-clean, mid-sequence partial-cleanup offer, empty-detect+skip-completes, `isEmptyPickMessage` + `OfferPartialBranchCleanup` cases | ➖ None needed |
| 10.31/10.32 | `internal/git/rerere_test.go`, `service_rerere_test.go` | Unit (pure) + Integration (record→reset→replay) | ✅ (green pre-task) | ✅ Written (`svc.RerereEnabled` undefined; `rerereResolvedPaths` undefined) | ✅ Passed | ✅ 3 parse cases + suggestion + real rerere replay leaving file unmerged-pending-confirmation | ➖ None needed |
| 10.33-10.36 | `internal/git/pick_verification_test.go`, `service_verification_test.go` | Unit (pure) + Integration | ✅ (green pre-task) | ✅ Written (`svc.VerifyPromotedContent` undefined; `DeltaAndValidationAllowed` undefined) | ✅ Passed | ✅ partial-promotion detect + clean-matches-source + 3 gating cases + warnings | ➖ None needed |
| HU-006 Test E2E | `internal/git/cherry_pick_e2e_test.go` (`TestHU006_CherryPick_E2E`) | Integration (`newTempRepo`, real git, full flow) | ✅ (green pre-task) | ➖ N/A — composes only already-GREEN production code; first real-git run is the empirical proof | ✅ Passed on first run | ✅ primary + 7 named subtests | ➖ None needed |

### Test Summary (Phase 10)

- **Total new top-level test functions this batch**: 21 (across `internal/git`); the whole module `go test -race ./...` is green (`internal/git` runs ~40s under `-race`).
- **`go test -race ./...`**: all packages `ok`, no data races.
- **`go test -short ./...`**: every new real-git integration test skips via the shared `newTempRepo`/`newTempRepoWithRemote` harness's `testing.Short()` guard; the pure classification/gate/rerere/verification/arg-builder unit tests still run.
- **Layers used**: Unit (porcelain `-z` classification, numstat binary marker, continue-gate, `git diff --check` marker parse, empty/rerere/name-only parsers, revision-form + contiguity selection, gpgsign arg builders, downstream gating), Integration (real cherry-pick sequence conflict/continue/skip/abort/reconcile/verify via `newTempRepo`), plus the consolidated HU-006 E2E.
- **Pure functions/types created**: `ConflictKind`/`ConflictFile`/`ClassifyConflicts`/`parsePorcelainZ`/`numstatIsBinary`, `CherryPickRevisions`/`IsContiguousSelection`, `EvaluateContinueGate`/`parseCheckMarkers`, `isEmptyPickMessage`/`OfferPartialBranchCleanup`, `rerereResolvedPaths`/`SuggestEnableRerere`, `PickVerification`/`DeltaAndValidationAllowed`; composed (Runner-backed): `Service.RepoState`/`CherryPick`/`ContinueCherryPick`/`SkipCherryPick`/`AbortCherryPick`/`KeepConflictFile`/`DeleteConflictFile`/`ResolveBinaryConflict`/`StagedConflictMarkers`/`AppliedPickCount`/`RerereEnabled`/`VerifyPromotedContent`; error `ErrContinueBlocked`.

## Work Unit Evidence (Phase 10)

| Evidence | Value |
|---|---|
| Focused test command and exact result | `go test ./internal/git/... -run 'TestClassifyConflicts\|TestService_RepoState\|TestService_CherryPick\|TestCherryPickRevisions\|TestEvaluateContinueGate\|TestService_ContinueCherryPick\|TestService_AbortCherryPick\|TestService_EmptyPick\|TestService_CherryPick_Rerere\|TestService_VerifyPromotedContent\|TestHU006_CherryPick_E2E'` → `ok`. |
| Runtime harness command/scenario and exact result | `go test ./internal/git/... -run TestHU006_CherryPick_E2E -v` → PASS (8 subtests): real temp git repo (`newTempRepo`), real multi-commit cherry-pick driven through pick → conflict (text/binary/modify-delete, classified) → external resolve reconciled on re-read → gated non-interactive `--continue` → completion; empty-pick `--skip`; mid-sequence abort with partial-branch cleanup offer; external-abort reconciliation; post-pick partial-promotion verification. `-short` skips it (real git). |
| Rollback boundary | Revert commits `<conflict classify>`, `<RepoState reconcile>`, `<sequencer engine+gpgsign>`, `<conflict-stop+resolution>`, `<continue-gate>`, `<abort+partial+empty>`, `<rerere>`, `<post-pick verify+gating>`, `<E2E>`, plus the `tasks.md`/`apply-progress.md` mark commit. Phases 1-9 remain fully buildable/testable alone; `internal/app` (Phase 11) does not exist yet, so nothing depends on the Phase 10 engine. |

## Deviations from Design (Phase 10)

1. **10.19/10.20 (`tea.Tick` live re-poll) and 10.37/10.38 (`internal/app` `go list -deps` dependency-boundary test) are DEFERRED to Phase 11**: all four tasks are physically located in `internal/app`, which the orchestrator explicitly instructed this batch NOT to start ("Do NOT start Phase 11 (`internal/app` wiring)"). AC4's ENGINE substance — the repo is re-read from disk on every `RepoState()` call so an external resolution/`--continue`/`--abort` is reflected automatically — is fully implemented and tested here (10.23/10.24, and the E2E "external resolve reconciled on re-read" + "external abort reconciled on re-read" subtests). The remaining piece is purely the TUI timer (`tea.Tick`) that periodically CALLS `RepoState()`, plus the dependency-direction boundary test, both of which require the `internal/app` package to exist. Flagged so Phase 11 picks them up (11.6 already references extending the 10.37 boundary test).
2. **Binary-conflict detection uses `git diff --numstat :2:<path> :3:<path>` (stage-2 vs stage-3 blobs), not a plain `git diff --numstat`**: design/tasks say "binary via numstat `-`". Verified empirically against git 2.50.1: during a conflict, a plain `git diff --numstat` reports `0\t0\t<path>` for a binary file (worktree-vs-unmerged-index is ambiguous), NOT the `-\t-` sentinel. Diffing the two conflict stages directly (`:2:`/`:3:`) DOES yield `-\t-` for binary and real counts for text. The pure `numstatIsBinary` parser is unchanged (it interprets the `-` sentinel exactly as specced); only the command that FEEDS it was chosen to be the per-file stage diff, run only for both-sides-present codes (`UU`/`AA`). Modify/delete (`DU`/`UD`) and both-deleted (`DD`) are classified by XY code alone and never probed. Documented in `binaryConflictPaths`.
3. **Empty-pick is surfaced as `Empty`+`EmptyMessage` (offer model), not silently auto-`--skip`-ed inside `CherryPick`**: the batch scope says "auto-`--skip` it rather than erroring" and the spec AC8 says "offer `--skip` with a clear explanatory message". These are reconciled by: `CherryPick` NEVER errors on an empty pick (returns it as DATA with `Empty=true`), and exposes `Service.SkipCherryPick` as the action. The engine leaves the actual skip decision to the caller (`internal/app`, Phase 11) so the user is INFORMED (spec's "offer") — while the "rather than erroring" requirement is satisfied because empty is a clean outcome, never a Go error. Both halves are proven in `TestService_EmptyPick_DetectedAndSkipped` and the E2E empty subtest.
4. **`RepoState.SequencerRemaining` counts ALL actionable `.git/sequencer/todo` lines, including the currently-conflicting commit**: verified against git 2.50.1 that on a conflict at commit N of M, `todo` still lists commit N plus the M−N after it. Reconciliation (AC6) only needs the count to change on external actions and drop to 0 on abort/completion, which holds; the integration tests assert `>=1` during a conflict and `==0` after abort/completion rather than a brittle exact mid-sequence count, so the behavior is version-robust. Documented on the field.
5. **`DeltaAndValidationAllowed(state, aborted bool)` takes an explicit `aborted` flag**: a successful `--abort` leaves `InProgress=false` (indistinguishable from a clean completion by repo state alone), but an aborted run must still block downstream delta/validation. Rather than invent a run-status enum in `internal/git` (that lifecycle belongs to `internal/app`'s state machine, Phase 11), the pure predicate takes the caller's known `aborted` boolean. Flagged for Phase 11 to pass its real run phase.
6. **Conflict-resolution + engine primitives live in `internal/git` as `Service` methods** (`CherryPick`, `ContinueCherryPick`, `KeepConflictFile`, `ResolveBinaryConflict`, …): consistent with every prior HU's "one domain, one package, grows without a cycle" precedent. `internal/app` (Phase 11) will compose these; none of them import or know about the TUI.

## Issues Found (Phase 10)

1. No blocking issues. Every Phase-10 acceptance criterion implementable at the `internal/git` engine layer (`specs/cherry-pick/spec.md` and `docs/HISTORIAS.md` HU-006, including its consolidated `### Test E2E` and all named variants) is implemented and green under `-race`; `gofmt -l .` is clean (the pre-existing Phase-7 `dependency_warning_test.go` nit flagged in the Phase-8 notes is no longer reported).
2. The only unimplemented HU-006 tasks (10.19/10.20, 10.37/10.38) are the `internal/app`/TUI-layer ones, deferred per the explicit "do not start Phase 11" instruction — see Deviation 1. HU-006's reopen-and-resume AC (`docs/HISTORIAS.md:395`) remains owned by HU-013 per the spec note, out of this change's scope.

## HU-006 Remediation Batch (adversarial review of the cherry-pick engine)

An adversarial review of the HU-006 `cherry-pick` engine found 3 real engine holes (one with
data-loss risk). All three were fixed under strict TDD (RED reproducing the real bug on a real temp
git repo FIRST, then minimal GREEN). The already-sound parts were NOT regressed: single
sequencer-driven invocation, the continue-gate, repo-as-source-of-truth reconciliation, conflict
classification, skip mechanics, and abort all remain green (full `TestHU006_CherryPick_E2E` and its
7 named variants pass unchanged).

### H1 (MEDIUM, DATA-LOSS) — empty-pick detection was a spoofable substring match

- **Bug**: `isEmptyPickMessage` (`empty.go`) did `strings.Contains(output, "is now empty")`. A
  successful pick echoes each applied commit's SUBJECT to stdout, and a conflicting pick echoes it in
  the `could not apply <sha> <subject>` line, so a commit whose subject contains "is now empty" set
  `Empty=true` on a NORMAL or CONFLICTING pick. Worst case: `Empty=true` co-occurred with a real
  unresolved conflict, and an Empty-first consumer would `--skip` and SILENTLY DROP the conflicting
  selected commit. The SAME CLASS existed in `rerere.go`: the un-anchored regex over combined echoed
  output false-flagged a commit whose subject mirrored `Resolved '<path>' using previous resolution.`.
- **Fix**: Empty is now derived from reconciled REPO STATE — `isEmptyPickState(state)` = a pick in
  progress (`CHERRY_PICK_HEAD` set) AND clean tree AND zero unmerged paths. The rerere parser now
  line-anchors the match (`(?m)^Resolved …`) AND reads only git's STDERR diagnostic (the subject echo
  is on stdout, always `[branch sha] `-prefixed), so an echoed subject can never trigger it.
- **RED (real git)**, all watched failing against the old code first:
  (a) `TestHU006_EmptyDetection_SpoofSubjectCleanApply_NotEmpty` — subject contains "is now empty",
      applies cleanly → old: `Empty=true`; new: `Empty=false`.
  (b) `TestHU006_EmptyDetection_SpoofSubjectConflict_NotEmptySurfacesConflict` — same subject on a
      CONFLICTING pick → old: `Empty=true` alongside `Unmerged=1` (the data-loss co-occurrence);
      new: `Empty=false` AND the a.cls conflict surfaced.
  (c) `TestHU006_EmptyDetection_GenuinelyAlreadyApplied_IsEmpty` — genuine already-applied commit →
      still `Empty=true` (no regression).
  (d) `TestHU006_Rerere_SpoofSubject_NotFalseFlaggedAsAutoResolved` — subject echoing git's rerere
      line, clean apply → old: `RerereResolved=[evil.cls]`; new: `[]`.
- **Commit**: `a9ceeb8` fix(git): detect empty cherry-pick from repo state not echoed output.

### H2 (MEDIUM, latent) — range form `A^..B` did not verify its precondition

- **Bug**: `CherryPickRevisions` used the ancestry range `commits[0].SHA^..commits[last].SHA` for
  "contiguous" selections, which includes EVERY commit between A and B in the DAG. Safety rested
  entirely on the caller passing a complete gap-free list to `IsContiguousSelection`; a mis-wire could
  silently promote an unselected interleaved commit.
- **Fix**: the range form is now DEFENSIVE. For a multi-commit contiguous selection, `cherryPickRevs`
  verifies via `git rev-list --reverse <A>^..<B>` that the range commit set equals EXACTLY the selected
  SHA set before using it; on any mismatch (or if the range cannot be listed, e.g. a root commit) it
  falls back to the explicit ordered SHA list, which is unconditionally correct. Single/non-contiguous
  selections are unaffected.
- **RED (real git)**: `TestHU006_CherryPick_RangeGuard_DoesNotPromoteInterleavedUnselected` — history
  `base ─ A(selected) ─ X(unselected) ─ B(selected)`, cherry-picking `[A,B]` → old: `x.cls` promoted
  (watched failing); new: `x.cls` absent, A and B present.
  `TestHU006_CherryPick_RangeGuard_KeepsRangeFormWhenProvablySafe` proves the guard does not over-fire
  on a genuinely contiguous selection; the existing single-invocation test still asserts the
  `<first>^..<last>` range form is used.
- **Commit**: `d342d25` fix(git): guard cherry-pick range against unselected commits.

### H3 (LOW-MEDIUM) — post-pick verification was weak (false-pass + false-fail)

- **Bug**: `VerifyPromotedContent` did `git diff --name-only -z HEAD <source> -- <touchedFiles>`:
  tip-relative, name-only, scoped to caller-supplied files. It FALSE-PASSED on an extra file dragged
  in by a wrong pick (not in `touchedFiles` → never checked) and FALSE-FAILED on an intentional
  non-contiguous subset promotion (files legitimately differ from later, deliberately-excluded commits
  on the source tip). Both were reproduced on real git against the exact old command before the fix.
- **Fix**: verification now runs two checks. (a) SPURIOUS: any file changed between the pre-pick base
  (`origin/<target>`) and HEAD that the selected commit set never touched is flagged — closing the
  false-pass. (b) PARTIAL: selected files are compared against the LAST SELECTED COMMIT (the intended
  selected tip), not the source branch tip — closing the false-fail for subset promotions.
  `PickVerification` now carries `SpuriousFiles` alongside `PartialFiles`; `OK()`/`Warnings()` cover
  both. The residual content-equality limitation of the partial check is documented on the function.
- **Signature change** (no production callers yet — only tests): `VerifyPromotedContent(ctx, dir,
  base, selectedTip string, selectedFiles []string)`. The 4 existing call sites were updated to
  `origin/UAT` + `feature2` (last selected) and their intent (clean == source; divergent resolution
  flagged) is preserved.
- **RED**: `TestService_VerifyPromotedContent_DetectsSpuriousExtraFile` (extra `x.cls` → flagged
  spurious) and `TestService_VerifyPromotedContent_IntentionalSubset_NoFalsePartial` (promote only A
  whose file is re-edited by excluded C → NOT flagged partial). Both bugs were first demonstrated
  failing against the old `git diff HEAD <source> -- <files>` command on a scratch real-git repo.
- **Commit**: `b7e23be` fix(git): strengthen post-pick verification against spurious files.

### Remediation TDD Cycle Evidence

| Hole | Test file (new) | Layer | RED (reproduced real bug) | GREEN | Triangulation |
|------|-----------------|-------|---------------------------|-------|---------------|
| H1 empty | `empty_detection_e2e_test.go` | Integration (real git) | ✅ (a)/(b) watched fail: `Empty=true` on clean spoof and alongside a real conflict | ✅ state-based `isEmptyPickState` | ✅ (c) genuine-empty guard + (d) rerere spoof + pure `TestIsEmptyPickState` (4 cases) |
| H1 rerere | `empty_detection_e2e_test.go` + `rerere_test.go` | Integration + Unit (pure) | ✅ (d) watched fail: `RerereResolved=[evil.cls]` | ✅ stderr-only + `(?m)^` anchor | ✅ pure `TestRerereResolvedPaths` new echoed-subject case; real rerere replay still detected |
| H2 range | `service_cherrypick_range_guard_test.go` | Integration (real git) | ✅ watched fail: interleaved `x.cls` promoted | ✅ `rev-list` set-equality guard + explicit-list fallback | ✅ provably-safe range still used; existing single-invocation range-form test green |
| H3 verify | `pick_verification_hardening_test.go` | Integration (real git) | ✅ both false-pass and false-fail reproduced on real git vs the old command | ✅ base-vs-HEAD spurious + selected-tip partial | ✅ existing partial/clean tests re-pass on new signature; pure `TestPickVerification_Warnings` spurious cases |

### Remediation Work Unit Evidence

| Evidence | Value |
|---|---|
| Focused test command and exact result | `go test ./internal/git/ -run 'TestHU006_EmptyDetection\|TestHU006_Rerere_SpoofSubject\|TestHU006_CherryPick_RangeGuard\|TestService_VerifyPromotedContent\|TestPickVerification\|TestIsEmptyPickState\|TestRerereResolvedPaths\|TestHU006_CherryPick_E2E' -count=1` → `ok`. |
| Runtime harness command/scenario and exact result | `go test -race ./...` → all packages `ok` (`internal/git` ~45s), no data races. Real temp git repos (`newTempRepo`) drive every remediation RED/GREEN through actual cherry-pick / conflict / rev-list / diff behavior; `-short` skips them via the shared harness guard. |
| Rollback boundary | Revert commits `a9ceeb8` (H1), `d342d25` (H2), `b7e23be` (H3) independently; each is self-contained (its own new test file + the narrow production edit) and the Phase-10 engine remains buildable/green after any subset. `internal/app` (Phase 11) does not exist yet, so nothing external depends on the changed signatures. |

### Remediation Deviations / Notes

1. **H1 empty signal moved from output parsing to repo state**: `isEmptyPickMessage(string)` was replaced by
   `isEmptyPickState(RepoState)`. This is strictly more correct (state cannot be spoofed by echoed text) and
   the genuine already-applied case is still detected (guard test (c) + the existing empty-pick E2E subtest).
2. **H1 rerere kept as a pure parser but scoped to stderr + line-anchored**: the task's preferred "real rerere
   state" and the accepted "at minimum scope the parse" were reconciled by reading only git's stderr diagnostic
   (empirically confirmed stream) and anchoring to line start — robust against the subject echo without an extra
   `git rerere` round-trip. Reconnaissance confirmed that on a rerere replay `git rerere status`/`remaining` are
   both empty while the path stays `UU`, so a `status`/`remaining` subtraction would have been fragile; the
   stderr+anchor parse is the cleaner, version-robust choice.
3. **H2 fallback is fail-safe**: any `rev-list` failure (not just a set mismatch) falls back to the explicit
   ordered SHA list — the guard never trusts a range it cannot prove safe.
4. **H3 signature changed** from `(dir, source, touchedFiles)` to `(dir, base, selectedTip, selectedFiles)`. It
   has no production callers yet (only tests), so the change is contained; a residual whole-blob content-equality
   limitation of the partial check is documented honestly on the function.
5. **Phase 11 NOT started** per instruction. This batch touched only the `internal/git` engine and its tests.

### Remediation Final Verification (verbatim)

```
$ go build ./... && go vet ./... && gofmt -l . && go test -race ./...
BUILD_OK
VET_OK
--- gofmt -l . (empty = clean) ---
--- gofmt done ---
ok  	deploydeck/cmd/deploydeck	5.525s
ok  	deploydeck/internal/config	(cached)
ok  	deploydeck/internal/exec	(cached)
ok  	deploydeck/internal/git	45.519s
ok  	deploydeck/internal/prereq	3.046s
ok  	deploydeck/internal/salesforce	(cached)
```

`gofmt -l .` printed nothing (clean); `go build`/`go vet` succeeded; `go test -race ./...` reported every
package `ok` with no data races.

## Status

**Phase 10 (HU-006 `cherry-pick`) engine complete under strict TDD — 34/38 Phase-10 tasks done; the 4 remaining
(10.19/10.20, 10.37/10.38) are `internal/app`/TUI tasks deferred to Phase 11 per the explicit "do not start
Phase 11" instruction.** The engine is fully in `internal/git`: `Service.CherryPick` issues ONE sequencer-driven
invocation (range `<first>^..<last>` for contiguous ancestry, explicit ordered SHA list otherwise — never a Go
loop of single-sha picks), carrying `-c commit.gpgsign=false` on its own commit-creating cherry-pick/`--continue`/`--skip`;
`Service.RepoState` reconciles `InProgress`/`CurrentSHA`/`Unmerged`(classified text/binary/modify-delete/both-deleted)/`SequencerRemaining`
from `CHERRY_PICK_HEAD` + `.git/sequencer/todo` + `git status --porcelain -z` on EVERY call (so external
`--continue`/`--abort` reconcile by construction); the pure continue-gate blocks on unmerged paths or staged
`<<<<<<<` markers and `ContinueCherryPick` refuses with `ErrContinueBlocked` until clean; modify/delete keep/delete
and binary theirs/ours resolution primitives; empty-pick detection + `--skip` safety-net; rerere auto-resolution
detection requiring confirmation; mid-sequence abort with a partial-branch-cleanup offer; and post-pick verification
(`git diff --name-only -z HEAD <source>`) surfacing per-file partial-promotion warnings before any delta step, with
`DeltaAndValidationAllowed` gating delta/validation off on any failed/in-progress/aborted run. All proved end-to-end
via the consolidated HU-006 Test E2E on the real `newTempRepo` harness (no FakeRunner for the pick) with seeded real
conflicts. `go build ./...`, `go vet ./...`, `gofmt -l .`, and `go test -race ./...` all green; `go test -short ./...`
skips the real-git integration tests. Ready for Phase 11 (`internal/app` wiring), which also picks up 10.19/10.20/10.37/10.38.

---

### Historical Status (Phases 1-9)

123/123 tasks in scope (Phases 1-9) complete (170 total tasks in `tasks.md`; 47 remain across Phases 10-12).
Phase 9 (HU-005 `promotion-branch`) complete under strict TDD: `Service.CreatePromotionBranch` always runs
`git fetch origin` before creating the branch and resolves the base ref as the literal `"origin/<target>"`
string (never a pre-fetch SHA), so a remote that advanced after the local clone/last fetch is followed
exactly — the critical correctness property this HU exists for, empirically confirmed via deliberate
mutation testing on both this property and the fetch-failure short-circuit; branch-name templating via a
literal `strings.NewReplacer` substitution sharing Phase 3's exact token allow-list (`RenderBranchName`);
existing local/remote branch names block creation with `ErrPromotionBranchExists` instead of overwriting;
a config-driven protected-branch predicate (`IsProtectedBranch`) lets the caller detect and inform the user,
while non-modification of the current branch is structurally guaranteed by construction; and a successful
creation's final branch name is persisted into `DeploymentPlan` (`RegisterPromotionBranch`) — all proved
end-to-end via the consolidated HU-005 Test E2E scenario from `docs/HISTORIAS.md`, run through the
documented two-repo bare-remote harness (`newTempRepoWithRemote`), plus its existing-branch,
protected-branch, and fetch-failure variants. `go build ./...`, `go vet ./...`, `gofmt -l .`, and
`go test -race ./...` all green; `go test -short ./...` correctly skips the real-git integration tests.
Ready for next batch (Phase 10: HU-006 `cherry-pick`).

---

## Completed Tasks (Phase 11: `internal/app` wiring + deferred Phase-10 TUI tasks)

- [x] 10.19 `[T]` RED (AC4 live re-poll): external resolution reflected on the next re-poll (`repoStateMsg` reconciliation).
- [x] 10.20 GREEN: `tea.Tick`-driven `RepoState` re-poll wired in `internal/app` (`onTick` polls ONLY during the cherry-pick screens; the Model derives UI from the repo, never execs).
- [x] 10.37 `[U]` RED (seam invariant): `internal/app/boundary_test.go` — the package's own (non-test) imports exclude `os/exec` and `deploydeck/internal/exec`.
- [x] 10.38 GREEN: every git/sf call in `internal/app` routes through `git.Service`/`salesforce.Client`; the interactive `$EDITOR` handoff (`tea.ExecProcess`) is injected via `Deps.Edit` from `main` (which may import `os/exec`), keeping `internal/app` exec-free.
- [x] 11.1 `Model`/`New(deps)` compose the Phase 5–10 services per the state-machine diagram (`PrereqCheck → … → PickVerification`, `PickVerification → CommitSelection` on partial promotion via the `e` key).
- [x] 11.2 `[T]` RED: direct `Model.Update` drives `PrereqCheck → TicketInput` on OK prereqs.
- [x] 11.3 GREEN: transition implemented (`onPrereqDone`; blocking checks keep the doctor screen, warnings pass with `c`).
- [x] 11.4 `[T]` RED: full-flow smoke test `PrereqCheck → PickVerification` on a real temp repo.
- [x] 11.5 GREEN: remaining `Update`/`View` wiring (ticket input, discovery, selection, target, plan preview, branch creation, cherry-pick, conflict, verification).
- [x] 11.6 The 10.37 boundary test covers the ENTIRE `internal/app` package (single package; `build.ImportDir` inspects all its non-test files).
- [x] Main entry: `deploydeck` with no subcommand launches the Bubble Tea flow; `doctor` subcommand unchanged; real `NewOSRunner`-backed services composed in `main`, fakes in tests.
- [x] 12.1 `go test ./...` green (unit + integration). 12.2 `go vet ./...` + `gofmt -l .` clean. (12.3 proposal Success-Criteria check-off left for the orchestrator's dedicated final pass.)

## TDD Cycle Evidence (Phase 11 — this batch)

| Task | Test File | Layer | Safety Net | RED | GREEN | TRIANGULATE | REFACTOR |
|------|-----------|-------|------------|-----|-------|-------------|----------|
| 10.37/10.38, 11.6 | `internal/app/boundary_test.go` | Unit (`go/build` direct-import inspection) | N/A (new pkg) | ✅ Written first: `build.ImportDir` reports empty imports → "must compose internal/git" fails | ✅ Passed once `app.go` composes the services with clean imports | ✅ **Mutation-verified**: adding a direct `import _ "os/exec"` makes the test FAIL (caught), removing it passes | ➖ None needed |
| 11.2/11.3 | `internal/app/prereq_test.go` | Unit (direct `Model.Update`) | N/A (new) | ✅ Written | ✅ Passed | ✅ 2 cases (all-OK → TicketInput / blocking → stays) + error-path + `c`-key gate | ➖ None needed |
| 10.19/10.20 | `internal/app/repoll_test.go` | Unit (direct `Model.Update`) | N/A (new) | ✅ Written | ✅ Passed | ✅ external-resolution reflected + re-conflict disables again + external-completion reconciles + tick polls ONLY during cherry-pick; **mutation-verified** (dropping the gate reflection fails 10.19) | ➖ None needed |
| 10.35/10.36 wiring | `internal/app/delta_gate_test.go` | Unit (direct `Model.Update`) | N/A (new) | ✅ Written | ✅ Passed | ✅ 3 cases (clean→allow / aborted-clean→deny / mid-conflict→deny) + abort terminal + verify-done; **mutation-verified** (passing `false` instead of `m.aborted` fails the abort case) | ➖ None needed |
| 11.5 | `internal/app/transitions_test.go` | Unit (direct `Model.Update` + `View`) | N/A (new) | ✅ Written | ✅ Passed | ✅ ticket→discovery, discovery→selection (merge disabled), selection confirm/empty-block, target mapped/unmapped, plan→branch, 4 View screens | ➖ None needed |
| 11.4 | `internal/app/flow_e2e_test.go` | Integration (real git on a bare-remote clone) | N/A (new) | ➖ N/A — composes only already-GREEN production; first real-git run is the proof | ✅ Passed on first run | ➖ Single full-flow scenario (PrereqCheck→PickVerification), asserts `Verification().OK()` + `DeltaAllowed()` + plan fields | ➖ None needed |
| Main entry | `cmd/deploydeck/root_test.go` | Unit (Cobra routing, injected `RunTUI`) | ✅ 4/4 (pre-existing root tests) | ✅ Written (`unknown field RunTUI`) | ✅ Passed | ✅ 2 cases (bare `deploydeck` launches TUI / `doctor` does NOT) | ➖ None needed |

### Test Summary (Phase 11)

- **New top-level test functions this batch**: 13 (`internal/app`: `TestApp_NeverImportsExecSeam`, `TestModel_PrereqCheck_To_TicketInput`, `TestModel_PrereqCheck_Error`, `TestModel_PrereqScreen_ContinueBlockedByBlocker`, `TestModel_Repoll_ReflectsExternalResolution`, `TestModel_Repoll_ReconcilesExternalCompletion`, `TestModel_Tick_PollsOnlyDuringCherryPick`, `TestModel_DeltaAllowed_ThreadsAbortedFlag`, `TestModel_Abort_SetsAbortedTerminal`, `TestModel_VerifyDone_ComputesDeltaAllowed`, `TestModel_TicketInput_To_Discovery`, `TestModel_Discovery_To_Selection`, `TestModel_Selection_Confirm_And_EmptyBlock`, `TestModel_Target_Resolve`, `TestModel_PlanPreview_To_BranchCreation`, `TestModel_View_RendersScreens`, `TestHU_FullFlow_PrereqToPickVerification`) + 2 in `cmd/deploydeck`.
- **`go test -race ./...`**: all packages `ok`, no data races.
- **`go test -short ./...`**: all packages `ok`; the `internal/app` full-flow integration test correctly skips (shells out to real git); every direct-`Model.Update` unit test still runs.
- **Layers used**: Unit (direct `Model.Update` for every state transition, `go/build` import inspection for the boundary invariant, `View` substring assertions), Integration (the full PrereqCheck→PickVerification flow driving the real git service against a bare-remote clone).
- **Approval tests** (refactoring): None — Phase 11 is exclusively new code composing the already-GREEN Phase 5–10 surface; nothing existing was refactored.
- **Mutation-verified assertions**: 2 (the exec-boundary invariant catches a direct `os/exec` import; the `aborted`-flag threading catches passing a constant `false`) plus the re-poll gate reflection — each was deliberately broken, confirmed FAILING, then reverted, per the Assertion Quality Rules.
- **Pure helpers created**: `preliminaryTarget`, `sourceRefName`, `resolveSource`, `selectedCommits`, `selectedSHASet`, `destinationIndex`, `prereqHasBlocking`, `conflictMark`/`selectionMark`/`statusMark` (view formatters).

## Work Unit Evidence (Phase 11)

| Evidence | Value |
|---|---|
| Focused test command and exact result | `go test ./internal/app/ ./cmd/deploydeck/` → both `ok`. `go test ./internal/app/ -run TestApp_NeverImportsExecSeam` → `ok` (and FAILS under a mutated direct `os/exec` import, reverted). |
| Runtime harness command/scenario and exact result | `go test ./internal/app/ -run TestHU_FullFlow_PrereqToPickVerification -v` → PASS: a real bare-remote clone (origin/UAT + origin/feature/PROJ-1 with two ticket commits touching distinct files) drives the model through PrereqCheck→TicketInput→Discovery (real `git log`/`branch`/`rev-list`)→Selection→TargetSelection→PlanPreview→BranchCreation (real `git fetch`+`checkout -b`)→CherryPicking (real sequencer cherry-pick)→PickVerification, asserting `Verification().OK()`, `DeltaAllowed()`, and the registered promotion branch. `-short` correctly skips it. |
| Rollback boundary | Revert commits `046aa16` (app scaffold + boundary invariant + bubbletea dep), `8e1591b` (state-machine `Update`/`View`/keys + tests), `e317330` (CLI main entry launches TUI), plus the `tasks.md`/`apply-progress.md` mark commit. Phases 1–10 (`internal/git` engine, `internal/prereq`, `internal/salesforce`, `internal/config`, `deploydeck doctor`) remain fully buildable/testable alone; removing `internal/app` + the root `RunE` restores the pre-Phase-11 CLI (doctor-only) exactly. |

## Deviations from Design (Phase 11)

1. **The exec-boundary test uses `go/build` direct-import inspection, NOT `go list -deps`**: task 10.37 names `go list -deps` "or equivalent". A literal `-deps` scan lists the FULL TRANSITIVE closure, which necessarily includes `os/exec` (bubbletea's `tea.ExecProcess` pulls it in) — so a transitive "os/exec absent" assertion is impossible and wrong. The design explicitly ALLOWS `tea.ExecProcess` for interactive handoff; the invariant it actually constrains is that `internal/app`'s OWN files never exec. `go/build.ImportDir(".").Imports` reports exactly those direct (non-test) imports, is stdlib (no subprocess, no `x/tools` dep), and is mutation-verified to catch a direct `os/exec` import. Documented in the test.
2. **The `$EDITOR`/mergetool interactive handoff is INJECTED via `Deps.Edit` from `main`, not built inside `internal/app`**: `tea.ExecProcess` requires constructing an `*os/exec.Cmd`, which would force `internal/app` to import `os/exec` and break the boundary invariant. The two design requirements ("`tea.ExecProcess` ALLOWED for interactive handoff" AND "`internal/app` must not import `os/exec`") are reconciled by having `main` (which may import `os/exec`) build the `tea.Cmd` and inject it as `Deps.Edit func(path string) tea.Cmd`; `internal/app`'s conflict screen calls it (nil-safe) on the `e` key. This preserves both the architecture invariant AND the handoff affordance. The primary conflict UX remains external-resolution + auto-detect re-poll, exactly as the mockup states ("Resuelve con tu herramienta preferida … esta pantalla detecta la resolucion automaticamente").
3. **A PRELIMINARY target drives discovery/classification before the target-selection screen**: HU-002 `Discover` needs a target to compute the `origin/<target>..origin/<source>` range and equivalence classification that HU-003 selection consumes, but the state machine orders `CommitSelection → TargetSelection`. Resolved exactly as the mockup shows ("Destino preliminar" during selection): `preliminaryTarget(cfg)` = the first `ListDestinations` branch (sorted by env key, deterministic) is used for discovery, and `TargetSelection` defaults its cursor to that same branch. If the user changes the target on the selection screen, the equivalence classification is not re-run for the new target in this MVP slice — a documented limitation (the happy path confirms the preliminary target; re-running discovery on a target change is a future enhancement).
4. **Source resolution runs discovery twice through the service**: `resolveSource` needs the candidate branches from a first `Discover(ticket-only)` pass before it can pick the single source; a second `Discover(ticket, source, target)` pass then produces the classified `OrderedCommits`. Both calls go through `git.Service` (app stays exec-free); the cost is two `git log`/`branch`/`rev-list` rounds per ticket, acceptable for an interactive TUI.
5. **Empty picks auto-`--skip` in the model loop**: `onPickDone` issues `skipCmd` automatically when the reconciled outcome is an empty pick, resuming the sequence (HU-006 AC8's safety net), rather than prompting. The engine already surfaces `Empty`/`EmptyMessage` for an informative screen; the MVP wiring skips-and-continues so the happy path never stalls. A future slice can add the explicit "offer `--skip`" pane.
6. **`internal/app` is a SINGLE package** (not split into sub-packages): keeps the boundary test's `build.ImportDir(".")` a complete cover of all app code (satisfying 11.6 by construction) and matches the design's single `internal/app` row in the package-layout table.

## Issues Found (Phase 11)

1. No blocking issues. The full-flow integration test passed on its first real-git run (distinct-file commits promote faithfully; `VerifyPromotedContent` returns `OK`), and both mutation-verification exercises (boundary `os/exec` import; `aborted`-flag threading) failed exactly as expected before reverting.
2. The out-of-slice screens in `docs/MOCKUPS_TUI.md` (Menu Principal, Resumen De Package, Cola De Deploys, Validacion, Push Y PR, Quick Deploy, Historial) are intentionally NOT implemented — they belong to later Fases (delta/validation/push/history), beyond this change's scope edge (PickVerification). The eight in-scope screens (Doctor, Ticket search, Commit selection, Target selection, Plan preview, Cherry-pick in progress, Conflict, Post-pick verification) are all rendered.

## Status (Phase 11)

**Phase 11 (`internal/app` wiring) complete under strict TDD, and the 4 deferred Phase-10 TUI tasks
(10.19/10.20 live re-poll, 10.37/10.38 exec-boundary invariant) closed.** The Bubble Tea `Model`
composes the existing `git.Service`/`salesforce.Client`/`config.Config`/`prereq.Checker` into the full
promotion state machine (`PrereqCheck → TicketInput → CommitDiscovery → CommitSelection →
TargetSelection → PlanPreview → BranchCreation → CherryPicking ⇄ CherryPickConflict/Aborted →
PickVerification`, scope edge at PickVerification — no delta/validation/push). The repo is the source of
truth: a `tea.Tick` re-poll re-reads `git.RepoState` ONLY during the cherry-pick screens so external
`--continue`/`--abort` reconcile live, and the Model never execs — enforced by a mutation-verified
`go/build` boundary test proving `internal/app` imports neither `os/exec` nor `internal/exec` directly.
The H3-hardened `VerifyPromotedContent` is wired with `base=origin/<target>`, `selectedTip=`last selected
commit, `selectedFiles=`the union the selected set touched; the run's real `aborted` flag threads into
`DeltaAndValidationAllowed` so a successful abort is never treated as clean completion. `deploydeck` with
no subcommand launches the flow (real `NewOSRunner`-backed services in `main`, fakes in tests), keeping
`doctor`. All proved via direct `Model.Update` transition tests plus a full PrereqCheck→PickVerification
integration flow on a real bare-remote clone. `go build ./...`, `go vet ./...`, `gofmt -l .`, and
`go test -race ./...` all green; `go test -short ./...` skips the real-git integration. 12.1/12.2 run
green; 12.3 (proposal Success-Criteria check-off) is left for the orchestrator's dedicated
final-verification pass.

---

## Post-Integration Field Fix: `sf plugins --json` top-level-array bug (HU-001)

**Discovered by**: running the real `deploydeck doctor` binary against the real `sf` CLI (2.135.7),
outside the `FakeRunner` safety net. `internal/salesforce.Plugins()` decoded `sf plugins --json` via
`decodeEnvelope` (the `{"status":0,"result":...}` wrapper `sf org list --json`/`sf --version` genuinely
use), but `sf plugins --json` actually returns a **top-level JSON ARRAY** of oclif plugin objects. Every
`FakeRunner` fixture for this call across the whole test suite (`internal/salesforce/client_test.go`,
`internal/prereq/checker_versions_test.go`, `internal/prereq/checker_check_test.go`,
`cmd/deploydeck/doctor_e2e_test.go`, `cmd/deploydeck/root_test.go`) had canned the WRONG (envelope) shape,
so every test was green while `deploydeck doctor` wrongly reported `sfdx-git-delta` as a BLOCKING JSON
parse error instead of correctly detecting presence/absence. `decodeEnvelope`/`Orgs()`/`Version()` were
NOT touched — confirmed correct against the real CLI.

### Remediation TDD Cycle Evidence

| Item | Test File(s) | RED (reproduces defect) | GREEN (minimal fix) |
|------|--------------|--------------------------|----------------------|
| Wrong envelope shape for `sf plugins --json` | `internal/salesforce/client_test.go` (`TestClient_Plugins_ParsesTopLevelArray`, `TestClient_Plugins_FlattensNestedChildren`, `TestClient_Plugins_EmptyListWhenNonePresent`) | ✅ Fixtures rewritten to the real top-level-array shape first; ran RED against the unfixed `Plugins()` and captured the exact reported bug: `salesforce: parsing sf JSON envelope: json: cannot unmarshal array into Go value of type salesforce.envelope` | ✅ `internal/salesforce/plugins.go` rewritten: new `oclifPlugin{Name,Version,Children}` struct, `Plugins()` unmarshals `result.Stdout` as `[]oclifPlugin` directly (no envelope), then `flattenOclifPlugins` recursively flattens self+children into `[]Plugin` |
| Same wrong shape in downstream fixtures | `internal/prereq/checker_versions_test.go` (+ new `TestChecker_CheckVersions_DeltaPluginNestedInChildren_StillDetected`), `internal/prereq/checker_check_test.go`, `cmd/deploydeck/doctor_e2e_test.go` (`pluginsJSON` helper), `cmd/deploydeck/root_test.go` | ✅ All 5 files' fixtures rewritten to array shape in the same RED pass; reran `go test ./internal/salesforce/... ./internal/prereq/... ./cmd/deploydeck/...` and confirmed every plugin-touching test failed with the same envelope-unmarshal error (or a derived blocking check) | ✅ Same `plugins.go` fix makes all of them GREEN; no other production code changed |

### Work Unit Evidence

| Evidence | Value |
|---|---|
| Focused test command and exact result | `go test ./internal/salesforce/... ./internal/prereq/... ./cmd/deploydeck/... -v` → all `PASS`/`ok`, including the new nested-children present/absent/below-min triangulation cases. |
| Runtime harness command/scenario and exact result | Rebuilt the real binary (`go build -o /tmp/dd ./cmd/deploydeck`) and ran `dd doctor` in a real temp git repo (real origin remote, committed `deploydeck.yaml`) against the REAL `sf` CLI (2.135.7, `sfdx-git-delta` genuinely not installed). Before the fix: `[blocking] sfdx-git-delta plugin: could not list sf plugins: salesforce: parsing sf JSON envelope: json: cannot unmarshal array into Go value of type salesforce.envelope`. After the fix: `[blocking] sfdx-git-delta plugin: sfdx-git-delta plugin is not installed` with `fix: sf plugins install sfdx-git-delta` — a clean, correct presence check, not a parse error. |
| Rollback boundary | Single commit touching `internal/salesforce/plugins.go` + the 5 test files listed above; reverting it alone restores the pre-fix (buggy-but-green-under-FakeRunner) state without touching any other Phase 1-11 code. |

### Deviations from Design

None — this is a bug fix aligning `Plugins()` with the REAL `sf` CLI's documented/observed output shape;
`decodeEnvelope` and its two other callers (`Version()`, `Orgs()`) are unchanged and were independently
re-verified against the real CLI to still be correct.

### Issues Found

1. **Root cause confirmed**: every `FakeRunner` fixture for `sf plugins --json` across the test suite used
   the `{"status":0,"result":[...]}` envelope shape (matching `sf org list --json`/`sf --version`'s real
   shape) instead of `sf plugins --json`'s actual top-level array shape — a fixture authoring mistake made
   during the original Phase 5 TDD batch, invisible under `FakeRunner` since the fake never validated the
   fixture against the real CLI's contract.
2. No other blocking issues. `go build ./...`, `go vet ./...`, `gofmt -l .`, and `go test -race ./...` all
   green after the fix.

### Status

Field fix complete under strict TDD. `sfdx-git-delta` plugin presence/absence/nested/below-minimum
detection now correctly parses the real `sf plugins --json` top-level array shape end-to-end, verified
both under `FakeRunner` and against the real `sf` CLI 2.135.7.
