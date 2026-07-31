package git

import "github.com/malavolta/DeployDeck/internal/config"

// environmentPipelineOrder is the fixed promotion sequence (RF-002) used
// to derive the "previous validated environment" for a target branch.
// config.Config.Branches keys are fixed by convention to these three
// environment names (see docs/ARQUITECTURA.md's example config and
// docs/HISTORIAS.md's "INT -> UAT -> Release -> main" pipeline
// description); Release/* and custom branches sit outside this pipeline
// (HU-004's sandbox glob) and are never suggested as a default source.
var environmentPipelineOrder = []string{"integration", "uat", "production"}

// SuggestDefaultSource returns the previous/validated environment branch
// as the suggested default source for a promotion into target, per
// RF-002: for promotions between two configured sandbox environments
// (e.g. INT -> UAT), the previous pipeline stage is suggested instead of
// a feature branch. ok is false when target does not match a configured
// environment branch, or when target is the first pipeline stage (no
// previous environment to suggest) — the caller falls back to whatever
// default behavior it uses for non-environment promotions.
//
// The suggestion is a UI default ONLY: it never bypasses single-source
// enforcement. Callers still run SelectSingleSource on whatever the user
// finally confirms (the suggestion itself, or an explicit override).
func SuggestDefaultSource(candidates []Branch, cfg config.Config, target string) (Branch, bool) {
	targetEnv, ok := environmentKeyForBranch(cfg, target)
	if !ok {
		return Branch{}, false
	}

	idx := -1
	for i, env := range environmentPipelineOrder {
		if env == targetEnv {
			idx = i
			break
		}
	}
	if idx <= 0 {
		return Branch{}, false
	}

	previousBranch := cfg.Branches[environmentPipelineOrder[idx-1]]
	if previousBranch == "" {
		return Branch{}, false
	}

	for _, c := range candidates {
		if c.Name == previousBranch || c.Name == "origin/"+previousBranch {
			return c, true
		}
	}
	return Branch{Name: previousBranch}, true
}

// NextEnvironmentBranch returns the NEXT environment in the fixed
// environmentPipelineOrder after currentTarget's environment, as the
// suggested default target for a re-promotion (HU-016): a forward mirror of
// SuggestDefaultSource's backward lookup, reusing the same
// environmentPipelineOrder/environmentKeyForBranch machinery. Unlike
// SuggestDefaultSource, there is no candidate list to resolve against here
// — the pipeline's next stage is a fixed, known branch name straight out of
// cfg.Branches, so the return value is always the PLAIN configured branch
// name (e.g. "UAT"), never origin/-prefixed. ok is false when
// currentTarget does not match a configured environment branch (e.g. a
// Release/* glob target or an unconfigured branch), or when currentTarget
// is already the LAST pipeline stage (no next environment to suggest) — the
// caller falls back to a manual target choice.
func NextEnvironmentBranch(cfg config.Config, currentTarget string) (branch string, ok bool) {
	currentEnv, ok := environmentKeyForBranch(cfg, currentTarget)
	if !ok {
		return "", false
	}

	idx := -1
	for i, env := range environmentPipelineOrder {
		if env == currentEnv {
			idx = i
			break
		}
	}
	if idx == -1 || idx+1 >= len(environmentPipelineOrder) {
		return "", false
	}

	nextBranch := cfg.Branches[environmentPipelineOrder[idx+1]]
	if nextBranch == "" {
		return "", false
	}

	return nextBranch, true
}

// environmentKeyForBranch reverse-looks-up which configured environment
// key (e.g. "uat") maps to branch (e.g. "UAT"), if any.
func environmentKeyForBranch(cfg config.Config, branch string) (string, bool) {
	for key, name := range cfg.Branches {
		if name == branch {
			return key, true
		}
	}
	return "", false
}

// IsProductionTarget reports whether target is a production destination,
// FAIL-CLOSED (HU-015 quick-deploy spec: "Production Target Blocked Without
// Explicit Configuration", BLOCKER correction + adversarial-review Finding
// M-2). A target counts as NON-production ONLY when it positively maps to a
// known non-production pipeline environment (integration/uat); EVERY other
// target is treated as production and therefore blocked unless AllowProduction
// is set. Concretely it is true when ANY of:
//   - target is empty (unclassifiable — a run with no recorded target must
//     never slip through as non-production);
//   - target maps to NO configured environment (environmentKeyForBranch
//     ok=false — an unmapped/unknown branch is unclassifiable, so fail closed
//     rather than fail OPEN as the prior version did);
//   - the configured "production" environment key maps to target;
//   - target is the literal ProductionBranchName ("main") per
//     IsProductionBranch — guaranteeing a repo with no "production" key
//     configured at all still blocks the literal main branch.
func IsProductionTarget(cfg config.Config, target string) bool {
	if target == "" {
		return true
	}
	env, ok := environmentKeyForBranch(cfg, target)
	if !ok {
		return true
	}
	return env == "production" || IsProductionBranch(target)
}
