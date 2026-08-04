package provenance

import (
	"strings"
	"testing"

	"github.com/malavolta/DeployDeck/internal/version"
)

// withSecret sets the package-level, ldflags-injected secret var for the
// duration of the test, restoring it in cleanup — mirrors internal/ai's
// doctorProbeTimeout mutation pattern (client_doctor_timeout_test.go) for
// testing a build-injected package var from an internal (non-_test suffix)
// test file.
func withSecret(t *testing.T, s string) {
	t.Helper()
	restore := secret
	secret = s
	t.Cleanup(func() { secret = restore })
}

// withVersion sets internal/version.Version for the duration of the test.
func withVersion(t *testing.T, v string) {
	t.Helper()
	restore := version.Version
	version.Version = v
	t.Cleanup(func() { version.Version = restore })
}

// --- 1.1: Sign determinism, hex[:16] lowercase truncation, payload canon,
// dev sentinel -----------------------------------------------------------

// TestSign_Deterministic is task 1.1 (RED): a fixed secret/owner-repo/
// headBranch/runID always recomputes to the SAME 16-character lowercase hex
// signature.
func TestSign_Deterministic(t *testing.T) {
	withSecret(t, "topsecret")

	got1 := Sign("org/repo", "deploy/PROJ-1-to-UAT", "PROJ-1-to-UAT-20260101000000")
	got2 := Sign("org/repo", "deploy/PROJ-1-to-UAT", "PROJ-1-to-UAT-20260101000000")
	if got1 != got2 {
		t.Fatalf("Sign() should be deterministic for fixed inputs, got %q then %q", got1, got2)
	}
	if len(got1) != 16 {
		t.Fatalf("Sign() length = %d, want 16", len(got1))
	}
	if got1 != strings.ToLower(got1) {
		t.Fatalf("Sign() must be lowercase hex, got %q", got1)
	}
}

// TestSign_PayloadCanonicalization is task 1.1 (RED): owner/repo is
// lowercased before signing (GitHub is case-insensitive on owner/repo), but
// headBranch and runID stay case-exact, and the "|" separator keeps fields
// from colliding across a boundary.
func TestSign_PayloadCanonicalization(t *testing.T) {
	withSecret(t, "topsecret")

	base := Sign("Org/Repo", "deploy/PROJ-1-to-UAT", "run-1")

	t.Run("owner/repo casing does not change the signature", func(t *testing.T) {
		other := Sign("org/repo", "deploy/PROJ-1-to-UAT", "run-1")
		if other != base {
			t.Fatalf("Sign(%q,...) = %q, want it to match Sign(%q,...) = %q (owner/repo must be lowered before signing)", "org/repo", other, "Org/Repo", base)
		}
	})

	t.Run("headBranch casing is case-exact", func(t *testing.T) {
		other := Sign("Org/Repo", "Deploy/PROJ-1-to-UAT", "run-1")
		if other == base {
			t.Fatalf("headBranch casing changed but the signature stayed %q — headBranch must be case-exact", base)
		}
	})

	t.Run("runID casing is case-exact", func(t *testing.T) {
		other := Sign("Org/Repo", "deploy/PROJ-1-to-UAT", "RUN-1")
		if other == base {
			t.Fatalf("runID casing changed but the signature stayed %q — runID must be case-exact", base)
		}
	})

	t.Run("newline separator keeps fields from colliding even when a field itself contains \"|\"", func(t *testing.T) {
		// "|" is a LEGAL character in a git branch name (and in the marker's
		// own \S+-matched runID), so a "|"-joined payload is NOT injective:
		// Sign("h/o/r","a|b","c") and Sign("h/o/r","a","b|c") would collide
		// under the old scheme. "\n" cannot occur in a git ref name, so the
		// "\n"-joined payload stays injective for every input reachable
		// through this package's own APIs.
		a := Sign("h/o/r", "a|b", "c")
		b := Sign("h/o/r", "a", "b|c")
		if a == b {
			t.Fatalf("Sign(\"h/o/r\",\"a|b\",\"c\") and Sign(\"h/o/r\",\"a\",\"b|c\") collided on %q — the payload separator must keep fields distinct even when a field contains \"|\"", a)
		}
	})
}

// TestSign_DevSentinel_EmptySecret is task 1.1 (RED): secret=="" (the
// dev/snapshot default) must NEVER compute a real HMAC — it returns the
// literal "dev" sentinel instead, per spec "Dev Builds Never Sign With An
// Empty Key".
func TestSign_DevSentinel_EmptySecret(t *testing.T) {
	withSecret(t, "")

	got := Sign("org/repo", "deploy/PROJ-1-to-UAT", "run-1")
	if got != "dev" {
		t.Fatalf("Sign() with empty secret = %q, want the literal \"dev\" sentinel", got)
	}
}

// --- 1.2: RenderMarker/ParseMarkers roundtrip; absent/foreign markers ----

