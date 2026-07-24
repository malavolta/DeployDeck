# Tasks: Foundation + MVP Git (HU-001..HU-006)

Strict TDD active (`go test ./...`). Every behavior task is RED (failing test) → GREEN (minimal
impl). `[U]` = table-driven unit test (`FakeRunner`/fake `ProcessProber`), `[I]` = temp-git-repo
integration via shared `newTempRepo` harness (skippable `testing.Short()`), `[T]` = TUI (`teatest`
or direct `Model.Update`). This artifact intentionally exceeds the generic word-budget guideline:
the orchestrator explicitly mandated per-acceptance-criterion RED/GREEN slicing for a 6-HU, first-code
foundational change (HU-006 alone has 15 GWT after the design/spec technical-review additions); a
more specific instruction overrides the default.

## Review Workload Forecast

| Phase | Capability/HU | Test Type | Est. Lines (prod+test) |
|---|---|---|---|
| 1 Bootstrap | scaffolding | `[U]` | ~150 |
| 2 `internal/exec` | foundation | `[U]` + `[I]` (OSRunner, env-merge, exit-code-as-data) | ~570 |
| 3 `internal/config` | foundation | `[U]` | ~350 |
| 4 `internal/git` core | foundation | `[I]` (newTempRepo) | ~500 |
| 5 HU-001 | prereq-check | `[U]` + `[I]` + sf-fake (lock race/atomic-write, hooks, gpgsign warn) | ~1350 |
| 6 HU-002 | commit-discovery | `[U]` + `[I]` (RF-002 default source) | ~730 |
| 7 HU-003 | commit-selection | `[U]` + `[I]` (real diff) | ~550 |
| 8 HU-004 | target-selection | `[U]` + `[I]` + sf-fake | ~550 |
| 9 HU-005 | promotion-branch | `[I]` (two-repo bare-remote) | ~550 |
| 10 HU-006 | cherry-pick | `[I]` (heaviest) + `[U]` (parsing, gpgsign, modify/delete, binary) | ~1550 |
| 11 `internal/app` wiring | composition, all HUs | `[T]` + `[I]` | ~1000 |
| 12 Final verification | all | CI run only | ~0 |
| **Total** | | | **~7,850** |

| Field | Value |
|-------|-------|
| Estimated changed lines | ~7,000–10,000 (rough; ~7,850 midpoint) |
| 400-line budget risk (default guard) | High |
| Session review budget (`review_budget_lines=40000`) | Fits — ~20% of the extended budget |
| Chained PRs recommended | Yes, under the default 400-line guard |
| Suggested split | Single PR (current session), with 4 natural slice boundaries below if split later |
| Delivery strategy | single-pr |
| Chain strategy | size-exception |

```text
Decision needed before apply: Yes
Chained PRs recommended: Yes
Chain strategy: size-exception
400-line budget risk: High
```

`delivery_strategy=single-pr` requires explicit maintainer `size:exception` before `sdd-apply`
starts implementation (~7.85k lines vs the 400-line default guard), even though the estimate fits
inside this session's explicit `review_budget_lines=40000`.

### Delivery Decision (recorded 2026-07-24)

**`size:exception` GRANTED by maintainer.** Single PR for the full slice (~7k lines) approved;
chained-PR split declined. The 4 work units above remain documented as the fallback split boundary
if review proves unwieldy, but the sanctioned delivery for this change is one PR under the session's
`review_budget_lines=40000` budget. `sdd-apply` is unblocked on the delivery gate.

### Suggested Work Units (natural split boundaries if chained later)

