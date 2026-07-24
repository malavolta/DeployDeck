# Apply Progress: Foundation + MVP Git (HU-001..HU-006)

**Mode**: Strict TDD
**Batch scope**: Phases 1-4 ONLY (Bootstrap, `internal/exec`, `internal/config`, `internal/git` core).
Phases 5-12 (HU-001..HU-006, `internal/app` wiring, final verification) are NOT started.

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

## Remaining Tasks (NOT in this batch)

- [ ] Phase 5: HU-001 `prereq-check` (5.1-5.26)
- [ ] Phase 6: HU-002 `commit-discovery` (6.1-6.20)
- [ ] Phase 7: HU-003 `commit-selection` (7.1-7.16)
- [ ] Phase 8: HU-004 `target-selection` (8.1-8.16)
- [ ] Phase 9: HU-005 `promotion-branch` (9.1-9.15)
- [ ] Phase 10: HU-006 `cherry-pick` (10.1-10.38)
- [ ] Phase 11: `internal/app` wiring (11.1-11.6)
- [ ] Phase 12: Final verification (12.1-12.3)

## TDD Cycle Evidence

| Task | Test File | Layer | Safety Net | RED | GREEN | TRIANGULATE | REFACTOR |
|------|-----------|-------|------------|-----|-------|-------------|----------|
| 1.3/1.4 | `cmd/deploydeck/root_test.go` | Unit | N/A (new) | ✅ Written (undefined `newRootCmd`/`Deps`) | ✅ Passed | ✅ 2 cases (registers subcommand + stub exits 0) | ➖ None needed |
| 2.1/2.2 | `internal/exec/fake_runner_test.go` | Unit | N/A (new) | ✅ Written (no non-test files) | ✅ Passed | ✅ 2 cases (matched vs unmatched) | ➖ None needed |
| 2.3/2.4 | `internal/exec/os_runner_test.go` | Integration | N/A (new) | ✅ Written (`NewOSRunner` undefined) | ✅ Passed | ➖ Single (structural capture) | ➖ None needed |
| 2.5/2.6 | `internal/exec/os_runner_env_test.go` | Integration | ✅ 4/4 (prior exec tests) | ✅ Written (env silently ignored) | ✅ Passed | ➖ Single (env-merge is one behavior) | ➖ None needed |
| 2.7/2.8 | `internal/exec/os_runner_timeout_test.go` | Integration | ✅ 5/5 | ✅ Written | ✅ Passed on first run — behavior already provided by `CommandContext` (2.4); no new production code (see Deviations) | ✅ 2 cases (timeout + explicit cancel) | ➖ None needed |
| 2.9/2.10 | `internal/exec/os_runner_exitcode_test.go` | Integration | ✅ 7/7 | ✅ Written (non-zero-exit case failed: `err != nil`) | ✅ Passed | ✅ 2 cases (non-zero exit vs start failure) | ➖ None needed |
| 3.1/3.2 | `internal/config/load_test.go` | Unit | N/A (new) | ✅ Written (no non-test files) | ✅ Passed | ✅ 2 table cases (defaults vs overrides) + parse-fields test | ➖ None needed |
| 3.3/3.4 | `internal/config/validate_test.go` | Unit | ✅ 3/3 | ✅ Written (`Validate` undefined) | ✅ Passed | ✅ 4 table cases | ➖ None needed |
| 3.5/3.6 | `internal/config/sandbox_for_test.go` | Unit | ✅ 7/7 | ✅ Written (`SandboxFor` undefined) | ✅ Passed | ✅ 3 table cases (exact/glob/no-match) | ➖ None needed |
| 4.1-4.3 | `internal/git/service_root_test.go` | Integration | N/A (new) | ✅ Written (no non-test files) | ✅ Passed | ✅ 2 cases (inside repo vs outside) | ➖ None needed |
| 4.4/4.5 | `internal/git/service_env_test.go` | Unit (`FakeRunner`) | ✅ 2/2 | ✅ Written (Env empty) | ✅ Passed | ➖ Single (env-injection is one behavior, shared builder covers all methods by construction) | ✅ extracted `newRequest` builder |
| 4.6/4.7 | `internal/git/service_branches_test.go` | Integration | ✅ 3/3 | ✅ Written (`ListBranches` undefined) | ✅ Passed | ✅ 3 assertions (local main, local feature branch, remote main) | ➖ None needed |
| 4.8/4.9 | `internal/git/service_status_test.go` | Integration | ✅ 5/5 | ✅ Written (`Status` undefined) | ✅ Passed | ✅ 2 cases (clean vs dirty) | ➖ None needed |

