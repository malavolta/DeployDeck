# Capability: Cherry-Pick

## Overview

The cherry-pick capability is the heart of the promotion engine. It applies selected commits sequentially to the promotion branch, detects and classifies conflicts, resolves them interactively, and verifies the final content against the source branch. It handles empty picks, rerere suggestions, and graceful abort with cleanup.

## Requirements

### Requirement: Sequential Ordered Cherry-Pick Execution

The system SHALL apply selected commits sequentially, one at a time, in topological order.

#### Scenario: Selected commits apply in order with matching content
- GIVEN a valid set of selected commits
- WHEN the cherry-pick runs
- THEN commits are applied in order and the resulting branch content matches the source branch

### Requirement: Conflict Detection And Classification

The system SHALL stop the flow when a cherry-pick fails and SHALL classify conflicting files by type (text, binary, modify/delete).

#### Scenario: Conflict stops the flow with classified files
- GIVEN a commit that conflicts on apply
- WHEN the cherry-pick command fails
- THEN the flow stops and conflicting files are shown classified by type

### Requirement: Modify/Delete Conflict Resolution Choice

For conflicting files classified as modify/delete, the system SHALL offer an explicit choice to keep the file (`git add`) or delete it (`git rm`).

#### Scenario: Modify/delete conflict offers keep or delete
- GIVEN a conflicting file classified as modify/delete
- WHEN the user resolves it in the TUI
- THEN the system offers an explicit choice to keep the file (`git add`) or delete it (`git rm`)

### Requirement: Binary Conflict Resolution Via Ours/Theirs

For conflicting files classified as binary (e.g. static resources), the system SHALL offer resolution by selecting `--theirs` or `--ours` via `git checkout`.

#### Scenario: Binary conflict offers theirs/ours selection
- GIVEN a conflicting file classified as binary
- WHEN the user resolves it in the TUI
- THEN the system offers `git checkout --theirs` or `git checkout --ours` as resolution options

### Requirement: Live Repo-State Re-Polling

The system SHALL periodically reread repository state and update the conflict list to reflect resolutions made outside DeployDeck, without requiring user action in the TUI.

#### Scenario: External resolution is reflected automatically
- GIVEN the user resolves conflicts through an external tool
- WHEN DeployDeck rereads the repository state
- THEN the conflict list updates automatically without TUI interaction

### Requirement: Continue Gating On Unresolved State

The system SHALL keep "continue" disabled while any path is unmerged or unstaged, or while any staged file still contains conflict markers (`<<<<<<<`), and SHALL show what remains pending.

#### Scenario: Unresolved or unstaged paths disable continue
- GIVEN unmerged or unstaged conflict paths remain
- WHEN the user attempts to continue
- THEN continue is disabled with the pending detail shown

#### Scenario: Conflict markers in staged file block continue
- GIVEN a staged file still contains conflict markers
- WHEN the user attempts to continue
- THEN continue is blocked and the offending file is identified

### Requirement: Non-Interactive Continue Execution

The system SHALL execute `git cherry-pick --continue` non-interactively (`GIT_EDITOR=true`) once all conflicts are resolved and staged.

#### Scenario: Fully resolved conflict continues non-interactively
- GIVEN all conflicts are resolved and staged with no markers
- WHEN the user selects continue
- THEN `git cherry-pick --continue` runs non-interactively

### Requirement: External Action Reconciliation

The system SHALL detect `--continue`/`--abort` executed outside DeployDeck by rereading `CHERRY_PICK_HEAD` and `.git/sequencer`, and SHALL resynchronize state accordingly.

#### Scenario: External continue or abort is detected and resynced
- GIVEN the user ran `--continue` or `--abort` outside DeployDeck
- WHEN the TUI refreshes
- THEN it detects the real repository state and resynchronizes

### Requirement: Abort Handling With Partial-Sequence Cleanup Offer

