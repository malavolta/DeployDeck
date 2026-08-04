// Package provenance computes and verifies the HMAC-signed marker
// DeployDeck appends to every PR body it creates (internal/app's
// createPRCmd composes it; cmd/deploydeck's `pr verify` re-checks it). It is
// a PURE leaf: no exec, no network, no github import (enforced by
// boundary_test.go) — every dependency is crypto/regexp/stdlib plus the
// sibling internal/version leaf, so it links safely into both the
// composing app package and the verifying CLI without pulling in either's
// I/O (design.md's "Provenance home" decision).
package provenance

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"

	"github.com/malavolta/DeployDeck/internal/version"
)

// secret is the HMAC key injected at release build time via goreleaser's
// `-ldflags -X .../provenance.secret=<CI secret>` (design.md's "Secret
// injection" decision). Its zero value ("") is the dev/snapshot default:
// Sign NEVER computes a real HMAC with an empty key — an empty-key HMAC is
// publicly forgeable and indistinguishable from a genuine release signature
// — it emits the literal "dev" sentinel instead, and Verify classifies both
// a dev-signed marker and a dev-built verifier as explicitly unverifiable
// (DevMarker/DevVerifier), never Verified and never silently Mismatch.
var secret string

// Result classifies a Verify outcome.
type Result int

const (
	// Verified means the recomputed signature matches: the marker was
	// genuinely signed for the given owner/repo, headBranch and runID.
	Verified Result = iota
	// Mismatch means the recomputed signature does NOT match — a forged
	// marker, or one copied to a different repo/branch/run than it was
	// signed for.
	Mismatch
	// DevMarker means the marker itself carries the "dev" sentinel
	// signature (it was created by a dev/snapshot build with an empty
	// secret): authenticity cannot be confirmed, but this is explicitly
	// NOT reported as forged.
	DevMarker
	// DevVerifier means the verifying binary itself was built with an
	// empty secret and therefore cannot recompute a release signature,
	// regardless of what the marker carries: authenticity cannot be
	// judged, and this is NEVER reported as forged/invalid.
	DevVerifier
)

// markerPattern matches a v1 marker anywhere in a PR body: run:<RunID>
// (\S+ — branch names/run IDs never carry spaces) and sig:<16-hex|dev>.
// ParseMarkers returns EVERY match, in document order; "v1" tags the
// payload scheme so a future format change can coexist with (or reject)
// markers written under this one.
var markerPattern = regexp.MustCompile(`<!-- deploydeck: v1 run:(\S+) sig:([0-9a-f]{16}|dev) -->`)

// Sign computes the marker signature for ownerRepo/headBranch/runID:
// HMAC-SHA256(secret, lower(ownerRepo)+"\n"+headBranch+"\n"+runID),
// truncated to the first 16 lowercase hex characters.
//
// ownerRepo MUST be HOST-QUALIFIED ("host/owner/repo", e.g.
// "github.com/org/repo" — see github.OwnerRepo/github.ParsePRURL): a
// host-stripped "owner/repo" would let a marker signed for one GitHub host
// verify unchanged against the same org/repo/branch/runID on a different
// (e.g. Enterprise) host. It is lowercased before signing because GitHub
// (including Enterprise GitHub) treats host/owner/repo case-insensitively
// and creation-time origin-remote casing may legitimately differ from
// verify-time URL casing for the same PR; headBranch/runID stay case-exact.
//
// Payload fields are joined with "\n", never "|": "|" is a LEGAL character
// in a git branch name (and in the marker's own \S+-matched runID), so a
// "|"-joined payload is NOT injective — Sign(o,"a|b","c") and
// Sign(o,"a","b|c") would collide. "\n" cannot occur in a git ref name, and
// the marker regex's \S+ never matches a newline either, so the
// "\n"-joined payload stays injective for every input reachable through
// this package's own APIs.
//
// The head commit SHA is deliberately never part of the payload, so commits
// appended to the branch after signing never invalidate the marker.
// secret=="" (dev/snapshot builds) never computes a normal HMAC with an
// empty key — it returns the literal "dev" sentinel instead.
func Sign(ownerRepo, headBranch, runID string) string {
	if secret == "" {
		return "dev"
	}
	payload := strings.ToLower(ownerRepo) + "\n" + headBranch + "\n" + runID
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(payload))
	return hex.EncodeToString(mac.Sum(nil))[:16]
}

