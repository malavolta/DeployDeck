# Exploration: `project-dir-resolution`

> **Orchestrator gate note.** This document was produced by the `sdd-explore` phase and then
> spot-verified by the orchestrator against the repository. Six of seven load-bearing claims were
> confirmed verbatim. Two were corrected and are marked **[CORRECTED BY GATE]** below: one loose
> characterisation of `view.go:1355`, and one materially false premise in section B3 that had
> inverted its own recommendation. Everything else is reproduced as returned.

## Current State

DeployDeck's composition root (`cmd/deploydeck/main.go`) resolves exactly one directory —
`os.Getwd()` — and threads it everywhere as `app.Deps.Dir` / `prereq.Checker.Dir`. The codebase
implicitly assumes three distinct concerns collapse onto that single directory:

1. **GIT ROOT** — where `git` operations should anchor.
2. **SFDX PROJECT ROOT** — where `sf project ...` commands must run (they require
   `sfdx-project.json` in cwd) and where `deploydeck.yaml` currently must live (`config.Load(dir)`
   reads `<dir>/deploydeck.yaml` with no upward search).
3. **PROCESS CWD** — wherever the user happened to launch the binary.

`internal/git.Service` is architected correctly and defensively: every method resolves the true git
root itself via `RepoRoot` (`internal/git/service_root.go:14`, `git rev-parse --show-toplevel`), so
passing it *any* directory inside the repo works. The rest of the codebase is not this careful —
several call sites treat "the one `dir` I was given" as simultaneously the git root, the sf-CLI cwd,
and the artifacts root.

Two defects are already empirically confirmed (cited, not re-derived):

- **sgd repo root** — `sf sgd source delta`'s `--repo-dir` defaults to `./` and does **not** walk up
  like git does; `internal/delta/service.go`'s `Generate` never passes `--repo-dir` explicitly, so it
  silently relies on its `Dir` being the true git root. Observed failure from the project
  subdirectory: `Failure: './' is not a git repository`.
- **gitignore check** — `internal/prereq/checker_gitignore.go`'s `readGitignore` resolves the *git*
  root via `RepoRoot` and reads `<gitroot>/.gitignore`, but `.deploydeck/` is actually created under
  whatever directory the artifacts root (`m.deps.Dir`) happens to be — the SFDX project
  subdirectory in the real inspected repo, not the git root. The check therefore reads the wrong
  file, and `AddGitignoreEntry`'s "fix" writes an untracked `.gitignore` at the git root, which then
  trips `CheckWorkingTree`'s repo-wide `git status --porcelain` cleanliness check.

Additional root-cause context found during this exploration:

- **`docs/ARQUITECTURA.md:317-321`** already documents a reference `deploydeck.yaml` with
  `delta.sourceDirs: [up_saln0001_giss_salesforce/force-app]` — a **repo-root-relative** path
  prefixed with the exact nested-project folder name from the real inspected repo. The documented
  intent already anticipated the nested layout, and `sourceDirs` was meant to stay
  repo-root-relative. *(Gate: confirmed verbatim at those lines.)*
- In the real inspected repo, `deploydeck.yaml` and `sfdx-project.json` are co-located in the
  **project subdirectory**, not the git root. Today's "working" setup is: the user `cd`s into the
  project subdir before running `deploydeck` (which satisfies `sf project` commands and
  `config.Load`); only sgd's implicit repo-dir default and the gitignore-root check break.
- `/Users/am/Documents/salesforce` contains **exactly one** `sfdx-project.json`, at
  `up_saln0001_giss_salesforce/sfdx-project.json`. Auto-detection is viable for this repo shape.
- `internal/salesforce.Client.QuickDeploy` and `CancelDeploy` take **no `dir` parameter at all** —
  their `exec.CommandRequest` never sets `Dir`, so `internal/exec.OSRunner.Run` (`cmd.Dir = req.Dir`
  with `req.Dir == ""`) runs them in the **raw OS process cwd**. This currently "works" only because
  `m.deps.Dir` happens to equal the real process cwd today; it is a latent inconsistency independent
  of this feature, and it becomes a real bug the moment `Dir`/`ProjectDir` diverge. *(Gate:
  confirmed — neither file sets `Dir`.)*

---

## A. Full audit of root/cwd conflation