The system SHALL execute `git cherry-pick --abort` on user confirmation, mark the run as aborted, and SHALL offer to clean up the temp branch when the abort occurs mid-sequence with partial picks applied.

#### Scenario: Confirmed abort marks the run aborted
- GIVEN the user confirms abort
- WHEN DeployDeck executes it
- THEN `git cherry-pick --abort` runs and the run is marked aborted

#### Scenario: Mid-sequence abort offers to clean up the partial branch
- GIVEN the abort happens after some commits were already picked
- WHEN the user confirms
- THEN the user is offered to clean up the temp branch with the partial picks

### Requirement: Empty-Pick Squash Safety-Net

The system SHALL detect an empty cherry-pick (content already applied to the target under a different SHA) and SHALL offer `git cherry-pick --skip` with a clear explanatory message. This safety-net SHALL be implemented regardless of the repository's merge/squash policy.

#### Scenario: Empty pick offers skip
- GIVEN a cherry-pick becomes empty because its content is already in the target
- WHEN it is detected
- THEN DeployDeck informs the user and offers `--skip`

### Requirement: Rerere Suggestion And Auto-Resolution Confirmation

The system SHALL suggest enabling `git rerere`; when rerere auto-resolves a conflict, it SHALL be presented as auto-resolved from a prior resolution and SHALL require explicit user confirmation before continuing.

#### Scenario: Rerere auto-resolution requires confirmation
- GIVEN `git rerere` auto-resolves a conflict during a pick
- WHEN the result is shown
- THEN it is labeled auto-resolved from a prior resolution and requires user confirmation

### Requirement: Post-Pick Verification Against Source Branch

The system SHALL verify, once all picks complete, the final state of touched files against the source branch (`git diff HEAD <source> -- <files>`) and SHALL show a per-file partial-promotion warning for any difference before any delta-generation step.

#### Scenario: Differing file after all picks warns of partial promotion
- GIVEN a touched file differs from the source branch after all picks complete
- WHEN post-pick verification runs
- THEN a per-file partial-promotion warning is shown before any delta-generation step

### Requirement: Failed Cherry-Pick Blocks Downstream Steps

The system SHALL NOT generate a delta package or run Salesforce validation when a cherry-pick fails.

#### Scenario: Failed cherry-pick blocks delta and validation
- GIVEN a cherry-pick fails
- WHEN the flow evaluates next steps
- THEN no delta is generated and no Salesforce validation runs

### Requirement: Resumed Entry Accepts Rehydrated Conflict Context

The cherry-pick conflict screen SHALL accept entry either from a freshly started sequence or from a resumed run, and SHALL display the resumed run's ticket, pick index/total, and current commit identically to a fresh sequence. Once entered, the existing live re-polling and continue-gating requirements apply unchanged regardless of entry path.

#### Scenario: Resumed run shows ticket and pick N of M
- GIVEN `run-resume` routes into `StateCherryPickConflict` for a run with a persisted ticket, `PickIndex`, and `PickTotal`
- WHEN the conflict screen renders
- THEN it shows the ticket and "pick N of M" matching the resumed record (HU-013 AC `docs/HISTORIAS.md:846`)

#### Scenario: Resumed conflict screen behaves like a fresh one
- GIVEN a resumed conflict screen with unresolved paths
- WHEN the user attempts to continue
- THEN continue is disabled per the existing unresolved-state gating, same as a freshly started sequence

### Requirement: Cherry-Pick Source Provenance Trailer

The system SHALL invoke `git cherry-pick` with the `-x` flag for every applied commit, so each promoted commit's message carries a `(cherry picked from commit <sha>)` trailer identifying its source SHA.

#### Scenario: Promoted commit message carries the source-SHA trailer
- GIVEN a commit is selected and cherry-picked onto the promotion branch
- WHEN the pick completes
- THEN the resulting commit's message contains `(cherry picked from commit <sha>)`, where `<sha>` is the source commit's SHA

