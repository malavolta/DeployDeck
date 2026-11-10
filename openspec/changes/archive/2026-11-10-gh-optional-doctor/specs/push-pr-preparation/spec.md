# Delta for push-pr-preparation

## MODIFIED Requirements

### Requirement: `gh` Detection Via A Single `gh auth status` Call

The system SHALL detect `gh` availability/auth with one `gh auth status`
call scoped to the `origin` host (`--hostname <host>`) when the host is
known, bounded by a timeout of about 5 seconds, classifying it as absent,
present-unauthenticated, present-authenticated, or timed-out. WHEN the host
cannot be derived, the call SHALL run without `--hostname`, still bounded by
the timeout. Timed-out SHALL be a distinct state and SHALL NOT be classified
as unauthenticated.

(Previously: one unscoped, untimed call with three states.)

#### Scenario: gh absent
- GIVEN `gh` is not installed
- WHEN `gh auth status` is attempted
- THEN the system classifies `gh` as absent

#### Scenario: gh present but unauthenticated
- GIVEN `gh` is installed but not logged in to the origin host
- WHEN `gh auth status --hostname <host>` runs
- THEN the system classifies `gh` as present-unauthenticated

#### Scenario: gh present and authenticated
- GIVEN `gh` is installed and logged in to the origin host
- WHEN `gh auth status --hostname <host>` runs
- THEN the system classifies `gh` as present-authenticated

#### Scenario: gh auth status times out
- GIVEN the `gh auth status` call does not finish within about 5 seconds
- WHEN the deadline is exceeded
- THEN the system classifies `gh` as timed-out, not unauthenticated

### Requirement: Compare URL Fallback Derived From Origin

When `gh` is unavailable (absent, present-unauthenticated, or timed-out), the
system SHALL derive and show a compare URL from `origin` as
`https://<host>/<org>/<repo>/compare/<target>...<deploy>`, supporting SSH
(`git@<host>:org/repo(.git)`), HTTPS, and `ssh://git@<host>/...` origin forms
— including non-github.com Enterprise hosts — and the flow SHALL continue
without error. On the PR screen, the compare URL SHALL be offered directly
and the screen SHALL NOT offer a `gh pr create` option. WHEN the fallback is
caused by a timeout during PR preparation, the screen SHALL show a one-line
note that `gh` could not be verified. On an unrecognized origin form, the
system SHALL fail gracefully: raw origin URL plus manual base/compare/title,
never a malformed link.

(Previously: applied only to absent or present-unauthenticated `gh`, and did
not state that the gh-create option is withheld or cover timeouts.)

#### Scenario: Compare URL derives from SSH and Enterprise origins
- GIVEN `origin` is `git@github.com:org/repo.git` or an Enterprise SSH/HTTPS
  URL such as `github.ibm.com`
- WHEN `gh` is unavailable and the compare URL is derived (HU-014 AC6)
- THEN the compare URL uses the matching host (e.g. `github.com` or
  `github.ibm.com`) and the flow continues without error

#### Scenario: Unavailable gh offers the compare URL and no gh-create option
- GIVEN `gh` is absent, present-unauthenticated, or timed-out
- WHEN the PR screen is shown after a successful push
- THEN the compare URL is offered directly and no `gh pr create` option is
  shown

#### Scenario: PR preparation timeout falls back with a one-line note
- GIVEN the `gh` check times out during PR preparation
- WHEN the PR screen renders
- THEN the compare URL is offered and a single-line note says `gh` could not
  be verified, with no error screen and no "not authenticated" wording

#### Scenario: Unrecognized origin form fails gracefully
- GIVEN `origin` matches no recognized SSH/HTTPS/`ssh://` form
- WHEN the compare URL is derived
- THEN the raw origin URL and manual base/compare/title are shown instead of
  a malformed link

### Requirement: PR Creation Requires Explicit Confirmation And Records The URL

(Only the following scenario is changed; the requirement text and all other
scenarios are unchanged.)

#### Scenario: PR lookup itself unavailable degrades to manual flow
- GIVEN `gh` is absent, present-unauthenticated, or timed-out
- WHEN the system would otherwise check for an existing open PR
- THEN the check is skipped and the flow degrades to the existing manual
  push+confirm compare-URL behavior
