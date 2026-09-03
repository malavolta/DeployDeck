# Design: Wire `Config.Validate()` Into The Prerequisite Report

## Technical Approach

One new pure method, `(*prereq.Checker).CheckConfig`, wraps the existing `Config.Validate()` and is
registered FIRST in `Checker.Check()`. `config.Load`, `resolveRoots` and the resolution sequence are
untouched. `resolveRoots` already computes `configDir` and discards it; it now returns it on the
`roots` triple (making it a quadruple) so `newChecker` can set `Checker.ConfigPath` and the
`FixCommand` names the real file the upward search actually found.

No new import edge: `internal/prereq` already imports `internal/config` (`checker.go:5`). `internal/app`
gains nothing — `boundary_test.go`'s allow-list is provably untouched — and the TUI inherits the check
for free, because `runPrereqCmd` (`commands.go:516`) and `deploydeck doctor` (`main.go:170`) consume
the same `Checker.Check()` report. Maps proposal scope items 1-5 and the spec delta's ADDED
"Config Validity Check (Blocking)".

## Data Flow

    os.Getwd() ──► resolveRoots(ctx, g, cwd)
                     ├── git.RepoRoot ─────► gitRoot, stopDir
                     ├── config.Locate ────► configDir      ◄── computed today, DISCARDED today
                     ├── config.Load(configDir) ──► cfg     ◄── applyDefaults RAN HERE (the invariant)
                     └── cfg.ProjectRoot ──► projectDir
                              │
                              ▼
                     roots{GitRoot, ProjectDir, ArtifactsRoot, ConfigDir}
                              │
                              ▼
                     newChecker(cfg, r)
                       └─► prereq.Checker{ Config: cfg,
                                           ConfigPath: filepath.Join(r.ConfigDir, config.FileName) }
                              │
                              ▼
                     Checker.Check(ctx)
                       1. CheckConfig      ◄── NEW, FIRST
                       2. CheckVersions          (consumes cfg.MinVersions)
                       3. CheckRepository → [WorkingTree, Gitignore, Hooks, GpgSign]
                       4. CheckAliases           (consumes cfg.Sandboxes)
                       5. CheckGH, CheckAI, CheckLock
                              │
                              ▼
                     renderPrereqChecks (main.go:187)  /  view.go:158
                     both: ONE line per Detail + optional "fix:" line

The load-bearing arrow is `config.Load ──► cfg ──► Checker.Config`. `Validate()` is only ever correct
downstream of `applyDefaults`; every production path already satisfies this because `resolveRoots` is
the single choke point. See ADR-6 for what enforces it and what does not.

## Architecture Decisions