| Unit | Goal | Likely PR | Focused test command | Runtime harness | Rollback boundary |
|------|------|-----------|----------------------|-----------------|-------------------|
| 1 | Bootstrap + `exec` + `config` + `git` core (Phases 1–4): scaffolding, `Runner` seam, config, repo/branch/status, no user-facing behavior | PR 1 (single-pr: bundled) | `go test ./internal/exec/... ./internal/config/... ./internal/git/... ./cmd/...` | `go test ./internal/git/... -run TestRepoRoot` (real git, `newTempRepo`) | Revert Phase 1–4 commits; nothing downstream exists yet |
| 2 | HU-001..HU-005 (Phases 5–9): doctor/prereq, discovery, selection, target, promotion branch | PR 2 (single-pr: bundled) | `go test ./internal/prereq/... ./internal/salesforce/... ./internal/git/... ./internal/config/...` | `deploydeck doctor` on a temp repo + `go test ./internal/git/... -run TestPromotionBranch` (bare-remote fetch) | Revert Phase 5–9 commits; Unit 1 stays buildable/testable alone |
| 3 | HU-006 cherry-pick engine (Phase 10): sequential pick, conflict classify, live re-poll, continue/skip/abort, rerere, post-pick verify | PR 3 (single-pr: bundled) | `go test ./internal/git/... -run TestCherryPick` | `go test ./internal/git/... -run TestCherryPickConflict -v` (seeded real conflict) | Revert Phase 10 commits only; Units 1–2 (doctor, discovery, selection, branch creation) remain fully functional |
| 4 | `internal/app` TUI wiring + final verification (Phases 11–12) | PR 4 (single-pr: bundled) | `go test ./internal/app/... -run TestModelUpdate` | `go test ./internal/app/... -run TestFullFlow` (teatest, PrereqCheck→PickVerification on `newTempRepo`) | Revert Phase 11–12 commits; all `internal/*` packages remain independently testable without the TUI |

## Phase 1: Bootstrap

- [x] 1.1 Create `go.mod` (module `deploydeck`) at repo root.
- [x] 1.2 Create `.gitignore` including `.deploydeck/` + standard Go ignores.
- [x] 1.3 `[U]` RED: `cmd/deploydeck/root_test.go` — `newRootCmd(deps)` builds and registers a `doctor` subcommand.
- [x] 1.4 GREEN: `cmd/deploydeck/main.go` + `newRootCmd(deps)` — Cobra root, stub `doctor` (exit 0 placeholder; real behavior Phase 5).
- [x] 1.5 Verify `go build ./...` and `go test ./...` run clean (CI smoke check).

## Phase 2: `internal/exec` — Runner seam

- [x] 2.1 `[U]` RED: `FakeRunner.Run` returns canned `CommandResult` for a matching Name+Args key; unmatched request errors explicitly.
- [x] 2.2 GREEN: `internal/exec/runner.go` — `Runner`, `CommandRequest{Dir,Name,Args,Env}`, `CommandResult{Stdout,Stderr,ExitCode,Duration}`, `FakeRunner`. No git-specific env here — layered in `internal/git` (Phase 4), never globally.
- [x] 2.3 `[I]` RED: `NewOSRunner` runs a real command in `t.TempDir()`, captures stdout/exit/duration (skip `-short`).
- [x] 2.4 GREEN: implement `NewOSRunner` wrapping `os/exec.CommandContext`.
- [x] 2.5 `[I]` RED: `NewOSRunner` merges env — a `req.Env` override (e.g. a git-style var) is present AND an inherited process var (e.g. `PATH`) still resolves inside the child, proving `cmd.Env = append(os.Environ(), req.Env...)`, not a full replace.
- [x] 2.6 GREEN: implement env layering in `NewOSRunner` (append `req.Env` onto `os.Environ()`; never assign a bare `req.Env`).
- [x] 2.7 `[I]` RED: context timeout/cancel on `OSRunner` returns an error, no hang.
- [x] 2.8 GREEN: implement ctx-cancel/timeout handling in `OSRunner`. (Already satisfied by `os/exec.CommandContext` from 2.4 — no additional production code required; test written and passing to lock in the behavior.)
- [x] 2.9 `[I]` RED: a process that runs but exits non-zero yields a nil `Runner` error and a populated `CommandResult.ExitCode`, distinguishable from a start failure (missing/non-executable binary → non-nil error, `ExitCode=-1`) and from timeout/cancel (non-nil error with `ctx.Err()` set).
- [x] 2.10 GREEN: implement exit-code extraction from `*exec.ExitError` in `OSRunner`, keeping start/timeout failures as errors so downstream git-exit-code-as-data reads (`merge-base --is-ancestor`, `cherry-pick`, `diff --quiet`, `log --grep`) are interpreted per-command, not as `Runner` failures.

## Phase 3: `internal/config`

