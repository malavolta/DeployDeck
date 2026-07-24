# Design: Foundation + MVP Git (HU-001..HU-006)

## Technical Approach

Bootstrap the Go project (module `deploydeck`) as a Cobra CLI + Bubble Tea TUI over a
single external-command seam. `internal/exec` is the ONLY process boundary; `internal/git`
and `internal/salesforce` compose it; `internal/app` (TUI) never execs. The whole slice is
built on one injectable `Runner` interface: fakes make prereq/discovery/selection logic
pure-unit testable, while cherry-pick uses the real runner against a temp git repo. Git state
is owned by the repo (reconciled from `CHERRY_PICK_HEAD`/`.git/sequencer`/index), never by the
Bubble Tea model. Maps to proposal build order exec → config → git → HU-001..006.

## Package Layout & Public Surface

| Package | Responsibility | Public surface |
|---|---|---|
| `cmd/deploydeck` | Entry; Cobra root + `doctor` subcmd (non-zero exit on blockers); starts Bubble Tea | `main()`, `newRootCmd(deps)` |
| `internal/exec` | Sole external-command boundary; timeout/ctx-cancel; captures stdout/stderr/exit/duration | `Runner`, `CommandRequest`, `CommandResult`, `NewOSRunner`, `FakeRunner` |
| `internal/config` | Load/defaults/validate `deploydeck.yaml` | `Config`, `Load(dir)`, `(Config).Validate()`, `SandboxFor(branch)` |
| `internal/git` | repo/worktree validation, log search, topo-order, equivalence, temp branch, cherry-pick engine, repo-state reconcile | `Service`, `Commit`, `RepoState`, `New(Runner)` |
| `internal/salesforce` | read-only shim (3 calls); Fase-3-extensible | `Client`, `New(Runner)`, structs below |
| `internal/prereq` | HU-001 checks (reused by TUI + doctor); lock acquire | `Checker`, `[]Check`, `Lock` |
| `internal/app` | TUI state machine PrereqCheck→PickVerification; coordinates services; `tea.ExecProcess` handoffs | `Model`, `New(deps)`, `Update`, `View` |

## Architecture Decisions

### Decision: exec `Runner` interface as the universal seam
**Choice**: One `Runner` interface injected into git/salesforce/prereq.
**Alternatives**: package-level funcs / build tags for fakes.
**Rationale**: A single seam makes discovery/parsing pure-unit testable with canned output and keeps `internal/app` exec-free per `ARQUITECTURA.md:56-59`.

```go
type Runner interface { Run(ctx context.Context, req CommandRequest) (CommandResult, error) }
// CommandRequest/CommandResult per ARQUITECTURA.md:255-275.
// git.Service wraps every git req with Env: GIT_EDITOR=true, GIT_TERMINAL_PROMPT=0, GIT_PAGER=cat.
// NewOSRunner MUST LAYER env, not replace it: cmd.Env = append(os.Environ(), req.Env...).
//   Go's os/exec REPLACES the whole environment when cmd.Env != nil, so a bare cmd.Env = req.Env
//   strips inherited PATH/HOME/SSH_AUTH_SOCK/GIT_* and breaks `git fetch` over SSH (HU-005) while
//   FakeRunner unit tests stay green. The GIT_EDITOR/GIT_TERMINAL_PROMPT/GIT_PAGER overrides are
//   layered ON TOP of the inherited environment, not a substitute for it.
// Tool-created commits: git.Service passes `-c commit.gpgsign=false` on ITS OWN cherry-pick and
//   `--continue` invocations (these CREATE commits; promotion commits need no signature). With
//   commit.gpgsign=true (common in enterprise Salesforce repos) they invoke gpg with no tty and
//   hang — same non-interactive failure class GIT_EDITOR=true prevents.
// Repo authority: req.Dir = repo root (from `git rev-parse --show-toplevel`); never shell `cd`.
```
Tests inject `FakeRunner` (map `Name+Args` → canned `CommandResult`, the `sf-fake`/`git-fake`); integration uses `NewOSRunner` against `newTempRepo`.

