# Verification Report: `project-dir-resolution`

**Verdict**: `pass-with-warnings` → **upgraded to `pass` after the orchestrator addendum below.**
**Blockers**: 0 · **Critical findings**: 0 · **Requirements**: 12/12 · **Scenarios**: 21/25 fully compliant, 4 partial

---

## Build & test execution (verify phase)

`go build ./...` exit 0 · `go vet ./...` exit 0 · `gofmt -l` empty ·
`go test ./... -count=1 -timeout 900s` exit 0 — **15/15 packages ok, 1517 tests passed, 0 failed,
7 skipped** (all 7 pre-existing env-gated live-credential e2e; none new).

Real `sf sgd source delta` e2e confirmed executing, not skipped:

```text
--- PASS: TestService_Generate_RealSgd_NestedRepo (4.14s)
--- PASS: TestService_Generate_RealSgd_TempRepo (1.74s)
--- PASS: TestService_Generate_RealSgd_MultiSourceDir (2.72s)
```

Diff measured independently: 1707 insertions + 221 deletions = **1928** changed lines. Matches the
apply report and the orchestrator's own measurement exactly.

## Spec compliance

21/25 scenarios COMPLIANT, 4 PARTIAL, 0 UNTESTED, 0 FAILING. All 12 requirements across the three
delta specs are implemented with a named, executing test per scenario. Two of the four PARTIALs
(W-2, N-3) were **spec-wording defects, not code defects**, and are corrected below.

All eight proposal Success Criteria: MET.

## Directed audits

| Audit | Result |
|---|---|
| Git operations → `GitRoot` | VERIFIED — all 20 `commands.go` git sites; `grep 'c\.Dir\b' internal/prereq/` returns zero hits |
| Four `sf project deploy *` → `ProjectDir` | VERIFIED — `commands.go:1000/1276/1297/1314` |
| sgd `--repo-dir` → `GitRoot`, **not** `ProjectDir` | VERIFIED — the distinction flagged as most likely to be silently wrong is correct, and mutation-verified |
| `.deploydeck/` + gitignore → `ArtifactsRoot` | VERIFIED — no site anchored to any other root |
| `delta.sourceDirs` NOT reinterpreted | VERIFIED — `internal/delta` summarize has **zero diff**; entries pass through verbatim with no `filepath.Join` |
| `Config.Validate()` NOT wired into `Load` | VERIFIED — deferred follow-up remains genuinely deferred |
| `internal/github` untouched | VERIFIED — absent from `git status` |
| `boundary_test.go` allow-list unchanged | VERIFIED — byte-identical to `HEAD` |

## Mutation verification — guards proven non-vacuous

The tests were not trusted to be meaningful; production code was deliberately broken four ways and
each guard confirmed to fire. All files restored and SHA-256 verified byte-identical afterwards.

| Mutation | Guard that fired |
|---|---|
| sgd binds to `ProjectDir` instead of `GitRoot` | `TestModel_DeltaCmd_NestedDeps_UsesGitRootAndArtifactsRoot` FAIL |
| `--source-dir` joined with `req.Dir` (the empty-package failure mode) | `TestService_Generate_ComposesArgs_TableDriven` FAIL on all 3 cases |
| `validateCmd` uses `GitRoot` instead of `ProjectDir` | `TestModel_NestedDeps_SFCommands_UseProjectDir` FAIL |
| `_ = m.deps.Dir` added inside `contextBar` | `TestNoDirReadsOutsideNormalizeRoots` FAIL (ADR-6 violation) |

**The worst failure mode this change guards against — a silently empty-but-successful delta package
— is genuinely covered.**

## Coverage of new production code

`resolveRoots` 100% · `Config.ProjectRoot` 100% · `validateProjectDir` 100% · `Locate` 92.3% ·
`configExists`/`isAncestorOrSelf` 100% · `newChecker` 85.7%.
Package aggregates: config 95.8%, delta 97.1%, salesforce 90.8%, app 85.0%, prereq 80.3%.