### Test Summary
- **Total tests written**: 19 top-level test functions (several table-driven with multiple sub-cases)
- **Total tests passing**: 19/19 (`go test ./...`), plus all sub-cases
- **Layers used**: Unit (9), Integration (10)
- **Approval tests** (refactoring): None — no refactoring tasks in this batch, all new code
- **Pure functions created**: `requestKey`, `applyDefaults`, `validateBranchFormatTokens`/`isAllowedBranchFormatToken`, `Config.SandboxFor` (pure given map input)

## Work Unit Evidence

| Evidence | Value |
|---|---|
| Focused test command and exact result | `go test ./internal/exec/... ./internal/config/... ./internal/git/... ./cmd/...` → all packages `ok` (see verbatim `go test ./...` below) |
| Runtime harness command/scenario and exact result | `go test ./internal/git/... -run TestService_RepoRoot_ResolvesViaRevParse -v` → PASS, real git repo via `newTempRepo`, resolves `git rev-parse --show-toplevel` against a real temp repository with a wired bare origin remote |
| Rollback boundary | Revert commits `02f3f65` (bootstrap), `8f69d40` (exec), `695e8ac` (config), `b3c8928` (git core). Nothing downstream exists yet — no other package imports these. |

## Deviations from Design

1. **Tasks 2.7/2.8 (context timeout/cancel)**: writing the RED test revealed the behavior was already provided by `os/exec.CommandContext`, wired in task 2.4's `NewOSRunner` implementation. No additional production code was needed for GREEN — the test was written first per strict TDD and passed on first execution, documenting/locking in behavior that came "for free" from the stdlib rather than being newly implemented. This is not a violation of the "no code before a failing test" rule (no new code was written at all for this task); it is recorded transparently rather than silently claimed as a fabricated RED→GREEN cycle.
2. **`Config` fields scoped to this batch**: per design.md's explicit field list for `internal/config` (`branches`, `sandboxes`, `ticketPatterns`, `branchFormat`, `minVersions`, `runs`), the `delta` and `pollIntervalSeconds` fields shown in `ARQUITECTURA.md`'s example YAML (Fase 2 `sfdx-git-delta` concerns) were intentionally NOT added — they are out of scope for this slice per `proposal.md`.
3. **`RepoState` is intentionally partial**: only `Clean bool` in this batch. Phase 10 (out of this batch) extends it with `InProgress`, `CurrentSHA`, `Unmerged`, `SequencerRemaining` per design.md's cherry-pick decisions.
4. **`newTempRepo` also sets `commit.gpgsign=false` via `GIT_CONFIG_*` env** on its own seed commit, defensively, so the harness never hangs on a machine with `commit.gpgsign=true` configured globally — consistent with (but not identical to) the `-c commit.gpgsign=false` mechanism Phase 10 will add to the tool's own cherry-pick commits.

## Issues Found

None blocking. All Phase 1-4 acceptance criteria implemented and green.

## Workload / PR Boundary

- Mode: single PR (`size:exception` GRANTED by maintainer per tasks.md Delivery Decision, recorded 2026-07-24)
- Current work unit: Unit 1 of 4 suggested units — "Bootstrap + `exec` + `config` + `git` core (Phases 1-4)"
- Boundary: starts from empty repo (pre-code), ends with `internal/exec`, `internal/config`, `internal/git` core all independently tested and green; no HU-level behavior yet
- Estimated review budget impact: ~4 commits, well within Unit 1's suggested scope; full single-PR total still tracked against the `review_budget_lines=40000` session budget

## Status

29/29 tasks in scope (Phases 1-4) complete. Ready for next batch (Phase 5: HU-001 `prereq-check`).