Legend: **GIT** = needs true git repository root · **PROJ** = needs SFDX project root
(`sfdx-project.json` in cwd) · **CWD** = needs raw process cwd (display only) · **N/A** = no
directory dependency.

### cmd/deploydeck/main.go

| Site | Currently receives | Actually needs | Verdict |
|---|---|---|---|
| `main.go:61` root `RunE`, `dir, _ := os.Getwd()` → `deps.RunTUI(dir)` | process cwd | source of the single `dir` fed everywhere downstream | root cause |
| `main.go:106-110` `newRunsPruneCmd` RunE → `runPrune(w, dir)` | process cwd | artifacts root (see `runs.Writer`) | ambiguous-needs-decision |
| `main.go:120-140` `runPrune`: `config.Load(dir)` then `runs.NewWriter(dir).Prune(...)` | process cwd | artifacts root for `.deploydeck/runs` + wherever `deploydeck.yaml` is found | broken-in-nested-layout |
| `main.go:156-161` `newDoctorCmd` RunE → `deps.NewChecker(dir)` | process cwd | GIT for repo/worktree/gitignore/hooks checks, PROJ for config/SF checks | ambiguous-needs-decision |
| `main.go:198-226` `defaultChecker(dir)`: `config.Load(dir)`, `prereq.Checker{Dir: dir}`, lock at `filepath.Join(dir, ".deploydeck", "lock")` | one `dir` for config, checker root and lock path | PROJ for config, GIT for repo checks, artifacts root for the lock | broken-in-nested-layout — three needs, one field |
| `main.go:271-299` `defaultRunTUI(dir)`: `config.Load(dir)`, `runs.NewWriter(dir)`, `app.Deps{Dir: dir}` | one `dir` | PROJ for config and `sf`, GIT for sgd, artifacts root for runs/delta | broken-in-nested-layout — the composition root that must fork into (gitRoot, projectDir, configDir) |
| `main.go:306-322` `defaultCheckUpdate`, `decideUpdate` | N/A | N/A | correct |
| `main.go:328-334` `editHandoff` | N/A (opens `$EDITOR` on a resolved path) | N/A | correct |

### cmd/deploydeck/pr.go

| Site | Currently receives | Actually needs | Verdict |
|---|---|---|---|
| `pr.go:63-79` `newPRVerifyCmd`/`runPRVerify` | no dir — operates on a PR URL via `gh pr view <url>` | N/A | correct |

### internal/app — `Deps.Dir` and its consumers

`app.Deps.Dir` (`internal/app/app.go:295-297`) is read at `commands.go:506, 530, 576, 612, 628, 662,
682, 755, 771, 784, 795, 806, 817, 838, 914, 992, 1091, 1121, 1136, 1196, 1304, 1326, 1344, 1397,
1445, 1466` plus `view.go:116, 151`.

