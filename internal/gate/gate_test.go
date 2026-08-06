package gate

import (
	"errors"
	"strings"
	"testing"

	"github.com/malavolta/DeployDeck/internal/config"
	"github.com/malavolta/DeployDeck/internal/provenance"
)

func ptrBool(b bool) *bool { return &b }
func ptrInt(n int) *int    { return &n }

// TestToggleOn is the remediation-pass readability fix (Fix 7): ToggleOn was
// exported so internal/app's requireValidationCommentOn could reuse it
// instead of duplicating the nil-check — this is direct coverage of the now-
// public contract (nil means on; an explicit value is used as-is).
func TestToggleOn(t *testing.T) {
	if !ToggleOn(nil) {
		t.Error("ToggleOn(nil) = false, want true (nil/omitted means on)")
	}
	if !ToggleOn(ptrBool(true)) {
		t.Error("ToggleOn(&true) = false, want true")
	}
	if ToggleOn(ptrBool(false)) {
		t.Error("ToggleOn(&false) = true, want false")
	}
}

// --- 5.1: !PRResolved short-circuits ---------------------------------------

// TestEvaluate_PRResolutionFailsClosed is task 5.1 (RED): !Facts.PRResolved
// yields EXACTLY ONE failed pr-resolution condition; no other condition is
// evaluated (deploy-gate spec: "No resolvable PR blocks the deploy").
func TestEvaluate_PRResolutionFailsClosed(t *testing.T) {
	result := Evaluate(Facts{
		PRResolved: false,
		Config:     config.GateConfig{Enabled: true, Approvers: []string{"alice"}},
		// Every other fact is set to something that WOULD otherwise pass, to
		// prove none of it is ever consulted.
		Reviews:         []ApproverReview{{Login: "alice", State: "APPROVED"}},
		UnresolvedCount: 0,
		CommentPresent:  true,
		Provenance:      provenance.Verified,
	})

	if result.Passed {
		t.Fatal("Evaluate() with PRResolved=false should never pass")
	}
	if len(result.Conditions) != 1 {
		t.Fatalf("Evaluate() with PRResolved=false should yield exactly 1 condition, got %d: %+v", len(result.Conditions), result.Conditions)
	}
	if result.Conditions[0].Name != "pr-resolution" || result.Conditions[0].Passed {
		t.Fatalf("Evaluate() condition = %+v, want a failed pr-resolution condition", result.Conditions[0])
	}
}

// --- 5.2/5.3: normalizeLogin + approvals ------------------------------------