| # | Decision | Choice | Rejected | Rationale |
|---|---|---|---|---|
| ADR-1 | Where `Validate()` runs | A new `prereq.Checker` check (`CheckConfig`), reported as a normal `PrereqCheck` | wiring into `config.Load` or `resolveRoots` | Both entry points return a bare `error` BEFORE `checker.Check()` or `app.New(deps)` exists. `doctor` would print one Go error and exit, never reaching `renderPrereqChecks` — destroying the structured `[status] name: detail` + `fix:` report for EVERY other check, not just this one. The checker surface gives the TUI's first screen and `doctor` the check for free, with no new code path and no new screen |
| ADR-2 | Position in the report | FIRST, before `CheckVersions` (revises exploration's "right after") | after `CheckVersions`; anywhere later | `cfg.MinVersions` feeds `CheckVersions` and `cfg.Sandboxes` feeds `CheckAliases` — a malformed config must surface before the checks that consume it. `CheckConfig` spawns no process, so it cannot slow the report. Verified free: `Check()` has no status-based short-circuit (every `return` in it is an error return), and `checker_check_test.go` locates checks via `findCheck(t, checks, name)`, never by index |
| ADR-3 | Config path plumbing | ADD `roots.ConfigDir`; `newChecker` composes `Checker.ConfigPath = filepath.Join(ConfigDir, config.FileName)` | a bare `"deploydeck.yaml"` fix string; re-deriving the path inside `prereq` | `config.Locate` searches UPWARD, so in a nested layout a bare filename is genuinely ambiguous — the operator cannot tell which of two candidate files to edit. `ConfigDir` is a FOURTH distinct fact, not a redundant alias: unlike `ArtifactsRoot` (ADR-2 of `directory-resolution`, same value as `ProjectDir`, different name), `ConfigDir` can differ in VALUE from all three whenever `projectDir` is set. Re-deriving inside `prereq` would re-introduce the resolution that `directory-resolution` ADR-5 removed. Widening the struct is therefore consistent with, not against, its minimalism: the rule was "one field per semantic root", not "exactly three fields" |
| ADR-4 | Granularity | Report only the FIRST error `Validate()` returns | collecting all violations into a multi-line `Detail`; splitting `Validate()` into 11 per-rule functions | `Detail` is rendered by a single-line `fmt.Fprintf(w, "[%s] %s: %s\n", ...)` in `main.go:187` AND by `view.go:158` — a multi-line Detail breaks BOTH render contracts. `Validate()`'s error already names the rule and the offending key, and its gate iteration is sorted, so the reported error is deterministic. Splitting the function would touch its 5 existing test sites for no user-visible gain. Accepted cost, stated plainly: a config with N violations needs N run-fix cycles |
| ADR-5 | Severity | Uniform `StatusBlocking` for all 11 rules | per-rule severity (e.g. warning for the ProjectDir rule, already enforced by `ProjectRoot`) | Every rule describes concrete downstream WRONG behavior, not style: rule #8 alone is the silent always-empty delta package this change exists to kill. False-positive risk is empirically zero, not inferred — all three real configs (the user's live nested config, the README example, the ARQUITECTURA example) were EXECUTED against all 11 rules and pass through `Load`. A severity table would add a per-rule classification surface with no case that currently needs it; the mid-setup user is explicitly not locked out, because rule #2 checks structural presence and a placeholder alias is a non-empty string |
| ADR-6 | Defaults-order invariant | Field doc on `Checker.Config` + doc on `CheckConfig` + TWO pinned tests (one positive via `Load`, one negative on a bare literal) | making `CheckConfig` apply defaults to a local copy; a `defaulted bool` flag or a distinct `LoadedConfig` type | Local defaulting is actively WRONG, not merely redundant: `CheckConfig` would validate a defaulted COPY while `Checker.Config` — and `CheckVersions`, `CheckAliases` and the whole TUI's `deps.Config` — still hold the un-defaulted struct. `config file` would report OK for a config the app then runs with `PollIntervalSeconds == 0`, i.e. a poll loop with a zero interval: the check would be lying. It would also force exporting `applyDefaults`. A type-level distinction is the only real enforcement, and it ripples through ~20 `Config: config.Config{...}` test literals — out of budget, and the failure it prevents is loud (doctor blocks on the first run), not silent |

## Interfaces / Contracts

```go
// internal/prereq/checker.go — two additions to an existing struct.
type Checker struct {
    /* GitRoot, ArtifactsRoot, Git, SF, Lock, GH, AI unchanged */

    // Config MUST have been produced by config.Load, i.e. applyDefaults has
    // already run. CheckConfig calls Config.Validate(), whose
    // pollIntervalSeconds/pollTimeoutSeconds rules reject the ZERO value —
    // and omitting both keys is the normal way to author deploydeck.yaml, so
    // an un-defaulted Config fails spuriously on EVERY real config (verified
    // by execution against three of them). Never assign a bare
    // config.Config{} literal here outside a test that intends the block.
    Config config.Config

    // ConfigPath is the absolute path of the deploydeck.yaml Config was
    // loaded from, used only to build CheckConfig's FixCommand. config.Locate
    // searches UPWARD, so a bare filename would be ambiguous in a nested
    // layout. The zero value degrades to config.FileName — never an empty
    // path, never a panic.
    ConfigPath string
}

// internal/prereq/checker_config.go — NEW.
const nameConfigFile = "config file"

func (c *Checker) CheckConfig(ctx context.Context) (PrereqCheck, error) {
    if err := c.Config.Validate(); err != nil {
        return PrereqCheck{
            Name:       nameConfigFile,
            Status:     StatusBlocking,
            Detail:     err.Error(),                        // verbatim; already names rule + key
            FixCommand: "edit " + c.configFilePath(),
        }, nil
    }
    return PrereqCheck{
        Name:   nameConfigFile,
        Status: StatusOK,
        Detail: c.configFilePath() + " is valid",           // discloses WHICH file was located
    }, nil
}

// configFilePath degrades a zero-value ConfigPath to the bare file name
// rather than emitting "edit " with an empty operand.
func (c *Checker) configFilePath() string {
    if c.ConfigPath == "" {
        return config.FileName // "deploydeck.yaml"
    }
    return c.ConfigPath
}

// cmd/deploydeck/roots.go
type roots struct{ GitRoot, ProjectDir, ArtifactsRoot, ConfigDir string }
```

`ctx` is accepted and unused, and the `error` return is always nil. Both are signature parity with the
eight sibling single-check methods (`CheckGitignore` also ignores its `ctx`), keeping `Check()`'s body
one uniform shape. The simpler `CheckConfig() PrereqCheck` was rejected for exactly that: one entry
with a different call shape in an otherwise uniform aggregator.

`FixCommand` is a NON-RUNNABLE hint, matching the established `fixCommandGitUpgrade =
"https://git-scm.com/downloads"` precedent. Nothing in DeployDeck ever executes a `FixCommand`;
`main.go:189` and `view.go:160` only print it.

## Registration

`checker_check.go` gains four lines before the `CheckVersions` block, in the file's existing shape:

```go
cfgCheck, err := c.CheckConfig(ctx)
if err != nil {
    return nil, err
}
all = append(all, cfgCheck)
```

`Check`'s doc comment gains `config validity` at the head of its enumerated list.

## Confirmed Regressions — THREE sites, not one

`explore.md` recorded only the first. The other two were found while designing and are equally
compile-clean but assertion-breaking, so all three must land in the same commit.

| # | Site | Why it breaks | Fix |
|---|---|---|---|
| R1 | `cmd/deploydeck/doctor_e2e_test.go:216` — `config.Config{MinVersions, Sandboxes}` across **10** `doctorVariant` cases | the file sets no poll seconds anywhere; rules #4/#5 fire on all 10, flipping `anyBlocking(checks)` and breaking the exit-0 assertion of `"all prerequisites pass"` | `baselineConfig()` helper (below) |
| R2 | `cmd/deploydeck/root_test.go:163` — `TestNewRootCmd_Doctor_AllChecksPass_ExitsZero` builds `Config: config.Config{}` and asserts `doctor` exits 0 | same rules fire; doctor now exits non-zero | same-package reuse: `Config: baselineConfig()` |
| R3 | `internal/prereq/checker_check_test.go:41` — `TestChecker_Check_AllPrerequisitesPass_AllCriticalChecksOK` loops every check asserting none blocks | the new `config file` entry blocks | load a real minimal config via the new `loadMinimalConfig` helper — see below |

### R1/R2 — the `baselineConfig()` helper

Mirrors the file's existing zero-arg `baselineMinVersions()` / `baselineSandboxes()` shape, so the
harness keeps one convention:

```go
// baselineConfig returns the Config config.Load produces for a minimal
// deploydeck.yaml — i.e. applyDefaults' output. doctorVariant builds its
// Config as a struct literal, bypassing Load, so without this the new
// `config file` check blocks on pollIntervalSeconds in every variant,
// including "all prerequisites pass".
func baselineConfig() config.Config {
    return config.Config{
        BranchFormat:        config.DefaultBranchFormat,
        PollIntervalSeconds: config.DefaultPollIntervalSeconds,
        PollTimeoutSeconds:  config.DefaultPollTimeoutSeconds,
        Runs: config.RunsConfig{
            KeepLast: config.DefaultRunsKeepLast,
            KeepDays: config.DefaultRunsKeepDays,
        },
    }
}
```

Line 216 becomes:

```go
cfg := baselineConfig()
cfg.MinVersions = v.minVersions
cfg.Sandboxes = v.sandboxes
```

Only the two poll fields are load-bearing today; `BranchFormat` and `Runs` are included so the helper
is a faithful mirror of `applyDefaults` and a future defaults-dependent rule cannot silently re-break
all 10 variants. Referencing the exported `Default*` constants (not literal numbers) means the fixture
tracks a changed default automatically. `root_test.go:163` reuses it directly — same `package main`.

Rejected: writing a temp `deploydeck.yaml` and calling `config.Load`. It would add YAML authoring to a
harness whose subject is prerequisite checks, not config parsing. The tradeoff is stated honestly: this
fixture is a HAND-MIRROR of `applyDefaults`; the `Load`-backed test below is what actually guards the
production path.

### R3 — `loadMinimalConfig`, the positive pin in its strongest position

New helper in `internal/prereq/helpers_test.go`:

```go
// loadMinimalConfig writes body as deploydeck.yaml into a FRESH t.TempDir()
// (never the repo dir under test — an uncommitted config file would dirty
// the working tree and trip CheckWorkingTree) and returns the LOADED,
// defaulted Config plus its path.
func loadMinimalConfig(t *testing.T, body string) (config.Config, string)
```

`TestChecker_Check_AllPrerequisitesPass_AllCriticalChecksOK` then builds its `Checker` from
`loadMinimalConfig(...)` and adds `"config file"` to its critical-names list. This puts the
"a `Load`ed minimal config passes" invariant inside the suite's flagship all-pass test — the one test
that already fails loudly if ANY check blocks — rather than in an isolated test a future refactor could
delete without noticing.

## What ADR-6 Does And Does Not Prevent

| Prevents | Does NOT prevent |
|---|---|
| Silent removal or regression of `applyDefaults`' poll entries (the `Load`-backed pin in R3 goes red) | A future caller writing `prereq.Checker{Config: config.Config{...}}` by hand from a non-`Load` source |
| A future reader doubting whether the precondition is real (docs on the field, the method, and `Validate()` itself) | A future `Config` producer other than `Load` (none exists today; `resolveRoots` is the single choke point) |
| Re-introducing the defaults bug through `doctor_e2e_test.go`'s literals (R1's helper) | Anything at compile time — Go cannot express "this struct went through `applyDefaults`" without a new type |

