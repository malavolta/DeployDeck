# Proposal: Project Directory Resolution

**Delivery phase**: Fase 5 — Productizacion (EPICA.md `Plan De Entrega`, "Config YAML"). It corrects cross-cutting defects in capabilities delivered in Fases 2–4.

## Intent

DeployDeck collapses three distinct directories — **git root**, **SFDX project root**, **artifacts root** — onto one `dir` from `os.Getwd()`. When the SFDX project lives in a subdirectory of the git repo:

- `sf sgd source delta` fails (`Failure: './' is not a git repository`) — sgd's `--repo-dir` defaults to `./` and does not walk up.
- The `.deploydeck/` gitignore check reads `<gitRoot>/.gitignore` while `.deploydeck/` is created elsewhere → false blocking; its "fix" writes an untracked `<gitRoot>/.gitignore` that then trips the working-tree check.
- `QuickDeploy`/`CancelDeploy` set no `Dir` at all and inherit the raw process cwd — latent today, real the moment the directories diverge.

**Success**: a nested-layout repo runs the full flow with no config migration, and every flat-layout repo behaves identically.

## Scope

### In Scope

1. Additive top-level config key `projectDir` (relative to the git root, zero-value-safe).
2. `config.Locate(startDir)` — upward `deploydeck.yaml` search bounded at the git root. `Load(dir)` unchanged.
3. Composition root resolves `(gitRoot, projectDir, artifactsRoot)` once; `app.Deps` and `prereq.Checker` carry them as separate plain-string fields.
4. `delta.Request` gains an explicit repo root; `buildArgs` emits `--repo-dir <gitRoot>` and the command runs with `Dir = gitRoot`.
5. All four `sf project deploy *` calls (validate / report / quick / cancel) run with cwd = `projectDir`; `QuickDeploy`/`CancelDeploy` gain a `dir` parameter (`salesforce.Client` interface change).
6. Gitignore check and its fix anchored at the artifacts root, falling back to the git-root `.gitignore`.

### Out of Scope

- **Reinterpreting `delta.sourceDirs`** — stays repo-root-relative (shipped in v1.3.1, mandated by `openspec/specs/delta-generation/spec.md:9`, in live user configs). Reinterpreting it would double-prefix and yield an empty-but-green package. A project-relative surface needs a *separate* key, in its own change.
- `sfdx-project.json` auto-detection (revised away — see Approach).
- `internal/github` / `gh` cwd behavior — unchanged here, and its nested-subdir correctness is **verified**: `gh repo view --json nameWithOwner` from the real repo's nested project subdirectory returned `{"nameWithOwner":"giss-salesforce/salesforce"}`, so `gh` walks up to find `.git` the way git does. No change needed.
- Splitting `prereq.Checker.Dir` further than the gitignore fix requires.
- `internal/git` — already `RepoRoot`-safe, no change.

**Why the `sf` cwd is in, not deferred**: this change makes `gitRoot != projectDir` real. Fixing only sgd and gitignore would ship a half-resolved model where a nested repo still fails at validation — the tool's central step. Both confirmed defects and the latent `Quick`/`Cancel` one are the same bug class; splitting them leaves the class alive.

## Capabilities

### New Capabilities
- `directory-resolution`: how git root, SFDX project root and the `.deploydeck/` artifacts root are resolved, and which command consumes which.

### Modified Capabilities
- `prereq-check`: "`.deploydeck/` Gitignore Enforcement" says *the repository's `.gitignore`*; becomes the gitignore governing the artifacts root, with git-root fallback.
- `delta-generation`: the sgd invocation MUST pass an explicit `--repo-dir <gitRoot>`; `sourceDirs` stated explicitly as repo-root-relative.

`deploy-validation`, `run-persistence`, `quick-deploy`, `validation-cancel`, `run-retention`, `standalone-modes` were checked: they reference `.deploydeck/…` as relative paths only and never name an anchor, so they stay true. The anchor is defined once in `directory-resolution`.

## Approach

