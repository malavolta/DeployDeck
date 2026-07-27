package salesforce

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"deploydeck/internal/exec"
)

// deployQueueSOQL is HU-009's exact query (HISTORIAS.md:606-614,
// EPICA.md:371-375): active DeployRequest jobs ordered by CreatedDate
// ascending. It is passed as ONE slice element to --query, never
// shell-joined/interpolated (design.md Threat Matrix "PR / argument
// composition").
const deployQueueSOQL = "SELECT Id,Status,CheckOnly,CreatedDate,StartDate,CompletedDate,CreatedBy.Name,CreatedBy.Username,NumberComponentsTotal,NumberComponentsDeployed,NumberComponentErrors,NumberTestsTotal,NumberTestsCompleted,NumberTestErrors FROM DeployRequest WHERE Status IN ('Pending','InProgress') ORDER BY CreatedDate ASC"

// ErrQueuePermission is the typed sentinel ListDeployQueue returns when the
// sf CLI error matches the Tooling-API-permission heuristic
// (permissionErrorKeywords). Callers should warn and continue the flow
// without the queue view (HU-009 AC: "avisa y el flujo continua sin la
// vista de cola") rather than treating it as a generic, actionable failure.
// Unverified against a real restricted profile (design.md Open Questions):
// a deliberately broad heuristic; any unmatched error stays generic and is
// never swallowed.
var ErrQueuePermission = errors.New("salesforce: profile lacks Tooling API permission to query DeployRequest")

// permissionErrorKeywords are case-insensitive substrings that, when present
// in the sf CLI error message, indicate a Tooling-API permission denial
// rather than a generic query failure.
var permissionErrorKeywords = []string{
	"insufficient access",
	"insufficient_access",
	"not authorized",
	"unauthorized",
	"permission",
}

// QueueComponentProgress is one DeployRequest's metadata-component progress
// counters (HU-009: "progreso").
type QueueComponentProgress struct {
	Total    int
	Deployed int
	Errors   int
}

// QueueTestProgress is one DeployRequest's Apex-test progress counters.
type QueueTestProgress struct {
	Total     int
	Completed int
	Errors    int
}

// DeployQueueEntry is one parsed DeployRequest queue record (HU-009).
// CreatedBy is the display name (CreatedBy.Name); Username is the stable
// identity key (CreatedBy.Username) own-job matching keys off — a display
// name can rename/collide, a username cannot (design.md "Own-job identity
// source").
type DeployQueueEntry struct {
	JobID     string
	Status    string
	CheckOnly bool
	CreatedBy string
	Username  string

	CreatedDate   time.Time
	StartDate     time.Time
	CompletedDate time.Time

	Components QueueComponentProgress
	Tests      QueueTestProgress
}

// deployQueueResult is the envelope's `result` shape for
// `sf data query --use-tooling-api --json`, confirmed empirically
// (2026-07-27) against AM-DEV-EDITION: {status, result:{records[],
// totalSize, done}, warnings}. NOT a bare records[] slice.
type deployQueueResult struct {
	TotalSize int                   `json:"totalSize"`
	Done      bool                  `json:"done"`
	Records   []deployRequestRecord `json:"records"`
}

// deployRequestRecord is one DeployRequest record's raw JSON shape. Date
// fields stay strings here (Salesforce datetime literals aren't always
// RFC3339-clean) and are parsed best-effort by toEntry.
type deployRequestRecord struct {
	ID            string `json:"Id"`
	Status        string `json:"Status"`
	CheckOnly     bool   `json:"CheckOnly"`
	CreatedDate   string `json:"CreatedDate"`
	StartDate     string `json:"StartDate"`
	CompletedDate string `json:"CompletedDate"`
	CreatedBy     struct {
		Name     string `json:"Name"`
		Username string `json:"Username"`
	} `json:"CreatedBy"`
	NumberComponentsTotal    int `json:"NumberComponentsTotal"`
	NumberComponentsDeployed int `json:"NumberComponentsDeployed"`
	NumberComponentErrors    int `json:"NumberComponentErrors"`
	NumberTestsTotal         int `json:"NumberTestsTotal"`
	NumberTestsCompleted     int `json:"NumberTestsCompleted"`
	NumberTestErrors         int `json:"NumberTestErrors"`
}

