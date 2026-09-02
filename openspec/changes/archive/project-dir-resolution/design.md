# Design: Project Directory Resolution

## Technical Approach

One resolver at the composition root turns the process cwd into an immutable triple
`(gitRoot, projectDir, artifactsRoot)`; every downstream consumer reads the root it semantically
needs instead of a single conflated `dir`. The triple is computed by a new `cmd/deploydeck`
helper `resolveRoots` over a new PURE `config.Locate` (bounded upward `deploydeck.yaml` search,
no git dependency) and a new PURE `Config.ProjectRoot` (composition + path guard). `internal/exec`
stays the only execution boundary and `internal/app` gains no import (plain `string` fields only,
`boundary_test.go` allow-list untouched). Maps proposal scope items 1-6.

The load-bearing constraint on shape is the review budget: `internal/app` has **241
`Deps{... Dir: ...}` construction sites across 34 test files** (measured). Deleting `Deps.Dir` to
force compiler enumeration would add roughly +480 diff lines in test files alone and blow the
1200-line budget on its own. `internal/prereq` has only **6 `c.Dir` reads and ~16 test sites**, so
it can afford the clean compiler-enforced split. That asymmetry is deliberate and is ADR-3/ADR-4.

## Resolution Sequence

    os.Getwd() ──► cwd
      │
      ▼
    git.RepoRoot(ctx, cwd)
      ├── err (cwd not inside a git repo) ──► gitRoot = cwd ; stopDir = cwd   (probe cwd only)
      └── ok ───────────────────────────────► gitRoot = <toplevel> ; stopDir = gitRoot
      │
      ▼
    config.Locate(cwd, stopDir) ── (dir, true) ──► configDir = dir
      │                         └─ (_,  false) ──► configDir = cwd
      │                                            (Load then emits today's EXACT
      │                                             "config: reading <cwd>/deploydeck.yaml" error)
      ▼
    config.Load(configDir) ──► cfg
      │
      ▼
    cfg.ProjectRoot(gitRoot, configDir)
      ├── cfg.ProjectDir == ""        ──► projectDir = configDir
      ├── absolute or contains ".."   ──► ERROR (actionable, abort before any command runs)
      └── otherwise                   ──► projectDir = filepath.Join(gitRoot, cfg.ProjectDir)
      │
      ▼
    artifactsRoot = projectDir                                      (ADR-2)
      │
      ├──► app.Deps{GitRoot, ProjectDir, ArtifactsRoot,
      │             Runs: runs.NewWriter(artifactsRoot),
      │             NewChecker: func() (*prereq.Checker, error) { newChecker(cfg, roots) }}
      └──► prereq.Checker{GitRoot, ArtifactsRoot}
                 + prereq.NewLock(<artifactsRoot>/.deploydeck/lock, ...)

