# Push And PR Preparation Specification

## Purpose

After a successful Salesforce deploy validation, DeployDeck offers to push
the validated deploy branch and prepares base/compare/title PR information,
without replacing the formal review process. Push and PR creation always
require explicit user confirmation. (HU-014)

## Requirements

### Requirement: Push Offered Only After A Successful Validation

The system SHALL offer push only when validation is `Succeeded` or
`SucceededPartial`, and SHALL NOT offer push as a primary action for
`Failed`, `Canceled`, `Aborted`, or `Error`.

#### Scenario: Successful validation offers push
- GIVEN validation finished `Succeeded` or `SucceededPartial`
- WHEN the terminal screen is reached (HU-014 AC1)
- THEN push of the deploy branch is offered

#### Scenario: Failed validation does not offer push
- GIVEN validation finished `Failed` or `Canceled`
- WHEN the terminal screen is reached (HU-014 AC2)
- THEN push is not offered as a primary action

### Requirement: Explicit-Confirm Push Execution

The system SHALL show the push command before running it and SHALL execute
`git push -u origin <deploy-branch>` only after explicit user confirmation.

#### Scenario: Confirmed push runs with upstream tracking
- GIVEN the user is on the push-preparation screen
- WHEN the user explicitly confirms push (HU-014 AC3)
- THEN `git push -u origin <deploy-branch>` runs against `origin`

### Requirement: Base, Compare, And Suggested Title Shown After Push Success

The system SHALL, after a successful push, show the base branch
(`TargetBranch`), the compare branch (`PromotionBranch`), and a suggested
title `<ticket> - Promote changes to <target>`.

#### Scenario: Successful push reveals PR data
- GIVEN the push completed successfully
- WHEN the push result is processed (HU-014 AC4)
- THEN base, compare, and the title `<ticket> - Promote changes to <target>`
  are shown

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

### Requirement: `gh` Detection Via A Single `gh auth status` Call

The system SHALL detect `gh` availability/auth with one `gh auth status`
call, classifying it as absent, present-unauthenticated, or
present-authenticated.

#### Scenario: gh absent
- GIVEN `gh` is not installed
- WHEN `gh auth status` is attempted
- THEN the system classifies `gh` as absent

#### Scenario: gh present but unauthenticated
- GIVEN `gh` is installed but not logged in
- WHEN `gh auth status` runs
- THEN the system classifies `gh` as present-unauthenticated

#### Scenario: gh present and authenticated
- GIVEN `gh` is installed and logged in
- WHEN `gh auth status` runs
- THEN the system classifies `gh` as present-authenticated

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

### Requirement: Compare URL Fallback Derived From Origin

When `gh` is absent or present-unauthenticated, the system SHALL derive and
show a compare URL from `origin` as
`https://<host>/<org>/<repo>/compare/<target>...<deploy>`, supporting SSH
(`git@<host>:org/repo(.git)`), HTTPS, and `ssh://git@<host>/...` origin forms
— including non-github.com Enterprise hosts — and the flow SHALL continue
without error. On an unrecognized origin form, the system SHALL fail
gracefully: raw origin URL plus manual base/compare/title, never a malformed
link.

#### Scenario: Compare URL derives from SSH and Enterprise origins
- GIVEN `origin` is `git@github.com:org/repo.git` or an Enterprise SSH/HTTPS
  URL such as `github.ibm.com`
- WHEN `gh` is absent/unauthenticated and the compare URL is derived
  (HU-014 AC6)
- THEN the compare URL uses the matching host (e.g. `github.com` or
  `github.ibm.com`) and the flow continues without error

#### Scenario: Unrecognized origin form fails gracefully
- GIVEN `origin` matches no recognized SSH/HTTPS/`ssh://` form
- WHEN the compare URL is derived
- THEN the raw origin URL and manual base/compare/title are shown instead of
  a malformed link

### Requirement: PR Creation Failure Does Not Abort The Flow

The system SHALL show the error and manual base/compare/title data when PR
creation fails, and the flow SHALL continue.

#### Scenario: PR creation fails
- GIVEN the user confirmed PR creation with `gh` present-authenticated
- WHEN `gh pr create` fails (HU-014 AC7)
- THEN the error and manual base/compare/title data are shown and the flow
  continues