- [x] 3.1 `[U]` RED: `Load(dir)` table-driven — defaults applied for omitted fields, fixture YAML via `t.TempDir()`.
- [x] 3.2 GREEN: `Config` struct (`branches`, `sandboxes`, `ticketPatterns`, `branchFormat`, `minVersions`, `runs`) + `Load(dir)` with defaults.
- [x] 3.3 `[U]` RED: `Validate()` table-driven — invalid `ticketPatterns` regex, sandbox missing alias, `branchFormat` containing a token outside the shared `{{ticket}}`/`{{target}}` allow-list all fail (same allow-list the Phase 9 renderer uses — not Go `text/template` parsing).
- [x] 3.4 GREEN: implement `Validate()` checking `branchFormat` tokens against the shared render allow-list.
- [x] 3.5 `[U]` RED: `SandboxFor(branch)` table-driven — exact match, `Release/*` glob match, no-match error.
- [x] 3.6 GREEN: implement `SandboxFor()`.

## Phase 4: `internal/git` core

- [x] 4.1 Create shared `newTempRepo(t)` test helper — inits temp repo via `NewOSRunner`, sets `user.name`/`user.email`, wires a bare origin remote so `origin/<branch>` refs exist (reused by HU-002 Phase 6 and HU-004 Phase 8; Phase 9 `newTempRepoWithRemote` extends it with a remote that advances after clone), skip on `-short`.
- [x] 4.2 `[I]` RED: `Service` resolves repo root via `git rev-parse --show-toplevel`; errors outside a repo.
- [x] 4.3 GREEN: `git.Service`, `New(Runner)`; `req.Dir` = rev-parse result, never shell `cd` (threat matrix: Git repository selection).
- [x] 4.4 `[U]` RED: every git request carries `GIT_EDITOR=true`, `GIT_TERMINAL_PROMPT=0`, `GIT_PAGER=cat` (assert on captured `Env` via `FakeRunner`).
- [x] 4.5 GREEN: implement env injection in `git.Service` request builder.
- [x] 4.6 `[I]` RED: branch listing returns local + remote branches on `newTempRepo`.
- [x] 4.7 GREEN: implement branch-listing method.
- [x] 4.8 `[I]` RED: working-tree status distinguishes clean vs dirty (`git status --porcelain`).
- [x] 4.9 GREEN: implement status method + partial `RepoState` porcelain parse.

## Phase 5: HU-001 `prereq-check`

