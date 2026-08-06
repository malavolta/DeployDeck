// Package gate is the PURE deploy-gate evaluator (design.md "Gate package"
// decision): it depends only on internal/config (GateConfig) and
// internal/provenance (Result) — no exec, no gh, no network — so every
// condition is exhaustively unit-testable without a real PR
// (boundary_test.go enforces this). internal/app maps gh DTOs into Facts at
// the boundary and calls Evaluate; internal/github owns every actual gh
// call.
package gate

import (
	"fmt"
	"strings"

	"github.com/malavolta/DeployDeck/internal/config"
	"github.com/malavolta/DeployDeck/internal/provenance"
)

// MarkerPrefix is the hidden HTML-comment prefix every validation-comment
// marker starts with — HasValidationComment's presence check, and
// ValidationComment's own output (deploy-gate spec: "Validation-Comment
// Condition And Posting").
const MarkerPrefix = "<!-- deploydeck-validation:"

// ApproverReview is one reviewer's LATEST review state on a PR, mapped from
// github.Review at the app boundary — only the fields Evaluate's approvals
// condition needs.
type ApproverReview struct {
	Login string
	State string
}

// Facts is everything Evaluate needs to decide a gate's outcome: the
// resolved GateConfig plus every already-fetched, already-mapped gh datum.
// Each *Err field is set ONLY when the corresponding gh read failed
// (Runner error, non-zero exit, or a parse/graphql error) — Evaluate fails
// CLOSED on that condition alone, never crashes, never treats a fetch
// failure as "unknown but ignorable".
type Facts struct {
	Config config.GateConfig

	// PRResolved is false when the run's PR could not be located at all
	// (fail-closed PR resolution, deploy-gate spec's "Fail-Closed PR
	// Resolution") — Evaluate then reports a single failed pr-resolution
	// condition and evaluates nothing else.
	PRResolved bool

	Reviews    []ApproverReview
	ReviewsErr error

	UnresolvedCount int
	ThreadErr       error

	CommentPresent bool
	CommentErr     error

	Provenance    provenance.Result
	ProvenanceErr error
}

// Condition is one gate condition's outcome. Name is one of
// "pr-resolution", "approvals", "threads", "validation-comment",
// "signature".
type Condition struct {
	Name   string
	Passed bool
	Detail string
}

// Result is a full gate evaluation's outcome: Passed iff every EVALUATED
// condition passed. A condition disabled via an explicit Require*=false
// toggle stays OUT of Conditions entirely — it was never evaluated, not
// evaluated-and-ignored.
type Result struct {
	Passed     bool
	Conditions []Condition
}

// ToggleOn resolves a *bool Require* toggle's effective on/off state: nil
// (omitted) means "on" (design's "*bool nil ≠ false" decision) — the
// caller has already confirmed the gate itself is Enabled before ever
// building Facts, so this only decides whether ONE condition is evaluated,
// never whether the gate as a whole applies. Exported (remediation-pass
// readability fix) so internal/app's requireValidationCommentOn can reuse
// this SAME default-on convention instead of duplicating it, nil-check and
// all, bound only by a comment.
func ToggleOn(toggle *bool) bool {
	if toggle == nil {
		return true
	}
	return *toggle
}

// effectiveMinApprovals returns Config.MinApprovals's effective value: nil
// (omitted) defaults to 1 (design's "MinApprovals default" decision).
// config.Validate rejects an explicit value < 1 for an enabled gate before
// a Config ever reaches here; this floors defensively anyway rather than
// trusting that invariant blindly.
func effectiveMinApprovals(cfg config.GateConfig) int {
	if cfg.MinApprovals == nil {
		return 1
	}
	if *cfg.MinApprovals < 1 {
		return 1
	}
	return *cfg.MinApprovals
}

// normalizeLogin strips a leading "@" and lowercases s, so approver-list
// comparison is case-insensitive and tolerant of an "@handle" vs "handle"
// mismatch (deploy-gate spec: "Login comparison SHALL be normalized").
func normalizeLogin(s string) string {
	return strings.ToLower(strings.TrimPrefix(strings.TrimSpace(s), "@"))
}

