# Capability: PR Provenance

## Overview

Give anyone looking at a DeployDeck-created pull request a cheap, verifiable
way to confirm it originated from the tool. Every PR body DeployDeck submits
gains a visible footer and an invisible signed marker; a new
`deploydeck pr verify <url>` subcommand recomputes the signature and reports
whether it is authentic. This is a deliberately minimal control — it deters
casual/manual forgery, not a motivated attacker who extracts the release
secret from the binary (see Design Notes).

## Requirements

### Requirement: HMAC Signature Scheme Excludes headSHA

The system SHALL compute the marker signature as `HMAC-SHA256(secret, payload)`
where the payload is the string `<host/owner/repo>\n<headBranch>\n<runID>` —
fields joined with a newline, never `|` — with the `host/owner/repo`
component LOWERCASED before signing; `headBranch` and `runID` are case-exact.
The `host/owner/repo` component SHALL include the GitHub host itself (e.g.
`github.com/org/repo`, `ghe.corp/org/repo`), not a bare `owner/repo`. The
signature SHALL be encoded as lowercase hex truncated to the first 16
characters. The signed payload SHALL NOT include the PR's head commit SHA.
(owner/repo is lowered because GitHub treats owner and repository names
case-insensitively, and creation-time origin-remote casing may legitimately
differ from verify-time URL casing for the same PR. The payload is
HOST-QUALIFIED so a marker signed on one GitHub host can never verify on a
different host — e.g. an Enterprise instance — that happens to share the
same org/repo/branch/runID. Fields are newline-joined, not `|`-joined,
because `|` is a legal character in a git branch name (and in the marker's
own `\S+`-matched runID), so a `|`-joined payload is not injective across a
field boundary; `\n` cannot occur in a git ref name, so the newline-joined
payload stays injective for every input reachable through this scheme.)

#### Scenario: Signature is deterministic for given inputs
- GIVEN a fixed secret, `host/owner/repo`, `headBranch`, and `runID`
- WHEN the signature is computed
- THEN it is a 16-character lowercase hex string, and recomputing it with
  the same inputs yields the same value

#### Scenario: Owner/repo casing differences do not break verification
- GIVEN a marker signed at creation with `host/owner/repo` cased as derived
  from the origin remote URL
- WHEN verification derives the same `host/owner/repo` from a PR URL typed
  with different casing
- THEN the recomputed signature matches, because both sides lowercase the
  `host/owner/repo` component before signing

#### Scenario: Appended commits do not invalidate the signature
- GIVEN a PR was signed at creation with a given `headBranch` and `runID`
- WHEN additional commits are later pushed to the same branch, changing the
  head SHA
- THEN the marker still verifies successfully, because the head SHA was
  never part of the signed payload

#### Scenario: A marker copied to a different host fails verification
- GIVEN a marker signed for `github.com/org/repo` with a given
  `headBranch`/`runID`
- WHEN the same marker text is verified against `ghe.corp/org/repo` with the
  SAME `headBranch`/`runID`
- THEN the recomputed signature does not match, because the payload is
  host-qualified — the host is part of what was signed, not just the
  owner/repo pair

#### Scenario: Payload fields cannot collide across a separator boundary
- GIVEN two payload fields where a `|` character straddles what would be a
  `|`-joined separator boundary (e.g. `headBranch="a|b"`, `runID="c"` versus
  `headBranch="a"`, `runID="b|c"`) — both plausible because `|` is legal in a
  git branch name
- WHEN each is signed
- THEN the two signatures differ, because the payload is newline-joined and
  `\n` cannot appear in either field

### Requirement: Marker And Footer Format

The system SHALL append to a DeployDeck-created PR body a visible footer
`Created with DeployDeck v<version>` (version sourced from the build's
injected version) followed by an invisible HTML-comment marker
`<!-- deploydeck: v1 run:<RunID> sig:<sig> -->`, where `v1` identifies the
marker's payload scheme. Composing the body SHALL first STRIP every
pre-existing deploydeck marker already present in the incoming body (e.g. an
AI-generated description that echoed a prior marker verbatim) before
appending the genuine footer+marker, so that "exactly one marker" holds as
an UNCONDITIONAL invariant regardless of what the incoming body already
contained.

#### Scenario: Marker parses from a body with surrounding content
- GIVEN a PR body containing arbitrary text before and after a valid marker
  line
- WHEN the marker is parsed
- THEN the scheme tag, RunID, and signature are extracted correctly
  regardless of surrounding content

#### Scenario: Absent marker parses as not-found, not an error
- GIVEN a PR body with no `<!-- deploydeck: ... -->` marker
- WHEN the marker is parsed
- THEN parsing reports "not found" and does not return an error

#### Scenario: A body already containing a marker is sanitized before composing
- GIVEN a body that already contains a deploydeck marker (e.g. the AI model
  echoed a previous marker verbatim into its drafted description)
- WHEN the body is composed with a fresh footer and marker
- THEN the pre-existing marker is stripped, and the resulting body carries
  EXACTLY ONE marker — the newly-signed, genuine one