`resolveRoots(ctx, g, cwd)` is called from `defaultRunTUI(cwd)`, `defaultChecker(cwd)` and
`runPrune(w, cwd)` — always from the RAW cwd, never re-derived from an already-resolved root
(re-resolving from `projectDir` can miss a sibling `configDir`; see ADR-5's rejected option).
Cobra `RunE` bodies keep their current `os.Getwd()` + seam call, so `cmd/deploydeck/root_test.go`
and `doctor_e2e_test.go` are untouched.

## Root → Consumer Map

| Root | Consumers | Why |
|---|---|---|
| `GitRoot` | all ~20 `g.<GitMethod>(ctx, ...)` sites in `commands.go`; `deltaCmd`'s `delta.Request.Dir` (→ cwd AND `--repo-dir`); `g.ChangedFiles`; `view.go` context bar + `Repo:` label; `prereq` repository/worktree/hooks/gpgsign checks; gitignore FALLBACK read | sgd does not walk up; `git.Service` self-resolves so the git sites are already correct — they read `GitRoot` for CONSISTENCY (a `git` call reading a non-git-named field is exactly the conflation this change removes) |
| `ProjectDir` | `ValidateDeploy`, `ReportDeploy`, `QuickDeploy`, `CancelDeploy` (all four `sf project deploy *`) | `sf project` requires `sfdx-project.json` in cwd |
| `ArtifactsRoot` | `deltaCmd`'s `outputDir`; `runs.NewWriter`; the `.deploydeck/lock` path; gitignore check PRIMARY read + fix target | `.deploydeck/` must not relocate for any existing install |

`view.go:1355 runPackagePath` needs **no change**: it returns a path RELATIVE to the artifacts
root (`deltaBaseDir(cfg)/<ticket>-to-<target>/package/package.xml`), is display-only, and is never
joined to a root. It cannot drift from `deltaCmd` because both compose the same relative segment
under the same `ArtifactsRoot`.

## Architecture Decisions

| # | Decision | Choice | Rejected | Rationale |
|---|---|---|---|---|
| ADR-1 | `Locate` signature | PURE `config.Locate(startDir, stopDir string) (dir string, found bool)` — walks `filepath.Dir` upward probing `<d>/deploydeck.yaml`, stops AFTER probing `stopDir`, or probes `startDir` only when `stopDir` is empty/not an ancestor | `Locate(startDir)` resolving the git root internally (explore B2c) | `internal/config` is a leaf and MUST NOT reach `internal/exec`/git (ARQUITECTURA boundary). Caller-supplied bound keeps it pure and `t.TempDir()`-testable with no git binary. `(string, bool)` mirrors the house `SandboxFor`/`GateFor` idiom; not-found needs no new error type because falling back to `startDir` reproduces `Load`'s existing message verbatim |
| ADR-2 | Artifacts root | `artifactsRoot = projectDir`, but carried as its OWN named field, not aliased at the call site | one `ProjectDir` field serving both | `.deploydeck/` must not move for any existing install, so the VALUE must equal `projectDir`; the NAME must stay separate because (a) it is a policy decided once at the composition root, not a fact, and (b) a test can set `ArtifactsRoot != ProjectDir` and prove `deltaCmd` writes to the artifacts root — an assertion that is unexpressible if the two collapse |
| ADR-3 | `app.Deps` shape | ADD `GitRoot`, `ProjectDir`, `ArtifactsRoot`; KEEP `Dir` as the zero-value-safe **compatibility base**, normalized once in `New`: `GitRoot ??= Dir`, `ProjectDir ??= Dir`, `ArtifactsRoot ??= ProjectDir`. Production sets the three roots and leaves `Dir` unset | delete `Dir` so the compiler enumerates all 28 reads | 241 measured `Deps{...Dir:...}` test sites; deleting `Dir` costs ~+480 diff lines and exceeds the budget alone. The fallback IS the repo's established additive/zero-value-safe pattern (`QuickDeployConfig`, `Gates`, `AIConfig`), and it makes every existing test assert the flat-layout invariant for free. Compensating control: ADR-6's guard test |
| ADR-4 | `prereq.Checker` shape | CLEAN split, no fallback: RENAME `Dir` → `GitRoot`, ADD `ArtifactsRoot`. Repository/working-tree/hooks/gpgsign → `GitRoot`; gitignore check + `AddGitignoreEntry` → `ArtifactsRoot` (with `GitRoot` fallback read). NO `ProjectDir` field | a third `ProjectDir` field mirroring `app.Deps` | Only 6 non-test reads + ~16 test sites, so the compiler CAN enumerate here — take the clean shape where it is affordable. No Checker check runs an `sf project` command (`CheckSF` is `sf --version`/`sf plugins`, cwd-independent) and `Config` arrives pre-loaded, so a `ProjectDir` field would be dead weight. `Checker` has no constructor, so a fallback would need accessor methods — worse than the rename |
| ADR-5 | `NewChecker` seams | `app.Deps.NewChecker` becomes **zero-arg** `func() (*prereq.Checker, error)`, wired by `defaultRunTUI` as a closure over the already-resolved `(cfg, roots)`. `cmd/deploydeck.Deps.NewChecker func(dir string)` is UNCHANGED (`dir` = raw cwd; `defaultChecker` resolves) | keep `func(dir string)` in app and re-resolve inside | Re-resolution is not idempotent across roots: with `deploydeck.yaml` in `<gitRoot>/a` and `projectDir: b`, re-resolving from `projectDir` probes `b` then `gitRoot` and finds nothing. `internal/app` must not choose directories the composition root already decided; a param the closure ignores is worse than no param |
| ADR-6 | Migration guard | New `internal/app/roots_guard_test.go`: reads the package's non-test `.go` files and fails if `deps.Dir` appears outside `normalizeRoots` | trust a one-off grep during apply | ADR-3 trades away compiler enforcement; this buys it back as an executable invariant. A `_test.go` file only adds `TestImports`, which `build.ImportDir(".",0).Imports` does not report — `boundary_test.go`'s allow-list is provably untouched |
| ADR-7 | sgd repo root | Reuse `delta.Request.Dir` for BOTH the child cwd and a newly-emitted `--repo-dir <Dir>`; caller passes `GitRoot`. Doc updated; NO new field | add a separate `Request.RepoRoot` alongside `Dir` | Both values are `gitRoot` in every reachable configuration, so a second field would be permanently redundant and immediately ambiguous — the defect class this change removes. `--source-dir` is repo-dir-relative (verified) so `sourceDirs` stays repo-root-relative and untouched; `--output-dir` stays absolute and is therefore independent of the child cwd. Saves ~15 lines. If cwd and repo-dir ever must diverge, add the field then |
| ADR-8 | `sf` dir parameter | `QuickDeploy(ctx, jobID, targetOrg, dir string)` and `CancelDeploy(ctx, jobID, targetOrg, dir string)` — `dir` LAST, mirroring the existing `ReportDeploy(ctx, jobID, targetOrg, dir)` | `dir` first; a `Request` struct per method | Positional consistency with the one method that already carries a dir; a struct for 3 scalars is ceremony the package does not use for `Report`/`Cancel` |
| ADR-9 | Gitignore lookup | Two-file read: `<ArtifactsRoot>/.gitignore` then, only when different, `<GitRoot>/.gitignore`; the entry in EITHER passes. Fix always writes `<ArtifactsRoot>/.gitignore`. The `RepoRoot` exec call is DROPPED — the check becomes pure filesystem | `git check-ignore --quiet <artifactsRoot>/.deploydeck/` | `check-ignore` is strictly more faithful: it honors `.git/info/exclude`, `core.excludesFile`, intermediate `.gitignore` files and `!` negations. It costs a new `git.Service` method whose exit code **1 means "not ignored" — data, not failure** — so `internal/exec`'s exit-code contract must be re-interpreted at a new call site, plus a nested-repo integration test. Deferred, not dismissed. See "Failure modes" below |

## Failure Modes of ADR-9 (stated honestly)

| Case | Direction | Consequence | Why acceptable |
|---|---|---|---|
| Entry lives in `.git/info/exclude`, a global excludes file, or an intermediate `.gitignore` between the two probes | **False block** | Doctor reports blocking and offers to append a redundant `.deploydeck/` line | The fix is idempotent, one line, in the right file, and leaves the repo correct either way |
| A `!.deploydeck/` negation elsewhere re-includes the directory | **False pass (fails open)** | Check says OK while git would actually track `.deploydeck/` | Never false-blocks a working repo; and the consequence is not silent — `CheckWorkingTree`'s repo-wide `git status --porcelain` immediately reports the untracked artifacts, so a downstream blocking check catches it |
| Not inside a git repo | check no longer errors, reads `<cwd>/.gitignore` | Reports blocking instead of an error | `CheckRepository` already reports "is not inside a git repository" as the primary blocking diagnostic; the old error only masked it |

Existing nested installs that already carry `.deploydeck/` in `<gitRoot>/.gitignore` keep passing
via the fallback read (a pattern without a leading `/` matches at any depth), which is exactly why
the fallback exists.

## Interfaces / Contracts

```go
// internal/config — PURE, no exec, no git.
type Config struct { /* ... */
    // ProjectDir is the SFDX project's location RELATIVE TO THE GIT ROOT
    // (e.g. "up_saln0001_giss_salesforce"). Empty (default) means "the
    // directory deploydeck.yaml was found in IS the project root" — today's
    // behavior for every existing install, unchanged.
    ProjectDir string `yaml:"projectDir"`
}
func Locate(startDir, stopDir string) (dir string, found bool)
func (c Config) ProjectRoot(gitRoot, configDir string) (string, error) // "" → configDir; abs/".." → error
func (c Config) Validate() error                                      // now also calls validateProjectDir

// internal/app — plain strings only; NO new import (boundary_test.go allow-list untouched).
type Deps struct { /* ... */
    Dir           string // compatibility base (ADR-3): seeds the three roots when they are empty
    GitRoot       string
    ProjectDir    string
    ArtifactsRoot string
    NewChecker    func() (*prereq.Checker, error) // was func(dir string) (...)
}

// internal/prereq
type Checker struct {
    GitRoot       string // MUST be the resolved repository root: the gitignore fallback reads
                         // <GitRoot>/.gitignore directly instead of re-resolving it
    ArtifactsRoot string // where .deploydeck/ actually lives; primary gitignore probe and fix target
    /* Git, SF, Config, Lock, GH, AI unchanged */
}

// internal/salesforce
CancelDeploy(ctx context.Context, jobID, targetOrg, dir string) (CancelResult, error)
QuickDeploy(ctx context.Context, jobID, targetOrg, dir string) (QuickDeployResult, error)
// ValidateRequest.Dir doc corrected: the SFDX PROJECT ROOT (the directory holding
// sfdx-project.json) the sf CLI runs in — NOT "the repository root".

// internal/delta — Request.Dir now doubles as the emitted --repo-dir (ADR-7).
// buildArgs: sgd source delta --from .. --to .. --output-dir .. --generate-delta
//            --repo-dir <Dir> (--source-dir X)... [--ignore-file ..] [--ignore-destructive-file ..]

// cmd/deploydeck (new roots.go)
type roots struct{ GitRoot, ProjectDir, ArtifactsRoot string }
func resolveRoots(ctx context.Context, g *git.Service, cwd string) (roots, config.Config, error)
func newChecker(cfg config.Config, r roots) (*prereq.Checker, error) // pure composition, no resolution
```

### Sequencing the `salesforce.Client` change so the compiler finds every site

1. RED: extend `quick_test.go`/`cancel_test.go` to assert `exec.CommandRequest.Dir == projectDir`.
   For a signature change the RED is a **compile failure in the new test** — record it as such.
2. Change the `Client` interface and both `*client` impls in one commit-step.
3. `go build ./...` now fails at exactly the remaining callers: `commands.go` `quickDeployCmd` and
   `cancelCmd`. **Verified: no hand-written fake implements `salesforce.Client`** — the three
   `salesforce.Client`-returning test helpers (`resume_test.go:31`, `delta_validation_test.go:243`,
   `standalone_modes_e2e_test.go:337`) all return `salesforce.New(FakeRunner)`. `FakeRunner` keys on
   Name+Args only, so adding `Dir` does **not** invalidate any existing `fr.When(...)` stub.

### Where `ProjectDir` validation actually bites

`Config.Validate()` is **not called anywhere in production code** (verified: only `_test.go` files
and two `internal/git` e2e tests call it). Satisfying the acceptance criterion through `Validate()`
alone would therefore have zero runtime effect. `ProjectRoot` carries the same guard on the
consumption path, so an absolute or `..`-bearing `projectDir` aborts at `resolveRoots` before any
external command runs. One `validateProjectDir` implementation, two entry points.

## File Changes

| File | Action | Description |
|---|---|---|
| `internal/config/locate.go` | Create | pure `Locate(startDir, stopDir) (string, bool)` |
| `internal/config/config.go` | Modify | `ProjectDir` field (no `applyDefaults` entry — empty stays empty) |
| `internal/config/project_root.go` | Create | `ProjectRoot` + shared `validateProjectDir` |
| `internal/config/validate.go` | Modify | call `validateProjectDir` |
| `cmd/deploydeck/roots.go` | Create | `roots`, `resolveRoots`, `newChecker` |
| `cmd/deploydeck/main.go` | Modify | `defaultRunTUI`/`defaultChecker`/`runPrune` use `resolveRoots`; `NewChecker` closure; `runs.NewWriter(artifactsRoot)`; lock under `artifactsRoot` |
| `internal/app/app.go` | Modify | three root fields + `normalizeRoots` in `New`; zero-arg `NewChecker` |
| `internal/app/commands.go` | Modify | 26 `m.deps.Dir` reads → the correct root; `deltaCmd` (`GitRoot` + `ArtifactsRoot`); `validateCmd`/`reportCmd`/`quickDeployCmd`/`cancelCmd` → `ProjectDir`; `runPrereqCmd` → `newChecker()` |
| `internal/app/view.go` | Modify | 2 reads → `GitRoot` (`runPackagePath` unchanged) |
| `internal/delta/service.go` | Modify | emit `--repo-dir req.Dir`; doc |
| `internal/salesforce/{client,quick,cancel}.go` | Modify | `dir` param threaded into `exec.CommandRequest.Dir` |
| `internal/salesforce/validate.go` | Modify | doc-only correction of `ValidateRequest.Dir` |
| `internal/prereq/checker.go` | Modify | `Dir`→`GitRoot`, add `ArtifactsRoot` |
| `internal/prereq/checker_{repository,workingtree,hooks,gpgsign}.go` | Modify | `c.Dir` → `c.GitRoot` (5 reads) |
| `internal/prereq/checker_gitignore.go` | Modify | two-candidate read, artifacts-root fix target, drop `RepoRoot` |
| `internal/runs/writer.go` | Modify | doc-only: `baseDir` is the ARTIFACTS root, not the repository root |
| `docs/ARQUITECTURA.md`, `README.md` | Modify | document `projectDir` and the three-root model |

## Testing Strategy (strict TDD — RED first)

| Layer | What | Where |
|---|---|---|
| Unit config | `Locate`: found at `startDir` (flat, zero hops); found after walking up; found in a subdir with the bound at the git root; not found anywhere; `stopDir == ""` probes `startDir` only; `stopDir` not an ancestor | `config/locate_test.go` (new, `t.TempDir()`, no git) |
| Unit config | `ProjectRoot`: `""`→`configDir`; relative→`Join(gitRoot, v)`; absolute→error; `..`→error. `Validate` rejects the same two | `config/project_root_test.go`, `validate_test.go` |
| Unit delta | `buildArgs` emits `--repo-dir <Dir>`; `sourceDirs` still passed verbatim (repo-root-relative, NOT re-prefixed) | `delta/service_test.go` |
| Unit salesforce | `QuickDeploy`/`CancelDeploy` thread `dir` into `exec.CommandRequest.Dir`; arg slice otherwise byte-identical | `salesforce/{quick,cancel}_test.go` |
| Integration prereq | nested: entry only in `<artifactsRoot>/.gitignore` passes; entry only in `<gitRoot>/.gitignore` passes (fallback); neither → blocks with `FixCommand` naming the ARTIFACTS path; `AddGitignoreEntry` writes `<artifactsRoot>/.gitignore` and creates NO file at the git root | `prereq/checker_gitignore_test.go` |
| Integration git | `RepoRoot(ctx, projectDir)` resolves the same root from the nested subdir | `git/service_root_test.go` |
| App | nested `Deps`: `deltaCmd` sends `Request.Dir == GitRoot` and `OutputDir` under `ArtifactsRoot`; `validateCmd`/`reportCmd`/`quickDeployCmd`/`cancelCmd` all send `Dir == ProjectDir` (assert via `FakeRunner` capture) | `app/delta_validation_test.go`, `app/standalone_modes_e2e_test.go` |
| App | `normalizeRoots`: `Deps{Dir: d}` alone yields all three roots `== d` (the flat-layout invariant every existing test now asserts for free) | `app/app_test.go` |
| App guard | `deps.Dir` is read nowhere outside `normalizeRoots` (ADR-6) | `app/roots_guard_test.go` (new) |
| E2E delta | nested fixture: real `sf sgd source delta` from a project subdir produces a NON-EMPTY package | `delta/generate_e2e_test.go` |
| Regression | `boundary_test.go` passes with no allow-list change | unchanged |

### Test seams — nested fixtures alongside the flat ones

Add a SIBLING helper in each package; never modify the existing flat helpers, so every current
test keeps its exact harness:

- `internal/git/helpers_test.go` — `newNestedTempRepo(t) (gitRoot, projectDir string)`: same
  `init -b main` / user config / bare-origin / `README.md` commit sequence as `newTempRepo` (line 58),
  then `os.MkdirAll(<gitRoot>/project)` plus a committed file inside it. Returns BOTH paths so each
  assertion names the root it means.
- `internal/prereq/helpers_test.go` — `newNestedTempRepo(t, withOrigin bool) (gitRoot, projectDir string)`,
  mirroring the duplicated flat `newTempRepo` (line 43). Go test helpers are not importable across
  packages, so the duplication is the existing, accepted convention.
- `internal/delta/generate_e2e_test.go` — nested variant of the existing fixture copy: copy
  `test-e2e-org/` into `<gitRoot>/project/` instead of `<gitRoot>/`, and set `sourceDirs` to the
  repo-root-relative `project/force-app`.
- The ~16 `prereq.Checker{Dir: dir, ...}` literals become `{GitRoot: dir, ArtifactsRoot: dir, ...}`
  (compiler-enforced by ADR-4); the 241 `app.Deps{Dir: ...}` literals stay untouched (ADR-3).

## Threat Matrix

| Boundary | Applicability | Design response | RED test |
|---|---|---|---|
| Git repository selection | **Applicable** — sgd's implicit `./` repo-dir is confirmed bug #1 | `--repo-dir` is ALWAYS emitted explicitly with the resolved absolute `gitRoot`; the child cwd is set to the same value; never relies on a default or on the process cwd. `--output-dir` stays absolute so it is independent of the child cwd | `buildArgs` emits `--repo-dir <gitRoot>`; nested e2e produces a non-empty package |
| PR commands / argument composition | **Applicable** — new `dir` args on two `sf project deploy *` methods | `dir` reaches the child ONLY as `exec.CommandRequest.Dir`; it is NEVER appended to `Args`, never joined, never shell-interpolated. Arg slices stay byte-identical | assert the exact arg slice AND `Dir` separately per method |
| Commit state | **Applicable** (indirect) — the old gitignore fix wrote an untracked `<gitRoot>/.gitignore` that then tripped `CheckWorkingTree` | the fix writes `<ArtifactsRoot>/.gitignore` only; no `git add`, no staging, no commit anywhere in this change | nested `AddGitignoreEntry` creates NO file at the git root |
| Push state | N/A | no ref, remote or push behavior is touched |  |
| Documentation-like paths | N/A | no file is classified or executed; `.gitignore` is read as text and appended to |  |
| Path traversal via config | **Applicable** — `projectDir` is user-supplied and becomes a process cwd | `validateProjectDir` rejects absolute paths and any `..` segment on BOTH entry points (`Validate` and `ProjectRoot`), aborting before any command runs | absolute and `..` values rejected by `ProjectRoot` and by `Validate` |

## Migration / Rollout

No migration. `projectDir` is additive and zero-value-safe; with it absent, `projectDir == configDir`
and, for a flat repo, `configDir == gitRoot == cwd`, so every path, command cwd and artifact location
is byte-identical to today. `Load` uses `yaml.Unmarshal` without `KnownFields`, so a reverted binary
silently ignores a leftover `projectDir:` key. Single revert restores the `os.Getwd()` threading.

## Budget

Forecast ~1000-1150 changed lines against 1200 — inside budget **because of ADR-3**; the naive
`Deps.Dir` removal would exceed it on test churn alone. Cut levers, in the order they should be
spent, if `sdd-tasks`' refined forecast exceeds 1200:

1. `internal/git` nested helper + `service_root` nested test (−45): re-confirms an invariant the
   audit already proved (`git.Service` self-resolves from any in-repo dir).
2. `internal/app/roots_guard_test.go` (−35): replace with a one-time grep during apply, accepting
   the loss of the executable invariant.

The nested delta e2e is **not** a lever (proposal R5): it is the only guard against this change's
worst failure mode — a silently empty-but-successful delta package that validates green.

## Open Questions

- [x] Where the git call sites read from → RESOLVED: `GitRoot`, for consistency, not correctness
  (they are already correct either way).
- [x] `runPackagePath` → RESOLVED: no change; it is a display-only relative path.
- [x] `Checker` needing a `ProjectDir` → RESOLVED: no. No Checker check runs an `sf project` command.
- [ ] `git check-ignore` (ADR-9) remains deferred. Revisit if a real repo reports a false block from
  `.git/info/exclude` or a global excludes file.
