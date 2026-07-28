# Exploration: HU-017 — Limpieza De Ramas Y Runs (`cleanup`)

Sixth slice, stacked on `re-promote`. HU-017 (`docs/HISTORIAS.md:1079-1114`) is operational: it adds branch-lifecycle plumbing (none exists today) and wires the ALREADY-COMPLETE run-retention mechanism into the TUI. Anchor on REAL code, not the ARQUITECTURA sketch.

## Current state (code-anchored)
- **No branch-identity / branch-mutation plumbing exists.** `Grep CurrentBranch|abbrev-ref|branch -[dD]|push.*--delete` → zero hits outside archived docs. `git.Service` (`internal/git/service.go:15-22`) today: `RepoRoot`, `ListBranches`/`CandidateBranches` (`service_branches.go:20,34`), `BranchExists`/`RemoteHead`/`revParseVerify` (`service_target.go:39-57`), `CreatePromotionBranch` (only `checkout -b`), `Push` (`service_push.go:15`, `-u origin <branch>`), `RemoteURL`, `Status`/`RepoState`. Every method funnels through `newRequest` (`request.go:19-26`) which injects the non-interactive env — new methods inherit it free. `Branch{Name,Remote}` (`service_branches.go:10-15`) has NO commit-date/age field.
- **Retention EXISTS end-to-end but is CLI-only.** `selectPruneCandidates` (pure, `retention.go:15`) + `Writer.Prune` (`:34`); config `DefaultRunsKeepLast=30`/`DefaultRunsKeepDays=90` + `RunsConfig` (`config.go:13-16,51-54`); wired to `deploydeck runs prune` (`cmd/deploydeck/main.go:72-128`). `Prune`'s ONLY callers are the CLI + tests — `internal/app` never calls it. `run-retention/spec.md:5`: *"a future HU-017 branch cleanup MUST reuse this config and mechanism rather than reimplement."*
- **Pipeline shape.** Linear Bubble Tea machine (`app.go:35-120`, 20 states, NO main menu — HU-018's "menu" is aspirational). `StateTicketInput` (`app.go:40`) is the de-facto hub (landing after prereqs `update.go:104`, after declining resume `keys.go:520-523`). `StateRunHistory` is startup-only. **17 `return m, tea.Quit` sites** in `keys.go` — none do cleanup today; `main.go:236` discards the final model.
- **Push status is NOT persisted** (`runs.Record` `writer.go:29-74` has no `Pushed`); HU-014 push only flips ephemeral model state. Derivable live from git instead.
- **HU-016 explicitly deferred** "clean the prior temp branch" to HU-017 (`archive/re-promote/archive-report.md:235-239`). This HU satisfies it.
- **Harness precedent matches the story**: `helpers_test.go:50-144` (`newTempRepo`, `newTempRepoWithRemote` — bare `origin`, real git subprocess) + `flow_e2e_test.go:47`. No new harness needed.

## Resolved decisions
1. **Original-branch restore**: new `git.CurrentBranch` captured once at `onPrereqDone` (`update.go:88-106`) into `Model.originalBranch`. Restore via a new `m.quitCmd()` that runs `git.Checkout(originalBranch)` SYNCHRONOUSLY in the tea.Cmd closure, then returns `tea.Quit`'s msg (Bubble Tea runs cmds to completion before the msg → checkout finishes before `Program.Run()` returns; zero `main.go` change). Swap all 17 `return m, tea.Quit` → `return m, m.quitCmd()`. **Guard (load-bearing)**: skip restore when `m.repoState.InProgress` (a `q` mid-conflict must NOT checkout — git refuses unmerged paths AND HU-013 resume-detection reads `CHERRY_PICK_HEAD` off the current branch), or `originalBranch` is `""`/`"HEAD"` (detached) or `== currentBranch`, or original no longer `BranchExists`.
2. **New `StateBranchCleanup` screen** (not folded into the dense, startup-only `StateRunHistory`). Entry key `b` from `StateTicketInput` (the hub); `q`/`esc` returns there (mirrors `keyRunHistory`).
3. **Delete triggers = two**: (a) the CURRENT run's temp branch offered INLINE at `StateSucceeded`/`StateAborted` (`m.branchName` is live there) behind a confirm; (b) PRIOR orphans only in `StateBranchCleanup` batch screen. **"Merged" is OUT for real gh detection** (`github.Client` only has `AuthStatus`/`CreatePR`; harness is org-free). Use a git-native best-effort LABEL: after fetch, `git merge-base --is-ancestor <deploy> origin/<target>` (exit-code-as-data, like `revParseVerify`) → label "likely merged" vs "abandoned"; the delete ALWAYS stays behind explicit confirmation regardless of label, so a false negative just asks for a confirm it didn't strictly need.
4. **Push-status & unpushed detection (live from git, no persisted field)**: push status = does `origin/<branch>` resolve (`revParseVerify` pattern). Unpushed = new `git.Service` method `git rev-list origin/<branch>..<branch> --count`, or ALL commits when no remote ref exists. Age = extend a struct via `git for-each-ref --format='%(refname:short)|%(committerdate:iso-strict)'` scoped to `deploy/*` (like `branchesByPattern`) — ONE call, avoids N+1. All plain porcelain, safe in temp+bare-remote, no org.
5. **Retention surface**: invoke the EXISTING `Prune` from `StateBranchCleanup` (NOT auto-fire on flow end — surprising destructive side effect, contra the codebase's "confirm before destructive" precedent; NOT CLI-only — AC3 wants a TUI surface). **Do NOT change `Prune`/`selectPruneCandidates`/config/`Record` schema** — HU-017 is purely a new caller.
6. **Scope OUT**: real PR-merge gh/API polling; cross-repo/multi-remote (only `origin` handled anywhere); HU-018 (menu)/HU-019; any change to retention rule/defaults/schema. IN: this HU satisfies HU-016's deferred temp-branch cleanup.

## New capability + git.Service surface
- **NEW spec domain `branch-cleanup`** (no existing spec covers it; `promotion-branch`/`run-retention` are narrower and owned by other HUs). References the unchanged retention mechanism rather than modifying `run-retention`.
- New `git.Service` methods (all exec-free-preserving; `internal/app` calls only via tea.Cmd): `CurrentBranch`, `Checkout` (plain, no `-b`), delete-local (`branch -D`), delete-remote (`push origin --delete`), `UnpushedCommits`/ahead-count, `IsMergedInto` (ancestor), and a `deploy/*`-with-age lister (single `for-each-ref`). Use `-D` force delete AFTER the app's own unpushed/strong-confirm gate (git's `-d` safety would double-gate and mis-refuse squash-merged branches).

## ACs → testability (strict-TDD, temp + bare-remote + fs, NO org, CI: si — `:1108-1114`)
- finish/abort → original branch restored (`symbolic-ref --short HEAD` in temp dir); **variant: abort mid-conflict (`StateCherryPickConflict`+`q`) → NO restore** (guard negative test — load-bearing).
- confirm → temp branch deleted locally AND remote ref dropped in bare remote if pushed.
- orphans listed with age + push-status (seed `deploy/*` with differing `--date` commits, push only some).
- branch with unpushed commits → NOT deleted without STRONG (typed) confirm (reuse CANCELAR idiom `keys.go:540-579`); normal confirm refused, strong confirm deletes.
- runs outside `keepLast`/`keepDays` pruned, recents kept — reuses existing `runs` tests; only the app-level wiring caller is new.

## Risks
- **Squash/rebase-merge blind spot**: `--is-ancestor` only sees ff/regular merges; a squash-merge is a new SHA → false "not merged" → extra confirm, NEVER a wrong delete. Document, don't treat as bug.
- **Restore-vs-resume guard** on `RepoState.InProgress` is load-bearing (protects HU-013). Explicit negative test required.
- **`-D` force** delete is deliberate (app gates first; git's `-d` would mis-refuse squash-merges).
- Large multi-file slice (new state + ~6 git methods + inline + batch screen + retention wiring) — batch the apply, mind the review-workload; single stacked branch, no origin, ships as one logical slice.

## Ready for Proposal: yes
Propose must lock: spec domain name `branch-cleanup`; exact key bindings (`b` entry, confirm + strong-confirm keys); and whether the inline delete offer and the batch orphan+retention screen ship as ordered task groups within one change (recommended) vs split.
