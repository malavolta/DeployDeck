# Delta for Push And PR Preparation

## ADDED Requirements

### Requirement: On-Demand AI Suggestion Affordance At pushReady

The system SHALL offer an on-demand request action at `pushReady`,
available ONLY when AI is configured, that asks the `ai-pr-summary`
capability for a title/description suggestion, and SHALL offer a distinct
explicit accept action; the system SHALL NOT show this affordance when no
`ai` configuration is present, and the affordance MUST NOT trigger AI
generation automatically after push.

#### Scenario: No ai config leaves pushReady unchanged
- GIVEN no `ai` configuration block is present
- WHEN the user reaches `pushReady`
- THEN no AI-suggestion affordance is shown and the screen matches the
  pre-existing `pushReady` behavior

#### Scenario: Explicit request at pushReady produces a suggestion
- GIVEN AI is configured and the endpoint is reachable
- WHEN the user presses the AI-suggestion request key at `pushReady`
- THEN a title/description suggestion is generated and shown for review

#### Scenario: Ignoring the suggestion keeps the formula title
- GIVEN a suggestion has been generated but not accepted
- WHEN the user proceeds to PR creation without accepting it
- THEN `github.SuggestedTitle` remains the title source

#### Scenario: Unreachable endpoint at pushReady degrades silently
- GIVEN AI is configured but the endpoint is unreachable
- WHEN the user presses the AI-suggestion request key at `pushReady`
- THEN no suggestion is shown, no error is surfaced, and the screen
  continues to offer PR creation via `github.SuggestedTitle`

## MODIFIED Requirements

### Requirement: PR Creation Requires Explicit Confirmation And Records The URL

When `gh` is present-authenticated, the system SHALL offer
`gh pr create --base <target> --head <deploy> --title "..."`, using
`github.SuggestedTitle` as the title source by default, or the
AI-suggested title ONLY when the user has explicitly accepted it
(`ai-pr-summary` capability); the system SHALL show the command before
running it, run it only after explicit confirmation, and on success show
and persist the resulting PR URL.
(Previously: the title source was always `github.SuggestedTitle`, with no
AI-suggested alternative.)

#### Scenario: Authenticated gh with confirmation creates the PR
- GIVEN `gh` is present-authenticated
- WHEN the user explicitly confirms PR creation (HU-014 AC5)
- THEN `gh pr create` runs and the resulting URL is shown and recorded on
  the run

#### Scenario: No PR is created without explicit confirmation
- GIVEN `gh` is present-authenticated and PR data is shown
- WHEN the user does not explicitly confirm
- THEN no PR is created

#### Scenario: Explicitly accepted AI title is used as the PR-creation title source
- GIVEN `gh` is present-authenticated and the user has explicitly accepted
  an AI-suggested title at `pushReady`
- WHEN the user explicitly confirms PR creation
- THEN `gh pr create --title` uses the accepted AI title instead of
  `github.SuggestedTitle`