Assertion quality: no tautologies, no orphan assertions, no smoke-only cases. Every new test calls
production code and asserts a concrete value.

## Design coherence

All nine ADRs followed. **No design deviation found.**

---

## Findings

### CRITICAL
None.

### WARNING

- **W-1 — Symlinked checkout silently disabled the upward config search.** `config.Locate`'s bound
  check is lexical, but `git rev-parse --show-toplevel` returns the symlink-*resolved* path while
  `os.Getwd()` returns the logical one. On a symlinked checkout the two differ as strings, so the
  bound collapses to `startDir` and only the cwd is probed — the operator is told the config is
  missing while it sits one level up in the same repository. **RESOLVED — see the addendum below.**
- **W-2 — Spec scenario contradicted the implementation.** "Absent projectDir defaults to the git
  root" conflicted with its own sibling requirement ("*not* unconditionally the git root"). A spec
  defect, not a code defect. **RESOLVED — reworded.**
- **W-3 — No test binds a `commands.go` git call to `GitRoot` specifically in a nested layout.** The
  ADR-6 guard proves no site reads `deps.Dir`, but would pass equally if a git site read
  `ProjectDir`. Nothing is broken today: the requirement's own disjunction is satisfied because
  `internal/git` self-resolves via `RepoRoot`, proven by `TestResolveRoots_NestedRepo` against real
  git. Residual gap created by budget lever 2. **OPEN — accepted.**
- **W-4 — No `apply-progress` artifact.** No Engram tooling in this session; `tasks.md` labels every
  task RED/GREEN/VERIFY and every claimed test was independently confirmed to exist and pass. A
  pipeline-artifact gap, not an evidence gap. **OPEN — accepted.**
- **W-5 — Stale justification comment.** `internal/prereq/checker_aliases.go:32` still reads
  *"config.Validate() already rejects a missing alias; skip"* — a check skipped on the strength of
  validation that never runs. Correctly out of scope here. **CARRIED into the
  `config-validation-wiring` follow-up.**

### SUGGESTION (all open, none blocking)

- **S-1** — `QuickDeploy`/`CancelDeploy` accept `dir string` with no empty-value guard;
  `CommandRequest.Dir == ""` means "inherit process cwd", precisely the bug class this change
  removes. Consider rejecting empty at the shim.
- **S-2** — The nested delta e2e sets both child cwd and `--repo-dir` to `gitRoot`, faithfully
  reproducing production, so the flag is not proven load-bearing end to end. A variant with cwd at
  `projectDir` would close that.
- **S-3** — Adding `projectDir` to a config living *at the git root* relocates `ArtifactsRoot`, so
  `.deploydeck/` moves. Holds harmless for existing installs (they keep the config at their project
  root and set no `projectDir`), but neither README nor ARQUITECTURA says so. One sentence would
  prevent a surprised operator.
- **S-4** — Task 7.2 overstates its test: the nested e2e does not itself assert `--repo-dir`; only
  `service_test.go` does.

### NOTE — the apply phase's "pre-existing bugs" claim, disputed

- **N-1** — `cmd/deploydeck/root_test.go` was **not** a pre-existing bug. Under the old design
  `Checker.Dir` was only the starting point for `c.Git.RepoRoot(ctx, c.Dir)`, stubbed by
  `FakeRunner` to return the seeded root (confirmed via `git show HEAD:...`). The old test was
  correct; ADR-9's move to a pure-filesystem gitignore check is what made the fixture wrong. **The
  fix itself is correct and is not a test bent to pass** — the assertion is unchanged, and leaving
  `dir` in place would have made the check read the `go test` process cwd. The characterization is
  disputed, not the change.
- **N-2** — `roots_test.go`'s `EvalSymlinks` need is not a pre-existing bug either; the file is new.
  It is the same root cause as W-1 surfacing in test code.