### Requirement: Dev Builds Never Sign With An Empty Key

WHEN the injected secret is empty (dev/snapshot builds), the system SHALL
NOT compute a normal HMAC using the empty key; instead `Sign` SHALL emit a
distinguishable `sig:dev` marker.

#### Scenario: Empty secret produces a dev marker
- GIVEN the build's injected secret is empty
- WHEN a PR body is signed
- THEN the marker's signature field is `dev`, not a computed HMAC hex value

#### Scenario: Verifying a dev marker reports dev-signed, not verified
- GIVEN a PR body carries a `sig:dev` marker
- WHEN verification runs
- THEN it reports the result as dev-signed/unverifiable (development
  build), distinct from both "verified" and "invalid", and exits non-zero

### Requirement: `pr verify <url>` Verification Semantics

The `deploydeck pr verify <url>` subcommand SHALL derive `host/owner/repo`
from the URL argument, fetch the PR's `headRefName` and `body`, parse EVERY
marker present in the body (never just the first), classify each one, and
report the BEST outcome across all of them by priority verified > dev >
mismatch: verified (exit 0) if ANY parsed marker verifies, else
dev-unverifiable (non-zero, distinct message — either a parsed marker itself
carries `sig:dev`, or the verifying binary is a dev build with an empty
secret and therefore cannot recompute release signatures) if any parsed
marker is dev-classified, else invalid-or-missing marker (non-zero) if every
parsed marker mismatches or no marker parses at all. It SHALL also degrade
(non-zero, clear message, never a crash) when `gh` is unavailable/
unauthenticated, the PR URL is unparseable, or an internal Result value is
unrecognized (treated as indeterminate, never as a forgery accusation).

#### Scenario: Genuine marker verifies successfully
- GIVEN a PR body carries a marker signed for that PR's `host/owner/repo`,
  `headBranch`, and `runID`
- WHEN `deploydeck pr verify <url>` runs
- THEN it recomputes the signature, confirms the match, reports verified,
  and exits 0

#### Scenario: Marker copied to a different repo, branch, or host fails verification
- GIVEN a marker signed for one `host/owner/repo`/`headBranch` is present in
  a PR body on a different repo, branch, or GitHub host (including an
  Enterprise host sharing the same org/repo/branch/runID)
- WHEN `deploydeck pr verify <url>` runs
- THEN the recomputed signature does not match, verification fails, and
  the command exits non-zero

#### Scenario: Missing marker fails verification
- GIVEN a PR body has no marker
- WHEN `deploydeck pr verify <url>` runs
- THEN it reports invalid-or-missing marker and exits non-zero

#### Scenario: A prepended forged marker does not discredit a genuine one
- GIVEN a PR body carries a genuine marker AND a forged marker (a bogus
  hex signature, or a `sig:dev` marker) that someone with edit access
  prepended before it
- WHEN `deploydeck pr verify <url>` runs
- THEN it classifies EVERY marker in the body, the genuine marker's
  "verified" outcome wins over the forged one regardless of ordering, and
  the command exits 0 — never reporting a genuine PR as forged

#### Scenario: Only a forged marker is present
- GIVEN a PR body carries no genuine marker, only a marker with a bogus
  signature
- WHEN `deploydeck pr verify <url>` runs
- THEN it reports a signature mismatch and exits non-zero

#### Scenario: Dev-built verifier cannot judge a release marker
- GIVEN the verifying binary was built with an empty secret and the PR body
  carries a normal HMAC marker
- WHEN `deploydeck pr verify <url>` runs
- THEN it reports dev-unverifiable (it cannot recompute release signatures)
  and exits non-zero, and SHALL NOT report the marker as invalid/forged

#### Scenario: gh unavailable degrades without crashing
- GIVEN `gh` is not installed or not authenticated
- WHEN `deploydeck pr verify <url>` attempts to fetch PR details
- THEN it reports a clear degraded-state message and exits non-zero,
  without panicking or crashing

#### Scenario: An unrecognized Result value degrades, never accuses of forgery
- GIVEN an internal classification produces a Result value outside the
  known Verified/Mismatch/DevMarker/DevVerifier set (e.g. a future addition)
- WHEN `deploydeck pr verify <url>` runs
- THEN it reports an indeterminate/degraded outcome and exits non-zero,
  and SHALL NOT report the marker as invalid/forged

## Design Notes

- Threat model is deliberately minimal: the HMAC secret is injected into
  the released binary and is therefore extractable by a motivated
  attacker; the strong alternative (GitHub App / bot-identity attribution)
  is out of scope.
- `host/owner/repo` for verification is derived from the URL argument
  itself, not from `gh`'s reported head repository, so cross-fork PRs
  verify against the base repo the marker was signed against.
- Verification is deliberately any-of across every parsed marker, not
  first-match: a PR body is untrusted, editable text, so trusting whichever
  marker happens to appear first would let a stale echoed marker or a
  bad-faith prepended forgery either mask a genuine marker (false
  "forged") or discredit it. A single genuine signature anywhere in the
  body always wins.
