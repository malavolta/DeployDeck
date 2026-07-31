# Tasks: Source Resolution — Dedupe Candidates + Current-Branch Confirm

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | ~420-520 |
| 400-line budget risk | Medium |
| Chained PRs recommended | No |
| Suggested split | Single PR (fits 800-line session budget) |
| Delivery strategy | single-pr |
| Chain strategy | size-exception |

Decision needed before apply: Yes
Chained PRs recommended: No
Chain strategy: size-exception
400-line budget risk: Medium

### Suggested Work Units

| Unit | Goal | Likely PR | Focused test command | Runtime harness | Rollback boundary |
|------|------|-----------|----------------------|-----------------|-------------------|
| 1 | Dedupe (D1) — `CandidateBranches` collapses local+origin twin | PR 1 (single) | `go test ./internal/git/... -run CandidateBranches` | `go test ./internal/git/... -run TestHU002_Discover_E2E` (real temp repo, no network) | Revert `dedupeByLocalName` + its call site; `ListBranches`/`branchesByPattern` untouched |
| 2 | Confirm prompt (D2) — tri-state `resolveSource` + `StateSourceConfirm` | PR 1 (single) | `go test ./internal/app/... -run ResolveSource` and `-run Confirm` | `go test ./internal/app/... -run TestHU_FullFlow` | Revert `flow.go`/`app.go`/`keys.go`/`update.go`/`view.go` confirm additions; `discoverCmd` degrade path unchanged |

Both units share the same PR because D2 depends on D1's `discoverCmd`/`discoverDoneMsg` shape (design ADR-3); splitting would leave an intermediate PR with a broken confirm wiring. Kept as one PR under `single-pr` delivery strategy — flag `size:exception` for maintainer sign-off given the Medium 400-line risk.

## Phase 1: Git layer — dedupe (D1)

- [x] 1.1 RED: in `internal/git/service_branches_test.go`, add `TestService_CandidateBranches_DedupesLocalAndRemoteTrackingOfSameBranch` (real temp repo: local branch `X` + push to `origin/X`, both match ticket pattern) asserting `len(candidates) == 1` and the surviving entry is the bare/local form. Run `go test ./internal/git/...` to confirm it fails.
- [x] 1.2 RED: add an over-aggression guard case — two candidates with genuinely distinct names (e.g. `feature/TICKET-a`, `hotfix/TICKET-b`) still yield `len(candidates) == 2` after dedupe (extend the existing `TestService_CandidateBranches_ReturnsLocalAndRemoteMatchesByTicketName` or add a sibling test).
- [x] 1.3 RED: add/confirm a guard that `ListBranches` (via `TestService_ListBranches_ReturnsLocalAndRemoteBranches`) still returns BOTH local and remote forms undeduplicated — must stay green unmodified, proving no leak into `branchesByPattern`.
- [x] 1.4 GREEN: implement `dedupeByLocalName(branches []Branch) []Branch` in `internal/git/service_branches.go` — drop each `origin/X` when local `X` is present (compare via `strings.TrimPrefix(name, "origin/")`), keep the bare/local form, preserve first-seen order, never collapse distinct names.
- [x] 1.5 GREEN: call `dedupeByLocalName` at the END of `CandidateBranches` (after `branchesByPattern` returns), NOT inside `branchesByPattern`/`ListBranches`. Run `go test ./internal/git/...` — 1.1-1.3 pass, `TestStandaloneBranchesCmd_ComposesListBranches` (`internal/app`) and `standalone_modes_e2e_test.go` stay green.

## Phase 2: App layer — resolveSource tri-state (D2, part 1)

- [x] 2.1 RED: create `internal/app/flow_test.go` with table-driven `TestResolveSource` covering: zero candidates → degrade; pipeline suggestion present → ready (ignores `currentBranch`); no suggestion + `currentBranch` matches a candidate → needs-confirm; `currentBranch == ""` → degrade (existing behavior); `currentBranch == "HEAD"` → degrade; `currentBranch` not among candidates → degrade; single deduped candidate (no suggestion, no `currentBranch` match) → ready. Run `go test ./internal/app/... -run TestResolveSource` to confirm RED.
- [x] 2.2 GREEN: in `internal/app/flow.go`, add `resolveOutcome` type (`resolveDegrade`, `resolveReady`, `resolveNeedsConfirm`) and change `resolveSource` signature to `resolveSource(candidates []git.Branch, cfg config.Config, target, currentBranch string) (git.Branch, resolveOutcome)`. Ordering: `SuggestDefaultSource` first (unchanged priority); only when it yields nothing AND `currentBranch != "" && != "HEAD"` AND a candidate satisfies `sourceRefName(c) == currentBranch`, return `resolveNeedsConfirm`; otherwise fall through to `SelectSingleSource` for `resolveReady`/`resolveDegrade`. Run `go test ./internal/app/... -run TestResolveSource` — GREEN.

## Phase 3: App layer — confirm state + flow split (D2, part 2)