**Non-zero-exit-as-data contract**: a process that RAN but exited non-zero is NOT a `Runner` error — `CommandResult.ExitCode` is populated (from `*exec.ExitError`) and a clean non-zero exit is distinguishable from a *start* failure (binary missing / not executable → non-nil error, `ExitCode` = -1) and from *timeout/ctx-cancel* (non-nil error with `ctx.Err()` set). Git exit codes used as DATA are interpreted per-command, never treated as failures: `merge-base --is-ancestor` (1 = not ancestor), `cherry-pick` (1 = conflict), `diff --quiet` (1 = differs), `log --grep` (1 = no match).

### Decision: `.deploydeck/lock` — PID + start-time file with injected liveness prober
**Choice**: JSON lock; atomic `O_CREATE|O_EXCL` acquire; on collision probe owner liveness AND process start-time through an injected `ProcessProber`; stale (dead owner OR reused PID) → take over by removing then re-winning O_EXCL, live → refuse naming the owner (HU-001 AC `HISTORIAS.md:53`).
**Alternatives**: bare PID file (PID reuse false-positives); OS advisory `flock` (not testable cross-platform, no owner name); rename-over-lock for takeover (rejected: `rename()` over an existing lock is NOT exclusive, so two instances observing the same stale lock both rename and both believe they own it).
**Rationale**: Liveness + start-time comparison detects a dead owner and defeats PID reuse; the prober seam makes takeover/refuse pure-unit testable. Takeover MUST re-win via `O_CREATE|O_EXCL` (never rename over an existing lock) so exactly one instance owns a contested stale lock.

```go
type LockInfo struct {
    PID int `json:"pid"`; PName string `json:"pname"`; Host string `json:"host"`
    StartedAt time.Time `json:"startedAt"`; CreatedAt time.Time `json:"createdAt"`
}
// Prober seam carries StartedAt so the PID-reuse guard is REAL, not decorative: Alive(pid) alone
// cannot compare the OS process start-time to the stored StartedAt. Widened signature reads the
// live start-time (darwin: sysctl KERN_PROC / `ps -o lstart`; linux: /proc/<pid>/stat) and returns
// false when the PID exists but its start-time != stored StartedAt (reused PID).
type ProcessProber interface {
    Alive(pid int, startedAt time.Time) bool // dead OR start-time mismatch → false (owner gone)
}
```
Acquire: `OpenFile(O_CREATE|O_EXCL)` → own it; write `LockInfo` to a temp file first, then rename it onto the exclusively-created path (or write the full record to the O_EXCL fd before it is discoverable) so a concurrent reader never observes a partial record — a reader that hits a truncated/unparseable record re-reads once before deciding. Defer release (remove only if PID matches; also on SIGINT/SIGTERM). On `EEXIST` read `LockInfo`: same host + `Alive(pid, StartedAt)` → **refuse**, blocking `Check` = `"otra instancia activa: <pname> (pid <PID>)"`; owner dead or PID reused → **stale**: `os.Remove` the stale lock, then RETRY `OpenFile(O_CREATE|O_EXCL)` in a bounded loop — never rename over an existing lock — so O_EXCL yields exactly one winner; the loser re-reads the new owner's `LockInfo` and refuses. Different host (cannot probe) → refuse conservatively naming host+pid.

### Decision: `internal/salesforce` = interface Fase 3 extends, not rewrites
**Choice**: Small `Client` interface over `Runner`; Fase 3 defines `DeployClient interface { Client; DeployValidate(...); DeployReport(...); Queue(...) }`.
**Alternatives**: free functions (no seam) / one fat struct now (premature).
**Rationale**: Adding methods to an interface backed by the same runner needs no change to the read-only calls.

```go
type Client interface {
    Version(ctx context.Context) (VersionInfo, error) // sf --version
    Plugins(ctx context.Context) ([]Plugin, error)    // sf plugins --json
    Orgs(ctx context.Context) (OrgList, error)         // sf org list --json
}
type VersionInfo struct { CLIVersion string; Raw string }
type Plugin struct { Name string `json:"name"`; Version string `json:"version"` }
type Org struct { Alias, Username, ConnectedStatus string; IsDefault bool `json:"isDefaultUsername"` }
type OrgList struct {
    NonScratch []Org `json:"nonScratchOrgs"`; Scratch []Org `json:"scratchOrgs"`
    Sandboxes []Org `json:"sandboxes"`; DevHubs []Org `json:"devHubs"`; Other []Org `json:"other"`
}
// Alias existence is checked across ALL five categories (nonScratchOrgs, scratchOrgs, sandboxes,
// devHubs, other), not just two, to avoid false-negatives when the alias lives in a non-default bucket.
// sf JSON envelope {"status":0,"result":...} decoded per-call; parsing is pure-unit tested.
```