- **N-3** — Spec wrote `config.Locate(startDir)`; the implementation is `Locate(startDir, stopDir)`
  per ADR-1, which is the better design. **RESOLVED — spec corrected.**

---

## Orchestrator addendum — post-verification remediation

Three of the five warnings were closed after the verify phase returned. Recorded here so the report
matches the shipped state rather than the state at verification time.

### W-1 fixed under strict TDD

The symlink finding was **independently reproduced by the orchestrator** before acting on it:

```text
os.Getwd()   = .../scratchpad/symcheck/link/repo/sub
git toplevel = .../scratchpad/symcheck/real/repo
lexically related? NO
```

**RED** — added `TestResolveRoots_SymlinkedCheckout_StillSearchesUpward` to
`cmd/deploydeck/roots_test.go`. It failed for exactly the predicted reason:

```text
resolveRoots() through a symlinked path: config: reading .../link/repo/sub/deploydeck.yaml:
no such file or directory
```

**GREEN** — `resolveRoots` now canonicalises its cwd with `filepath.EvalSymlinks` before resolving
anything else, so every subsequent path comparison happens in one path space. A resolution failure
degrades to the raw cwd rather than aborting, because a nonexistent path is already reported far
more usefully downstream by `RepoRoot` or by `Load`'s own read error.

**Collateral, and why it is not a test bent to pass**: `TestResolveRoots_NotInsideGitRepo` broke,
because it used `t.TempDir()` (`/var/...`) and asserted the roots equalled that *unresolved* string
— true only incidentally. Its stated intent is that all three roots COLLAPSE to the cwd, not which
spelling is returned. The fixture now uses the file's own established `realTempDir` helper, the same
one every other `resolveRoots` test already uses. The assertion is unchanged.

### W-2 and N-3 fixed

Both spec-wording defects corrected in
`specs/directory-resolution/spec.md` before archive: the `projectDir` default scenario now reads
"defaults to the config file's directory, which in a flat layout is the git root", and `Locate`'s
signature is corrected to `Locate(startDir, stopDir)` with ADR-1's rationale and the symlink
canonicalisation requirement stated.

### Re-verification after the fix

Run by the orchestrator, not inherited:

| Command | Result |
|---|---|
| `gofmt -l .` | clean |
| `go build ./...` | clean |
| `go vet ./...` | clean |
| `go test ./... -race -short` | **15/15 packages ok** |
| `go test ./... -count=1 -timeout 900s` (full, real git/sf/sgd e2e) | **15/15 packages ok** — git 63.2s, app 31.2s, delta 10.5s |

### Remaining open items (none blocking)

W-3, W-4, S-1 through S-4 are accepted as-is and recorded above. W-5 is carried into the
`config-validation-wiring` follow-up documented in `proposal.md`.

---

## Result contract

- **status**: `pass` (was `pass-with-warnings`; W-1, W-2 and N-3 closed by the addendum above)
- **executive_summary**: All 12 requirements implemented and all 40 tasks complete. 21/25 scenarios
  fully compliant, the 4 partials being two spec-wording defects (now fixed), one budget-lever gap,
  and one pipeline-artifact gap. The three highest-risk invariants — sgd→`GitRoot`,
  `sf project`→`ProjectDir`, and `sourceDirs` staying repo-root-relative — were mutation-verified to
  be genuinely guarded. One real latent defect found in new code (symlinked checkouts silently
  disabling the upward config search) was reproduced, fixed test-first, and re-verified.
- **artifacts**: `openspec/changes/project-dir-resolution/verify-report.md`
- **next_recommended**: `sdd-archive`
- **risks**: No blocking risk. Carried: S-1 (an empty `dir` passed to `QuickDeploy`/`CancelDeploy`
  silently reverts to process-cwd inheritance with no guard); W-3 (no dedicated nested-layout test
  binding a git call to `GitRoot`); W-5 (stale comment carried into the follow-up change).
- **skill_resolution**: `paths-injected` — go-testing, plus phase and shared protocol skills.