Composition-root sequence: `os.Getwd()` → `git.RepoRoot(cwd)` → `config.Locate(cwd)` bounded at `gitRoot` → `config.Load(found)` → resolve `projectDir`.

**`projectDir` defaults to the directory `deploydeck.yaml` was found in** — not the git root, not a detected `sfdx-project.json`. Rationale: today `config.Load(dir)` and `sf project` consume the *same* `dir`, so every currently-working install already has `deploydeck.yaml` at its SFDX project root. Config-dir anchoring is therefore *provably* identity-preserving for flat and nested layouts alike, and requires zero migration.

This **revises** the exploration's fail-closed `sfdx-project.json` auto-detection: in a repo with two SFDX projects, `projectDir` is unset today and the flow works (cwd disambiguates), but fail-closed detection would hard-error — a regression against the backward-compatibility criterion. Config-dir anchoring resolves that repo correctly and costs no filesystem walk.

**Artifacts root = `projectDir`**, so `.deploydeck/` (runs, manifests, lock) never relocates for an existing install. Consequence: the gitignore check must be anchored there too.

**sgd** gets both `--repo-dir <gitRoot>` and `Dir = gitRoot`. `--source-dir` is **confirmed** repo-dir-relative, not cwd-relative, on two independent grounds: `sf sgd source delta --help` documents it as "source folders focus location relative to `--repo-dir`", and an empirical run with cwd set to the project subdirectory, `--repo-dir` set to the git root, and a repo-root-relative `--source-dir` produced a 12-type package — which is only possible if the base is `--repo-dir` (a cwd base would have resolved to a nonexistent path and produced zero types). Setting both to `gitRoot` is therefore belt-and-braces, not a hedge against unknown semantics. `--output-dir` stays absolute so it is independent of the child cwd.

**Not inside a git repo**: skip the upward search, `Load(cwd)`, `projectDir = cwd` — so prereq's "not a git repository" stays the primary diagnostic instead of being masked.

**Validation**: reject an absolute `projectDir` or one containing `..`.

## Affected Areas

| Area | Impact | Description |
|---|---|---|
| `internal/config/config.go`, `validate.go` | Modified | `ProjectDir` field, path validation |
| `internal/config/locate.go` | New | `Locate(startDir)` bounded upward search |
| `cmd/deploydeck/main.go` | Modified | Resolve the triple once; thread to `defaultRunTUI`, `defaultChecker`, `runPrune` |
| `internal/app/app.go`, `commands.go`, `view.go` | Modified | `Deps` split; `deltaCmd`, `validateCmd`, `reportCmd`, `quickDeployCmd`, `cancelCmd`, `runPackagePath` |
| `internal/delta/service.go` | Modified | Explicit repo root + `--repo-dir` |
| `internal/salesforce/{client,quick,cancel,validate}.go` | Modified | `dir` params; fix `ValidateRequest.Dir` doc ("repository root" is wrong) |
| `internal/prereq/checker*.go` | Modified | Artifacts-root-anchored gitignore check and fix |
| `docs/ARQUITECTURA.md`, `README.md` | Modified | Document `projectDir` and the three-root model |

## Risks

| Risk | Likelihood | Mitigation |
|---|---|---|
| R1 `salesforce.Client` signature change breaks callers | Low | No hand-written fakes exist (tests use `salesforce.New(runner)`); compiler catches all sites |
| ~~R2 sgd resolves `--source-dir` relative to cwd, not `--repo-dir`~~ | **CLOSED** | Verified by `--help` text and by an empirical 12-type run with cwd ≠ `--repo-dir`. `--source-dir` is repo-dir-relative. Setting cwd AND `--repo-dir` to `gitRoot` is retained as defense in depth, not as a hedge |
| ~~R3 `gh` may not resolve the repo from a nested subdir~~ | **CLOSED** | Verified: `gh repo view` from the real nested subdirectory resolved the correct repo. `internal/github` needs no change |
| R4 Gitignore fallback misreads a `!.deploydeck/` negation | Very low | Fails *open* (passes the check), never false-blocks. `git check-ignore` deferred |
| R5 Line budget overrun (forecast ~1170 / 1200) | Med | Cut lever available but **deliberately NOT taken yet**: the nested delta e2e (~70 lines) is the only guard against this change's worst failure mode — a silently empty-but-successful delta package that validates green and deploys nothing. R2's closure removed one of its justifications (proving `--source-dir` semantics) but not its main one (regression protection). `sdd-tasks` may take the cut only if its refined forecast actually exceeds 1200; the second lever (`view.go` label, −25) should be spent first |

