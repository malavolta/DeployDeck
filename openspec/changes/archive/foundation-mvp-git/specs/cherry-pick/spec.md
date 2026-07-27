# Delta for Cherry Pick

## ADDED Requirements

### Requirement: Sequential Ordered Cherry-Pick Execution
The system SHALL apply selected commits sequentially, one at a time, in topological order. (HU-006)

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

**Note:** HU-006's reopen-and-resume acceptance criterion (`docs/HISTORIAS.md:395` — resuming a `CherryPickConflict` run after closing and reopening DeployDeck) is intentionally owned by HU-013 (run persistence/resume, out of scope for this change). Its absence here is deliberate, not a gap.

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
The system SHALL detect an empty cherry-pick (content already applied to the target under a different SHA) and SHALL offer `git cherry-pick --skip` with a clear explanatory message. This safety-net SHALL be implemented regardless of the repository's merge/squash policy (DEC-001 is process context, not a precondition).

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