The residual risk is accepted deliberately. Its manifestation is loud and immediate — `doctor` blocks
on the first run with a `Detail` naming `pollIntervalSeconds` — not a silent wrong result.

## File Changes

| File | Action | Description |
|---|---|---|
| `internal/prereq/checker_config.go` | Create | `nameConfigFile`, `CheckConfig`, `configFilePath` |
| `internal/prereq/checker.go` | Modify | add `ConfigPath`; precondition doc on `Config` (ADR-6) |
| `internal/prereq/checker_check.go` | Modify | register `CheckConfig` FIRST; extend `Check`'s doc list |
| `internal/prereq/checker_aliases.go` | Modify | **W-5**, comment-only: line 32's "config.Validate() already rejects a missing alias" becomes TRUE under ADR-1; note that `CheckConfig` reports it and this `continue` stays correct with no double-reporting. Zero behavior change |
| `internal/config/validate.go` | Modify | doc-only: delete the now-false "NOT called anywhere in production code" note; state the `applyDefaults`-first precondition and name `CheckConfig` as the production caller |
| `cmd/deploydeck/roots.go` | Modify | `roots.ConfigDir`; `resolveRoots` returns it; `newChecker` sets `ConfigPath` |
| `cmd/deploydeck/doctor_e2e_test.go` | Modify | `baselineConfig()` + line 216 (**R1**) |
| `cmd/deploydeck/root_test.go` | Modify | one literal (**R2**) |
| `cmd/deploydeck/roots_test.go` | Modify | `ConfigDir` / `ConfigPath` assertions |
| `internal/prereq/checker_check_test.go` | Modify | **R3** + a first-position ordering test |
| `internal/prereq/helpers_test.go` | Modify | `loadMinimalConfig` |
| `internal/prereq/checker_config_test.go` | Create | `CheckConfig` unit tests |
| `internal/config/validate_test.go` | Modify | bare-literal negative pin |
| `internal/app/discover_roots_test.go` | Create | **W-3** (see below) |
| `internal/delta/generate_e2e_test.go` | Modify | **S-2** (see below) |
| `openspec/changes/archive/project-dir-resolution/tasks.md` | Modify | **S-4**: APPEND a note under 7.2, never rewrite the checked item |
| `openspec/specs/prereq-check/spec.md` | Modify | apply the delta at archive time |
| `README.md` | Modify | new `## Update` section after `## Install` |