## Rollback Plan

- **Code**: single revert of the PR. All changes are additive or internal signature changes; reverting restores the `os.Getwd()` threading verbatim.
- **Config**: `Load` uses `yaml.Unmarshal` without `KnownFields` (verified `internal/config/load.go:27`), so a reverted binary **silently ignores** a leftover `projectDir:` key. No config migration to undo.
- **Filesystem**: the artifacts root is unchanged for every currently-working install, so `.deploydeck/` does not move and no run history is stranded.
- **Shared Git state**: the only Git-visible write is the optional `.deploydeck/` gitignore entry, now targeting `<artifactsRoot>/.gitignore`. Users who already ran the old fix may hold an untracked `<gitRoot>/.gitignore`; instruct them to delete it (`rm <gitRoot>/.gitignore`) if they did not author it. No branches, cherry-picks or refs are touched.
- **Salesforce sandboxes**: no deploy semantics change — same commands, same jobIds, only cwd differs. A wrong cwd makes `sf project deploy *` fail *before* submission (no `sfdx-project.json`), so no partial sandbox state is reachable.

## Dependencies

None external. `cmd/deploydeck` already imports `internal/git`, so `RepoRoot` is available at the composition root with no boundary violation.

## Success Criteria

- [ ] With `projectDir` absent and a flat repo, every existing test passes unchanged and no path, command cwd or artifact location differs.
- [ ] The real nested repo runs the full flow with **zero** edits to its `deploydeck.yaml` (`sourceDirs` untouched).
- [ ] Nested `Generate` produces a non-empty package; the sgd command carries `--repo-dir <gitRoot>`.
- [ ] Nested gitignore check passes against the artifacts-root `.gitignore` and its fix creates no untracked file at the git root.
- [ ] All four `sf project deploy *` invocations assert `Dir == projectDir`.
- [ ] `projectDir` that is absolute or contains `..` is rejected on the **resolution path**
      (`Config.ProjectRoot`), the path production actually executes — not only inside
      `Config.Validate()`. See the deferred follow-up below for why that distinction is load-bearing.
- [ ] `internal/app/boundary_test.go` passes with no allow-list change.
- [ ] `go test ./...` green; `go build ./...` clean.

## Deferred Follow-Up (out of scope for this change)

**`Config.Validate()` is dead code in production.** Verified during this change's design gate: the
method is invoked only from `internal/config/config_test.go` and `internal/config/validate_test.go`.
`config.Load` does not call it (`internal/config/load.go` ends at `applyDefaults`), and neither does
any `cmd/deploydeck` entry point. Its **10 validation rules therefore never execute at runtime** —
invalid `ticketPatterns` regexes, missing sandbox aliases, and inconsistent `delta` blocks all pass
silently into the flow.

Compounding it, `internal/prereq/checker_aliases.go:32` carries the comment *"config.Validate()
already rejects a missing alias; skip"* — a check deliberately skipped on the strength of a
validation that never runs.

This predates the present change and is **not** fixed here: wiring `Validate()` into `Load` would
turn ten currently-dormant rules live at once, which can reject configs that work today. That is a
behavior change deserving its own change, its own spec delta, and its own migration note — not a
silent rider on a directory-resolution fix. This change works around it narrowly, by binding the
`projectDir` guard to `Config.ProjectRoot` (the executed path) instead of relying on `Validate()`.

Recommended follow-up change: `config-validation-wiring` — call `Validate()` from `Load`, audit each
of the ten rules for false rejections against real in-the-wild configs, and either drop or soften any
rule that would break a working install.
