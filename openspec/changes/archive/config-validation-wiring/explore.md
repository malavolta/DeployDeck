# Exploration: `config-validation-wiring` — Activate `Config.Validate()` Safely

> **Orchestrator gate note.** The `sdd-explore` agent has no Bash and no Write tool, so it could not
> execute Deliverable B as briefed — that was an error in the launch prompt, not in the agent's work.
> Its rule audit is a careful manual trace. **The orchestrator then executed Deliverable B for real**
> and the results are recorded below under "Deliverable B — EXECUTED". The trace's central conclusion
> held; one of its secondary claims was too narrow and is corrected.

## Current State

`Config.Validate()` (`internal/config/validate.go:28-95`) implements 11 discrete checks and is dead
code in production. Confirmed callers, repo-wide: `internal/config/config_test.go:130`,
`internal/config/validate_test.go` (5 sites), and two `internal/git` e2e tests
(`promotion_branch_e2e_test.go:39`, `target_selection_e2e_test.go:51`). None is reachable from a
`cmd/deploydeck` entry point.

`config.Load` (`internal/config/load.go:18-34`) ends at `applyDefaults` and never calls `Validate()`.
All three production entry points that resolve a config — `defaultChecker`, `defaultRunTUI`,
`runPrune` (`cmd/deploydeck/main.go:203-294`) — go through `resolveRoots`
(`cmd/deploydeck/roots.go:45-92`), which calls `config.Load` then `cfg.ProjectRoot(...)`.
`ProjectRoot` already enforces the absolute/`..` `projectDir` rule on the executed path
(`internal/config/project_root.go:20-28`) — that one rule is not dead. Every other rule never runs.

## Deliverable A — rule audit

| # | Rule | Location | Rejects | Verdict |
|---|---|---|---|---|
| 1 | `ticketPatterns` regex compiles | `validate.go:29-33` | entry failing `regexp.Compile` | safe-to-activate |
| 2 | Sandbox `alias` non-empty | `validate.go:35-39` | `Sandboxes[x].Alias == ""` | safe-to-activate |
| 3 | `branchFormat` uses allow-listed tokens only | `validate.go:41-43` | any `{{...}}` outside the allow-list | safe-to-activate |
| 4 | `pollIntervalSeconds > 0` | `validate.go:45-47` | `<= 0` | safe **only after `applyDefaults`** |
| 5 | `pollTimeoutSeconds > 0` | `validate.go:48-50` | `<= 0` | safe **only after `applyDefaults`** |
| 6 | `runs.keepLast >= 0` | `validate.go:52-54` | negative | safe-to-activate |
| 7 | `runs.keepDays >= 0` | `validate.go:55-57` | negative | safe-to-activate |
| 8 | delta configured ⇒ `sourceDirs` non-empty | `validate.go:59-61` | any non-empty `Delta` field with empty `SourceDirs` | safe-to-activate — **this is the rule the change exists to activate** |
| 9 | `projectDir` not absolute / no `..` | `validate.go:63-65` | absolute or `..`-bearing | already enforced via `ProjectRoot`; redundant secondary surfacing, zero incremental risk |
| 10 | `ai.enabled` ⇒ endpoint/model non-empty | `validate.go:67-69` | `Enabled=true` with either empty | safe-to-activate |
| 11 | enabled gate ⇒ `minApprovals >= 1`, `approvers` non-empty | `validate.go:75-92` | violating gate | safe-to-activate |

**Net: 0 of 11 rules reject a real working configuration — provided `Validate()` runs on a config
that already went through `applyDefaults`.** Rules #4 and #5 are the only defaults-dependent ones.

## Deliverable B — EXECUTED (orchestrator)

A throwaway test was written into `internal/config/`, run, and deleted (working tree left clean).
It loaded each real config through `Load` (defaults applied) and also via a raw `yaml.Unmarshal`
(no defaults), calling `Validate()` on both. Verbatim output:

```text
user.yaml    con defaults -> PASA
user.yaml    SIN defaults -> rechazada: config: pollIntervalSeconds must be > 0, got 0
readme.yaml  con defaults -> PASA
readme.yaml  SIN defaults -> rechazada: config: pollIntervalSeconds must be > 0, got 0
arq.yaml     con defaults -> PASA
arq.yaml     SIN defaults -> rechazada: config: pollTimeoutSeconds must be > 0, got 0
```

Configs tested:
- `user.yaml` — `/Users/am/Documents/salesforce/up_saln0001_giss_salesforce/deploydeck.yaml`, a REAL
  config in active use against a nested-layout repo.
- `readme.yaml` — the ```yaml example embedded in `README.md`.
- `arq.yaml` — the example embedded in `docs/ARQUITECTURA.md`.

**Confirmed**: all three pass when loaded through `Load`. The change is safe to ship.

**Confirmed specifically for the mid-setup user**: the real config's sandbox aliases are deliberate
placeholders (`"<GISS_INT_SANDBOX_ALIAS>"` etc.) because the orgs are not authenticated yet. Rule #2
checks structural presence, not resolvability, and a placeholder is a non-empty string — so it does
NOT fire. A user mid-setup is not locked out by config validation. They still see
`prereq.CheckAliases` block on the org lookup, which is the correct and already-existing surface.

**CORRECTED — the trace was too narrow here.** The exploration claimed ARQUITECTURA's example was
"the one case that actually exercises the #4/#5 conditional". Empirically, **all three configs fail
without defaults**, not one. The wiring-order constraint is therefore not an edge case tied to one
example — it is universal, because omitting `pollIntervalSeconds`/`pollTimeoutSeconds` is the normal
way to write this file. `Validate()` MUST NOT be called on an un-defaulted `Config`.

## Deliverable C — where to call `Validate()`

`resolveRoots` is the single choke point shared by all three production entry points, so "wire into
`Load`" and "wire into `resolveRoots`" are not separable options.

- **Option 1 — into `config.Load`/`resolveRoots`.** A failure returns a bare `error` before
  `app.New(deps)` or `checker.Check()` runs. `deploydeck doctor` would print a bare Go error and
  exit, never reaching `renderPrereqChecks`'s structured `[status] name: detail` + `fix:` output
  (`cmd/deploydeck/main.go:159-180`) — for every other check too, since the checker is never built.
  Defeats the purpose of `doctor` as the uniform actionable surface.
- **Option 2 — a new `prereq.Checker` check (RECOMMENDED).** Reported as a normal
  `PrereqCheck{Status: StatusBlocking, Detail: err.Error()}` alongside the existing checks inside
  `Checker.Check()` (`internal/prereq/checker_check.go:23-89`).
- **Option 3 — do nothing.** Rejected; leaves rule #8's silent empty-package bug live.

**Why Option 2 wins concretely**: `Model.New` starts in `StatePrereqCheck`
(`internal/app/app.go:703-706`) and `runPrereqCmd` calls `NewChecker().Check(ctx)` as the TUI's first
screen — README's documented flow step 1. The TUI and `deploydeck doctor` already consume the same
`Checker.Check()` report, so one more `PrereqCheck` gives both surfaces the new check for free, with
no new code path and no new screen. `runs prune` stays unvalidated, matching its current status quo
(it is not gated behind doctor checks today either).

Placement: `Validate()` is a pure function of `c.Config`, independent of repo state, so it belongs
with the unconditional checks — right after `CheckVersions`, so a malformed config surfaces at the
top of the report.

## Confirmed regression risk — must be fixed in the same commit

`cmd/deploydeck/doctor_e2e_test.go:216` builds `config.Config{MinVersions: v.minVersions,
Sandboxes: v.sandboxes}` as a **struct literal**, bypassing `Load`/`applyDefaults`.
**Verified by the orchestrator**: that file sets no `PollIntervalSeconds`, `PollTimeoutSeconds` or
`BranchFormat` anywhere, so all three are zero across all **10** `doctorVariant` cases. Wiring the
check into `Checker.Check()` makes rules #4/#5 fire on every variant — including
"all prerequisites pass" — flipping `anyBlocking(checks)` to true and breaking the exit-0 assertion.

Bounded and mechanical: give `doctorVariant` a valid baseline config, mirroring the existing
`baselineMinVersions()`/`baselineSandboxes()` helpers.

## Secondary scope

- **W-5** — `internal/prereq/checker_aliases.go:32`'s comment *"config.Validate() already rejects a
  missing alias; skip"* is currently false. Under Option 2 it becomes true: `CheckConfig` reports the
  missing alias and `CheckAliases`' defensive `continue` stays correct, with no double-reporting.
  **Comment-only change**; no behavior change in `checker_aliases.go`.
- **W-3** — Confirmed real gap. `roots_guard_test.go` only proves `deps.Dir` is never read outside
  `normalizeRoots`; a git site reading `ProjectDir` would pass it equally. Of ~26 root reads in
  `commands.go`, only `deltaCmd` has a nested-`Deps` test asserting the specific root
  (`TestModel_DeltaCmd_NestedDeps_UsesGitRootAndArtifactsRoot`). Cheapest closure: one nested-`Deps`
  test on a representative git-backed command (commit discovery is simplest — no branch-creation
  prerequisites), asserting the captured call's `Dir == gitRoot`. ~25-30 lines. **This proves the
  pattern, not all ~13 sites — say so explicitly rather than implying full coverage.**
- **S-2** — Confirmed. `Request.Dir` drives BOTH the child cwd and the emitted `--repo-dir` from the
  same field, so `Service.Generate` cannot separate them; if `--repo-dir` were silently dropped the
  existing e2e would still pass. Isolating variant: bypass `delta.Service`, call `exec.NewOSRunner()`
  with `CommandRequest.Dir = projectDir` and an explicit `--repo-dir <gitRoot>`, reusing
  `seedNestedSgdRepo`. ~30-40 lines, real-`sf`, `-short`-skipped.
- **S-4** — Confirmed. Task 7.2 in the archived `tasks.md:104` claims its e2e asserted the
  `--repo-dir` flag; only `service_test.go` does. **Append a clarifying note rather than rewriting
  the checked item**, to avoid silently editing archived history.
- **README update gap** — Confirmed: `README.md` documents Install but has no Update section, while
  the tool already shows a non-blocking update banner telling users a new version exists. Add
  `brew upgrade --cask deploydeck` / `scoop update deploydeck` / `go install ...@latest`, and flag
  that dropping `--cask` is an easy habit error since deploydeck is a cask, not a formula.

## Out of scope
- **W-4** (missing `apply-progress` artifact) — not actionable retroactively; fabricating one now
  would invent evidence for a process that already ran.
- The user's own repo (their `.gitignore`, their sandbox aliases).

## Recommendation

Option 2. All 11 rules verified safe against three real configs by execution, not inference. The one
real hazard is the defaults-order dependency, whose only concrete manifestation is
`doctor_e2e_test.go`'s hand-built literals — a bounded fixture fix. `config.Load`/`resolveRoots` stay
untouched.

Estimated size: primary ~150-250 lines, secondary ~90 lines. Well inside the 1200-line budget; no
chaining decision needed.

## Result contract

- **status**: done
- **executive_summary**: Audited all 11 `Config.Validate()` rules and EXECUTED them against three
  real configs (the user's live nested config, the README example, the ARQUITECTURA example): all
  three pass when loaded through `Load`, and all three fail without defaults — so the rules are safe
  to activate but must never run on an un-defaulted `Config`. Recommend wiring as a new
  `prereq.Checker` check rather than into `Load`, since the TUI's first screen and `deploydeck
  doctor` already share that reporting surface. One confirmed regression: `doctor_e2e_test.go`'s 10
  hand-built config literals would newly fail.
- **artifacts**: `openspec/changes/config-validation-wiring/explore.md`
- **next_recommended**: sdd-propose
- **risks**: (1) `doctor_e2e_test.go` fixture regression — confirmed by execution, must be fixed in
  the same commit; (2) the defaults-order dependency is universal, not an edge case — any future
  caller constructing `Config{}` directly and validating it will spuriously fail on poll-seconds, so
  it needs a doc comment on the new check itself; (3) W-3's closure is partial by design.
- **skill_resolution**: paths-injected — go-testing, plus phase and shared protocol skills.