// RenderFooter formats the visible "Created with DeployDeck vX.Y.Z" line
// from internal/version.Version, sourcing it internally so callers never
// need their own dependency on internal/version — internal/app in
// particular must not import it directly (its own boundary_test.go
// enforces that). The unbuilt "dev" default renders a distinct dev-build
// variant, never the nonsensical "v dev".
func RenderFooter() string {
	if version.Version == "dev" {
		return "Created with DeployDeck (dev build)"
	}
	return fmt.Sprintf("Created with DeployDeck v%s", version.Version)
}

// RenderMarker formats the invisible HTML-comment marker embedding runID
// and sig.
func RenderMarker(runID, sig string) string {
	return fmt.Sprintf("<!-- deploydeck: v1 run:%s sig:%s -->", runID, sig)
}

// Marker holds one parsed v1 marker's fields.
type Marker struct {
	RunID string
	Sig   string
}

// ParseMarkers extracts EVERY v1 marker found anywhere in body, in the
// order they appear — never just the first. A body produced by Compose
// itself always carries exactly one (Compose sanitizes away any
// pre-existing marker before appending its own), but a PR body fetched from
// a live PR is untrusted, editable text: an AI-generated description can
// echo a prior marker verbatim, and anyone with edit access to the PR can
// prepend a forged one to try to discredit a genuine marker elsewhere in
// the body. Callers classify EVERY parsed marker and report the best
// outcome (any-of), never trust whichever one happens to come first. A nil
// slice — never an error — means no marker is present, or only a foreign/
// unrecognized form (e.g. a v2 marker, or free-form text) exists: an absent
// marker is expected, ordinary data for most PRs, not a failure.
func ParseMarkers(body string) []Marker {
	matches := markerPattern.FindAllStringSubmatch(body, -1)
	if len(matches) == 0 {
		return nil
	}
	markers := make([]Marker, 0, len(matches))
	for _, m := range matches {
		markers = append(markers, Marker{RunID: m[1], Sig: m[2]})
	}
	return markers
}

// Verify recomputes the signature for ownerRepo/headBranch/runID and
// classifies the result against sig (a marker's parsed signature field). A
// "dev" sig (DevMarker) or a "dev" verifying secret (DevVerifier) is
// classified BEFORE the byte comparison — both are explicitly unverifiable,
// never silently Mismatch (which would misreport an environmental/dev
// condition as forgery) nor Verified (which would wrongly assert an
// authenticity nothing here can actually confirm).
func Verify(ownerRepo, headBranch, runID, sig string) Result {
	if sig == "dev" {
		return DevMarker
	}
	if secret == "" {
		return DevVerifier
	}
	if hmac.Equal([]byte(Sign(ownerRepo, headBranch, runID)), []byte(sig)) {
		return Verified
	}
	return Mismatch
}

// Compose appends RenderFooter (always) and, when ownerRepo is non-empty,
// the invisible signed marker, to body. Before doing so it STRIPS every
// pre-existing deploydeck marker already present in body (e.g. an
// AI-generated description that echoed a prior marker verbatim) so
// "exactly one marker" holds as an UNCONDITIONAL invariant — never two, with
// a stale one first (that would make a genuine PR fail first-match
// verification). An empty (or marker-only, once stripped) body degrades to
// footer(+marker) only, never a leading blank line. ownerRepo=="" (the
// origin remote URL could not be parsed) degrades to footer-only, WITHOUT a
// marker: a marker whose signature could not be bound to a real owner/repo
// must never be written (push-pr-preparation spec's "Unparseable origin
// degrades to footer-only" scenario).
func Compose(body, ownerRepo, headBranch, runID string) string {
	body = stripMarkers(body)
	var b strings.Builder
	if body != "" {
		b.WriteString(body)
		b.WriteString("\n\n")
	}
	b.WriteString(RenderFooter())
	if ownerRepo != "" {
		b.WriteString("\n")
		b.WriteString(RenderMarker(runID, Sign(ownerRepo, headBranch, runID)))
	}
	return b.String()
}

// stripMarkers removes every deploydeck marker already present in body and
// collapses the blank-line runs marker removal can leave behind, so the
// sanitized body composes cleanly with Compose's own footer/marker instead
// of leaving visible gaps where a stripped marker used to be.
func stripMarkers(body string) string {
	if !markerPattern.MatchString(body) {
		return body
	}
	stripped := markerPattern.ReplaceAllString(body, "")
	for strings.Contains(stripped, "\n\n\n") {
		stripped = strings.ReplaceAll(stripped, "\n\n\n", "\n\n")
	}
	return strings.TrimSpace(stripped)
}