## Secondary Items — Concrete Shapes

- **W-5** — comment-only edit at `checker_aliases.go:32`. No code, no test.
- **W-3** — ONE representative nested-`Deps` test, `TestModel_DiscoverCmd_NestedDeps_RunsGitAtGitRoot`
  in a new `internal/app/discover_roots_test.go`. Reuses the existing `candidateFakeRunner(t, root,
  ticket, local, remote)` helper (`source_confirm_test.go:30`) and `run(t, cmd)`; builds
  `Deps{GitRoot: gitRoot, ProjectDir: <gitRoot>/project, ArtifactsRoot: <gitRoot>/project}`, drives
  `m.discoverCmd()`, then asserts EVERY recorded `fr.Calls[i].Dir == gitRoot`. `exec.FakeRunner`
  records the full `CommandRequest` including `Dir` (`fake_runner.go:33`). ~25 lines.
  **Explicitly partial: this proves the pattern for 1 of ~13 git-backed sites, not all of them.**
  The failure it closes is one `roots_guard_test.go` cannot: that guard only proves `deps.Dir` is never
  read outside `normalizeRoots`, so a site reading `ProjectDir` for a git call would pass it equally.
- **S-2** — appended to `internal/delta/generate_e2e_test.go` (same `delta_test` package, so
  `seedNestedSgdRepo`/`runGit`/`copyFixtureTree` are already in scope).
  `TestSgd_RepoDirIndependentOfCwd_RealSgd` BYPASSES `delta.Service` — `Request.Dir` drives both the
  child cwd and the emitted `--repo-dir` from one field (ADR-7 of `directory-resolution`), so the
  Service structurally cannot separate them and the existing e2e would still pass if `--repo-dir` were
  silently dropped. It calls `exec.NewOSRunner().Run` directly with
  `CommandRequest{Dir: <gitRoot>/project}` and an explicit `--repo-dir <gitRoot>`, `--source-dir
  project/force-app`, absolute `--output-dir`, then asserts exit 0 and a `package.xml` containing
  `<types>` and `AccountService`. Real `sf`/`sgd`, `-short`-skipped via `seedNestedSgdRepo`. ~35 lines.
