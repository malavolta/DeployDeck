# Tasks: Project Directory Resolution

## Review Workload Forecast

| Area | Files | Est. lines (add+del) |
|---|---|---|
| `internal/config` (Locate, ProjectRoot, ProjectDir field, Validate wiring + tests) | `locate.go`, `locate_test.go`, `config.go`, `project_root.go`, `project_root_test.go`, `validate.go`, `validate_test.go` | ~275 |
| `internal/prereq` (Checker split, gitignore two-candidate read, ~16 test-literal updates, nested gitignore scenarios) | `checker.go`, `checker_{repository,workingtree,hooks,gpgsign,gitignore}.go`, `helpers_test.go`, `checker_gitignore_test.go` + existing test literals | ~205 |
| `internal/salesforce` (Client dir param, compiler-enumerated) | `client.go`, `quick.go`, `cancel.go`, `validate.go`, `quick_test.go`, `cancel_test.go` | ~70 |
| `internal/delta` (repo-dir emission) | `service.go`, `service_test.go` | ~30 |
| `internal/app` (Deps roots + atomic zero-arg `NewChecker` + 26 `commands.go` reads + `view.go` + guard test + ~9-site ripple) | `app.go`, `app_test.go`, `commands.go`, `view.go`, `roots_guard_test.go`, `delta_validation_test.go`, `standalone_modes_e2e_test.go`, `resume_test.go`, `standalone_validate_test.go` | ~284 |
| `cmd/deploydeck` (composition root: `roots.go`, `main.go` wiring) | `roots.go`, `roots_test.go`, `main.go` | ~220 |
| `internal/delta` e2e nested fixture (regression guard, NOT a lever — proposal R5) | `generate_e2e_test.go` | ~70 |
| `internal/runs/writer.go` (doc-only) | `writer.go` | ~6 |
| Docs | `docs/ARQUITECTURA.md`, `README.md` | ~40 |
| **Subtotal before levers** | | **~1200** |
| Lever 1 applied: `view.go` "Repo:" label — code migrated to `GitRoot` (required for correctness, `Deps.Dir` is unset in production per ADR-3), dedicated test skipped | | −25 |
| Lever 2 applied: `internal/git` nested helper + `service_root_test.go` nested test cut (invariant already proven; nested e2e delta covers it indirectly) | | −45 |
| **Forecast after levers** | | **~1130** |

Lever 3 (`roots_guard_test.go`) is **not needed** and is **not applied** — the forecast clears budget after levers 1–2, and this test is ADR-3's compensating control for keeping `Deps.Dir`, worth keeping.

```text
Decision needed before apply: No
Chained PRs recommended: No
Chain strategy: pending
1200-line budget risk: Medium
```

Budget levers applied: **1** (view.go label, no dedicated test) and **2** (internal/git nested helper cut). Lever 3 (roots_guard_test.go) not applied. If actual implementation trends back over 1200, lever 3 is pre-approved next in the session's priority order — do not ask again.

### Suggested Work Units (commits within the single PR)

| Unit | Goal | Focused test command | Runtime harness | Rollback boundary |
|---|---|---|---|---|
| 1 | `internal/config`: `Locate` + `ProjectRoot` + `ProjectDir` field + `Validate` wiring | `go test ./internal/config/...` | N/A — pure package, no external command | Revert `internal/config/{locate,project_root}.go`, config.go field, validate.go call |
| 2 | `internal/prereq`: `Checker` split + gitignore two-candidate read | `go test ./internal/prereq/...` | N/A — filesystem-only, no external command | Revert `checker*.go` field renames; `Dir`→`GitRoot` rename is compiler-enumerated |
| 3 | `internal/salesforce`: `Client` dir param | `go test ./internal/salesforce/...` | N/A — asserts `exec.CommandRequest`, no real `sf` call | Revert `client.go`/`quick.go`/`cancel.go` signatures |
| 4 | `internal/delta`: `--repo-dir` emission | `go test ./internal/delta/...` | `go test ./internal/delta/... -run E2E` (nested fixture, opt-in) | Revert `service.go` buildArgs change |
| 5 | `internal/app`: Deps roots + atomic zero-arg `NewChecker` + commands/view routing | `go test ./internal/app/...` | N/A — Bubble Tea model tests only | Revert `app.go`, `commands.go`, `view.go`, ripple test sites together (atomic) |
| 6 | `cmd/deploydeck`: composition root wiring | `go test ./cmd/deploydeck/...` | `go build ./... && ./deploydeck doctor` against a real nested-layout checkout | Revert `roots.go` (new file, delete) + `main.go` wiring |
| 7 | Docs + final full-suite verification | `go test ./...` | `go build ./...` | N/A — docs only, no rollback risk |