### Decision: repo-as-source-of-truth Bubble Tea model
**Choice**: `Model` holds `state`, `DeploymentPlan`, service deps; on CherryPicking/Conflict it `tea.Tick`-repolls `git.RepoState()` and derives UI from repo, reconciling external `--continue`/`--abort`.
**Alternatives**: internal state machine as truth (drifts from repo).
**Rationale**: `ARQUITECTURA.md:408` mandates repo state as truth; live re-poll satisfies HU-006 auto-refresh AC.

### Decision: cherry-pick engine driven by git's sequencer, NOT a Go loop of single-sha picks
**Choice**: The engine hands git the full ordered set in ONE invocation — `git cherry-pick <sha1>..<shaN>` (or the explicit ordered SHA list `git cherry-pick <sha1> <sha2> ... <shaN>` when selection is non-contiguous) — so git applies them one at a time and populates `.git/sequencer/todo`.
**Alternatives**: a Go loop calling `git cherry-pick <single-sha>` per iteration — REJECTED. A single-commit pick only writes `CHERRY_PICK_HEAD` and NEVER creates `.git/sequencer`, so `RepoState.SequencerRemaining` (derived from `.git/sequencer/todo`) would always be empty and AC6 reconciliation of external `--continue`/`--skip`/`--abort` would have no remaining-count source.
**Rationale**: Letting git own the sequence makes `SequencerRemaining` a real value read from `.git/sequencer/todo`, and native `git cherry-pick --continue`/`--skip`/`--abort` plus external-action reconciliation work exactly as specced. The continue-gate predicate still runs before the tool issues `--continue`; empty picks route to `--skip`. No wording in this design implies a Go loop of single-sha picks.

## Data Flow

    app.Model.Update(tea.Msg) ──cmd──▶ git.Service ──req──▶ exec.Runner ──▶ git/sf CLI
         ▲  (derives UI)                    │                                    │
         └──── RepoState (CHERRY_PICK_HEAD/sequencer/status) ◀───── repo ◀───────┘
    doctor CLI ──▶ prereq.Checker ──▶ {git.Service, salesforce.Client, config, Lock}

## State Machine (cherry-pick subset)

```mermaid
stateDiagram-v2
    PrereqCheck --> TicketInput: OK
    TicketInput --> CommitDiscovery --> CommitSelection
    CommitSelection --> TargetSelection --> PlanPreview --> BranchCreation --> CherryPicking
    CherryPicking --> CherryPickConflict: conflict
    CherryPickConflict --> CherryPicking: --continue (gate passed)
    CherryPicking --> CherryPicking: empty pick --> --skip
    CherryPickConflict --> Suspended: quit (HU-013 resume — OUT of slice)
    Suspended --> CherryPickConflict: reopen (HU-013 — OUT of slice)
    CherryPickConflict --> Aborted: --abort
    CherryPicking --> PickVerification: all picks done
    PickVerification --> CommitSelection: partial promotion
    PickVerification --> [*]: content == source (scope edge; DeltaGeneration OUT)
```

## Interfaces / Contracts