// TestMarker_RoundTrip is task 1.2 (RED): RenderMarker's output, embedded
// anywhere inside a larger body, parses back to the exact same runID/sig via
// ParseMarkers.
func TestMarker_RoundTrip(t *testing.T) {
	runID := "PROJ-1-to-UAT-20260101000000"
	sig := "0123456789abcdef"
	marker := RenderMarker(runID, sig)

	body := "Some PR description.\n\nMore detail.\n\n" + marker + "\ntrailing text after the marker"
	markers := ParseMarkers(body)
	if len(markers) != 1 {
		t.Fatalf("ParseMarkers should find exactly 1 marker embedded in %q, got %d: %+v", body, len(markers), markers)
	}
	if markers[0].RunID != runID || markers[0].Sig != sig {
		t.Fatalf("ParseMarkers()[0] = %+v, want {RunID: %q, Sig: %q}", markers[0], runID, sig)
	}
}

// TestMarker_DevSigRoundTrips is task 1.2 (RED): the "dev" sentinel sig
// value round-trips through Render/Parse like any other sig.
func TestMarker_DevSigRoundTrips(t *testing.T) {
	marker := RenderMarker("run-1", "dev")
	markers := ParseMarkers(marker)
	if len(markers) != 1 || markers[0].RunID != "run-1" || markers[0].Sig != "dev" {
		t.Fatalf("ParseMarkers(%q) = %+v, want exactly 1 marker {RunID: \"run-1\", Sig: \"dev\"}", marker, markers)
	}
}

// TestParseMarkers_AbsentAndForeign is task 1.2 (RED): a missing marker, a
// foreign/differently-versioned marker, or garbage HTML-comment content all
// parse to an empty slice — never a panic, never a partial match.
func TestParseMarkers_AbsentAndForeign(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{"no marker at all", "Just a plain PR body with no marker."},
		{"empty body", ""},
		{"foreign v2 marker", "<!-- deploydeck: v2 run:run-1 sig:0123456789abcdef -->"},
		{"garbage html comment", "<!-- something entirely unrelated -->"},
		{"malformed sig (too short hex)", "<!-- deploydeck: v1 run:run-1 sig:abc -->"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			markers := ParseMarkers(tt.body)
			if len(markers) != 0 {
				t.Fatalf("ParseMarkers(%q) should report no markers, got %+v", tt.body, markers)
			}
		})
	}
}

// TestParseMarkers_MultipleInOrder is task B (RED, remediation): a body
// carrying more than one marker (a stale/echoed one, or a bad-faith
// prepended forgery) parses ALL of them, in document order — never just the
// first — so a caller can classify every one instead of trusting whichever
// happens to come first.
func TestParseMarkers_MultipleInOrder(t *testing.T) {
	first := RenderMarker("run-1", "0000000000000000")
	second := RenderMarker("run-2", "1111111111111111")
	body := "Bogus prepended marker:\n" + first + "\n\nGenuine marker:\n" + second

	markers := ParseMarkers(body)
	if len(markers) != 2 {
		t.Fatalf("ParseMarkers() should find both markers, got %d: %+v", len(markers), markers)
	}
	if markers[0].RunID != "run-1" || markers[1].RunID != "run-2" {
		t.Fatalf("ParseMarkers() should preserve document order, got %+v", markers)
	}
}

// --- 1.3: RenderFooter, Verify, Compose -----------------------------------

// TestRenderFooter is task 1.3 (RED): an injected semver renders
// "Created with DeployDeck vX.Y.Z"; the unbuilt "dev" default renders a
// distinct dev-build variant, never the nonsensical "v dev".
func TestRenderFooter(t *testing.T) {
	t.Run("injected semver", func(t *testing.T) {
		withVersion(t, "1.2.3")
		got := RenderFooter()
		want := "Created with DeployDeck v1.2.3"
		if got != want {
			t.Fatalf("RenderFooter() = %q, want %q", got, want)
		}
	})

	t.Run("default dev build", func(t *testing.T) {
		withVersion(t, "dev")
		got := RenderFooter()
		want := "Created with DeployDeck (dev build)"
		if got != want {
			t.Fatalf("RenderFooter() = %q, want %q", got, want)
		}
	})
}