- [x] 3.1 RED: in `internal/app/app.go` test coverage (new or existing `_test.go`), assert `StateSourceConfirm` exists as a distinct `State` constant.
- [x] 3.2 GREEN: add `StateSourceConfirm State` to `internal/app/app.go`.
- [x] 3.3 RED: add `discoverDoneMsg.confirm bool` field expectation and a `Model.Update` test asserting that when pass-1 discovery resolves to `resolveNeedsConfirm`, the resulting message carries `confirm: true` and the pending candidate, WITHOUT running the ranged discover (no `OrderedCommits` populated yet).
- [x] 3.4 GREEN: in `internal/app/commands.go`, split `discoverCmd` into pass 1 (candidates + `resolveSource` using `m.originalBranch` as `currentBranch`) and pass 2. Add `discoverDoneMsg.confirm bool`. On `resolveNeedsConfirm`, return `discoverDoneMsg{result: base, source: pending, confirm: true}` instead of running the ranged discover. Resolved (`resolveReady`) and degrade (`resolveDegrade`) paths stay byte-for-byte unchanged.
- [x] 3.5 RED: `Model.Update` test — `onDiscoverDone` with `msg.confirm == true` sets `m.discovery = msg.result`, `m.source = msg.source` (pending), `m.state = StateSourceConfirm`, and leaves `m.items` empty (no premature selection screen).
- [x] 3.6 GREEN: in `internal/app/update.go`, route `onDiscoverDone` on `msg.confirm` to `StateSourceConfirm` before the existing `m.items = ...` / `StateCommitSelection` assignment; non-confirm path unchanged.
- [x] 3.7 RED: `Model.Update` test for `keySourceConfirm` — `"s"` transitions to `StateCommitDiscovery` and returns a `confirmSourceCmd()`-shaped command (assert via the follow-up `discoverDoneMsg` reaching `StateCommitSelection` with `len(m.items) > 0`, or by asserting the command is non-nil and state transition); `"n"`, `"N"`, `"enter"` (default-No) clear `m.source` and degrade straight to `StateCommitSelection` with `m.discovery.OrderedCommits == nil`; `"esc"` returns to `StateTicketInput`.
- [x] 3.8 GREEN: in `internal/app/keys.go`, add `keySourceConfirm(msg tea.KeyMsg)` wired from `handleKey`'s `StateSourceConfirm` case, implementing the `s` / `n`·`N`·`enter` / `esc` behavior from 3.7.
- [x] 3.9 GREEN: in `internal/app/commands.go`, add `func (m Model) confirmSourceCmd() tea.Cmd` — pass-2 ranged discover: `git.SelectSingleSource(m.discovery.CandidateBranches, m.source.Name)` (UNMODIFIED helper) then `g.Discover(ctx, dir, git.DiscoverOptions{Ticket, Target: m.prelim, Source: sourceRefName(sel)})`, returning the same `discoverDoneMsg` shape as pass 1's resolved branch (`confirm: false`).
- [x] 3.10 RED: view test asserting `viewSourceConfirm()` (state `StateSourceConfirm`) renders `"¿Usar la rama actual '<X>' como origen? [s/N]"` with `<X>` = the pending `m.source.Name`.
- [x] 3.11 GREEN: in `internal/app/view.go`, add `viewSourceConfirm()` and wire it from `viewBody`'s `StateSourceConfirm` case.

## Phase 4: E2E

- [x] 4.1 RED then GREEN (verifying Phase 1+2+3 together): extend `internal/git/discovery_e2e_test.go` — repro scenario (local+origin same branch, first-pipeline-env target) now returns exactly 1 deduped candidate with non-empty `OrderedCommits`; add a distinct-candidates scenario (two genuinely different branches) exercising `SelectSingleSource` directly to confirm dedupe does not over-collapse.
- [x] 4.2 RED then GREEN: extend `internal/app/flow_e2e_test.go` (reuse `setupFlowRepo`/`gitRun`/`advance`) — full-flow repro reaches `StateCommitSelection` with `len(m.items) > 0` (dedupe path, no confirm needed); a second scenario drives the confirm-prompt path end to end (`StateSourceConfirm` → `"s"` → `StateCommitSelection` with `len(m.items) > 0`) and a decline scenario (`"n"` → `StateCommitSelection` with `len(m.items) == 0`).

## Phase 5: Regression guard

- [x] 5.1 Run `go test ./...` and confirm all pre-existing green suites stay green unmodified: `internal/app/standalone_delta_test.go` (`TestStandaloneBranchesCmd_ComposesListBranches`), `internal/app/standalone_modes_e2e_test.go` (`pickStandaloneBase` local+remote rows), `internal/app/re_promote_e2e_test.go` (`SuggestDefaultSource` priority over current-branch inference), and `internal/app/original_branch_test.go`/`original_branch_e2e_test.go` (`quitCmd`'s `m.originalBranch` guard, untouched by the new confirm reuse of the same field).
- [x] 5.2 Confirm `internal/app/boundary_test.go` (exec-seam boundary) still holds — no new git/exec call introduced in `internal/app` by the confirm flow.