- `git.RepoState{ InProgress bool; CurrentSHA string; Unmerged []ConflictFile; SequencerRemaining int }` from `git status --porcelain -z` (NUL-delimited for safe parsing of paths with spaces/unicode), `git diff --name-only -z --diff-filter=U`, `CHERRY_PICK_HEAD`, and `.git/sequencer/todo` (`SequencerRemaining` = remaining todo lines; populated because the engine runs a single ranged/multi-SHA cherry-pick, see decision above).
- `ConflictFile{ Path string; Kind ConflictKind }` — Kind ∈ {Text, Binary, ModifyDelete} from porcelain XY codes (`DU`/`UD` → ModifyDelete) + binary via numstat `-`.
- Continue-gate predicate (pure): enabled ⇔ zero unmerged ∧ resolved staged ∧ no `<<<<<<<` in staged blobs.
- Equivalence (pure, given canned output): `git cherry` `-` mark ∨ `merge-base --is-ancestor` ∨ `patch-id --stable` match → `alreadyApplied`; merge commit (`>1` parent) → disabled.
- `CommitSelectionItem{ Commit; Selected; Disabled; Reason }`; per-file dependency warning from intermediate unselected commits touching same file in `origin/<target>..<source>`.
- `config.Config`: `branches`, `sandboxes` (alias+testLevel, `Release/*` glob), `ticketPatterns`, `branchFormat`, `minVersions`, `runs`. Defaults applied on load; `Validate` compiles patterns, checks each sandbox has alias, and validates `branchFormat` tokens against the render allow-list.
- `branchFormat` is rendered with `strings.NewReplacer` over literal `{{ticket}}`/`{{target}}` tokens — NOT Go `text/template`, which parses `{{ticket}}` as a function node and fails at execution (invalid identifier). `Validate` and the renderer share ONE token allow-list so the validate test and the render agree by construction. (If a template engine is later preferred, tokens switch to `{{.Ticket}}`/`{{.Target}}` in both places together.)

## Testing Strategy

| Layer | What | Approach |
|---|---|---|
| Unit (table-driven) | log/porcelain(`-z`)/`cherry`/patch-id parsing, sf JSON (all org categories), config validate + `branchFormat` render/validate agreement, selection disabled/reason, dependency warning, conflict classification, lock takeover/refuse/PID-reuse (start-time mismatch) + stale-takeover re-win via O_EXCL | `FakeRunner` + fake `ProcessProber` (canned start-time); no real processes |
| Integration | HU-001 repo/origin/dirty/.gitignore; HU-002 grep/topo/cherry; HU-003 real diff; HU-004 `rev-parse origin/<t>`+sf-fake; HU-005 two-repo bare-remote fetch; HU-006 cherry-pick/conflict/reconcile/skip/abort/verify | shared `newTempRepo` helper, `NewOSRunner`, `t.TempDir()`, skip on `testing.Short()` |
| TUI | doctor render (golden), full flow, conflict live re-poll | `teatest`; direct `Model.Update` for transitions |
| E2E | real org | OUT of slice (`DEPLOYDECK_E2E_ORG`, local only) |

CI: `go test ./...` runs all six HUs' unit + integration; no real org.

## Threat Matrix

| Boundary | Applicability | Design response | Planned RED tests |
|---|---|---|---|
| Documentation-like paths | N/A: no doc-file classification/execution; only fixed `git`/`sf` subcommands run | — | — |
| Git repository selection | Applicable | `req.Dir` = repo root from `rev-parse --show-toplevel`; never shell `cd`/relative | temp-repo run resolves + acts in intended root |
| Commit state | Applicable | staged/unmerged/empty-index semantics; empty pick → `--skip`; continue-gate requires clean staged | conflict-then-continue, empty-pick `--skip`, marker-in-staged blocks continue |
| Push state | N/A: no push in this slice (scope edge HU-006; push Fase 5) | — | — |
| PR commands | N/A: no `gh`/PR automation (HU-014 Fase 5) | — | — |

## Error Handling / Messaging Boundaries

`exec` returns structured `CommandResult` (exit+stderr) + error. git/salesforce/prereq map to domain errors; `internal/app` maps domain errors to user-facing message + `FixCommand` (mockup panes). Raw stderr only in detail views. `doctor` exits non-zero on any blocking check. Credentials never logged; `RedactArgs` honored. Git always non-interactive so `--continue` never opens an editor and hangs. `doctor` detects `commit.gpgsign` (via `git config --get commit.gpgsign`) and WARNS when it is `true`, noting the tool neutralizes it with `-c commit.gpgsign=false` on its own cherry-pick/`--continue` commits so a no-tty gpg invocation can never hang the run.

## Migration / Rollout

No migration (first code). All work on local temp branches; runtime rollback = `cherry-pick --abort` + delete temp branch; change rollback = remove `cmd/`+`internal/` tree.

## Open Questions

- [ ] None blocking. Cross-host lock refusal is conservative by design (cannot probe remote PID); revisit if repos live on shared network mounts.
