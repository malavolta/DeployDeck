# Run History Delta (tui-ux-polish-b2)

Tags: `[U1]` parity hardening, `[U2]` windowing. Scenarios run through `Model.Update`/`View` under the Ascii profile.

## ADDED Requirements

### Requirement: [U2] History List Is Windowed To The Terminal

When the model has a height greater than 0, the run-history list MUST fit the window: the rendered view MUST have at most `height` lines, the selected row MUST be visible, and hidden rows MUST be indicated by `↑ N más` and `↓ N más` (see `tui-presentation`, window rules: bottom-anchored, minimal scroll down). The chrome (banner, header row, detail panel, notice, footer) is measured per render, with a minimum of 3 visible rows. Rows MUST be truncated to the width when width is greater than 0. With height 0 and width 0 the output is unchanged.

#### Scenario: Long history fits the window
- GIVEN 50 recorded runs, a `WindowSizeMsg` of 80x24 and the cursor on the last row
- WHEN the view renders
- THEN it has at most 24 lines, the selected row is visible, and `↑ N más` shows the number of hidden rows above

#### Scenario: Moving down scrolls minimally
- GIVEN the same history with the cursor on the last visible row
- WHEN the user moves down one row
- THEN the selected row is visible and the window moved by one row

#### Scenario: Unsized history is unchanged
- GIVEN 50 recorded runs and width and height 0
- WHEN the view renders
- THEN all rows render and the output equals the pre-change output

#### Scenario: Long rows are truncated
- GIVEN a run row wider than the width
- WHEN it renders at width 60
- THEN the line is at most 60 cells wide

## MODIFIED Requirements

### Requirement: [U1] Enter On A Resumable Run Initiates Resume

Pressing `Enter` on a resumable run (one with an in-progress cherry-pick or a non-terminal `jobId`) SHALL initiate the same resume routing as startup resume-detection. `Enter` is a binding of the history state ONLY while the selected row is resumable: on a non-resumable (terminal) row the footer MUST NOT advertise `Enter reanudar` and the handler MUST NOT act on `Enter`. The footer MUST reflect the selected row as the cursor moves.
(Previously: `Enter reanudar` was always advertised, and on terminal rows Enter was a silent no-op.)

#### Scenario: Enter on a conflicted run resumes to the conflict screen
- GIVEN the selected run is in `CherryPickConflict` with a matching in-progress cherry-pick
- WHEN the user presses `Enter`
- THEN the TUI routes to the conflict screen with the run's rehydrated context

#### Scenario: Enter on a run with a jobId resumes to polling
- GIVEN the selected run has a non-terminal `jobId`
- WHEN the user presses `Enter`
- THEN the TUI re-attaches `deploy report` polling for that `jobId`

#### Scenario: Terminal row neither advertises nor acts on Enter
- GIVEN the selected run reached a terminal status (e.g. `Succeeded`, `Canceled`)
- WHEN the view renders
- THEN the footer does not contain `Enter reanudar`
- AND WHEN the user presses `Enter`
- THEN the state is unchanged, no command is returned and the detail view remains shown

#### Scenario: Footer follows the cursor
- GIVEN history with a resumable row followed by a terminal row
- WHEN the cursor moves from the first to the second
- THEN `Enter reanudar` disappears from the footer, and returns when the cursor moves back

## REMOVED Requirements

None.