| Group | Currently receives | Actually needs | Verdict |
|---|---|---|---|
| Every `g.<GitMethod>(ctx, dir, ...)` call (~20 sites: `discoverCmd`, `confirmSourceCmd`, `rePromoteRemapCmd`, `depWarningsCmd`, `branchCreateCmd`, `reuseBranchCmd`, `deleteAndRecreateBranchCmd`, `cherryPickCmd`, `continueCmd`, `skipCmd`, `abortCmd`, `repoStateCmd`, `verifyCmd`, `resumeDetectCmd`, `originalBranchCmd`, `unpushedCountCmd`, `quitCmd`) | `m.deps.Dir` | GIT (self-resolving) | **correct** — every `git.Service` method resolves `RepoRoot` internally, so any directory inside the repo works |
| `deltaCmd` (`commands.go:911-958`): `dir` at 914; `outputDir := filepath.Join(dir, deltaBaseDir(cfg), ...)` at 922; `delta.Request{Dir: dir, SourceDirs: cfg.Delta.SourceDirs}` at 924-931; `g.ChangedFiles(ctx, dir, ...)` at 954 | `m.deps.Dir` for sgd's implicit `--repo-dir`, the artifacts base, and the diff start dir | GIT for sgd; artifacts root for `outputDir`; GIT (self-resolving) for `ChangedFiles` | **broken-in-nested-layout** — confirmed bug #1 |
| `validateCmd` (`commands.go:988-1003`): `sf.ValidateDeploy(ctx, salesforce.ValidateRequest{Dir: dir, ...})` | `m.deps.Dir` | **PROJ** — `sf project deploy validate` requires `sfdx-project.json` in cwd | **broken-in-nested-layout**. `internal/salesforce/validate.go:22-24` mislabels `ValidateRequest.Dir` as "the repository root"; that comment is wrong for `sf project` commands and must be corrected |
| `reportCmd` (`commands.go:1300-1312`): `sf.ReportDeploy(ctx, jobID, alias, dir)` | `m.deps.Dir` | **PROJ** | **broken-in-nested-layout** |
| `quickDeployCmd` (`commands.go:1283-1293`): `sf.QuickDeploy(ctx, jobID, alias)` — **no dir** | raw process cwd (`internal/salesforce/quick.go:41-50` sets no `Dir`) | **PROJ** | **broken-in-nested-layout** + latent inconsistency today; interface change required |
| `cancelCmd` (`commands.go:1264-1273`): `sf.CancelDeploy(ctx, jobID, alias)` — **no dir** | raw process cwd (`internal/salesforce/cancel.go:86-95`) | **PROJ** | **broken-in-nested-layout**; interface change required |
| `queueCmd` (`commands.go:1238-1256`): `sf.Orgs`, `sf.ListDeployQueue` | no dir | N/A — `sf org list` / `sf data query` are not `sf project` commands | correct (low risk: no `Dir` today, none proposed) |
| `runPrereqCmd` (`commands.go:502-517`) → `newChecker(dir)` | `m.deps.Dir` | mirrors `defaultChecker`'s multi-need split | ambiguous-needs-decision |
| `resumeDetectCmd` (`commands.go:1085-1106`) | `m.deps.Dir` for git; `Runs.Writer` already bound at construction | GIT (self-resolving) | correct as-is; the real decision is upstream at `runs.NewWriter(dir)` |
| `view.go:116, 151` — `filepath.Base(m.deps.Dir)`, `"Repo: %s"` | `m.deps.Dir` | CWD — pure display | correct (cosmetic; the label changes if `Dir` becomes project-root vs git-root — a one-line UX decision for design, not a defect) |
| `view.go:1355` `runPackagePath` — `filepath.Join(deltaBaseDir(cfg), rec.Ticket+"-to-"+rec.Target, "package", "package.xml")` | a **relative** path built from `delta.outputDir` | must be joined against the same artifacts root `deltaCmd` used at `commands.go:922` | **[CORRECTED BY GATE]** the explore draft described this as reading "`cfg.Delta.SourceDirs`-adjacent config"; it does not touch `SourceDirs` at all. It composes the delta **output** base (`deltaBaseDir(cfg)`). Still in scope: it is the second consumer of the artifacts-root decision and must not drift from `deltaCmd` |

### internal/prereq

| Site | Currently receives | Actually needs | Verdict |
|---|---|---|---|
| `checker.go:32-58` `Checker.Dir` field | one `dir` for the whole `Checker` | split: GIT for repo checks, PROJ for SF/config checks | ambiguous-needs-decision |
| `checker_repository.go:13` → `RepoRoot(ctx, c.Dir)` | `c.Dir` | GIT (self-resolving) | correct |
| `checker_workingtree.go:11` → `Status(ctx, c.Dir)` | `c.Dir` | GIT (self-resolving; deliberately whole-repo — a dirty tree anywhere blocks checkout/cherry-pick) | correct |
| `checker_hooks.go:28-33` → `RepoRoot` then `<root>/.git/hooks` | `c.Dir` | GIT | correct |
| `checker_gpgsign.go:13` → `ConfigGet(ctx, c.Dir, ...)` | `c.Dir` | GIT | correct |
| `checker_gitignore.go:59-74` `readGitignore` → `RepoRoot` then `<gitroot>/.gitignore` | `c.Dir` resolved up to the git root | must match wherever `.deploydeck/` is **actually created** | **broken-in-nested-layout** — confirmed bug #2 |
| `checker_gitignore.go:38-57` `AddGitignoreEntry` | same base | same | **broken-in-nested-layout** — writes to the git root, creating an untracked file that then trips `CheckWorkingTree` |

### internal/salesforce