- **S-4** — append one clarifying line under `tasks.md:104`, e.g. a `> Note (config-validation-wiring):`
  block recording that the `--repo-dir` flag assertion lives in `service_test.go`, and that the
  cwd-vs-`--repo-dir` separation is covered by S-2's isolating e2e. The `- [x]` item's own text is NOT
  edited — archived history is append-only.
- **README `## Update`** — placed directly after `## Install`, mirroring its three-platform shape:
  `brew upgrade --cask deploydeck` (macOS), `scoop update deploydeck` (Windows),
  `go install github.com/malavolta/DeployDeck/cmd/deploydeck@latest` (Linux / any OS). Flag explicitly
  that **DeployDeck is a cask, not a formula**, so dropping `--cask` is an easy habit error. Motivated
  by the existing non-blocking update banner, which tells users a new version exists but not how to get
  it.

## Testing Strategy (strict TDD — RED first)

| Layer | What | Where | Reuses |
|---|---|---|---|
| Unit prereq | `CheckConfig` blocking: `Detail` == the `Validate()` error VERBATIM; `FixCommand` == `"edit " + ConfigPath` | `checker_config_test.go` (new) | `loadMinimalConfig` |
| Unit prereq | `CheckConfig` OK on a `Load`ed minimal config; `Detail` names the resolved path | `checker_config_test.go` | `loadMinimalConfig` |
| Unit prereq | zero-value `ConfigPath` degrades to `deploydeck.yaml` — no empty operand, no panic | `checker_config_test.go` | — |
| Unit prereq | delta rule #8 concretely: `Delta{OutputDir: set, SourceDirs: nil}` blocks naming `sourceDirs` | `checker_config_test.go` | `loadMinimalConfig` |
| Ordering | `Check()` returns `config file` at index 0 and before every `CheckVersions` entry | `checker_check_test.go` | `newTempRepo`, `findCheck` |
| Regression | all-pass test still reports ZERO blocking checks, with `config file` present and OK (**R3**) | `checker_check_test.go` | `loadMinimalConfig` |
| Unit config | bare `config.Config{}` rejects naming `pollIntervalSeconds` (ADR-6 negative pin) | `validate_test.go` | existing table |
| Unit cmd | `resolveRoots` returns `ConfigDir` == the LOCATED dir, flat and nested-with-`projectDir` | `roots_test.go` | `TestResolveRoots_NestedRepo*` fixtures |
| Unit cmd | `newChecker` sets `ConfigPath == <ConfigDir>/deploydeck.yaml` | `roots_test.go` | mirrors `TestNewChecker_LockPathUsesArtifactsRoot` |
| E2E cmd | all 10 doctor variants keep their exit codes (**R1**) | `doctor_e2e_test.go` | `baselineConfig()` |
| E2E cmd | `..._AllChecksPass_ExitsZero` stays exit 0 (**R2**) | `root_test.go` | `baselineConfig()` |
| App | **W-3**: nested `Deps` — `discoverCmd` runs git with `Dir == GitRoot` | `discover_roots_test.go` (new) | `candidateFakeRunner`, `run` |
| E2E delta | **S-2**: cwd ≠ `--repo-dir` still produces a non-empty package | `generate_e2e_test.go` | `seedNestedSgdRepo` |