## Scope Guardrails (do not create tasks beyond these)

- Do NOT wire `Config.Validate()` into `Load` — dead code stays dead; the enforcement point is `Config.ProjectRoot` (deferred follow-up: `config-validation-wiring`).
- Do NOT touch `internal/github` — verified `gh` resolves from a nested subdirectory.
- Do NOT reinterpret `delta.sourceDirs` — stays repo-root-relative (INVARIANT requirement).
- Do NOT delete `app.Deps.Dir` — it is the ADR-3 compatibility base; only ADD the three new fields.
- Do NOT add `internal/prereq.Checker.ProjectDir` — no Checker check runs `sf project` (ADR-4 rejected option).

## Phase 1: `internal/config` Foundation (pure, no dependents yet)

- [x] 1.1 RED: `config/locate_test.go` — `Locate` scenarios: found at `startDir`; found after walking up to the git-root bound; found in a subdir with the bound at the git root; not found anywhere; `stopDir == ""` probes `startDir` only; `stopDir` not an ancestor. Use `t.TempDir()`, no git binary.
- [x] 1.2 GREEN: create `internal/config/locate.go` — `Locate(startDir, stopDir string) (dir string, found bool)` per ADR-1.
- [x] 1.3 RED: `config/project_root_test.go` — `ProjectRoot`: `""` → `configDir`; relative → `filepath.Join(gitRoot, v)`; absolute → error naming the value; `..` segment → error naming the value.
- [x] 1.4 GREEN: create `internal/config/project_root.go` — `Config.ProjectRoot(gitRoot, configDir string) (string, error)` + shared `validateProjectDir` (Threat Matrix: path traversal via config).
- [x] 1.5 GREEN: `internal/config/config.go` — add `ProjectDir string \`yaml:"projectDir"\`` field with doc comment (relative to git root); no `applyDefaults` entry.
- [x] 1.6 RED: `config/validate_test.go` — add cases: absolute `projectDir` rejected; `..`-bearing `projectDir` rejected.
- [x] 1.7 GREEN: `internal/config/validate.go` — `Validate()` calls `validateProjectDir` (same shared rule as 1.4, second surfacing point per spec).

## Phase 2: `internal/prereq` Checker Split (clean, compiler-enumerated — ADR-4)

