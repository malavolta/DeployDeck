// Package git's promotion_branch.go holds HU-005's pure, git-free helpers:
// branch-name rendering and the protected-branch guard. See
// service_promotion.go for the Service methods (fetch + branch creation).
package git

import (
	"regexp"
	"strings"

	"github.com/malavolta/DeployDeck/internal/config"
)

// RenderBranchName renders format via a literal strings.NewReplacer token
// substitution — NEVER Go text/template, which would fail parsing
// "{{ticket}}" as a function node (invalid identifier). This shares the
// EXACT token set with config.AllowedBranchFormatTokens, so
// config.Validate (Phase 3) and this renderer agree by construction: any
// format that passes Validate renders here without an unresolved token
// surviving in the output, and vice versa. See design.md's Interfaces/
// Contracts decision.
func RenderBranchName(format, ticket, target string) string {
	replacer := strings.NewReplacer(
		"{{ticket}}", ticket,
		"{{target}}", target,
	)
	return replacer.Replace(format)
}

// ProtectedBranches returns the config-driven set of branches HU-005's
// guard treats as protected: every branch name cfg.Branches maps an
// environment to (e.g. "main", "UAT") — the branches this tool always
// promotes INTO via a temporary branch, never modifies directly (HU-005
// AC: "Validar que no se esta actualmente sobre rama protegida para
// modificarla directamente"). This mirrors HU-004's IsProductionBranch
// policy of naming environment branches literally from config, rather
// than treating every branch in the repo as protected.
func ProtectedBranches(cfg config.Config) []string {
	branches := make([]string, 0, len(cfg.Branches))
	for _, branch := range cfg.Branches {
		branches = append(branches, branch)
	}
	return branches
}

// IsProtectedBranch reports whether branch is one of cfg's configured
// environment branches (see ProtectedBranches). CreatePromotionBranch
// itself never needs this to stay safe — it always bases the new branch on
// the explicit origin/<target> ref, never on the caller's current HEAD, so
// a protected branch's own ref is structurally untouched regardless of
// what branch the user started on. IsProtectedBranch is the config-driven
// predicate the caller (internal/app, Phase 11) uses to detect and inform
// the user they are currently on a protected branch before starting the
// flow.
func IsProtectedBranch(cfg config.Config, branch string) bool {
	for _, protected := range ProtectedBranches(cfg) {
		if protected == branch {
			return true
		}
	}
	return false
}

// promotionBranchTokenPattern matches the two BranchFormat substitution
// tokens config.AllowedBranchFormatTokens allows — config.Validate rejects
// any other "{{...}}" token before a Config ever reaches here, so matching
// exactly these two literal tokens (not a generic "{{...}}" shape) is safe.
var promotionBranchTokenPattern = regexp.MustCompile(`\{\{ticket\}\}|\{\{target\}\}`)

// PromotionBranchMatcher compiles cfg.BranchFormat into an ANCHORED regex
// matching the tool's own promotion-branch shape — the FULL rendered name,
// not just its literal prefix. This replaces a prior prefix-only match
// (design D1 over-exclusion fix): a prefix match wrongly excluded
// legitimate source branches that merely START WITH the same literal text
// but aren't actually promotion branches (e.g. "deploy/DEMO-2", which has
// no "-to-<target>" suffix at all and is a real user branch). Literal
// segments are regexp.QuoteMeta'd; "{{ticket}}"/"{{target}}" become
// "[^/]+" (a non-empty branch segment that never crosses a "/"); the whole
// pattern is anchored with ^...$ so a branch must match the ENTIRE shape,
// not merely contain it.
//
// Returns nil — the caller's signal to SKIP filtering entirely rather than
// drop every candidate — when cfg.BranchFormat is empty, or when stripping
// its tokens leaves no literal content at all (e.g. a bare "{{ticket}}"
// format). Either case would otherwise compile a degenerate pattern
// ("^$" or "^[^/]+$") that matches virtually any simple single-segment
// candidate branch name, not just the tool's own promotion branches —
// which is exactly the "would drop the entire candidate set" failure mode
// PromotionBranchPrefix's old "" guard existed to prevent.
func PromotionBranchMatcher(cfg config.Config) *regexp.Regexp {
	format := cfg.BranchFormat
	if format == "" {
		return nil
	}
	if promotionBranchTokenPattern.ReplaceAllString(format, "") == "" {
		return nil
	}

	var pattern strings.Builder
	pattern.WriteString("^")
	last := 0
	for _, loc := range promotionBranchTokenPattern.FindAllStringIndex(format, -1) {
		pattern.WriteString(regexp.QuoteMeta(format[last:loc[0]]))
		pattern.WriteString("[^/]+")
		last = loc[1]
	}
	pattern.WriteString(regexp.QuoteMeta(format[last:]))
	pattern.WriteString("$")

	re, err := regexp.Compile(pattern.String())
	if err != nil {
		// Unreachable in practice — QuoteMeta'd literals plus a fixed
		// "[^/]+" token always compile — but never let a malformed pattern
		// panic or silently over-match; skip filtering instead.
		return nil
	}
	return re
}

// RegisterPromotionBranch saves branchName as plan's final promotion
// branch name (HU-005 AC: "Dado que la rama se crea correctamente,
// entonces el plan registra el nombre final"). It does not itself create
// or verify the branch — callers run Service.CreatePromotionBranch (and
// act on its result) BEFORE calling this, the same gate-then-persist shape
// ConfirmTargetSelection's doc comment describes for HU-004. Every other
// field already on plan is preserved.
func RegisterPromotionBranch(plan DeploymentPlan, branchName string) DeploymentPlan {
	plan.PromotionBranch = branchName
	return plan
}
