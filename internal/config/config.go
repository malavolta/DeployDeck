// Package config loads, validates and queries deploydeck.yaml, the per-repo
// configuration for branches, Salesforce sandboxes, ticket patterns, the
// promotion-branch naming format and minimum tool versions.
package config

// DefaultBranchFormat is applied when branchFormat is omitted from
// deploydeck.yaml. It is rendered (Phase 9, internal/git) via a literal
// strings.NewReplacer token substitution, never Go text/template.
const DefaultBranchFormat = "deploy/{{ticket}}-to-{{target}}"

// DefaultRunsKeepLast/DefaultRunsKeepDays are applied when the runs section
// (or its fields) are omitted from deploydeck.yaml.
const (
	DefaultRunsKeepLast = 30
	DefaultRunsKeepDays = 90
)

// AllowedBranchFormatTokens are the only "{{...}}" substitution tokens
// permitted in branchFormat. Validate (this package) and the Phase 9
// branch-name renderer (internal/git) both check against this exact list so
// the two agree by construction — rendering is a literal
// strings.NewReplacer substitution, never Go text/template (which would
// fail parsing "{{ticket}}" as a function node).
var AllowedBranchFormatTokens = []string{"{{ticket}}", "{{target}}"}

// SandboxConfig associates a branch (or a glob like "Release/*") with a
// Salesforce sandbox alias and the deploy test level to use against it.
type SandboxConfig struct {
	Alias     string `yaml:"alias"`
	TestLevel string `yaml:"testLevel"`
}

// RunsConfig controls local run-history retention under .deploydeck/runs.
type RunsConfig struct {
	KeepLast int `yaml:"keepLast"`
	KeepDays int `yaml:"keepDays"`
}

// Config is the parsed, defaulted deploydeck.yaml.
type Config struct {
	// Branches maps a logical environment name (e.g. "integration", "uat",
	// "production") to the Git branch that represents it.
	Branches map[string]string `yaml:"branches"`

	// Sandboxes maps a branch name or glob (e.g. "Release/*") to the
	// Salesforce sandbox it deploys to. See SandboxFor.
	Sandboxes map[string]SandboxConfig `yaml:"sandboxes"`

	// TicketPatterns are regexes used to recognize ticket identifiers in
	// commit messages/branch names.
	TicketPatterns []string `yaml:"ticketPatterns"`

	// BranchFormat is the promotion-branch naming template. See
	// AllowedBranchFormatTokens.
	BranchFormat string `yaml:"branchFormat"`

	// MinVersions maps a tool name (git, sf, sfdx-git-delta) to the minimum
	// version required by prereq checks.
	MinVersions map[string]string `yaml:"minVersions"`

	// Runs controls local run-history retention.
	Runs RunsConfig `yaml:"runs"`
}
