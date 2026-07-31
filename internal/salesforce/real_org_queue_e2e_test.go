package salesforce_test

// TestE2ERealOrg_Queue — opt-in end-to-end test (Phase 7.1) confirming
// ListDeployQueue's real-org record shape (HU-009), following
// real_org_e2e_test.go's convention EXACTLY: activated only via
// DEPLOYDECK_E2E_ORG=<alias>, skipped (never failed) when unset, so
// `go test ./...` stays green in CI and for any contributor without a
// connected org. No build tag — the env-var gate alone is the documented
// activation mechanism.
//
// READ-ONLY: `sf data query` never mutates the org — this test issues no
// writes and is safe to run repeatedly against a shared sandbox.
//
// PURPOSE beyond smoke coverage: confirms the {totalSize,done,records[]}
// envelope shape and the CreatedBy.Username projection (design.md's
// "Own-job identity source" decision) against a REAL Tooling API response,
// and exercises the exact own-job identification path queueCmd uses
// (Orgs()->FindByAlias(alias).Username matched against each record's
// Username).

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/malavolta/DeployDeck/internal/exec"
	"github.com/malavolta/DeployDeck/internal/salesforce"
)

func TestE2ERealOrg_Queue(t *testing.T) {
	alias := e2eOrgAlias(t)

	runner := exec.NewOSRunner()
	client := salesforce.New(runner)

	queueCtx, queueCancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer queueCancel()

	entries, err := client.ListDeployQueue(queueCtx, alias)
	if err != nil {
		if errors.Is(err, salesforce.ErrQueuePermission) {
			t.Skipf("profile for %q lacks Tooling API permission to query DeployRequest: %v", alias, err)
		}
		t.Fatalf("ListDeployQueue against real org %q: %v", alias, err)
	}
	t.Logf("real queue: %d active DeployRequest job(s) against %q", len(entries), alias)

	// Own-job identification: resolve identity exactly as queueCmd does
	// (Orgs()->FindByAlias(alias).Username), confirming the alias round-trips
	// to a real, non-empty Username.
	orgCtx, orgCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer orgCancel()
	orgs, err := client.Orgs(orgCtx)
	if err != nil {
		t.Fatalf("Orgs() against real org %q: %v", alias, err)
	}
	org, ok := orgs.FindByAlias(alias)
	if !ok {
		t.Fatalf("alias %q not found via FindByAlias — own-job identification would silently fail", alias)
	}
	if org.Username == "" {
		t.Fatalf("resolved org has an empty Username for alias %q", alias)
	}
	t.Logf("resolved own identity for %q: %s", alias, org.Username)

	// Shape confirmation — assertions strong enough to catch a struct-tag
	// mismatch in deployRequestRecord (do NOT weaken these).
	ownFound := false
	for _, e := range entries {
		if e.JobID == "" {
			t.Error("a queue entry has an empty JobID; the records[] shape may have shifted")
		}
		if e.Status != "Pending" && e.Status != "InProgress" {
			t.Errorf("queue entry %q Status = %q, want Pending or InProgress (the WHERE clause should have filtered this)", e.JobID, e.Status)
		}
		if e.Username == "" {
			t.Errorf("queue entry %q has an empty Username — CreatedBy.Username shape mismatch", e.JobID)
		}
		if e.CreatedBy == "" {
			t.Errorf("queue entry %q has an empty CreatedBy — CreatedBy.Name shape mismatch", e.JobID)
		}
		if e.CreatedDate.IsZero() {
			t.Errorf("queue entry %q has a zero CreatedDate — date parsing may not match the real format", e.JobID)
		}
		if e.Username == org.Username {
			ownFound = true
		}
	}
	if ownFound {
		t.Logf("own job (%s) found in the live queue during this run", org.Username)
	} else {
		t.Logf("own job NOT present in the live queue right now (expected unless a validation/deploy is active)")
	}
}