// Evaluate decides a gate's outcome from Facts. !PRResolved short-circuits
// to a single failed pr-resolution condition — no other condition is
// evaluated, since none of them mean anything without a resolved PR.
// Otherwise every condition ENABLED on Config is evaluated and appended
// (approvals is NEVER toggleable — it always applies once Enabled; the
// other three default ON when their Require* field is omitted, and an
// explicit `false` skips that ONE condition entirely — it never appears in
// Conditions at all); Result.Passed is true iff every evaluated condition
// passed.
func Evaluate(f Facts) Result {
	if !f.PRResolved {
		return Result{
			Passed: false,
			Conditions: []Condition{
				{Name: "pr-resolution", Passed: false, Detail: "no se pudo resolver el PR del run (ni PRUrl ni PRForBranch)"},
			},
		}
	}

	var conditions []Condition
	conditions = append(conditions, evaluateApprovals(f))

	if ToggleOn(f.Config.RequireResolvedThreads) {
		conditions = append(conditions, evaluateThreads(f))
	}
	if ToggleOn(f.Config.RequireValidationComment) {
		conditions = append(conditions, evaluateValidationComment(f))
	}
	if ToggleOn(f.Config.RequireSignature) {
		conditions = append(conditions, evaluateSignature(f))
	}

	passed := true
	for _, c := range conditions {
		if !c.Passed {
			passed = false
			break
		}
	}
	return Result{Passed: passed, Conditions: conditions}
}

// evaluateApprovals implements the Approval Condition (deploy-gate spec):
// at least effectiveMinApprovals distinct, normalized approver-list logins
// whose LATEST review state is APPROVED, with no listed login's LATEST
// state being CHANGES_REQUESTED (a later changes-requested negates an
// earlier approval). Reviews is iterated in order, so the LAST entry for a
// given login wins — mirrors github.PRReviews' own `--json latestReviews`
// contract, which already reports one entry per author, but Evaluate stays
// robust even if a caller passes a fuller review history.
func evaluateApprovals(f Facts) Condition {
	if f.ReviewsErr != nil {
		return Condition{Name: "approvals", Passed: false, Detail: fmt.Sprintf("no se pudieron leer los reviews del PR: %v", f.ReviewsErr)}
	}

	allowed := make(map[string]bool, len(f.Config.Approvers))
	for _, a := range f.Config.Approvers {
		allowed[normalizeLogin(a)] = true
	}

	approved := map[string]bool{}
	for _, r := range f.Reviews {
		login := normalizeLogin(r.Login)
		if !allowed[login] {
			continue
		}
		switch r.State {
		case "APPROVED":
			approved[login] = true
		case "CHANGES_REQUESTED":
			delete(approved, login)
		}
	}

	// required (remediation-pass readability fix): was named "min", shadowing
	// the Go builtin min().
	required := effectiveMinApprovals(f.Config)
	if len(approved) >= required {
		return Condition{Name: "approvals", Passed: true, Detail: fmt.Sprintf("%d/%d aprobaciones requeridas", len(approved), required)}
	}
	return Condition{Name: "approvals", Passed: false, Detail: fmt.Sprintf("%d/%d aprobaciones requeridas de la lista de approvers", len(approved), required)}
}

// evaluateThreads implements the Unresolved-Threads Condition (deploy-gate
// spec): passes only when UnresolvedCount == 0; a thread-status lookup/auth
// failure (ThreadErr) fails closed, never treated as unknown-but-ignorable.
func evaluateThreads(f Facts) Condition {
	if f.ThreadErr != nil {
		return Condition{Name: "threads", Passed: false, Detail: fmt.Sprintf("no se pudo determinar el estado de los threads: %v", f.ThreadErr)}
	}
	if f.UnresolvedCount > 0 {
		return Condition{Name: "threads", Passed: false, Detail: fmt.Sprintf("%d thread(s) sin resolver", f.UnresolvedCount)}
	}
	return Condition{Name: "threads", Passed: true, Detail: "todos los threads resueltos"}
}