## Threat Matrix

| Boundary | Applicability | Design response | RED test |
|---|---|---|---|
| Documentation-like paths | **N/A** — `deploydeck.yaml` is read as YAML by the pre-existing `config.Load`; this change adds no read, no classification and no execution. `ConfigPath` is a display string only | — | — |
| Git repository selection | **N/A for the primary change** — `CheckConfig` is pure in-memory and touches no repo, cwd or root. **Applicable to S-2 only**, which re-proves the already-shipped ADR-7 binding | S-2 sets the child cwd DELIBERATELY different from `--repo-dir`, both absolute and explicit; nothing relies on a default or the process cwd | S-2's non-empty `package.xml` |
| Commit state | **N/A** — nothing is staged, added or committed anywhere in this change | — | — |
| Push state | **N/A** — no ref, remote or push behavior is touched | — | — |
| PR commands / argument composition | **Applicable (degenerate)** — `FixCommand` is a newly COMPOSED string containing a filesystem path | The path enters `FixCommand` only; it is NEVER appended to any `Args` slice, never joined into a command line, never shell-interpolated, and no code path executes a `FixCommand` (`main.go:189` and `view.go:160` only print it) — the same contract `fixCommandGitUpgrade`'s URL already relies on | assert `FixCommand == "edit " + ConfigPath` exactly, and that the zero value yields `"edit deploydeck.yaml"` |