// toEntry maps the raw record into the public DeployQueueEntry shape.
func (r deployRequestRecord) toEntry() DeployQueueEntry {
	return DeployQueueEntry{
		JobID:         r.ID,
		Status:        r.Status,
		CheckOnly:     r.CheckOnly,
		CreatedBy:     r.CreatedBy.Name,
		Username:      r.CreatedBy.Username,
		CreatedDate:   parseSFDateTime(r.CreatedDate),
		StartDate:     parseSFDateTime(r.StartDate),
		CompletedDate: parseSFDateTime(r.CompletedDate),
		Components: QueueComponentProgress{
			Total:    r.NumberComponentsTotal,
			Deployed: r.NumberComponentsDeployed,
			Errors:   r.NumberComponentErrors,
		},
		Tests: QueueTestProgress{
			Total:     r.NumberTestsTotal,
			Completed: r.NumberTestsCompleted,
			Errors:    r.NumberTestErrors,
		},
	}
}

// sfDateTimeLayouts are candidate layouts for Salesforce's CreatedDate/
// StartDate/CompletedDate strings (Tooling API JSON typically emits
// "2026-07-27T10:00:00.000+0000").
var sfDateTimeLayouts = []string{
	"2006-01-02T15:04:05.000-0700",
	time.RFC3339,
}

// parseSFDateTime is a best-effort parse: an empty or unparseable value
// degrades to a zero time.Time rather than failing the whole queue decode.
func parseSFDateTime(raw string) time.Time {
	if raw == "" {
		return time.Time{}
	}
	for _, layout := range sfDateTimeLayouts {
		if t, err := time.Parse(layout, raw); err == nil {
			return t
		}
	}
	return time.Time{}
}

// isQueuePermissionError reports whether an sf CLI error message matches the
// Tooling-API-permission heuristic (case-insensitive substring match).
func isQueuePermissionError(message string) bool {
	lower := strings.ToLower(message)
	for _, kw := range permissionErrorKeywords {
		if strings.Contains(lower, kw) {
			return true
		}
	}
	return false
}

// ListDeployQueue runs the HU-009 query and decodes the envelope's
// {totalSize,done,records[]} result into DeployQueueEntry, preserving the
// server-side CreatedDate ASC ordering. A CLI error is classified via
// validateErrorMessage's decoded-message precedent: a permission-shaped
// message returns ErrQueuePermission (wrapped, so errors.Is still matches);
// any other message is a generic, actionable error — never swallowed.
func (c *client) ListDeployQueue(ctx context.Context, targetOrg string) ([]DeployQueueEntry, error) {
	result, err := c.runner.Run(ctx, exec.CommandRequest{
		Name: "sf",
		Args: []string{
			"data", "query",
			"--target-org", targetOrg,
			"--use-tooling-api",
			"--json",
			"--query", deployQueueSOQL,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("salesforce: running sf data query (deploy queue): %w", err)
	}

	raw := combineOutput(result.Stdout, result.Stderr)
	if result.ExitCode != 0 {
		msg := validateErrorMessage(result.Stdout, raw)
		if isQueuePermissionError(msg) {
			return nil, fmt.Errorf("%w: %s", ErrQueuePermission, msg)
		}
		return nil, fmt.Errorf("salesforce: sf data query (deploy queue) exited %d: %s", result.ExitCode, msg)
	}

	var decoded deployQueueResult
	if err := decodeEnvelope(result.Stdout, &decoded); err != nil {
		return nil, err
	}

	entries := make([]DeployQueueEntry, 0, len(decoded.Records))
	for _, rec := range decoded.Records {
		entries = append(entries, rec.toEntry())
	}
	return entries, nil
}
