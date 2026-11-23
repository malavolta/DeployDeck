# Cherry-Pick Delta (tui-ux-consistency)

## ADDED Requirements

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

## MODIFIED Requirements

None. (Spanish conflict-kind labels, ADR-9, were cut for the 800-line budget.)

## REMOVED Requirements

None.