#### Scenario: Trailer is present regardless of conflict resolution path
- GIVEN a commit cherry-picks cleanly, or is applied via conflict resolution followed by `--continue`
- WHEN the resulting commit is inspected
- THEN it carries the `-x` provenance trailer in both cases

### Requirement: Conflict Screen Has Its Own Selection Cursor

The cherry-pick conflict screen MUST keep its own cursor over the list of unmerged files, independent of the commit-selection cursor. The cursor MUST be reset to the first file every time the conflict screen is entered (fresh sequence, resumed run, or return after continue produced a new conflict). `up`/`k` and `down`/`j` MUST move the cursor, clamped to the list bounds. The view MUST draw a `>` marker on the selected row and a blank marker column on the others. The cursor MUST be clamped when the list shrinks (for example after live re-polling or an external resolution).

#### Scenario: Cursor starts on the first file regardless of commit selection
- GIVEN the commit-selection cursor sits at index 3 and the user starts a sequence that conflicts on three files
- WHEN the conflict screen is entered
- THEN the `>` marker is on the first file

#### Scenario: Cursor resets on every entry
- GIVEN the conflict cursor was moved to the second file and the sequence continued into a second conflict
- WHEN the conflict screen is entered again
- THEN the `>` marker is on the first file

#### Scenario: Navigation keys move the marker
- GIVEN the conflict screen with three files and the marker on the first
- WHEN the user presses `down`, then `j`
- THEN the marker is on the third file
- AND WHEN the user then presses `down` the marker stays on the third file
- AND WHEN the user presses `up`, then `k` the marker is on the first file, and a further `up` leaves it there

#### Scenario: Marker rendering
- GIVEN the conflict screen with two files and the cursor on the second
- WHEN it renders
- THEN exactly one row carries the `>` marker and it is the second file's row

#### Scenario: Cursor is clamped when the list shrinks
- GIVEN the cursor is on the third of three files
- WHEN re-polling reports only one unmerged file
- THEN the marker is on that file and no index error occurs

#### Scenario: Resumed entry resets the cursor
- GIVEN a resumed run routes into `StateCherryPickConflict`
- WHEN the screen renders
- THEN the marker is on the first file

### Requirement: Edit Key Opens The Selected Conflict File

Pressing `e` on the conflict screen MUST open the file at the conflict cursor, not a file chosen by any other screen's cursor. With a single conflicted file `e` opens that file.

#### Scenario: e opens the highlighted file with several conflicts
- GIVEN the conflict screen with files `a`, `b` and `c` and the marker moved to `b`
- WHEN the user presses `e`
- THEN the editor command targets `b`

#### Scenario: e does not follow a stale commit-selection cursor
- GIVEN the commit-selection cursor was left at index 2 and the conflict screen has 3 files with the marker on the first
- WHEN the user presses `e`
- THEN the editor command targets the first file

### Requirement: Conflict Footer Lists Navigation

The conflict screen footer MUST list the navigation keys (`↑/↓`) and the other keys that act in that state (including `e`), per the footer parity requirement in `tui-presentation`.

#### Scenario: Footer names navigation and edit
- GIVEN the conflict screen with at least two files
- WHEN it renders
- THEN the footer contains `↑/↓` and an `e` entry

## Design Notes

- Conflict file classification uses `git status --porcelain -z` (NUL-delimited) to reliably parse paths containing spaces or special characters.
- The `DU`/`UD` codes from porcelain output map to modify/delete conflicts; binary conflicts are detected via the `-` marker in numstat output.
- All cherry-pick invocations carry `-c commit.gpgsign=false` to prevent GPG signing failures due to missing TTY during automation.
- The promotion branch state is never cached; it is reread from `.git/CHERRY_PICK_HEAD`, `.git/sequencer/todo`, and `git status` on every update.
- Resume entry and routing (reopen/resume) is now provided by the `run-resume` capability (HU-013).