- [x] 5.1 `[U]` RED: `sf --version`/`sf plugins --json`/`sf org list --json` JSON parsing, incl. malformed-JSON error case (canned `FakeRunner`).
- [x] 5.2 GREEN: `internal/salesforce/client.go` — `Client`, `New(Runner)`, `VersionInfo`/`Plugin`/`Org`/`OrgList`.
- [x] 5.3 `[U]` RED: lock acquire/refuse/stale-takeover table-driven with fake `ProcessProber.Alive(pid, startedAt)` — same-host + alive (matching `StartedAt`) → refuse naming pname+pid; dead owner OR PID-reused (`StartedAt` mismatch) → stale takeover; different host → conservative refuse.
- [x] 5.4 GREEN: `internal/prereq/lock.go` — `LockInfo{PID,PName,Host,StartedAt,CreatedAt}`, `ProcessProber` interface widened to `Alive(pid int, startedAt time.Time) bool`; unit layer uses the fake prober only — the darwin OS start-time reader (`sysctl KERN_PROC` / `ps -o lstart`) is integration/manual, not fake-covered.
- [x] 5.5 `[U]`/`[I]` RED: stale-lock takeover race — two acquirers hit the same stale lock, both `os.Remove` it then retry `OpenFile(O_CREATE|O_EXCL)` in a bounded loop; exactly one wins, the loser re-reads the winner's `LockInfo` and refuses (never rename over an existing lock).
- [x] 5.6 GREEN: implement stale takeover as remove-then-bounded-retry `O_CREATE|O_EXCL` with an explicit iteration/backoff bound.
- [x] 5.7 `[U]` RED: `LockInfo` is never observable half-written — a concurrent reader sees either no file or a fully valid record.
- [x] 5.8 GREEN: implement atomic full-write-before-discoverable `LockInfo` persistence (temp-file+rename onto the `O_EXCL` path, or full write before the fd is discoverable).
- [x] 5.9 `[U]` RED: git/sf/`sfdx-git-delta` version-vs-`minVersions` check — below-min blocks with `FixCommand` (including `sfdx-git-delta` VERSION below min, not just presence); missing `sfdx-git-delta` plugin shows install `FixCommand`.
- [x] 5.10 GREEN: implement version/plugin checks in `internal/prereq/checker.go`, incl. `sfdx-git-delta` version comparison.
- [x] 5.11 `[I]` RED: repo-membership + missing-origin blocks with corrective action.
- [x] 5.12 GREEN: implement repo/origin checks composing `git.Service`.
- [x] 5.13 `[I]` RED: dirty working tree blocks branch-modifying operations.
- [x] 5.14 GREEN: implement dirty-tree check.
- [x] 5.15 `[I]` RED: `.gitignore` missing `.deploydeck/` blocks + offers to add; present entry passes.
- [x] 5.16 GREEN: implement gitignore check + add-entry action.
- [x] 5.17 `[U]` RED: repository Git hooks that could interfere with checkout/cherry-pick are detected and reported as an informative, non-blocking `PrereqCheck` (RNF-005).
- [x] 5.18 GREEN: implement git-hooks interference detection (informative-only, never blocking) in `internal/prereq/checker.go`.
- [x] 5.19 `[U]` RED: `commit.gpgsign=true` (via `git config --get commit.gpgsign`) produces a non-blocking doctor warning noting the tool neutralizes it with `-c commit.gpgsign=false` on its own commits.
- [x] 5.20 GREEN: implement gpgsign warning check in `internal/prereq/checker.go`.
- [x] 5.21 `[U]` RED: configured alias absent from `sf org list --json`, scanned across all five categories (`nonScratchOrgs`/`scratchOrgs`/`sandboxes`/`devHubs`/`other`), blocks that sandbox.
- [x] 5.22 GREEN: implement alias validation composing `salesforce.Client` + config.
- [x] 5.23 `[U]` RED: full report assembly — all-pass (`Status=OK`) and missing-git-binary-blocks (`FixCommand` shown) scenarios.
- [x] 5.24 GREEN: implement `Checker.Check()` aggregating `[]PrereqCheck{Status,Detail,FixCommand}`.
- [x] 5.25 `[U]`/`[I]` RED: `doctor` exits non-zero (distinct from success) on any blocker, zero when all pass.
- [x] 5.26 GREEN: wire `doctor` subcommand to `Checker` with distinct non-zero exit.

## Phase 6: HU-002 `commit-discovery`

- [x] 6.1 `[U]` RED: ticket-grep parsing from canned `git log --grep` output; commit with two tickets matches either.
- [x] 6.2 GREEN: `SearchByTicket` message-parse logic in `internal/git`.
- [x] 6.3 `[I]` RED: seeded commits — ticket found in messages lists related commits (both scenarios above, real git).
- [x] 6.4 GREEN: wire real `git log --grep` into `git.Service.SearchCommits`.
- [x] 6.5 `[I]` RED: candidate local+remote branches whose name contains the ticket are listed.
- [x] 6.6 GREEN: implement branch-name search.
- [x] 6.7 `[U]` RED: single-source-branch enforcement blocks continuing when >1 candidate branch exists.
- [x] 6.8 GREEN: implement enforcement (pure, given candidate list).
- [x] 6.9 `[U]` RED (RF-002): env-to-env promotion (both source and target are configured sandbox environments, e.g. `INT`→`UAT`) suggests the previous/validated environment branch as the default source, not a feature branch; selecting a different candidate overrides the suggestion and single-source enforcement (6.7/6.8) still applies.
- [x] 6.10 GREEN: implement suggested-default-source selection for env-to-env promotions composing `config.Config` (pure, given candidate branches + config).
- [x] 6.11 `[I]` RED: seeded commits with inverted author-date vs topo order — `rev-list --reverse --topo-order origin/<target>..origin/<source>` final order is topological.
- [x] 6.12 GREEN: implement topo-order commit listing.
- [x] 6.13 `[U]` RED: merge-commit detection from canned `git log --parents` (parent count>1) flags + blocks.
- [x] 6.14 GREEN: implement merge-commit flagging.
- [x] 6.15 `[U]` RED: equivalence classification — canned `git cherry` `-`, `merge-base --is-ancestor`, `patch-id --stable` match → already-applied/equivalent, not selected by default.
- [x] 6.16 GREEN: implement equivalence classifier (pure).
- [x] 6.17 `[I]` RED: deleted source branch narrows search to grep-only with warning.
- [x] 6.18 GREEN: implement deleted-branch detection + warning.
- [x] 6.19 `[U]` RED: squash-merge-history warning trigger + no-results alternatives (manual search/change ticket/select branch).
- [x] 6.20 GREEN: implement diagnostics/warnings assembly.

