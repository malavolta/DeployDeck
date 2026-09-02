package salesforce_test

// TestE2ERealOrg_Cancel — opt-in, BEST-EFFORT end-to-end test (Phase 7.2)
// exercising CancelDeploy against a real org (HU-012). A real cancel is
// inherently timing-hard: a validation can reach a terminal state before the
// cancel lands, so the run's resulting status is NOT deterministic and is NOT
// asserted here (design.md Testing Strategy: "Cancel timing-hard -> fake
// primary/required; real best-effort: assert CLI runs, not terminal status").
// The required behavioral coverage lives in the FakeRunner unit tests
// (cancel_test.go) and the app-layer cancel-confirm flow tests; this test only
// confirms the CLI call EXECUTES end to end against a live org.
//
// DOUBLE-GATED so it can never run by default and never accidentally cancels a
// stranger's or an unintended job:
//   - DEPLOYDECK_E2E_ORG=<alias>            selects the org, and
//   - DEPLOYDECK_E2E_CANCEL_JOBID=<jobId>   is a jobId the operator explicitly
//     supplies and is willing to attempt cancelling.
//
// When either is unset the test SKIPS (never fails), so `go test ./...` stays
// green in CI and for any contributor. The assertion is deliberately weak: the
// `sf project deploy cancel` invocation must actually run (produce output),
// whether it succeeds or the CLI rejects it (e.g. the job already finished) —
// only a transport/process-start failure is treated as a failure.

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/malavolta/DeployDeck/internal/exec"
	"github.com/malavolta/DeployDeck/internal/salesforce"
)

func TestE2ERealOrg_Cancel(t *testing.T) {
	alias := os.Getenv("DEPLOYDECK_E2E_ORG")
	jobID := os.Getenv("DEPLOYDECK_E2E_CANCEL_JOBID")
	if alias == "" || jobID == "" {
		t.Skip("set DEPLOYDECK_E2E_ORG=<alias> and DEPLOYDECK_E2E_CANCEL_JOBID=<jobId> to run the best-effort real-org cancel e2e (local only, destructive)")
	}

	client := salesforce.New(exec.NewOSRunner())
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// A real directory is now REQUIRED: CancelDeploy rejects an empty dir up
	// front (ErrMissingProjectDir), because internal/exec would otherwise
	// treat it as "inherit the process cwd" — the conflation the
	// directory-resolution change removes. This test exercises the CLI call
	// surviving end-to-end for an operator-supplied jobId, not a real deploy
	// flow, so a temp dir is enough: its contract is that the command
	// EXECUTED and produced output, and a CLI-level rejection counts.
	result, err := client.CancelDeploy(ctx, jobID, alias, t.TempDir())

	// The CLI must have EXECUTED: a non-empty Raw proves `sf project deploy
	// cancel` actually ran and produced output — success OR a CLI-level
	// rejection (job already terminal, etc.), both acceptable here. An empty
	// Raw together with an error means the process never started (a transport /
	// binary-missing failure), which is a real failure worth surfacing.
	if result.Raw == "" && err != nil {
		t.Fatalf("CancelDeploy did not execute against %q (transport/start failure, no output): %v", alias, err)
	}

	// Intentionally NOT asserting the run reached Canceled — timing-hard.
	if err != nil {
		t.Logf("cancel CLI executed against %q for job %q and returned a (non-fatal) CLI-level error: %v", alias, jobID, err)
	} else {
		t.Logf("cancel CLI executed against %q for job %q; raw output captured (terminal status intentionally unchecked)", alias, jobID)
	}
}