## Migration / Rollout

No migration. The check is read-only and in-memory; no shared Git state, Salesforce sandbox, or
persisted artifact is touched. `roots.ConfigDir` is additive and zero-value-safe. Revert the single
commit — nothing to unwind. A wrongly blocked operator's interim escape hatch is fixing the reported
key, which the `Detail` names directly.

## Budget

Forecast ~320 changed lines against 1200: primary ~185 (check + registration + `ConfigPath` plumbing +
its tests), regressions ~35 (R1/R2/R3), secondary ~100 (W-5 3, W-3 25, S-2 35, S-4 3, README 12,
`validate.go` doc 8). The two regression sites this design discovered beyond `explore.md` add ~10
lines — not material against the proposal's ~312 forecast. No chaining decision needed. If a refined
forecast ever overruns, the first cut lever is W-3's app test (−25), which re-confirms a binding
`directory-resolution` already shipped and tested at one other site; S-2 is NOT a lever, since it is
the only test that can distinguish cwd from `--repo-dir`.

## Open Questions

- [x] Placement, ordering, path plumbing, granularity and severity → RESOLVED as ADR-1..ADR-5.
- [x] Whether reordering breaks existing tests → RESOLVED: no. `Check()` has no status-based
  short-circuit and `checker_check_test.go` looks checks up by NAME, not index.
- [ ] W-3 stays partial by design (1 of ~13 sites). Revisit only if a second root-conflation defect
  ever surfaces at a non-`deltaCmd`, non-`discoverCmd` site.

---

## Orchestrator gate — regression-site inventory verified complete

The design names three regression sites (R1 `doctor_e2e_test.go`, R2 `root_test.go:163`,
R3 `checker_check_test.go:41`). The orchestrator independently enumerated **every** test that calls
the aggregator `Checker.Check()` — the only entry point through which the new blocking check can
reach an assertion — and confirmed the list is exactly right, neither short nor long.

All `Check()` call sites in the test suite:

| Call site | Asserts | Verdict |
|---|---|---|
| `cmd/deploydeck/doctor_e2e_test.go:228` | doctor exit code across 10 variants | **BREAKS** (R1) |
| `cmd/deploydeck/root_test.go` (via `doctor`) | `Config: config.Config{}`, expects exit 0 | **BREAKS** (R2) |
| `internal/prereq/checker_check_test.go:46` | loops all checks, fails if ANY blocks | **BREAKS** (R3) |
| `internal/prereq/checker_check_test.go:85` | `findCheck(..., "git version")`, asserts that one blocks | SAFE |
| `internal/prereq/checker_ai_test.go:116` | scans for `Name == "AI model"`, asserts on it | SAFE |
| `internal/prereq/checker_gh_test.go:103` | scans for `Name == "gh CLI"`, asserts on it | SAFE |

The three safe sites survive by construction: they locate one check **by name** rather than
asserting a global property of the report, so an added blocking entry is invisible to them. This is
the same property that makes reordering free (`checker_check_test.go` uses `findCheck`, not indices).

`internal/prereq/real_org_e2e_test.go:117` builds a `config.Config` literal but calls individual
check methods, never the aggregator — out of reach, no fix needed.

The remaining ~17 `Config: config.Config{` literals across `internal/app` and the per-check
`internal/prereq` tests never construct a `Checker` that reaches `CheckConfig`; `sdd-tasks` must not
plan work for them.