## Phase 7: HU-003 `commit-selection`

- [x] 7.1 `[U]` RED: `CommitSelectionItem` row shows short SHA/message/author/date/flags.
- [x] 7.2 GREEN: implement `CommitSelectionItem{Commit,Selected,Disabled,Reason}` + rendering fields.
- [x] 7.3 `[U]` RED: already-applied commit → `Disabled=true` + `Reason`; select attempt is a no-op.
- [x] 7.4 GREEN: implement disabled/reason logic for already-applied.
- [x] 7.5 `[U]` RED: merge commit → `Disabled=true` (cherry-pick `-m` unsupported in MVP).
- [x] 7.6 GREEN: implement merge-commit disable rule.
- [x] 7.7 `[U]` RED: commit message referencing other tickets sets a multi-ticket notice.
- [x] 7.8 GREEN: implement multi-ticket notice detection.
- [x] 7.9 `[I]` RED: real `git diff --name-only` — selected commit's file also touched by an unselected intermediate commit triggers a dependency warning.
- [x] 7.10 GREEN: implement per-file dependency-warning computation.
- [x] 7.11 `[U]` RED: confirming with zero selected commits is blocked.
- [x] 7.12 GREEN: implement empty-selection guard.
- [x] 7.13 `[U]` RED: advanced-mode reorder shows conflict-risk warning; non-advanced reorder unavailable.
- [x] 7.14 GREEN: implement advanced-mode reorder + warning.
- [x] 7.15 `[U]` RED: valid non-empty confirmed selection generates a preliminary `DeploymentPlan`.
- [x] 7.16 GREEN: implement `DeploymentPlan` generation on confirm.

## Phase 8: HU-004 `target-selection`

- [x] 8.1 `[U]` RED: destination list from `config.Config.branches` shows associated sandbox.
- [x] 8.2 GREEN: implement destination-listing assembly composing config.
- [x] 8.3 `[I]` RED: nonexistent local/remote destination branch blocks continuing.
- [x] 8.4 GREEN: implement existence check via `git.Service`.
- [x] 8.5 `[U]` RED: `Release/*` resolves sandbox via configured glob; unmapped `Release/*` blocks with actionable message.
- [x] 8.6 GREEN: wire `config.SandboxFor` (Phase 3) into target-selection flow.
- [x] 8.7 `[I]` RED: `git rev-parse origin/<target>` remote HEAD is displayed.
- [x] 8.8 GREEN: implement remote-HEAD lookup.
- [x] 8.9 `[U]` RED (sf-fake): sandbox alias absent from `sf org list --json`, scanned across all five categories (`nonScratchOrgs`/`scratchOrgs`/`sandboxes`/`devHubs`/`other`), shows a non-blocking warning before validation.
- [x] 8.10 GREEN: implement unauthenticated-sandbox warning composing `salesforce.Client`.
- [x] 8.11 `[U]` RED: selecting `main` shows a production-environment warning.
- [x] 8.12 GREEN: implement production-branch warning rule.
- [x] 8.13 `[I]` RED: custom branch name — rejected when absent locally/remotely, accepted when present.
- [x] 8.14 GREEN: implement custom-branch validation.
- [x] 8.15 `[U]` RED: confirming a valid destination+sandbox saves branch/alias/testLevel into `DeploymentPlan`.
- [x] 8.16 GREEN: implement selection persistence.

## Phase 9: HU-005 `promotion-branch`

