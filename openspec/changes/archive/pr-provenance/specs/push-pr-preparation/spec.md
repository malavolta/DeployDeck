# Delta for Push And PR Preparation

## MODIFIED Requirements

### Requirement: PR Creation Requires Explicit Confirmation And Records The URL

When `gh` is present-authenticated, the system SHALL first check whether an open PR already exists for the deploy branch; if one is open, the system SHALL skip `gh pr create` and instead show/report the existing PR URL (the confirmed push still updates that PR). Otherwise the system SHALL offer
`gh pr create --base <target> --head <deploy> --title "..."`, using
`github.SuggestedTitle` as the title source by default, or the
AI-suggested title ONLY when the user has explicitly accepted it
(`ai-pr-summary` capability); the system SHALL show the command before
running it, run it only after explicit confirmation, and on success show
and persist the resulting PR URL. The submitted body SHALL always end with
the visible `Created with DeployDeck v<version>` footer and the invisible
signed marker (`pr-provenance` capability), on BOTH the AI-accepted and
non-AI paths; on the non-AI path, a body consisting of the footer and
marker only is the accepted default. If the body already contains a
deploydeck marker (e.g. an AI-generated description echoed a prior marker
verbatim), that pre-existing marker is stripped before the genuine
footer+marker is appended, so exactly one — genuine — marker ever ships.
EXCEPTION: when `owner/repo` cannot be
derived from the origin remote URL (a signature payload cannot be built),
the body SHALL end with the visible footer only and NO marker — the system
SHALL never write a marker whose signature could not be bound to the
repository. The marker is written once, at the PR's original creation, and
is NOT refreshed or reappended by later pushes to an already-open PR.
(Previously: the submitted body carried only `github.SuggestedTitle`/AI
description content with no footer or marker, and the non-AI path could
submit an entirely empty body.)

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

#### Scenario: AI-accepted body gains footer and marker
- GIVEN the user has explicitly accepted an AI-suggested description at `pushReady`
- WHEN the user explicitly confirms PR creation
- THEN the submitted body is the accepted AI description followed by the
  visible footer and the invisible signed marker

#### Scenario: Non-AI path submits a footer-and-marker-only body
- GIVEN the user has not accepted an AI-suggested description
- WHEN the user explicitly confirms PR creation
- THEN the submitted body consists only of the visible footer and the
  invisible signed marker

#### Scenario: Unparseable origin degrades to footer-only, never an unbindable marker
- GIVEN the origin remote URL cannot be parsed into `owner/repo`
- WHEN the user explicitly confirms PR creation
- THEN the submitted body ends with the visible footer only, with no
  marker, and PR creation itself proceeds normally

#### Scenario: Reuse-and-append leaves the original marker intact
- GIVEN an open PR already exists for the deploy branch and was created with a valid marker
- WHEN the user pushes additional commits and the confirmed push updates that PR
- THEN `gh pr create` is skipped, the PR body is not rewritten, and the
  original marker remains present and still verifies successfully

#### Scenario: An AI-echoed marker in the accepted description is sanitized away
- GIVEN the user has explicitly accepted an AI-suggested description that
  already contains a deploydeck marker (the model echoed a prior marker
  verbatim)
- WHEN the user explicitly confirms PR creation
- THEN the pre-existing marker is stripped from the submitted body, and the
  body ends with exactly one — genuine — marker, never two
