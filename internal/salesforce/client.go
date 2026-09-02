// Package salesforce is a minimal, READ-ONLY shim over the `sf` CLI, backed
// by internal/exec.Runner. It covers exactly the three calls HU-001
// (prereq-check) and HU-004 (target-selection alias validation) need:
// `sf --version`, `sf plugins --json`, `sf org list --json`. Fase 3 extends
// the Client interface with deploy/validate/report/queue methods on the same
// Runner seam; this package is never rewritten, only grown.
package salesforce

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/malavolta/DeployDeck/internal/exec"
)

// Client is the read-only Salesforce CLI shim.
type Client interface {
	// Version runs `sf --version`. Its output is plain text, not JSON.
	Version(ctx context.Context) (VersionInfo, error)
	// Plugins runs `sf plugins --json`.
	Plugins(ctx context.Context) ([]Plugin, error)
	// Orgs runs `sf org list --json`.
	Orgs(ctx context.Context) (OrgList, error)
	// ValidateDeploy runs `sf project deploy validate --async --json`
	// (HU-010): async CheckOnly validation of a delta package, returning
	// the jobId to poll via ReportDeploy.
	ValidateDeploy(ctx context.Context, req ValidateRequest) (ValidateResult, error)
	// ReportDeploy runs `sf project deploy report --json` once (HU-011):
	// a single poll of an async job's current status/progress/failures.
	// Callers (internal/app's ValidationPolling state) drive the repeated
	// polling themselves; this is never a blocking loop.
	ReportDeploy(ctx context.Context, jobID, targetOrg, dir string) (DeployReport, error)
	// ListDeployQueue runs `sf data query --use-tooling-api --json` (HU-009):
	// active DeployRequest jobs (Pending/InProgress) ordered by CreatedDate
	// ascending, giving operational visibility of the shared sandbox queue
	// before/during a validation. Returns ErrQueuePermission when the
	// profile lacks Tooling API access to DeployRequest — callers should
	// warn and continue without the queue view (non-blocking degrade); any
	// other query failure is a generic, actionable error, never swallowed.
	ListDeployQueue(ctx context.Context, targetOrg string) ([]DeployQueueEntry, error)
	// CancelDeploy runs `sf project deploy cancel --job-id <id> --target-org
	// <alias> --json` (HU-012): it cancels the CURRENT run's own async
	// validation/deploy job to free the shared sandbox queue. jobID is always
	// the run's own job (never another user's, never user-typed) and is passed
	// as a discrete slice arg, never shell-interpolated. dir is the SFDX
	// project root (the directory holding sfdx-project.json) the command
	// runs in — set as exec.CommandRequest.Dir, never appended to Args
	// (design ADR-8, directory-resolution spec: "`sf project` Commands Bind
	// To The SFDX Project Root"). Success and failure both preserve Raw;
	// error handling mirrors ValidateDeploy so the flow survives a CLI error
	// without crashing.
	CancelDeploy(ctx context.Context, jobID, targetOrg, dir string) (CancelResult, error)
	// QuickDeploy runs `sf project deploy quick --job-id <id> --target-org
	// <alias> --json` (HU-015): it re-runs a prior successful validation's
	// job as an actual deploy, reusing that job's already-passed test
	// results within Salesforce's quick-deploy window. jobID is always an
	// eligible run's own job (never another run's, never user-typed) and is
	// passed as a discrete slice arg, never shell-interpolated. dir is the
	// SFDX project root the command runs in — same contract as
	// CancelDeploy's dir (design ADR-8). Success and failure both preserve
	// Raw; error handling mirrors CancelDeploy so the flow survives a CLI
	// error without crashing.
	QuickDeploy(ctx context.Context, jobID, targetOrg, dir string) (QuickDeployResult, error)
}

// VersionInfo is the parsed `sf --version` output.
type VersionInfo struct {
	// CLIVersion is the version extracted from the "@salesforce/cli/<version>"
	// token in the raw output.
	CLIVersion string
	// Raw is the full, trimmed command output, kept for detail/error display.
	Raw string
}

// Plugin is one entry from `sf plugins --json`.
type Plugin struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// Org is one entry from any category of `sf org list --json`.
type Org struct {
	Alias           string `json:"alias"`
	Username        string `json:"username"`
	ConnectedStatus string `json:"connectedStatus"`
	IsDefault       bool   `json:"isDefaultUsername"`
}

// OrgList is the full `sf org list --json` result, holding all five
// categories the CLI buckets an org into.
type OrgList struct {
	NonScratch []Org `json:"nonScratchOrgs"`
	Scratch    []Org `json:"scratchOrgs"`
	Sandboxes  []Org `json:"sandboxes"`
	DevHubs    []Org `json:"devHubs"`
	Other      []Org `json:"other"`
}

// FindByAlias searches ALL FIVE categories for an org with the given alias,
// never just a default subset, so an alias living in a non-default bucket
// (e.g. devHubs) is never reported as a false-negative missing alias.
func (l OrgList) FindByAlias(alias string) (Org, bool) {
	for _, group := range [][]Org{l.NonScratch, l.Scratch, l.Sandboxes, l.DevHubs, l.Other} {
		for _, org := range group {
			if org.Alias == alias {
				return org, true
			}
		}
	}
	return Org{}, false
}

// client is the Runner-backed Client implementation.
type client struct {
	runner exec.Runner
}

// New returns a Client backed by runner.
func New(runner exec.Runner) Client {
	return &client{runner: runner}
}

// envelope is the `{"status":0,"result":...}` wrapper every `sf ... --json`
// command emits; Result is decoded per-call into the caller's expected shape.
type envelope struct {
	Status int             `json:"status"`
	Result json.RawMessage `json:"result"`
}

func decodeEnvelope(raw []byte, out any) error {
	var env envelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return fmt.Errorf("salesforce: parsing sf JSON envelope: %w", err)
	}
	if err := json.Unmarshal(env.Result, out); err != nil {
		return fmt.Errorf("salesforce: parsing sf JSON result: %w", err)
	}
	return nil
}

var cliVersionPattern = regexp.MustCompile(`@salesforce/cli/(\S+)`)

// parseVersionOutput extracts the CLI version token from `sf --version`'s
// plain-text output (e.g. "@salesforce/cli/2.63.6 darwin-arm64 node-v22.11.0").
func parseVersionOutput(raw string) VersionInfo {
	trimmed := strings.TrimSpace(raw)
	info := VersionInfo{Raw: trimmed}
	if m := cliVersionPattern.FindStringSubmatch(trimmed); len(m) == 2 {
		info.CLIVersion = m[1]
	}
	return info
}