// evaluateValidationComment implements the Validation-Comment Condition
// (deploy-gate spec): passes only when a marker comment is present on the
// PR (cooperative, presence-only trust — regardless of author); a
// comment-read failure (CommentErr) fails closed.
func evaluateValidationComment(f Facts) Condition {
	if f.CommentErr != nil {
		return Condition{Name: "validation-comment", Passed: false, Detail: fmt.Sprintf("no se pudieron leer los comentarios del PR: %v", f.CommentErr)}
	}
	if !f.CommentPresent {
		return Condition{Name: "validation-comment", Passed: false, Detail: "no existe un comentario de validación en el PR"}
	}
	return Condition{Name: "validation-comment", Passed: true, Detail: "comentario de validación presente"}
}

// evaluateSignature implements the Signature Condition (deploy-gate spec):
// passes ONLY on provenance.Verified — a missing/mismatched marker, a
// dev-signed marker, or a dev-built verifier ALL fail (unverifiable is
// never valid); a fetch/parse failure (ProvenanceErr) fails closed.
func evaluateSignature(f Facts) Condition {
	if f.ProvenanceErr != nil {
		return Condition{Name: "signature", Passed: false, Detail: fmt.Sprintf("no se pudo verificar la firma de procedencia: %v", f.ProvenanceErr)}
	}
	if f.Provenance == provenance.Verified {
		return Condition{Name: "signature", Passed: true, Detail: "firma de procedencia verificada"}
	}
	return Condition{Name: "signature", Passed: false, Detail: "firma de procedencia ausente o no verificable"}
}

// AggregateCoverage computes Σ(total-notCovered)/Σtotal across every
// per-class coverage entry (design's "Aggregate coverage" note: a computed
// figure, not a value stored elsewhere). totals/notCovered are paired by
// index; a shorter notCovered is treated as 0 for the missing indices. An
// all-zero (or empty) totals slice returns known=false rather than a
// divide-by-zero pct.
func AggregateCoverage(totals, notCovered []int) (pct int, known bool) {
	var sumTotal, sumCovered int
	for i, total := range totals {
		sumTotal += total
		nc := 0
		if i < len(notCovered) {
			nc = notCovered[i]
		}
		sumCovered += total - nc
	}
	if sumTotal == 0 {
		return 0, false
	}
	return sumCovered * 100 / sumTotal, true
}

// ValidationComment composes the marker-carrying validation comment
// (deploy-gate spec: "Validation-Comment Condition And Posting"): a hidden
// dedup marker keyed to jobID/runID plus human-readable detail (component/
// test error counts and the aggregate coverage figure). It embeds NO
// provenance secret (design's threat matrix: "Outward comment write").
func ValidationComment(jobID, runID string, componentErrs, testErrs, coveragePct int, coverageKnown bool) (marker, body string) {
	marker = fmt.Sprintf("%s job:%s run:%s -->", MarkerPrefix, jobID, runID)

	coverage := "desconocida"
	if coverageKnown {
		coverage = fmt.Sprintf("%d%%", coveragePct)
	}

	body = fmt.Sprintf(
		"%s\n\n**Validación de DeployDeck**\n\n- Job: %s\n- Run: %s\n- Errores de componentes: %d\n- Errores de tests: %d\n- Cobertura agregada: %s\n",
		marker, jobID, runID, componentErrs, testErrs, coverage,
	)
	return marker, body
}

// HasValidationComment reports whether ANY body in bodies carries
// MarkerPrefix — the idempotent-upsert dedup check (deploy-gate spec:
// "Re-validation does not duplicate the comment").
func HasValidationComment(bodies []string) bool {
	for _, b := range bodies {
		if strings.Contains(b, MarkerPrefix) {
			return true
		}
	}
	return false
}