// TestNormalizeLogin is task 5.2 (RED): strips a leading "@" and lowercases.
func TestNormalizeLogin(t *testing.T) {
	tests := []struct{ in, want string }{
		{"alice", "alice"},
		{"@alice", "alice"},
		{"Alice", "alice"},
		{"@Alice", "alice"},
		{"  @Alice  ", "alice"},
	}
	for _, tt := range tests {
		if got := normalizeLogin(tt.in); got != tt.want {
			t.Errorf("normalizeLogin(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

// TestEvaluate_Approvals is tasks 5.2-5.3 (RED): effectiveMin defaults to 1
// when Config.MinApprovals is nil; enough distinct, normalized
// approver-list logins with latest state APPROVED passes; non-listed
// logins never count; a listed login's LATER CHANGES_REQUESTED negates an
// earlier APPROVED; Facts.ReviewsErr fails the condition (fail-closed).
func TestEvaluate_Approvals(t *testing.T) {
	baseFacts := func() Facts {
		return Facts{
			PRResolved: true,
			Config: config.GateConfig{
				Enabled:                  true,
				Approvers:                []string{"@Alice", "bob"},
				RequireResolvedThreads:   ptrBool(false),
				RequireValidationComment: ptrBool(false),
				RequireSignature:         ptrBool(false),
			},
		}
	}

	approvalCondition := func(t *testing.T, result Result) Condition {
		t.Helper()
		for _, c := range result.Conditions {
			if c.Name == "approvals" {
				return c
			}
		}
		t.Fatalf("Evaluate() result has no approvals condition: %+v", result.Conditions)
		return Condition{}
	}

	t.Run("default effectiveMin=1: one normalized, case/@-insensitive approval passes", func(t *testing.T) {
		f := baseFacts()
		f.Reviews = []ApproverReview{{Login: "alice", State: "APPROVED"}} // matches "@Alice" once normalized
		result := Evaluate(f)
		if !approvalCondition(t, result).Passed {
			t.Fatalf("expected approvals to pass, got %+v", result.Conditions)
		}
	})

	t.Run("explicit minApprovals=2 requires 2 distinct listed approvals", func(t *testing.T) {
		f := baseFacts()
		f.Config.MinApprovals = ptrInt(2)
		f.Reviews = []ApproverReview{{Login: "alice", State: "APPROVED"}}
		result := Evaluate(f)
		if approvalCondition(t, result).Passed {
			t.Fatal("expected approvals to fail with only 1/2 required approvals")
		}

		f.Reviews = append(f.Reviews, ApproverReview{Login: "bob", State: "APPROVED"})
		result = Evaluate(f)
		if !approvalCondition(t, result).Passed {
			t.Fatalf("expected approvals to pass with 2/2 required approvals, got %+v", result.Conditions)
		}
	})

	t.Run("approvals from non-listed logins never count", func(t *testing.T) {
		f := baseFacts()
		f.Reviews = []ApproverReview{{Login: "mallory", State: "APPROVED"}}
		result := Evaluate(f)
		if approvalCondition(t, result).Passed {
			t.Fatal("expected approvals to fail: mallory is not on the approver list")
		}
	})

	t.Run("a listed login's later CHANGES_REQUESTED negates its earlier APPROVED", func(t *testing.T) {
		f := baseFacts()
		f.Reviews = []ApproverReview{
			{Login: "alice", State: "APPROVED"},
			{Login: "alice", State: "CHANGES_REQUESTED"}, // latest state for alice
		}
		result := Evaluate(f)
		if approvalCondition(t, result).Passed {
			t.Fatal("expected approvals to fail: alice's latest state is CHANGES_REQUESTED")
		}
	})

	t.Run("ReviewsErr fails the condition (fail-closed)", func(t *testing.T) {
		f := baseFacts()
		f.Reviews = []ApproverReview{{Login: "alice", State: "APPROVED"}, {Login: "bob", State: "APPROVED"}}
		f.ReviewsErr = errors.New("gh: not logged in")
		result := Evaluate(f)
		if approvalCondition(t, result).Passed {
			t.Fatal("expected approvals to fail when ReviewsErr is set, even with otherwise-passing reviews")
		}
	})

	// Remediation-pass SUGGESTION (reliability review): guards the per-user
	// negation (evaluateApprovals only ever deletes FROM the allowed/approved
	// map, keyed by a normalized LISTED login) against a future over-broad
	// "any CHANGES_REQUESTED anywhere blocks" regression. Already correct —
	// this is a test-only addition.
	t.Run("a non-listed reviewer's CHANGES_REQUESTED never vetoes enough listed approvals", func(t *testing.T) {
		f := baseFacts()
		f.Reviews = []ApproverReview{
			{Login: "alice", State: "APPROVED"},
			{Login: "mallory", State: "CHANGES_REQUESTED"}, // not on the approver list: cannot veto
		}
		result := Evaluate(f)
		if !approvalCondition(t, result).Passed {
			t.Fatalf("expected approvals to pass: a non-approver's CHANGES_REQUESTED must never veto, got %+v", result.Conditions)
		}
	})
}

// --- 5.4/5.5: threads --------------------------------------------------

// TestEvaluate_Threads is tasks 5.4-5.5 (RED): UnresolvedCount==0 passes,
// >0 fails; Facts.ThreadErr fails the condition (fail-closed).
func TestEvaluate_Threads(t *testing.T) {
	baseFacts := func() Facts {
		return Facts{
			PRResolved: true,
			Config: config.GateConfig{
				Enabled:                  true,
				Approvers:                []string{"alice"},
				RequireValidationComment: ptrBool(false),
				RequireSignature:         ptrBool(false),
			},
			Reviews: []ApproverReview{{Login: "alice", State: "APPROVED"}},
		}
	}
	threadsCondition := func(t *testing.T, result Result) Condition {
		t.Helper()
		for _, c := range result.Conditions {
			if c.Name == "threads" {
				return c
			}
		}
		t.Fatalf("Evaluate() result has no threads condition: %+v", result.Conditions)
		return Condition{}
	}

	t.Run("zero unresolved threads passes", func(t *testing.T) {
		f := baseFacts()
		f.UnresolvedCount = 0
		result := Evaluate(f)
		if !threadsCondition(t, result).Passed {
			t.Fatalf("expected threads to pass, got %+v", result.Conditions)
		}
	})

	t.Run("one or more unresolved threads fails", func(t *testing.T) {
		f := baseFacts()
		f.UnresolvedCount = 1
		result := Evaluate(f)
		if threadsCondition(t, result).Passed {
			t.Fatal("expected threads to fail with UnresolvedCount=1")
		}
	})

	t.Run("ThreadErr fails the condition (fail-closed)", func(t *testing.T) {
		f := baseFacts()
		f.UnresolvedCount = 0
		f.ThreadErr = errors.New("graphql: auth failure")
		result := Evaluate(f)
		if threadsCondition(t, result).Passed {
			t.Fatal("expected threads to fail when ThreadErr is set, even with UnresolvedCount=0")
		}
	})
}

// --- 5.6/5.7: validation-comment --------------------------------------------

// TestEvaluate_ValidationComment is tasks 5.6-5.7 (RED): CommentPresent
// true passes / false fails, regardless of author (cooperative,
// presence-only trust); Facts.CommentErr fails the condition (fail-closed).
func TestEvaluate_ValidationComment(t *testing.T) {
	baseFacts := func() Facts {
		return Facts{
			PRResolved: true,
			Config: config.GateConfig{
				Enabled:                true,
				Approvers:              []string{"alice"},
				RequireResolvedThreads: ptrBool(false),
				RequireSignature:       ptrBool(false),
			},
			Reviews: []ApproverReview{{Login: "alice", State: "APPROVED"}},
		}
	}
	commentCondition := func(t *testing.T, result Result) Condition {
		t.Helper()
		for _, c := range result.Conditions {
			if c.Name == "validation-comment" {
				return c
			}
		}
		t.Fatalf("Evaluate() result has no validation-comment condition: %+v", result.Conditions)
		return Condition{}
	}

	t.Run("comment present passes", func(t *testing.T) {
		f := baseFacts()
		f.CommentPresent = true
		result := Evaluate(f)
		if !commentCondition(t, result).Passed {
			t.Fatalf("expected validation-comment to pass, got %+v", result.Conditions)
		}
	})

	t.Run("comment absent fails", func(t *testing.T) {
		f := baseFacts()
		f.CommentPresent = false
		result := Evaluate(f)
		if commentCondition(t, result).Passed {
			t.Fatal("expected validation-comment to fail when absent")
		}
	})

	t.Run("CommentErr fails the condition (fail-closed)", func(t *testing.T) {
		f := baseFacts()
		f.CommentPresent = true
		f.CommentErr = errors.New("gh: not logged in")
		result := Evaluate(f)
		if commentCondition(t, result).Passed {
			t.Fatal("expected validation-comment to fail when CommentErr is set, even with CommentPresent=true")
		}
	})
}

// --- 5.8/5.9: signature ------------------------------------------------

// TestEvaluate_Signature is tasks 5.8-5.9 (RED): provenance.Verified
// passes; Mismatch/DevMarker/DevVerifier all fail (unverifiable is never
// valid); Facts.ProvenanceErr fails the condition (fail-closed).
func TestEvaluate_Signature(t *testing.T) {
	baseFacts := func() Facts {
		return Facts{
			PRResolved: true,
			Config: config.GateConfig{
				Enabled:                  true,
				Approvers:                []string{"alice"},
				RequireResolvedThreads:   ptrBool(false),
				RequireValidationComment: ptrBool(false),
			},
			Reviews: []ApproverReview{{Login: "alice", State: "APPROVED"}},
		}
	}
	sigCondition := func(t *testing.T, result Result) Condition {
		t.Helper()
		for _, c := range result.Conditions {
			if c.Name == "signature" {
				return c
			}
		}
		t.Fatalf("Evaluate() result has no signature condition: %+v", result.Conditions)
		return Condition{}
	}

	tests := []struct {
		name string
		prov provenance.Result
		want bool
	}{
		{"Verified passes", provenance.Verified, true},
		{"Mismatch fails", provenance.Mismatch, false},
		{"DevMarker fails (unverifiable, not valid)", provenance.DevMarker, false},
		{"DevVerifier fails (unverifiable, not valid)", provenance.DevVerifier, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := baseFacts()
			f.Provenance = tt.prov
			result := Evaluate(f)
			if got := sigCondition(t, result).Passed; got != tt.want {
				t.Fatalf("signature.Passed = %v, want %v (Provenance=%v)", got, tt.want, tt.prov)
			}
		})
	}

	t.Run("ProvenanceErr fails the condition (fail-closed)", func(t *testing.T) {
		f := baseFacts()
		f.Provenance = provenance.Verified
		f.ProvenanceErr = errors.New("gh: fetching PR body failed")
		result := Evaluate(f)
		if sigCondition(t, result).Passed {
			t.Fatal("expected signature to fail when ProvenanceErr is set, even with Provenance=Verified")
		}
	})
}

// --- 5.10/5.11: toggle wiring -----------------------------------------------

// TestEvaluate_ToggleWiring is tasks 5.10-5.11 (RED): a Require* explicit
// false skips that condition from Result.Passed even when its underlying
// fact would fail; Require* nil (omitted) IS evaluated (default-on);
// approvals is NEVER toggleable.
func TestEvaluate_ToggleWiring(t *testing.T) {
	conditionNames := func(result Result) []string {
		names := make([]string, len(result.Conditions))
		for i, c := range result.Conditions {
			names[i] = c.Name
		}
		return names
	}
	contains := func(names []string, want string) bool {
		for _, n := range names {
			if n == want {
				return true
			}
		}
		return false
	}

	t.Run("every Require* nil (omitted) is evaluated: a failing threads fact blocks the gate", func(t *testing.T) {
		result := Evaluate(Facts{
			PRResolved:      true,
			Config:          config.GateConfig{Enabled: true, Approvers: []string{"alice"}},
			Reviews:         []ApproverReview{{Login: "alice", State: "APPROVED"}},
			UnresolvedCount: 1, // would fail threads
			CommentPresent:  true,
			Provenance:      provenance.Verified,
		})
		names := conditionNames(result)
		if !contains(names, "threads") || !contains(names, "validation-comment") || !contains(names, "signature") {
			t.Fatalf("expected every default-on condition to be evaluated when Require* is nil, got %+v", names)
		}
		if result.Passed {
			t.Fatal("expected Result.Passed=false: the unresolved thread should block")
		}
	})

	t.Run("Require*=false skips that condition from Result even when its fact would fail", func(t *testing.T) {
		result := Evaluate(Facts{
			PRResolved: true,
			Config: config.GateConfig{
				Enabled:                  true,
				Approvers:                []string{"alice"},
				RequireResolvedThreads:   ptrBool(false),
				RequireValidationComment: ptrBool(false),
				RequireSignature:         ptrBool(false),
			},
			Reviews:         []ApproverReview{{Login: "alice", State: "APPROVED"}},
			UnresolvedCount: 5,                   // would fail threads, but it's toggled off
			CommentPresent:  false,               // would fail validation-comment, but toggled off
			Provenance:      provenance.Mismatch, // would fail signature, but toggled off
		})
		names := conditionNames(result)
		if contains(names, "threads") || contains(names, "validation-comment") || contains(names, "signature") {
			t.Fatalf("expected NO toggled-off condition to appear in Result.Conditions, got %+v", names)
		}
		if !result.Passed {
			t.Fatalf("expected Result.Passed=true: every evaluated condition (approvals only) passes, got %+v", result.Conditions)
		}
	})

	t.Run("approvals is never toggleable: it is always evaluated regardless of any Require* field", func(t *testing.T) {
		result := Evaluate(Facts{
			PRResolved: true,
			Config: config.GateConfig{
				Enabled:                  true,
				Approvers:                []string{"alice"},
				RequireResolvedThreads:   ptrBool(false),
				RequireValidationComment: ptrBool(false),
				RequireSignature:         ptrBool(false),
			},
			Reviews: nil, // nobody approved
		})
		if !contains(conditionNames(result), "approvals") {
			t.Fatal("expected approvals to always be evaluated (it is not a toggleable condition)")
		}
		if result.Passed {
			t.Fatal("expected Result.Passed=false: approvals has no qualifying review")
		}
	})
}

// --- 5.12: AggregateCoverage -------------------------------------------

// TestAggregateCoverage is task 5.12 (RED): Σ(total-notCovered)/Σtotal; an
// all-zero (or empty) totals slice returns known=false.
func TestAggregateCoverage(t *testing.T) {
	t.Run("sums covered/total across multiple classes", func(t *testing.T) {
		// class A: 100 locations, 20 not covered -> 80 covered
		// class B: 50 locations, 0 not covered -> 50 covered
		// total: 150 locations, 130 covered -> 86%
		pct, known := AggregateCoverage([]int{100, 50}, []int{20, 0})
		if !known {
			t.Fatal("expected known=true for a non-zero totals slice")
		}
		if pct != 86 {
			t.Fatalf("AggregateCoverage() pct = %d, want 86", pct)
		}
	})

	t.Run("all-zero totals returns known=false", func(t *testing.T) {
		_, known := AggregateCoverage([]int{0, 0}, []int{0, 0})
		if known {
			t.Fatal("expected known=false for an all-zero totals slice")
		}
	})

	t.Run("empty slices return known=false", func(t *testing.T) {
		_, known := AggregateCoverage(nil, nil)
		if known {
			t.Fatal("expected known=false for empty totals/notCovered slices")
		}
	})
}

// --- 5.14: ValidationComment / HasValidationComment -------------------------

// TestValidationComment is task 5.14 (RED): the composed marker carries
// MarkerPrefix; the body includes the error counts and coverage figure, and
// embeds NO provenance secret.
func TestValidationComment(t *testing.T) {
	marker, body := ValidationComment("0Af1", "run-1", 2, 3, 92, true)

	if !strings.HasPrefix(marker, MarkerPrefix) {
		t.Fatalf("ValidationComment() marker = %q, want it to start with MarkerPrefix %q", marker, MarkerPrefix)
	}
	if !strings.Contains(body, marker) {
		t.Fatalf("ValidationComment() body should embed the marker, got %q", body)
	}
	if !strings.Contains(body, "0Af1") || !strings.Contains(body, "run-1") {
		t.Fatalf("ValidationComment() body should mention jobID/runID, got %q", body)
	}
	if !strings.Contains(body, "2") || !strings.Contains(body, "3") {
		t.Fatalf("ValidationComment() body should mention the component/test error counts, got %q", body)
	}
	if !strings.Contains(body, "92") {
		t.Fatalf("ValidationComment() body should mention the coverage figure, got %q", body)
	}
	// A provenance HMAC signature is 16 lowercase hex chars; the body must
	// never embed anything resembling one (design's threat matrix: "Outward
	// comment write" — no provenance secret embedded).
	if strings.Contains(body, "sig:") {
		t.Fatalf("ValidationComment() body must never embed a provenance signature, got %q", body)
	}

	t.Run("unknown coverage renders distinctly from a real percentage", func(t *testing.T) {
		_, unknownBody := ValidationComment("0Af1", "run-1", 0, 0, 0, false)
		if strings.Contains(unknownBody, "0%") {
			t.Fatalf("ValidationComment() with coverageKnown=false must not render a fake 0%%, got %q", unknownBody)
		}
	})
}

// TestHasValidationComment is task 5.14 (RED): true iff any body contains
// MarkerPrefix.
func TestHasValidationComment(t *testing.T) {
	_, markerBody := ValidationComment("0Af1", "run-1", 0, 0, 100, true)

	if HasValidationComment([]string{"LGTM", "looks good"}) {
		t.Fatal("expected HasValidationComment=false when no body carries the marker")
	}
	if !HasValidationComment([]string{"LGTM", markerBody}) {
		t.Fatal("expected HasValidationComment=true when a body carries the marker")
	}
	if HasValidationComment(nil) {
		t.Fatal("expected HasValidationComment=false for an empty bodies slice")
	}
}
