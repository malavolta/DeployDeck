# Exploration: HU-013 — Histórico local + reanudar runs (`run-history`)

Builds on archived `foundation-mvp-git`, `delta-validation`, `deploy-queue`. 13 living specs. This slice EXTENDS the minimal `internal/runs` writer and CLOSES two deferred items.

## Current state (verified in code)
- `internal/runs/writer.go` today = MINIMAL: `Writer.Create`/`AppendReport`/`MarkCanceled` + `Record{SchemaVersion,RunID,Ticket,Target,Alias,JobID,Status,CreatedAt,UpdatedAt}`. No `List`/`Load`/`Prune`. Package doc (writer.go:1-7,25-27): "HU-013 extends this writer by scanning the same run.json files ... grows this struct additively (new fields), never replaces it, keeping every run.json forward-readable." `Create` hardcodes `SchemaVersion1` (writer.go:69).
- **Deferred #1 — HU-006 reopen/resume**: `docs/HISTORIAS.md:395`; archived `foundation-mvp-git/proposal.md:37` ("intentionally owned by HU-013"); living `cherry-pick/spec.md:142`. State-diagram `Suspended` is a design placeholder — **no `StateSuspended` exists in `internal/app` code**.
- **Deferred #2 — HU-011 resume-by-jobId**: living `validation-progress/spec.md:75-87` ("exit leaves job active/resumable") + HU-013 AC `docs/HISTORIAS.md:844` ("run con jobId → reanudar → consulta deploy report"). Mechanism = existing `salesforce.Client.ReportDeploy` (client.go:31-35, report.go:108+), already wired in `commands.go:518-530`.
- **Reusable git primitive**: `git.Service.RepoState(ctx,dir)` (service_repostate.go:18-55) re-reads `CHERRY_PICK_HEAD`/`.git/sequencer/todo`/`status --porcelain -z` every call; `RepoState.InProgress/CurrentSHA/SequencerRemaining` (service_status.go:14-32) are exactly what resume-detection needs. No new git primitive.
- **Config**: `RunsConfig{KeepLast,KeepDays}` exists (config.go:51-53), defaults 30/90 (config.go:14-15, load.go:40-45). GAP: `validate.go` has NO bound on KeepLast/KeepDays (negatives pass silently) — add validation.
- **App hook point**: `Model.Init()` (app.go:253-255) → `onPrereqDone` (update.go:77-91) unconditionally goes to `StateTicketInput`. This is the natural resume-detection hook.

## Scope IN/OUT
IN: run history list/browse (`Historial De Runs` screen, mockup `MOCKUPS_TUI.md:369-388`); retention `keepLast`/`keepDays` + `deploydeck runs prune` CLI; resume-by-jobId (re-attach `ReportDeploy`); resume open cherry-pick conflict on startup; resync when repo state no longer matches the persisted run.
OUT: HU-014 push/PR, HU-016 re-promote (SourceRunID), HU-017 branch cleanup, HU-015/018 futuro.

## Resolved decisions (from this exploration's Open Decisions — to be stated by propose)
1. **Schema evolution = additive, NO SchemaVersion bump** (Go json defaults missing fields to zero; the "forward-readable" contract). Add a backward-compat test: old run.json (from prior slices) still `Load`s cleanly.
2. **Retention semantics** = a run is KEPT if within the most-recent `keepLast` (by CreatedAt) OR age ≤ `keepDays`; pruned only if outside BOTH (the safe reading of "últimos 30 runs o 90 días", `ARQUITECTURA.md:412`/`EPICA.md:412`). Extract a pure `selectPruneCandidates(records, keepLast, keepDays, now)` for unit tests.
3. **HU-013/HU-017 boundary**: HU-013 OWNS `Prune` + `runs prune` fully. HU-017 owns only `deploy/*` BRANCH cleanup and reuses the same config, never re-implementing retention. (Doc overlap: both HUs reference keepLast/keepDays — this call resolves it.)
4. **Startup resume UX**: after prereqs pass, resume-detection runs; if an in-progress cherry-pick (RepoState.InProgress) OR a run with a non-terminal jobId is found → OFFER resume (accept → route to the conflict/polling screen with rehydrated context; decline → normal flow). Plus a browsable history screen (list + detail, `Enter reanudar`). Not a blocking interstitial for the no-resumable case.
5. **No literal `StateSuspended`** — "suspended" = process not running (no in-memory rep). Resume routes DIRECTLY to `StateCherryPickConflict` / `StateValidationPolling` with rehydrated `Model` fields.