| Site | Currently receives | Actually needs | Verdict |
|---|---|---|---|
| `client.go:20-61` `Client` interface | `ValidateDeploy`/`ReportDeploy` take `Dir`; `QuickDeploy`/`CancelDeploy`/`ListDeployQueue` do not | PROJ for all four `sf project deploy *` methods | interface is **inconsistent** — `QuickDeploy`/`CancelDeploy` need a `dir string` parameter |
| `validate.go:19-25, 76-81` `ValidateRequest.Dir` → `exec.CommandRequest{Dir: req.Dir}` | caller-supplied | PROJ | broken-in-nested-layout; doc comment mislabels it "repository root" |
| `report.go:238-248` `ReportDeploy(..., dir string)` | caller-supplied | PROJ | broken-in-nested-layout |
| `quick.go:41-50` `QuickDeploy` — no `Dir` | raw process cwd | PROJ | broken-in-nested-layout; interface change required |
| `cancel.go:86-95` `CancelDeploy` — no `Dir` | raw process cwd | PROJ | broken-in-nested-layout; interface change required |
| `queue.go:193-203` `ListDeployQueue` — no `Dir` | raw process cwd | N/A (`sf data query`) | correct |
| `client.go` `Version`/`Plugins`/`Orgs` | no `Dir` | N/A | correct |

### internal/delta

| Site | Currently receives | Actually needs | Verdict |
|---|---|---|---|
| `service.go:20-46` `Request.Dir` — doc: *"the repository root `sf sgd source delta` runs in"* | caller-supplied | GIT, and **must be explicit** (sgd does not walk up) | the field contract is right; the *caller* is what's broken |
| `service.go:80-102` `Generate` → `exec.CommandRequest{Dir: req.Dir}`; `buildArgs` (`service.go:108-126`) never emits `--repo-dir`, relying on sgd's `./` default | `req.Dir` | GIT | **broken-in-nested-layout** — bug #1. Fix options: (a) always pass a git-root `Dir`, or (b) emit an explicit `--repo-dir <gitroot>`, which decouples the child's cwd from its repo-dir target and is the more robust of the two |
| `package.go:97` `Summarize`, `package.go:142` `outsideSourceDirs`, `package.go:155` `underAnySourceDir` — raw string prefix match between `sourceDirs` and `changedFiles` | both must share one base | both **repo-root-relative** | **load-bearing constraint.** `changedFiles` comes from `git.ChangedFiles` (`internal/git/changed_files.go:13`), which runs `git diff --name-only` at the resolved repo root — always repo-root-relative. Whatever reaches `Summarize` and `buildArgs`'s `--source-dir` **must** stay repo-root-relative, or every changed file spuriously reports as "outside sourceDirs" |

### internal/runs

| Site | Currently receives | Actually needs | Verdict |
|---|---|---|---|
| `writer.go:109-119` `Writer{baseDir}`, doc: *"baseDir is always an explicit, already-resolved repository root passed by the caller — never assumed to be the process cwd"* | `runs.NewWriter(dir)` at `main.go:283`, where `dir` is the process cwd | the doc states GIT root; the composition root never honors it | **broken-in-nested-layout** — the intended contract is already written down and violated. `.deploydeck/runs`, `.deploydeck/manifest/delta` and the gitignore-check target must all agree on one root |

### internal/git — confirmed already correct

Every `Service` method resolves `RepoRoot(ctx, dir)` internally before building its
`exec.CommandRequest`, via the shared `newRequest(dir, args...)` helper
(`internal/git/request.go:19`) which is always called with the *resolved* root. This package needs
**no change**; `internal/git/service_root.go:9-13` states the invariant explicitly.

### internal/github

| Site | Currently receives | Actually needs | Verdict |
|---|---|---|---|
| `client.go:121-153` `AuthStatus`, `CreatePR` — no `Dir` | raw process cwd | GIT, or nothing (`gh` operates on branch names, not paths) | **correct — VERIFIED BY GATE.** `gh repo view --json nameWithOwner` run from the nested project subdirectory of the real repo resolved `{"nameWithOwner":"giss-salesforce/salesforce"}`, proving `gh` walks up to find `.git` the way git does, unlike `sf`. `internal/github` needs no change |
| `PRForBranch`, `PRDetails`, `PRReviews`, `PostComment`, `UnresolvedThreadCount` | URL/branch-name based | N/A | correct |

### internal/config

