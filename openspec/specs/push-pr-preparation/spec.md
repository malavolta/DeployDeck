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

When `gh` is present-authenticated, the system SHALL offer
`gh pr create --base <target> --head <deploy> --title "..."`, show it before
running, run it only after explicit confirmation, and on success show and
persist the resulting PR URL.

#### Scenario: Authenticated gh with confirmation creates the PR
- GIVEN `gh` is present-authenticated
- WHEN the user explicitly confirms PR creation (HU-014 AC5)
- THEN `gh pr create` runs and the resulting URL is shown and recorded on the
  run

#### Scenario: No PR is created without explicit confirmation
- GIVEN `gh` is present-authenticated and PR data is shown
- WHEN the user does not explicitly confirm
- THEN no PR is created

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
