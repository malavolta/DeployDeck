# Commit Selection Delta (tui-ux-polish-b2)

Tag: `[U2]` windowing and truncation. Scenarios run through `Model.Update`/`View` under the Ascii profile.

## ADDED Requirements

### Requirement: [U2] Commit List Is Windowed And Rows Are Truncated

When the model has a height greater than 0, the commit-selection list MUST fit the window: the view MUST have at most `height` lines, the cursor row MUST be visible, and hidden commits MUST be indicated by `↑ N más` and `↓ N más` (bottom-anchored window, minimal scroll down, per `tui-presentation`). The visible-row budget is measured from the actual chrome (banner, warnings, notice, footer), with a minimum of 3 rows. When width is greater than 0, each commit row MUST be truncated to the width, including rows containing status marks. Selection state (marked rows, blocked rows, already-applied and merge blocks, warnings) is unaffected by windowing: marking, blocking and confirming act on the full list, not only the visible rows. With height 0 and width 0 the output is unchanged.

#### Scenario: Long list fits the window
- GIVEN 60 discovered commits, a `WindowSizeMsg` of 100x24 and the cursor at the last commit
- WHEN the view renders
- THEN the view has at most 24 lines, the cursor row is visible and `↑ N más` reports the hidden count

#### Scenario: Cursor moving up follows
- GIVEN the cursor at the top of the visible window of a scrolled list
- WHEN the user moves up
- THEN the cursor row stays visible and the `↑`/`↓` counts update

#### Scenario: Marks persist outside the window
- GIVEN a commit marked while visible that then scrolls out of the window
- WHEN the user confirms the selection
- THEN the marked commit is included in the plan

#### Scenario: Unsized list is unchanged
- GIVEN 60 commits and width and height 0
- WHEN the view renders
- THEN every row renders and the output equals the pre-change output with no `más` indicator

#### Scenario: Rows are truncated to the width
- GIVEN a commit with a long message and width 60
- WHEN the view renders
- THEN its line is at most 60 cells wide and ends with `…`

## MODIFIED Requirements

None.

## REMOVED Requirements

None.