## `internal/runs` extension (additive)
- `Record` += `PickIndex int`, `PickTotal int`, `CurrentCommit string`, a `Phase` marker (git-conflict / validating / done), and `Commits []string` (selected SHAs, for the history detail). Canonical shape = the existing `Record` extended (reconcile the ARQUITECTURA vs HISTORIAS model disagreement toward the on-disk struct).
- `List() ([]Record, error)` — scan `.deploydeck/runs/*/run.json`, newest first.
- `Load(runID) (Record, error)` — export the existing private `readRecord`.
- `Prune(keepLast, keepDays int, now) ([]string, error)` — retention pass (via decision #2's pure helper).
- Progress updates during the flow write `PickIndex`/`PickTotal`/`CurrentCommit` (HU-013 AC `docs/HISTORIAS.md:836-837`). "pick N of M": `PickTotal` = count of selected commits stored at run creation; `PickIndex` derived from `PickTotal - RepoState.SequencerRemaining` — define the exact formula in design.

## `internal/app` wiring (never execs directly)
- Startup resume-detection `tea.Cmd` at `onPrereqDone`: call `Git.RepoState` + `Runs.List`; if `RepoState.InProgress` + matching run → offer resume to `StateCherryPickConflict` (populate Ticket/PickIndex/PickTotal); if `RepoState.InProgress` false but record says conflict → **resync** (correct/close the record, don't blindly trust, AC `docs/HISTORIAS.md:847`); if non-terminal jobId → offer resume to `StateValidationPolling` (re-arm jobID/runID/pollCtx + reportCmd).
- New `StateRunHistory` browse screen (list + detail, keys per `MOCKUPS_TUI.md:369-388`).
- `Model` += resumed-run context fields + `runs []runs.Record`/`runsCursor`.
- `cmd/deploydeck`: `deploydeck runs prune` Cobra subcommand alongside `newDoctorCmd`.
- Add `Runs.KeepLast`/`KeepDays` validation to `internal/config/validate.go`.

## Acceptance criteria + E2E
Per `docs/HISTORIAS.md:840-847` (AC) + `:888-894` (Test E2E). **Harness = `fs + temp + sf-fake`, NO org** (E2E index `docs/HISTORIAS.md:1285` — fully CI-safe, unlike HU-009/010/011/012 which are `parcial`). Seed: a run in CherryPickConflict WITH `CHERRY_PICK_HEAD`; another in CherryPickConflict but clean repo (externally resolved → resync); a run with jobId; a run without a job; >keepLast runs, some older than keepDays. A real-org resume-by-jobId re-attach test is an OPTIONAL extra (best-effort/timing-hard), not mandated by the docs.

## Testability (strict TDD)
- Pure-unit: `List` scan/sort; `selectPruneCandidates` (pure over `[]Record` + now); `Load` round-trip; **backward-compat** (old-shape run.json unmarshals with zero-valued new fields).
- Integration (temp git repo + temp `.deploydeck/runs/`, `-short`-skippable): seed a real in-progress cherry-pick (`CHERRY_PICK_HEAD`+sequencer via real `git cherry-pick`, per `service_cherrypick_test.go`) → assert startup routes to `StateCherryPickConflict` with correct pick N/M; resync case (CHERRY_PICK_HEAD removed); runs-dir fixtures for List/Prune; FakeRunner for resume-by-jobId.
- Opt-in real-org: only meaningful for `ReportDeploy` re-attach vs a genuinely non-terminal job (timing-hard) — best-effort, fake-primary. Git-side resume needs no org.

## Risks
- ARQUITECTURA vs HISTORIAS `RunRecord` shapes disagree → pick one canonical shape (extend on-disk `Record`), don't copy either doc literally.
- KeepLast/KeepDays unvalidated today → add bounds.
- HU-013/HU-017 retention overlap → resolved by decision #3 (HU-013 owns Prune).
- "pick N of M" has no direct field → derive via a formula defined in design.
- Startup resume-detection is on the cold path every launch → keep it cheap (RepoState + List are local I/O).

## Ready for Proposal: yes (resolve the 5 decisions above in the proposal).