| Site | Currently receives | Actually needs | Verdict |
|---|---|---|---|
| `load.go:14-34` `Load(dir)` — reads `<dir>/deploydeck.yaml`, errors if absent | `dir` verbatim from every call site | must be discoverable from wherever the user runs `deploydeck` | the crux of Part B |
| `validate.go:20-83` `Validate()` | pure | N/A | correct |
| `config.go:129-178` `Config` struct | N/A | needs an additive `ProjectDir` field | see Part B |

### internal/gate, internal/provenance

Pure computation over already-fetched `gh` data and HMAC signatures — no directory dependency
(`gate.go:124` `Evaluate(f Facts)`, `provenance.go:153` `Verify(...)`). **N/A**, no change needed.

---

## B. Design options for a new config parameter

### B1. Key name and shape

Add a **top-level** additive field on `Config` (`internal/config/config.go:129`), relative to the
git root, zero-value-safe (empty ⇒ "the git root IS the project root", today's implicit behavior):

```go
// ProjectDir is the SFDX project's location relative to the git root
// (e.g. "up_saln0001_giss_salesforce"), for repos where the git root and
// the sfdx-project.json directory differ. Empty (default) means the git
// root IS the project root — today's flat-layout behavior, unchanged.
ProjectDir string `yaml:"projectDir"`
```

It is not `delta`-scoped because it is consumed by `salesforce.ValidateRequest`/`ReportDeploy`/
`QuickDeploy`/`CancelDeploy`, the artifacts-root decision, and the gitignore check — not delta
alone. This follows the established additive, zero-value-safe pattern (`QuickDeployConfig`,
`AIConfig`, `GateConfig` all default to "off"/"same as before" on omission).

Validation (`validate.go`): reject an absolute path or one containing `..` segments — defense in
depth against escaping the repo.

### B2. Chicken-and-egg: where must `deploydeck.yaml` live?

| Option | Mechanism | Flat-layout compatibility | Nested-layout ergonomics |
|---|---|---|---|
| **(a) Keep it in the project dir** (status quo) | `Load(dir)` unchanged, `dir` = cwd | identical | works today — but `ProjectDir` becomes redundant with "where the file was found" |
| **(b) Require it at the git root** | `Load` always reads `<gitroot>/deploydeck.yaml` | identical for flat repos | forces migrating the real repo's existing file; matches `ARQUITECTURA.md`'s framing |
| **(c) Search upward from cwd to the git root, inclusive** (recommended) | new pure `config.Locate(startDir) (dir, error)`; tries `startDir`, walks `filepath.Dir` up to and including the resolved git root; first hit wins. `Load(dir)` unchanged — zero risk to its existing tests | identical: for a flat repo the first probe hits, zero hops | supports **both** conventions without forcing a migration |

**Recommendation: (c).** It is the only option that is unconditionally backward-compatible *and*
does not force the one real-world nested repo to move its existing `deploydeck.yaml`. The upward
bound is resolvable without config — `git.RepoRoot(cwd)` has no config dependency — so the
chicken-and-egg resolves by sequencing: (1) `os.Getwd()`, (2) `git.RepoRoot(cwd)` (main.go already
imports `internal/git`; no boundary issue), (3) `config.Locate(cwd)` bounded at that git root,
(4) `config.Load(foundDir)`, (5) compose `projectDir`.

Edge case to preserve: if step (2) fails (cwd is not inside any git repository), skip the upward
search and fall back to today's exact `Load(cwd)` behavior, so the existing "not a git repository"
prereq message stays the primary diagnostic instead of being masked by "deploydeck.yaml not found".

### B3. Should `delta.sourceDirs` become project-relative?

**[CORRECTED BY GATE — the explore draft recommended B3b on a false premise.]**

The draft justified making `sourceDirs` project-relative with: *"since this feature doesn't exist in
any shipped release yet, the doc-vs-code conflict costs nothing to resolve now."* That premise is
false. Verified by the orchestrator:

- `SourceDirs` is present in `internal/config/config.go` at tag **`v1.3.1`** (`git show
  v1.3.1:internal/config/config.go` → `SourceDirs []string \`yaml:"sourceDirs"\``). It has shipped.
- It is mandated by a **living spec**: `openspec/specs/delta-generation/spec.md:9` requires passing
  "every configured `delta.sourceDirs` entry as a repeated `--source-dir` flag".
- It is documented repo-root-relative in `docs/ARQUITECTURA.md:317-321`.
- It is in **active use in a real user config** in exactly that form:
  `/Users/am/Documents/salesforce/up_saln0001_giss_salesforce/deploydeck.yaml` carries
  `sourceDirs: [up_saln0001_giss_salesforce/force-app]`.

Concrete silent-failure scenario if `sourceDirs` were reinterpreted as project-relative: that same
user adds `projectDir: up_saln0001_giss_salesforce` and DeployDeck composes
`up_saln0001_giss_salesforce/up_saln0001_giss_salesforce/force-app`. sgd matches nothing, produces
an **empty but successful** package, and `Summarize` reports every changed file as "outside
sourceDirs". An empty package that validates green is the worst available failure mode for a deploy
tool — it looks like success.

**Corrected recommendation: B3a — keep `sourceDirs` repo-root-relative, unchanged.** `projectDir`
governs only the `sf`-command cwd, the artifacts root, and sgd's `--repo-dir`. This solves every
confirmed defect while touching zero existing configs, zero living specs, and zero shipped docs.

A project-relative surface remains a legitimate *future* ergonomic improvement, but it must be an
explicit, separately-keyed opt-in (so no existing value silently changes meaning), and it belongs in
its own change — not bundled here.

### B4. Auto-detection as a complement

Locate `sfdx-project.json` under the resolved git root (excluding `.git/`) as a fallback **only when
`ProjectDir` is unset**:

- **Zero matches** → `projectDir = gitRoot` (today's exact default; safe for non-SFDX/flat repos).
- **Exactly one match** → use its directory. Matches the one real-world repo inspected.
- **Two or more matches** → **never guess.** Fail closed with an actionable error requiring an
  explicit `projectDir:`.

Additive and strictly safer than requiring explicit configuration everywhere, but a *complement*:
explicit `projectDir` always wins. Part C must still test the zero and two-or-more branches.

### B5. Backward compatibility — explicit statement

| Existing behavior | With `ProjectDir` absent |
|---|---|
| `config.Load(cwd)` finds `deploydeck.yaml` immediately | `config.Locate(cwd)` probes `cwd` first — identical, zero-hop |
| `.deploydeck/runs` and delta `outputDir` anchor to `cwd` | `gitRoot == cwd` for a flat repo → identical paths |
| `sf project deploy validate/report/quick/cancel` run with `cwd` | `projectDir == gitRoot == cwd` → identical |
| `sourceDirs` interpreted repo-root-relative | unchanged under the corrected B3a — identical |
| gitignore check reads `<gitroot>/.gitignore` | artifacts root and `gitRoot` still coincide → same file |

Every row collapses to "no observable change" because `ProjectDir == ""` makes
`projectDir == gitRoot`, and for a flat repo `gitRoot == cwd`. Same reasoning the codebase already
applies to `QuickDeployConfig`/`AIConfig`/`GateConfig`.

---

## C. Test strategy under strict TDD

`go test ./...`, `strict_tdd: true` — every behavior change needs a RED test first.

### Existing temp-git-repo helpers (found, not guessed)

- `internal/git/helpers_test.go:58` `newTempRepo(t) string` — flat: `git init` at the returned dir,
  commits `README.md` at its root, pushes to a bare `origin`.
- `internal/git/helpers_test.go:101` `newTempRepoWithRemote(t)` — flat clone/remote-advance harness.
- `internal/prereq/helpers_test.go:43` `newTempRepo(t, withOrigin bool) string` — a duplicated flat
  variant (Go test helpers are not importable across packages), used by
  `checker_gitignore_test.go`.
- `internal/delta/generate_e2e_test.go:24-58` — copies the real `test-e2e-org/` fixture into a temp
  git repo **at its root**, then runs the real `sf sgd source delta`.

**Extension**: add a `newNestedTempRepo(t) (gitRoot, projectDir string)` sibling in each of
`internal/git/helpers_test.go` and `internal/prereq/helpers_test.go`, plus a nested variant of the
delta fixture copy: `git init` at `gitRoot`, then create a subdirectory and commit the SFDX
fixture / `deploydeck.yaml` / `.gitignore` **inside it**. Returning both paths lets each test assert
the correct one explicitly ("sgd's repo-dir must equal `gitRoot`", "`sf project deploy validate`'s
`Dir` must equal `projectDir`").

### Test files needing a nested-layout sibling

- `internal/git/service_root_test.go` — `RepoRoot(ctx, projectDir)` resolves the same root.
- `internal/prereq/checker_gitignore_test.go` — regression test for bug #2.
- `internal/delta/generate_e2e_test.go`, `internal/delta/multi_source_dir_e2e_test.go` — regression
  tests for bug #1.
- new `internal/config/locate_test.go` — found-immediately (flat), found-after-walking-up (config at
  git root), found-immediately-in-subdir (today's real repo), not-found-anywhere, and the
  not-inside-a-git-repo fallback.
- `internal/config/config_test.go` / `validate_test.go` — `ProjectDir` defaulting (empty stays
  empty; no `applyDefaults` entry, mirroring `QuickDeployConfig`) and rejection of absolute or
  `..`-containing values.
- `internal/app/standalone_modes_e2e_test.go`, `internal/app/delta_validation_test.go` — nested
  siblings asserting `deltaCmd`/`validateCmd` dispatch with the **correct, different** dirs.
- `internal/salesforce/quick_test.go`, `internal/salesforce/cancel_test.go` — once `QuickDeploy`/
  `CancelDeploy` gain a `dir` parameter, assert it is threaded into `exec.CommandRequest`, mirroring
  `validate_test.go`. No hand-written fake `salesforce.Client` exists (tests use the real
  `salesforce.New(runner)` over a fake runner), so the blast radius is the two implementation files
  plus their direct tests.

### `internal/app/boundary_test.go`

`TestApp_NeverImportsExecSeam` (`boundary_test.go:40-65`) is a direct-import check via
`build.ImportDir` against a fixed forbidden list. Adding plain-string `ProjectDir`/`GitRoot` fields
to `app.Deps`, resolved in `main.go`, introduces **no new imports** in `internal/app`. This change is
not expected to touch the allow-list — confirm once the concrete `Deps` shape is decided in design.

### Suggested TDD sequence (informational)

1. `internal/config` — `Locate` upward search.
2. `internal/config` — `ProjectDir` field + validation.
3. `internal/prereq` — gitignore nested case (bug #2; most isolated, highest value).
4. `internal/delta` — nested `Generate` case (bug #1).
5. `internal/salesforce` — `QuickDeploy`/`CancelDeploy` `dir` parameter + call sites.
6. `internal/app` — nested e2e siblings wiring it through `Model`.
7. `cmd/deploydeck` — composition root resolving gitRoot/configDir/projectDir once.

---

## Result contract

- **status**: done
- **executive_summary**: Audited every directory-consuming call site across `cmd/deploydeck` and
  `internal/{app,prereq,salesforce,delta,runs,git,github,config,gate,provenance}`. `internal/git` is
  already root-safe; `internal/salesforce`'s four `sf project deploy *` methods, `internal/delta`'s
  implicit sgd repo-dir, `internal/prereq`'s gitignore check, and `internal/runs.Writer`'s artifacts
  root all conflate git-root / project-root / cwd. Recommend an additive `projectDir` config key
  resolved via an upward `deploydeck.yaml` search bounded at the git root, with `sfdx-project.json`
  auto-detection as a fail-closed complement, and `sourceDirs` left repo-root-relative.
- **artifacts**: `openspec/changes/project-dir-resolution/explore.md`
- **next_recommended**: sdd-propose
- **risks**:
  1. ~~`internal/github`'s cwd-based repo resolution for `gh` is assumed correct.~~ **CLOSED by the
     orchestrator gate**: verified empirically from the nested project subdirectory of the real repo
     (`gh repo view --json nameWithOwner` → `{"nameWithOwner":"giss-salesforce/salesforce"}`). `gh`
     walks up to find `.git`. `internal/github` is out of scope for this change.
  2. `QuickDeploy`/`CancelDeploy` gaining a `dir` parameter is a `salesforce.Client` interface
     change. Low blast radius (no hand-written fakes) but a breaking signature change to sequence
     carefully.
  3. **[GATE]** The explore draft's B3b recommendation rested on a false "not yet shipped" premise
     and would have silently broken every existing `sourceDirs` config by double-prefixing. Corrected
     to B3a. Any future move to project-relative `sourceDirs` must use a separate key, never
     reinterpret the existing one.
- **skill_resolution**: paths-injected — 1 skill (go-testing)
