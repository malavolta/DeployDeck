# Delta for Push And PR Preparation

## MODIFIED Requirements

### Requirement: PR Creation Requires Explicit Confirmation And Records The URL

When `gh` is present-authenticated, the system SHALL first check whether an open PR already exists for the deploy branch; if one is open, the system SHALL skip `gh pr create` and instead show/report the existing PR URL (the confirmed push still updates that PR). Otherwise the system SHALL offer
`gh pr create --base <target> --head <deploy> --title "..."`, using
`github.SuggestedTitle` as the title source by default, or the
AI-suggested title ONLY when the user has explicitly accepted it
(`ai-pr-summary` capability); the system SHALL show the command before
running it, run it only after explicit confirmation, and on success show
and persist the resulting PR URL.
(Previously: the title source was always `github.SuggestedTitle`, with no
AI-suggested alternative, and PR creation was always offered when `gh` was
present-authenticated with no check for an already-open PR on the branch.)

#### Scenario: Authenticated gh with confirmation creates the PR
- GIVEN `gh` is present-authenticated and no PR is already open for the deploy branch
- WHEN the user explicitly confirms PR creation (HU-014 AC5)
- THEN `gh pr create` runs and the resulting URL is shown and recorded on
  the run

#### Scenario: No PR is created without explicit confirmation
- GIVEN `gh` is present-authenticated, no PR is already open, and PR data is shown
- WHEN the user does not explicitly confirm
- THEN no PR is created

#### Scenario: Explicitly accepted AI title is used as the PR-creation title source
- GIVEN `gh` is present-authenticated, no PR is already open, and the user has explicitly accepted
  an AI-suggested title at `pushReady`
- WHEN the user explicitly confirms PR creation
- THEN `gh pr create --title` uses the accepted AI title instead of
  `github.SuggestedTitle`

#### Scenario: Open PR already exists for the branch
- GIVEN `gh` is present-authenticated and an open PR is already reported for the deploy branch
- WHEN push completes
- THEN PR creation is skipped, the existing PR URL is shown, and the confirmed push has updated that PR

#### Scenario: PR lookup itself unavailable degrades to manual flow
- GIVEN `gh` is absent or present-unauthenticated
- WHEN the system would otherwise check for an existing open PR
- THEN the check is skipped and the flow degrades to the existing manual push+confirm compare-URL behavior
