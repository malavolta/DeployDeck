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

// DefaultPollIntervalSeconds/DefaultPollTimeoutSeconds are applied when
// pollIntervalSeconds/pollTimeoutSeconds are omitted (or zero) from
// deploydeck.yaml. They drive HU-011's Salesforce deploy-report polling
// loop (internal/app's ValidationPolling state): how often a tea.Tick
// re-polls, and the hard deadline after which a still-in-progress
// validation is treated as failed (timeout).
const (
	DefaultPollIntervalSeconds = 10
	DefaultPollTimeoutSeconds  = 3600
)

// DefaultDeltaOutputDir is applied when delta.outputDir is omitted. It is
// the BASE directory HU-007's delta artifacts are written under; the final
// per-run directory (<base>/<ticket>-to-<target>) is composed by the
// caller (internal/app), not by this package.
const DefaultDeltaOutputDir = ".deploydeck/manifest/delta"

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

// DeltaConfig configures HU-007's sgd-backed delta generation: the base
// output directory delta artifacts are written under, which source
// directories `sf sgd source delta` scans (each becomes a repeated
// --source-dir flag), and optional gitignore-style files passed through as
// sgd's --ignore-file/--ignore-destructive-file.
type DeltaConfig struct {
	OutputDir             string   `yaml:"outputDir"`
	SourceDirs            []string `yaml:"sourceDirs"`
	IgnoreFile            string   `yaml:"ignoreFile"`
	IgnoreDestructiveFile string   `yaml:"ignoreDestructiveFile"`
}

// QuickDeployConfig gates HU-015's opt-in Salesforce quick deploy
// execution. Both fields default to false (the zero value) — suggest-only
// and production-blocked — which is already the safe default, so Load
// applies no defaulting entry for this section (design.md ADR-3).
type QuickDeployConfig struct {
	// AllowExecution enables quick deploy EXECUTION at all; false (default)
	// means DeployDeck only ever displays the suggested command.
	AllowExecution bool `yaml:"allowExecution"`
	// AllowProduction permits quick deploy execution against a production
	// target; false (default) blocks it regardless of AllowExecution.
	AllowProduction bool `yaml:"allowProduction"`
}

// AIConfig gates the optional local-model PR title/description suggestion
// (ai-pr-summary, closing HU-014's deferred "Idea Futura"). Zero-value-safe
// like QuickDeployConfig: the zero value (Enabled=false, empty
// Endpoint/Model) IS the safe "feature off" default, so Load applies no
// defaulting entry for this section — an entirely omitted `ai:` block is
// exactly as valid as an explicit `enabled: false`.
type AIConfig struct {
	// Endpoint is the local, HTTP-reachable, OpenAI-compatible model
	// server root (e.g. "http://localhost:11434" for Ollama). Required
	// when Enabled is true.
	Endpoint string `yaml:"endpoint"`
	// Model is the configured model name/tag (e.g. "qwen2.5-coder:3b").
	// Required when Enabled is true.
	Model string `yaml:"model"`
	// Enabled turns the AI-suggestion affordance and CheckAI doctor check
	// on; false (default) means neither is wired at all (main.go leaves
	// Deps.GenerateSummary and Checker.AI nil).
	Enabled bool `yaml:"enabled"`
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

	// Delta configures HU-007's sgd-backed delta generation.
	Delta DeltaConfig `yaml:"delta"`

	// PollIntervalSeconds/PollTimeoutSeconds control HU-011's Salesforce
	// deploy-report polling loop: how often (seconds) to poll, and the
	// hard timeout (seconds) after which a still-in-progress validation is
	// treated as failed. See DefaultPollIntervalSeconds/
	// DefaultPollTimeoutSeconds for the values applied when omitted.
	PollIntervalSeconds int `yaml:"pollIntervalSeconds"`
	PollTimeoutSeconds  int `yaml:"pollTimeoutSeconds"`

	// QuickDeploy gates HU-015's opt-in Salesforce quick deploy execution.
	// Both fields zero-value-safe (false); see QuickDeployConfig.
	QuickDeploy QuickDeployConfig `yaml:"quickDeploy"`

	// AI gates the optional local-model PR title/description suggestion.
	// Zero-value-safe (Enabled=false); see AIConfig.
	AI AIConfig `yaml:"ai"`
}
