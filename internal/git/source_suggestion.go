package git

import "deploydeck/internal/config"

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
