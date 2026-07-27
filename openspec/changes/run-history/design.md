# Design: HU-013 — Run History, Retention & Resume (`run-history`)

## Technical Approach

Extend the minimal `internal/runs` writer additively (new `Record` fields + `List`/`Load`/`Save`/`Prune`), then wire a startup resume-detection `tea.Cmd` at `onPrereqDone` that composes the EXISTING `git.Service.RepoState` + `runs.Writer.List` — zero new exec seam; `internal/app` still never execs (`boundary_test.go` holds). The run record is now created at promotion start (branch creation) and updated in place through the flow, so a crash mid-cherry-pick or mid-validation leaves enough context to rehydrate. Browse and resume share ONE new `StateRunHistory` screen (mockup `MOCKUPS_TUI.md:369-388`); retention is a pure function behind `deploydeck runs prune` + config bounds. Closes the HU-006 (`cherry-pick/spec.md:142`) and HU-011 (`validation-progress/spec.md:75`) deferrals.

## Architecture Decisions

| Decision | Choice | Rejected | Rationale |
|---|---|---|---|
| Run creation point | Create at `onBranchCreated`, update in place; validate REUSES `m.runID` and merges git-phase fields | Keep create-at-validate | Resume needs Ticket + PickTotal persisted DURING cherry-pick, before any jobId exists |
| Resume offer surface | Detection routes to `StateRunHistory` pre-selected on the resumable run; `Enter` resumes, `q`/`esc` declines → `StateTicketInput` | New `StateResumeOffer` / `StateSuspended` | Mockup already defines history as the "Enter reanudar" surface; no redundant state (decision #5) |
| Schema evolution | Additive fields, NO `SchemaVersion` bump | Version bump + migration | Go json zero-defaults missing fields → prior-slice `run.json` stays forward-readable |
| Retention | Pure `selectPruneCandidates`; KEEP if within most-recent `keepLast` (by CreatedAt) OR age ≤ `keepDays`; prune only if outside BOTH | Prune by count-only / days-only | Safe reading of "últimos 30 runs o 90 días" (`ARQUITECTURA.md`) |
| Resync | Record says conflict but no `CHERRY_PICK_HEAD` → reconcile (mark aborted/resolved via `Save`) before normal flow | Trust the record | Repo is source of truth (AC `HISTORIAS.md:847`) |

### Locked formula — "pick N of M"

`.git/sequencer/todo` lists picks NOT yet applied (the current conflicting commit is held by `CHERRY_PICK_HEAD`, not the todo); a single-commit pick creates no sequencer (`SequencerRemaining==0`). The 1-based ordinal of the pick currently applying:

```go
// PickTotal = len(plan.SelectedCommits), stored at run creation.
// EMPIRICALLY VERIFIED (2026-07-27): `.git/sequencer/todo` INCLUDES the
// currently-conflicting commit. A cherry-pick of 3 commits conflicting on
// pick 2 leaves CHERRY_PICK_HEAD=pick2 AND a 2-line todo (pick2 + pick3), so
// SequencerRemaining counts the current pick. Therefore completed = PickTotal
// - SequencerRemaining, and the current (conflicting) pick index is that + 1.
func derivePickIndex(pickTotal int, st git.RepoState) int {
    if !st.InProgress { return pickTotal }            // sequence complete
    idx := pickTotal - st.SequencerRemaining + 1      // sequencer INCLUDES current → +1
    if idx < 1 { idx = 1 }                            // clamp: single pick / no sequencer file
    if idx > pickTotal { idx = pickTotal }            // clamp: conflict-on-last edge
    return idx
}
```

Verified against the clamp for every case: 2-commit conflict-on-first → SeqRem=2 → `2-2+1=1`; 3-commit conflict-on-second → SeqRem=2 → `3-2+1=2`; conflict-on-last → SeqRem=1 → `PickTotal` (or SeqRem=0 on a git that drops the file → clamped to PickTotal); single pick (no sequencer, SeqRem=0) → `2` → clamped to `1`. A RED integration test PINS `idx==1` (2-commit, conflict-on-first) AND `idx==2` (3-commit, conflict-on-second) to lock the `+1`; empty-pick/`--skip` recomputes after the skip lands.

## Data Flow

```
Init → prereq → onPrereqDone → resumeDetectCmd (Git.RepoState + Runs.List)
                                   │
      ┌────────────────────────────┼──────────────────────────┐
 in-progress pick            non-terminal jobId          nothing resumable
 (+ matching run:            (JobID set, Status                  │
  CurrentSHA ∈ Commits)       not terminal)               StateTicketInput
      └───────────────► StateRunHistory (offer) ◄──────────────┘
                          │ Enter=resume            q=decline→TicketInput
             rehydrate ticket / pickTotal / pickIndex / branch / runID
                ├ CHERRY_PICK_HEAD present → StateCherryPickConflict (repoStateCmd+tick)
                ├ record=conflict, no HEAD → RESYNC record (Save), stay in history
                └ jobId → StateValidationPolling (re-arm jobID/runID/pollCtx → reportCmd)
```

## File Changes

| File | Action | Description |
|---|---|---|
| `internal/runs/writer.go` | Modify | `Record` += `Commits`/`PickIndex`/`PickTotal`/`CurrentCommit`/`Phase`; `List`/`Load`/`Save`/`Prune` + pure `selectPruneCandidates` |
| `internal/app/update.go` | Modify | `resumeDetectCmd` at `onPrereqDone`; `onResumeDetect` (offer + resync); progress `Save` in `onBranchCreated`/`onPickDone`/`onRepoState`/`onVerifyDone`/`onAborted` |
| `internal/app/app.go` | Modify | `StateRunHistory` const; `Model` += `runs []runs.Record`, `runsCursor`, `pickIndex`, `pickTotal` |
| `internal/app/commands.go` | Modify | `resumeDetectMsg`/`resumeDetectCmd`; generate `runID` at branch creation; `validateCmd` reuses `m.runID` + merges git-phase fields into `Create` |
| `internal/app/keys.go` | Modify | `keyRunHistory` (`Enter` resume, `d` detalle, `↑/↓`, `q`) |
| `internal/app/view.go` | Modify | `viewRunHistory` (list + selected-run detail: branch/commits/package) |
| `internal/config/validate.go` | Modify | `Runs.KeepLast` / `Runs.KeepDays` must be ≥ 0 |
| `cmd/deploydeck/main.go` | New | `newRunsCmd` → `deploydeck runs prune` (composes `config.Load` + `runs.NewWriter` → `Prune`) |

## Interfaces / Contracts

```go
type Record struct {           // + additive fields (all omitempty, zero-default safe)
    // ... existing SchemaVersion,RunID,Ticket,Target,Alias,JobID,Status,CreatedAt,UpdatedAt
    Commits       []string `json:"commits,omitempty"`
    PickIndex     int      `json:"pickIndex,omitempty"`
    PickTotal     int      `json:"pickTotal,omitempty"`
    CurrentCommit string   `json:"currentCommit,omitempty"`
    Phase         string   `json:"phase,omitempty"` // cherry-pick|git-conflict|validating|done|aborted
}

func (w *Writer) List() ([]Record, error)                                  // scan *, newest-first by CreatedAt
func (w *Writer) Load(runID string) (Record, error)                        // export readRecord
func (w *Writer) Save(rec Record) (string, error)                          // upsert run.json (force SchemaVersion1)
func (w *Writer) Prune(keepLast, keepDays int, now time.Time) ([]string, error)
func selectPruneCandidates(records []Record, keepLast, keepDays int, now time.Time) []string
```

`resumeDetectMsg{ state git.RepoState; records []runs.Record; err error }`; matching run = newest non-terminal whose `Commits` contains `state.CurrentSHA`. Progress `Save` is best-effort inline (mirrors `onReportDone`'s `AppendReport`).

## Testing Strategy

| Layer | What | Approach |
|---|---|---|
| Unit | `selectPruneCandidates` (within keepLast / within keepDays / outside both / boundary age / keepLast=0 / empty); `List` sort + skips bad dirs; `Load` round-trip; backward-compat (old-shape `run.json` → zero-valued new fields); `derivePickIndex` edges | table-driven, `t.TempDir` fixtures |
| Integration | real in-progress cherry-pick (`seedConflictingFeature`) → offer → `Enter` → `StateCherryPickConflict` + `pickIndex`/`pickTotal`; resync (HEAD removed); `List`/`Prune` dir fixtures; `FakeRunner` jobId re-attach → `StateValidationPolling` re-armed | temp git repo + temp runs dir, `-short` skippable |
| E2E | `fs+temp+sf-fake`, no org (`HISTORIAS.md:891` seeds) | CI-safe; real-org `ReportDeploy` re-attach opt-in/best-effort (timing-hard) |
| Boundary | `internal/app` never execs on resume path | existing `boundary_test.go` |

## Threat Matrix

| Boundary | Applicability | Design response | Planned RED test |
|---|---|---|---|
| Git repository selection | Applicable | Reuse `RepoState`'s `RepoRoot`/`gitDir` authority (no new selector); `runs prune` resolves dir via `os.Getwd` like `doctor` | resume integration uses a real temp `.git` |
| Commit state | Applicable | `CHERRY_PICK_HEAD` present → resume; absent/clean → resync record | in-progress vs externally-resolved/aborted seeds |
| Documentation-like paths | N/A | no file classification/execution introduced | — |
| Push state / PR commands | N/A | HU-014 push/PR deferred, out of scope | — |

`Prune` removes ONLY `<baseDir>/.deploydeck/runs/<runID>` directories enumerated by `List` — never arbitrary or user-supplied paths.

## Migration / Rollout

No migration. Additive-only; backward-compat guaranteed by json zero-defaults. Rollback = revert the branch; existing `run.json` files stay valid (nothing to undo).

## Open Questions

- [ ] Does `.git/sequencer/todo` include the currently-conflicting commit? Pinned by the disambiguating RED integration test — the locked formula holds under "excludes"; clamp + test guard the "includes" case.