// TestVerify is task 1.3 (RED): Verified on a genuine match, Mismatch on
// tampered sig / wrong repo / wrong branch / wrong runID, DevMarker when the
// marker itself carries "sig:dev".
func TestVerify(t *testing.T) {
	withSecret(t, "topsecret")
	ownerRepo, headBranch, runID := "github.com/org/repo", "deploy/PROJ-1-to-UAT", "run-1"
	validSig := Sign(ownerRepo, headBranch, runID)

	tests := []struct {
		name       string
		ownerRepo  string
		headBranch string
		runID      string
		sig        string
		want       Result
	}{
		{"verified", ownerRepo, headBranch, runID, validSig, Verified},
		{"tampered sig", ownerRepo, headBranch, runID, "0000000000000000", Mismatch},
		{"wrong repo", "github.com/other/repo", headBranch, runID, validSig, Mismatch},
		// wrong host, SAME org/repo/branch/runID: a marker signed for
		// github.com must never verify on an Enterprise host that happens to
		// share the same org/repo/branch/runID — the payload must be
		// HOST-BOUND, not just owner/repo-bound.
		{"wrong host (same org/repo/branch/runID)", "ghe.corp/org/repo", headBranch, runID, validSig, Mismatch},
		{"wrong branch", ownerRepo, "deploy/OTHER-to-UAT", runID, validSig, Mismatch},
		{"wrong runID", ownerRepo, headBranch, "run-2", validSig, Mismatch},
		{"dev-signed marker", ownerRepo, headBranch, runID, "dev", DevMarker},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Verify(tt.ownerRepo, tt.headBranch, tt.runID, tt.sig)
			if got != tt.want {
				t.Fatalf("Verify() = %v, want %v", got, tt.want)
			}
		})
	}

	t.Run("dev verifier (empty secret) is never reported as mismatch", func(t *testing.T) {
		withSecret(t, "")
		got := Verify(ownerRepo, headBranch, runID, validSig)
		if got != DevVerifier {
			t.Fatalf("Verify() with an empty verifier secret = %v, want DevVerifier (never Mismatch/Verified)", got)
		}
	})
}

// TestCompose is task 1.3 (RED): body+footer+marker; empty body degrades to
// footer(+marker) only with no leading blank line; ownerRepo=="" degrades to
// footer ONLY with no marker at all; exactly one marker is ever written.
func TestCompose(t *testing.T) {
	withVersion(t, "1.2.3")
	withSecret(t, "topsecret")
	footer := RenderFooter()

	t.Run("body + footer + marker", func(t *testing.T) {
		got := Compose("Original body.", "org/repo", "deploy/PROJ-1-to-UAT", "run-1")
		if !strings.HasPrefix(got, "Original body.\n\n"+footer) {
			t.Fatalf("Compose() should start with body then the footer, got %q", got)
		}
		if strings.Count(got, "<!-- deploydeck:") != 1 {
			t.Fatalf("Compose() should contain exactly 1 marker, got %q", got)
		}
		markers := ParseMarkers(got)
		if len(markers) != 1 || markers[0].RunID != "run-1" {
			t.Fatalf("Compose() marker should parse back with runID %q, got %+v", "run-1", markers)
		}
	})

	t.Run("sanitizes a pre-existing marker so exactly one genuine marker ships", func(t *testing.T) {
		// A body that already contains a deploydeck marker — e.g. an
		// AI-generated description that echoed a prior marker verbatim —
		// must not survive Compose alongside the genuine one: "exactly one
		// marker" is an unconditional invariant, not a best-effort default.
		stale := RenderMarker("stale-run", "deadbeefdeadbeef")
		body := "AI drafted description.\n\n" + stale + "\n\nMore AI text."

		got := Compose(body, "org/repo", "deploy/PROJ-1-to-UAT", "run-1")
		if strings.Count(got, "<!-- deploydeck:") != 1 {
			t.Fatalf("Compose() with a pre-existing marker should still contain exactly 1 marker, got %q", got)
		}
		markers := ParseMarkers(got)
		if len(markers) != 1 || markers[0].RunID != "run-1" {
			t.Fatalf("Compose() should ship only the genuine marker (run-1), got %+v", markers)
		}
		if strings.Contains(got, "stale-run") || strings.Contains(got, "deadbeefdeadbeef") {
			t.Fatalf("Compose() must strip the stale marker entirely, got %q", got)
		}
		if Verify("org/repo", "deploy/PROJ-1-to-UAT", markers[0].RunID, markers[0].Sig) != Verified {
			t.Fatalf("the sanitized genuine marker should verify, got runID=%q sig=%q", markers[0].RunID, markers[0].Sig)
		}
	})

	t.Run("empty body degrades to footer+marker only", func(t *testing.T) {
		got := Compose("", "org/repo", "deploy/PROJ-1-to-UAT", "run-1")
		if !strings.HasPrefix(got, footer) {
			t.Fatalf("Compose() with an empty body should start directly with the footer, got %q", got)
		}
		if strings.HasPrefix(got, "\n") {
			t.Fatalf("Compose() with an empty body must not leave a leading blank line, got %q", got)
		}
		if strings.Count(got, "<!-- deploydeck:") != 1 {
			t.Fatalf("Compose() with an empty body should still contain exactly 1 marker, got %q", got)
		}
	})

	t.Run("ownerRepo empty degrades to footer only, no marker", func(t *testing.T) {
		got := Compose("Original body.", "", "deploy/PROJ-1-to-UAT", "run-1")
		want := "Original body.\n\n" + footer
		if got != want {
			t.Fatalf("Compose() with ownerRepo==\"\" = %q, want %q (footer only, no marker)", got, want)
		}
		if strings.Contains(got, "<!-- deploydeck:") {
			t.Fatalf("Compose() with ownerRepo==\"\" must never write a marker, got %q", got)
		}
	})
}