- [x] 2.1 RED: `prereq/checker_gitignore_test.go` — nested scenarios: entry only in `<artifactsRoot>/.gitignore` passes; entry only in `<gitRoot>/.gitignore` passes (fallback); neither → blocks with `FixCommand` naming the artifacts path; `AddGitignoreEntry` writes `<artifactsRoot>/.gitignore` and creates no file at the git root.
- [x] 2.2 GREEN: `internal/prereq/checker.go` — rename `Dir` → `GitRoot`, add `ArtifactsRoot` field; update doc comments.
- [x] 2.3 GREEN: `checker_{repository,workingtree,hooks,gpgsign}.go` — update the 5 `c.Dir` reads to `c.GitRoot` (compiler-enumerated).
- [x] 2.4 GREEN: `checker_gitignore.go` — rewrite `readGitignore` for the two-candidate read (`ArtifactsRoot` then, only if different, `GitRoot`), fix target `ArtifactsRoot`; **drop** the `c.Git.RepoRoot` exec call entirely (ADR-9: pure filesystem now).
- [x] 2.5 GREEN: mechanically update the ~16 `prereq.Checker{Dir: dir, ...}` test literals across the package to `{GitRoot: dir, ArtifactsRoot: dir, ...}` (compiler-enumerated, no behavior change). Note: `cmd/deploydeck/root_test.go`/`doctor_e2e_test.go` also construct `prereq.Checker{Dir: ...}` literals and will fail to compile until Phase 6 updates them alongside `main.go`/`roots.go` — deferred there deliberately (same package, same commit-step as ADR-5's `NewChecker` rewiring).
- [x] 2.6 GREEN: `internal/prereq/helpers_test.go` — add sibling `newNestedTempRepo(t, withOrigin bool) (gitRoot, projectDir string)`; never modify the existing flat `newTempRepo`.

## Phase 3: `internal/salesforce` Client Dir Parameter (compiler-driven sequencing)

- [x] 3.1 RED: `salesforce/quick_test.go` and `cancel_test.go` — extend to assert `exec.CommandRequest.Dir == projectDir`. For this signature change the RED is a **compile failure in the new test** — record it as such, do not work around it.
- [x] 3.2 GREEN: `internal/salesforce/client.go` — change `Client` interface: `QuickDeploy(ctx, jobID, targetOrg, dir string)`, `CancelDeploy(ctx, jobID, targetOrg, dir string)` (`dir` last, ADR-8); update both `*client` impls in `quick.go`/`cancel.go` in the same step.
- [x] 3.3 GREEN: run `go build ./...`, fix every compiler-enumerated caller (`internal/app/commands.go` `quickDeployCmd`, `cancelCmd` — deferred to Phase 5 since those lines also need `m.deps.ProjectDir` routing; do not touch other files here). No hand-written fake implements `salesforce.Client` (verified) — `FakeRunner` stubs key on Name+Args only and are unaffected. Also fixed compiler-enumerated `argcomposition_test.go` and `real_org_cancel_e2e_test.go` callers within the package itself.
- [x] 3.4 GREEN: `internal/salesforce/validate.go` — doc-only correction: `ValidateRequest.Dir` is the SFDX project root (holds `sfdx-project.json`), not "the repository root."

## Phase 4: `internal/delta` Repo-Dir Emission

- [x] 4.1 RED: `delta/service_test.go` — `buildArgs` emits `--repo-dir <Dir>`; assert `sourceDirs` still passed verbatim (repo-root-relative, NOT re-prefixed — regression guard for the `sourceDirs` invariant).
- [x] 4.2 GREEN: `internal/delta/service.go` — `buildArgs` appends `--repo-dir req.Dir`; update the function doc comment.

## Phase 5: `internal/app` Deps Split + Atomic `NewChecker` + Routing

- [x] 5.1 RED: `app/app_test.go` — assert `normalizeRoots`'s flat-layout invariant: `Deps{Dir: d}` alone yields `GitRoot == ProjectDir == ArtifactsRoot == d`.
- [x] 5.2 GREEN (ATOMIC — one step, per design sequencing constraint): `internal/app/app.go` — add `GitRoot`, `ProjectDir`, `ArtifactsRoot string` fields to `Deps`; add `normalizeRoots` (seeds the three roots from `Dir` when empty) called once from `New`; change `Deps.NewChecker` to zero-arg `func() (*prereq.Checker, error)`; in the SAME step, update every ripple call site to the new signature (actual ripple: `commands.go`'s 26 dir reads + `cancelCmd`/`quickDeployCmd`/`runPrereqCmd`, and `update_check_test.go`'s `fakeNewChecker` — `go vet ./internal/app/...` came back clean with no further sites; `delta_validation_test.go`/`standalone_modes_e2e_test.go`/`resume_test.go`/`standalone_validate_test.go` needed no signature-ripple edits, only the new RED assertions in 5.3/5.4). Not spread across multiple commits.
- [x] 5.3 RED: `app/delta_validation_test.go` — assert nested `Deps`: `deltaCmd` sends `Request.Dir == GitRoot` and `OutputDir` under `ArtifactsRoot`. Also fixed the pre-existing flat-layout `TestModel_DeltaCmd_ComposesGenerateAndSummarize`'s FakeRunner args to include Phase 4's `--repo-dir` (otherwise broken independently of Phase 5).
- [x] 5.4 RED: `app/standalone_modes_e2e_test.go` — assert nested `Deps`: `validateCmd`/`reportCmd`/`quickDeployCmd`/`cancelCmd` all send `Dir == ProjectDir` (capture via `FakeRunner`).
- [x] 5.5 GREEN: `internal/app/commands.go` — migrate the 26 `m.deps.Dir` reads to the correct root: `deltaCmd` → `GitRoot` (repo-dir + child cwd) and `ArtifactsRoot` (output dir); `validateCmd`/`reportCmd`/`quickDeployCmd`/`cancelCmd` → `ProjectDir`; `runPrereqCmd` → calls the zero-arg `m.deps.NewChecker()` closure (Phase 3.3's deferred callers land here — thread `ProjectDir` into `sf.QuickDeploy`/`sf.CancelDeploy`).
- [x] 5.6 GREEN: `internal/app/view.go` — migrate the context-bar read (line ~116) to `GitRoot` with test coverage (new `TestContextBar_NestedDeps_UsesGitRoot`); migrate the "Repo:" label read (line ~151) to `GitRoot` as well (required — `Deps.Dir` is unset in production) but **skipped** a new dedicated test for this one line (Lever 1). `runPackagePath` (line 1355) needed **no change** — display-only, relative path.
- [x] 5.7 GREEN: `internal/app/roots_guard_test.go` (new, ADR-6) — reads the package's non-test `.go` files and fails if `deps.Dir` appears outside `normalizeRoots`; confirms `boundary_test.go`'s allow-list stays untouched (`TestImports` is not reported by `build.ImportDir`). Caught and fixed one real false positive in `normalizeRoots`'s own doc comment.

## Phase 6: `cmd/deploydeck` Composition Root

- [x] 6.1 RED: `cmd/deploydeck/roots_test.go` — `resolveRoots` scenarios: flat repo (all three roots coincide); nested repo (distinct `gitRoot`/`projectDir`/`artifactsRoot`); not inside a git repo (probes `cwd` only, `Load(cwd)` preserves "not a git repository" as primary diagnostic); absolute/`..` `projectDir` aborts before any command runs.
- [x] 6.2 GREEN: create `cmd/deploydeck/roots.go` — `type roots struct{ GitRoot, ProjectDir, ArtifactsRoot string }`; `resolveRoots(ctx, g *git.Service, cwd string) (roots, config.Config, error)` implementing the resolution sequence (`os.Getwd` → `git.RepoRoot` → `config.Locate` → `config.Load` → `cfg.ProjectRoot` → `artifactsRoot = projectDir`); `newChecker(cfg config.Config, r roots) (*prereq.Checker, error)` (pure composition, no resolution).
- [x] 6.3 GREEN: `cmd/deploydeck/main.go` — `defaultRunTUI(cwd)` calls `resolveRoots`, wires `app.Deps{GitRoot, ProjectDir, ArtifactsRoot, Runs: runs.NewWriter(artifactsRoot), NewChecker: closure over (cfg, roots) → newChecker(cfg, roots)}` (Dir left unset per ADR-3); `defaultChecker(cwd)` and `runPrune(w, cwd)` follow the same `resolveRoots` call, always from the raw `cwd` (never re-derived from an already-resolved root — ADR-5).
- [x] 6.4 GREEN: verify `.deploydeck/lock` path uses `artifactsRoot`, not `gitRoot` — new `TestNewChecker_LockPathUsesArtifactsRoot`.
- [x] 6.5 VERIFY: `cmd/deploydeck/root_test.go` and `doctor_e2e_test.go` pass unchanged in behavior — Cobra `RunE` bodies kept their `os.Getwd()` + seam call unedited. Both files DID need the compiler-enumerated `Dir`→`GitRoot`/`ArtifactsRoot` literal rename (Phase 2 ripple, deferred here per plan). Also found and fixed one real pre-existing test bug in `TestNewRootCmd_Doctor_AllChecksPass_ExitsZero`: its inline `NewChecker` fake threaded the RunE-resolved `dir` (the real process cwd during `go test`, since the test never `t.Chdir`s) into `GitRoot`/`ArtifactsRoot`, previously masked because every OTHER check funneled through the Dir-blind `git.FakeRunner`; ADR-9's now-pure-filesystem gitignore check exposed it. Fixed by closing over the seeded `root` var directly instead of the ignored `dir` param.

## Phase 7: Regression Nets

- [x] 7.1 RED: `internal/delta/generate_e2e_test.go` — add the nested-fixture variant: copy `test-e2e-org/` into `<gitRoot>/project/`, set `sourceDirs` to the repo-root-relative `project/force-app`, run real `sf sgd source delta` from the nested layout.
- [x] 7.2 GREEN: confirm the nested e2e run's `Generate` produces a NON-EMPTY package (`package.xml` has `<types>` members) and the sgd invocation carried `--repo-dir <gitRoot>`. This is the only guard against a silently empty-but-successful delta package — not a lever, keep unconditionally. Ran against the real `sf`/`sgd` binaries in this environment — PASS.

  > Note (config-validation-wiring): the `--repo-dir <gitRoot>` flag
  > assertion above lives in `internal/delta/service_test.go` (unit level,
  > asserting `buildArgs`' composed argument list). Whether the flag is
  > merely EMITTED versus whether sgd actually HONORS it independently of
  > the child process's cwd is a distinct question — cwd-vs-`--repo-dir`
  > separation is covered by `config-validation-wiring`'s isolating e2e,
  > `TestSgd_RepoDirIndependentOfCwd_RealSgd` (`internal/delta/generate_e2e_test.go`),
  > which bypasses `delta.Service` and sets the two to deliberately
  > different directories.
- [x] 7.3 VERIFY: `internal/app/boundary_test.go` passes with no allow-list change.
- [x] 7.4 GREEN: `internal/runs/writer.go` — doc-only correction: `baseDir` is the ARTIFACTS root, not the repository root.

## Phase 8: Documentation

- [x] 8.1 Update `docs/ARQUITECTURA.md` — document the three-root model (`GitRoot`, `ProjectDir`, `ArtifactsRoot`) and which operation class binds to which root. New section written in English per the session's language contract (the rest of this file is Spanish); one existing Spanish sentence describing `.deploydeck/`'s location was also corrected in place for internal consistency.
- [x] 8.2 Update `README.md` — document the additive `projectDir` config key, its default (config file's directory), and validation rules (no absolute path, no `..`).

## Phase 9: Final Verification

- [x] 9.1 Run `go build ./...` — clean.
- [x] 9.2 Run `go test ./...` — green (full non-short suite, real git/sgd e2e included; `-race -short` also separately green).
- [x] 9.3 Confirm every proposal Success Criterion: flat-repo behavior unchanged (every pre-existing test passes unmodified in behavior); nested repo runs the full flow with zero `deploydeck.yaml` edits to `sourceDirs` (`TestService_Generate_RealSgd_NestedRepo` — real sgd, non-empty package); nested `Generate` produces a non-empty package with `--repo-dir <gitRoot>` (same test); nested gitignore check passes against the artifacts-root file with no untracked git-root file (`TestChecker_CheckGitignore_Nested`); all four `sf project deploy *` invocations assert `Dir == projectDir` (`TestModel_NestedDeps_SFCommands_UseProjectDir`, plus `quick_test.go`/`cancel_test.go`/`validate_test.go` unit coverage); absolute/`..` `projectDir` rejected on the resolution path (`TestConfig_ProjectRoot`, `TestResolveRoots_InvalidProjectDir_AbortsBeforeAnyCommandRuns`); `boundary_test.go` unchanged and green with no allow-list change.

  **BUDGET NOT MET — flagged for orchestrator decision, not silently absorbed.** Actual `git diff` (tracked + new untracked files, `openspec/` excluded) is **1707 insertions + 221 deletions = 1928 changed lines**, against the 1200 budget — ~728 lines over, despite both pre-applied levers (1 and 2) from the forecast. The single additional pre-approved lever (dropping `internal/app/roots_guard_test.go`, ~101 lines) was evaluated and **deliberately NOT applied**: even fully removed it leaves the diff at ~1827, nowhere near 1200, so cutting it would sacrifice ADR-6's real regression guard (explicitly noted in the forecast as "worth keeping") for no actual budget compliance. No further cut was pre-authorized, and inventing one unilaterally is out of scope for this executor role. Per-area actuals vs. forecast (largest overruns): `cmd/deploydeck` 464 vs ~220 (`roots_test.go` alone is 237 lines — six real-git scenarios with full init/commit sequences); `internal/config` 375 vs ~275; `internal/prereq` 306 vs ~205; `internal/app` 446 vs ~284 (includes the 101-line guard test); `internal/salesforce` 118 vs ~70; nested delta e2e 111 vs ~70 (not a lever, per instruction). All 40 tasks are implemented, tested, and green; this is purely a delivery-size/review-workload concern for the orchestrator to resolve (split into chained PRs post-hoc, or accept `size:exception`) before opening a single PR.

---

## Budget Resolution — `size:exception` ACCEPTED

**Decision**: the user accepted `size:exception` for this change. It ships as a single PR at
~1928 changed lines against the 1200-line budget. Recorded here so the exception is auditable rather
than implicit.

**Orchestrator-verified diff composition** (measured independently of the apply phase's report,
which matched exactly):

| Category | Lines |
|---|---|
| Production `.go` | 679 (460 modified + 219 new) |
| Test `.go` | 1198 (658 modified + 540 new) |
| Docs / other | 51 |
| **Total** | **1928** |

**Justification for the exception:**

1. **62% of the diff is test code.** The actual review surface — production logic a reviewer must
   reason about — is 679 lines, comfortably inside the 1200 budget on its own. The overage is a
   direct consequence of this project's own `strict_tdd: true` policy: every behavior change requires
   a preceding RED test, and this change touches six packages.
2. **Splitting would ship non-working slices.** The composition root (`cmd/deploydeck/roots.go`) is
   what resolves the three roots; every leaf fix depends on it. A PR excluding it leaves
   `config.Locate`/`ProjectRoot` as dead code, and leaves `delta`'s `--repo-dir` still receiving the
   process cwd — i.e. the original bug, unfixed. The design anticipated this: "any slice that
   excludes the composition root ships dead code."
3. **The remaining lever was correctly declined.** Dropping `internal/app/roots_guard_test.go`
   (~101 lines) leaves ~1827 — no meaningful budget compliance — while sacrificing ADR-6's
   compensating control for keeping `Deps.Dir` in the struct. The apply phase escalated rather than
   cutting it, which was the right call.

**Precedent**: `ai-pr-summary` shipped under `size:exception` at ~2280 lines by the same reasoning.

**Independent verification of green status** (run by the orchestrator, not inherited):
`gofmt -l` clean · `go build ./...` clean · `go vet ./...` clean · `go test ./... -race -short`
15/15 packages `ok`.
