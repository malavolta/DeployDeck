package git

import (
	"context"
	"fmt"
	"sort"

	"deploydeck/internal/config"
	"deploydeck/internal/salesforce"
)

// ProductionBranchName is the destination branch HU-004 treats as the
// production environment (docs/HISTORIAS.md AC: "Dado que el usuario
// selecciona main, entonces la TUI muestra advertencia de entorno
// productivo"). Literal, not config-driven: the spec names "main"
// explicitly, unlike config.Config.Branches["production"], which maps a
// logical key to whatever branch name a repo actually uses.
const ProductionBranchName = "main"

// IsProductionBranch reports whether branch triggers HU-004's
// production-environment warning.
func IsProductionBranch(branch string) bool {
	return branch == ProductionBranchName
}

// Destination is one configured destination branch (config.Config.Branches)
// shown to the user during HU-004 target selection, together with its
// resolved Salesforce sandbox (config.SandboxFor).
type Destination struct {
	// Environment is the logical config.Config.Branches key ("integration",
	// "uat", "production").
	Environment string
	// Branch is the Git branch this environment maps to (e.g. "INT").
	Branch string
	// Sandbox is the resolved sandbox config. Its zero value when
	// SandboxResolved is false.
	Sandbox config.SandboxConfig
	// SandboxResolved is false when config.SandboxFor found no mapping for
	// Branch — callers MUST block continuing on this destination until one
	// exists (HU-004 AC: "si no hay mapeo, se bloquea con mensaje
	// accionable").
	SandboxResolved bool
}

// ListDestinations assembles every configured destination branch with its
// resolved sandbox alias and test level (HU-004 AC: "cada destino se
// muestra con su sandbox asociada"). A branch with no matching sandbox
// entry is still listed, flagged SandboxResolved=false rather than
// silently dropped, so the caller can render/block it explicitly. Sorted
// by Environment for stable, deterministic output (config.Config.Branches
// is a Go map with no inherent order).
func ListDestinations(cfg config.Config) []Destination {
	envs := make([]string, 0, len(cfg.Branches))
	for env := range cfg.Branches {
		envs = append(envs, env)
	}
	sort.Strings(envs)

	destinations := make([]Destination, 0, len(envs))
	for _, env := range envs {
		branch := cfg.Branches[env]
		sandbox, err := cfg.SandboxFor(branch)
		destinations = append(destinations, Destination{
			Environment:     env,
			Branch:          branch,
			Sandbox:         sandbox,
			SandboxResolved: err == nil,
		})
	}
	return destinations
}

// ResolveSandbox resolves branch's sandbox via config.SandboxFor — covering
// both exact-configured branches and Release/* glob patterns alike — and
// turns a resolution failure into an actionable block message (HU-004 AC:
// "si el usuario selecciona una rama Release/* [...] si no hay mapeo, se
// bloquea con mensaje accionable"; the same rule applies to any other
// unmapped destination, not just Release/*).
func ResolveSandbox(cfg config.Config, branch string) (sandbox config.SandboxConfig, blockMessage string, err error) {
	sandbox, err = cfg.SandboxFor(branch)
	if err != nil {
		return config.SandboxConfig{}, fmt.Sprintf(
			"destination %q has no sandbox configured; add a matching entry under deploydeck.yaml's sandboxes section",
			branch,
		), err
	}
	return sandbox, "", nil
}

// UnauthenticatedSandboxWarning reports whether alias is absent from every
// category of orgs (salesforce.OrgList.FindByAlias scans all five) — a
// non-blocking warning shown BEFORE validation (HU-004 AC: "si la sandbox
// asociada no esta autenticada, se muestra advertencia antes de
// validar"). An empty alias never warns here: an unmapped sandbox is
// ResolveSandbox's concern, not this one's.
func UnauthenticatedSandboxWarning(orgs salesforce.OrgList, alias string) bool {
	if alias == "" {
		return false
	}
	_, found := orgs.FindByAlias(alias)
	return !found
}

// SandboxAuthWarning runs `sf org list --json` and reports whether alias is
// not currently authenticated, composing salesforce.Client with the pure
// UnauthenticatedSandboxWarning decision — the entry point target-selection
// uses to render the warning.
func SandboxAuthWarning(ctx context.Context, sf salesforce.Client, alias string) (bool, error) {
	orgs, err := sf.Orgs(ctx)
	if err != nil {
		return false, fmt.Errorf("git: checking sandbox alias %q authentication: %w", alias, err)
	}
	return UnauthenticatedSandboxWarning(orgs, alias), nil
}

// ConfirmTargetSelection saves the confirmed destination branch, sandbox
// alias, and test level into plan (HU-004 AC: "el usuario confirma una
// seleccion valida [...] branch, alias, y test level quedan en el
// DeploymentPlan"). It does not re-validate branch existence or sandbox
// resolution — callers run Service.BranchExists / ResolveSandbox (and act
// on their results) BEFORE calling this, the same gate-then-persist shape
// GenerateDeploymentPlan's ValidateSelection call uses for HU-003. Every
// other field already on plan (e.g. HU-003's Ticket/SelectedCommits) is
// preserved.
func ConfirmTargetSelection(plan DeploymentPlan, branch, sandboxAlias, testLevel string) DeploymentPlan {
	plan.TargetBranch = branch
	plan.SandboxAlias = sandboxAlias
	plan.TestLevel = testLevel
	return plan
}