- [x] 9.1 Extend shared helper with `newTempRepoWithRemote(t)` — local repo + local bare "remote", clone, then advance the bare remote independently.
- [x] 9.2 `[I]` RED: branch creation runs `git fetch origin` before creating the branch (assert ordering).
- [x] 9.3 GREEN: implement fetch-then-checkout sequencing in `git.Service.CreatePromotionBranch`.
- [x] 9.4 `[I]` RED: after remote advances post-clone, branch is created exactly from post-fetch `origin/<target>` HEAD, not the stale local ref.
- [x] 9.5 GREEN: implement base-ref resolution from freshly fetched `origin/<target>`.
- [x] 9.6 `[U]` RED: branch-name templating — default `deploy/{{ticket}}-to-{{target}}` renders via `strings.NewReplacer` literal-token substitution (NOT Go `text/template`, which would fail parsing `{{ticket}}` as a function node) + custom format + user-edit override.
- [x] 9.7 GREEN: implement `branchFormat` rendering via `strings.NewReplacer` over the shared `{{ticket}}`/`{{target}}` token allow-list (same allow-list as Phase 3 `Validate()`) + edit-before-create hook.
- [x] 9.8 `[I]` RED: existing local/remote branch with the same name prompts for an action.
- [x] 9.9 GREEN: implement collision detection + action prompt.
- [x] 9.10 `[I]` RED: user on a protected branch — starting the flow does not modify that branch directly.
- [x] 9.11 GREEN: implement protected-branch guard (config-driven list).
- [x] 9.12 `[I]` RED: `git fetch origin` failure stops the flow, current branch unchanged.
- [x] 9.13 GREEN: implement fetch-failure short-circuit.
- [x] 9.14 `[U]` RED: successful creation records the final branch name into `DeploymentPlan`.
- [x] 9.15 GREEN: implement `DeploymentPlan` branch-name registration.

## Phase 10: HU-006 `cherry-pick` (largest/highest-risk — one RED→GREEN per AC)

- [x] 10.1 `[U]` RED: porcelain XY + numstat parsing — `DU`/`UD`→ModifyDelete, other conflict codes→Text, binary `-` marker→Binary.
- [x] 10.2 GREEN: implement `ConflictFile{Path,Kind}` classification (pure).
- [x] 10.3 `[U]` RED: porcelain parsing uses `git status --porcelain -z` NUL-delimited output so paths containing spaces/unicode split safely (not the default space-delimited porcelain format, which breaks on such paths).
- [x] 10.4 GREEN: implement `-z` NUL-delimited porcelain parsing in the `ConflictFile`/status builder.
- [x] 10.5 `[I]` RED: `RepoState{InProgress,CurrentSHA,Unmerged,SequencerRemaining}` assembled from `CHERRY_PICK_HEAD` + `.git/sequencer/todo` + `git status --porcelain -z`, during a real multi-commit cherry-pick sequence so `.git/sequencer/todo` is actually populated.
- [x] 10.6 GREEN: implement `git.Service.RepoState()`.
- [ ] 10.7 `[I]` RED (AC1): selected commits apply in order via ONE sequencer-driven invocation — `git cherry-pick <sha1>..<shaN>` for a contiguous topo-ancestry selection, or the explicit ordered SHA-list form `git cherry-pick <sha1> <sha2> ... <shaN>` for a non-contiguous selection — final content matches the source branch and `.git/sequencer/todo` ordering matches the user's selection.
- [ ] 10.8 GREEN: implement `git.Service` range-vs-explicit-list form selection + a single sequencer-driven cherry-pick invocation (never a Go loop of single-sha picks).
- [ ] 10.9 `[U]` RED (M2): `git.Service` cherry-pick and `--continue` invocations carry `-c commit.gpgsign=false` in `Args` (assert via `FakeRunner` captured `Args`) so promotion commits never invoke gpg with no tty.
- [ ] 10.10 GREEN: add `-c commit.gpgsign=false` to both the initial cherry-pick and `--continue` command construction in `git.Service`.
- [ ] 10.11 `[I]` RED (AC2): a conflicting commit stops the flow; conflicting files shown classified by type.
- [ ] 10.12 GREEN: implement conflict-stop handling wiring `RepoState` + `ConflictFile`.
- [ ] 10.13 `[I]` RED: modify/delete conflict resolution offers an explicit choice to keep the file (`git add`) or delete it (`git rm`).
- [ ] 10.14 GREEN: implement `git.Service` keep/delete action for `ModifyDelete` conflicts.
- [ ] 10.15 `[I]` RED: binary conflict resolution offers `git checkout --theirs`/`--ours` selection.
- [ ] 10.16 GREEN: implement `git.Service` theirs/ours action for `Binary` conflicts.
- [ ] 10.17 `[U]` RED (AC3, continue-gate): unmerged/unstaged paths remain → disabled with pending detail; staged `<<<<<<<` → blocked naming the file; clean+no markers → enabled.
- [ ] 10.18 GREEN: implement pure `continueGate(RepoState, stagedBlobs)` predicate.
- [ ] 10.19 `[T]` RED (AC4, live re-poll): external resolution (simulated outside the TUI step) is reflected automatically on next re-poll tick.
- [ ] 10.20 GREEN: implement `tea.Tick`-driven `RepoState` re-poll in `internal/app` (Model derives UI from repo, never execs).
- [ ] 10.21 `[I]` RED (AC5): once the gate passes, `git cherry-pick --continue` runs non-interactively (`GIT_EDITOR=true`, no hang).
- [ ] 10.22 GREEN: implement continue action wiring gate + `git.Service.ContinueCherryPick()`.
- [x] 10.23 `[I]` RED (AC6): `--continue`/`--abort` run outside DeployDeck during a multi-commit sequencer run is detected on reread (real `.git/sequencer/todo` remaining count) and resynchronizes.
- [x] 10.24 GREEN: implement reconciliation re-read on every `RepoState()` call (no cached state trusted).
- [ ] 10.25 `[I]` RED (AC7a): confirmed abort runs `git cherry-pick --abort`, marks the run aborted.
- [ ] 10.26 GREEN: implement Abort action.
- [ ] 10.27 `[I]` RED (AC7b): abort after partial picks offers temp-branch cleanup.
- [ ] 10.28 GREEN: implement partial-sequence detection + cleanup offer.
- [ ] 10.29 `[I]` RED (AC8): content already applied under a different SHA → empty pick detected, `--skip` offered with an explanatory message (implemented regardless of merge/squash policy, DEC-001 is process context only).
- [ ] 10.30 GREEN: implement empty-pick detection + `--skip` action.
- [ ] 10.31 `[I]` RED (AC9): `git rerere` auto-resolves a repeat conflict — labeled auto-resolved-from-prior, requires explicit user confirmation before continue.
- [ ] 10.32 GREEN: implement rerere detection + confirmation-required flag; suggest enabling rerere when unset.
- [ ] 10.33 `[I]` RED (AC10): after all picks, `git diff HEAD <source> -- <files>` on a touched file with remaining diff shows a per-file partial-promotion warning before any delta step.
- [ ] 10.34 GREEN: implement post-pick verification against the source branch.
- [ ] 10.35 `[U]` RED (AC11): given a failed-cherry-pick run state, delta-generation and Salesforce-validation are never invoked.
- [ ] 10.36 GREEN: implement failure short-circuit gating downstream steps.
- [ ] 10.37 `[U]` RED (seam invariant): `internal/app` never imports `internal/exec`; `tea.ExecProcess` used only for interactive handoff ($EDITOR / manual conflict resolution), never for git/sf commands — automated `go list -deps` boundary test.
- [ ] 10.38 GREEN: remove any direct `internal/exec` usage from `internal/app`; route all git/sf calls through `git.Service`/`salesforce.Client`.

## Phase 11: `internal/app` wiring (composition)

- [ ] 11.1 Wire `Model`/`New(deps)` composing services from Phases 5–10 per the state-machine diagram (`PrereqCheck → ... → PickVerification`, `PickVerification → CommitSelection` on partial promotion).
- [ ] 11.2 `[T]` RED: direct `Model.Update()` call drives `PrereqCheck → TicketInput` on OK prereqs.
- [ ] 11.3 GREEN: implement that transition per design's state diagram.
- [ ] 11.4 `[T]` RED: full-flow smoke test — `PrereqCheck → PickVerification` happy path on `newTempRepo` (Success Criteria: full TUI flow works on a temp repo).
- [ ] 11.5 GREEN: complete remaining `Update`/`View` wiring found by 11.4.
- [ ] 11.6 Extend the 10.37 dependency-boundary test to cover all of `internal/app`, not just cherry-pick.

## Phase 12: Final verification

- [ ] 12.1 Run `go test ./...` (unit + integration) green in CI.
- [ ] 12.2 Run `go vet ./...` and `gofmt -l .` clean.
- [ ] 12.3 Check off proposal.md Success Criteria: `doctor` non-zero on blockers; full TUI flow on temp repo; all six HUs' tests green, no real e2e org required.
