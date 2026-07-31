package runs_test

import (
	"testing"
	"time"

	"github.com/malavolta/DeployDeck/internal/runs"
)

// TestQuickDeployEligible_TableDriven is task 1.4 (RED): a pure table test
// over QuickDeployEligible's 4 predicates (quick-deploy spec: "Eligibility
// Detection For Quick Deploy" — all 4 scenarios — plus the locked
// double-deploy-guard correction).
func TestQuickDeployEligible_TableDriven(t *testing.T) {
	now := time.Date(2026, 7, 27, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name         string
		rec          runs.Record
		wantEligible bool
		wantReason   string
	}{
		{
			name: "eligible: Succeeded + RunLocalTests + age 1d",
			rec: runs.Record{
				Status:    "Succeeded",
				TestLevel: "RunLocalTests",
				CreatedAt: now.Add(-1 * 24 * time.Hour),
			},
			wantEligible: true,
			wantReason:   "",
		},
		{
			name: "eligible: SucceededPartial + RunAllTestsInOrg + age 1d",
			rec: runs.Record{
				Status:    "SucceededPartial",
				TestLevel: "RunAllTestsInOrg",
				CreatedAt: now.Add(-1 * 24 * time.Hour),
			},
			wantEligible: true,
			wantReason:   "",
		},
		{
			name: "ineligible: age 11 days",
			rec: runs.Record{
				Status:    "Succeeded",
				TestLevel: "RunLocalTests",
				CreatedAt: now.Add(-11 * 24 * time.Hour),
			},
			wantEligible: false,
			wantReason:   "older than 10 days",
		},
		{
			name: "ineligible: RunSpecifiedTests not in whitelist",
			rec: runs.Record{
				Status:    "Succeeded",
				TestLevel: "RunSpecifiedTests",
				CreatedAt: now.Add(-1 * 24 * time.Hour),
			},
			wantEligible: false,
			wantReason:   "required tests not run",
		},
		{
			name: "ineligible: TestLevel empty (old run.json)",
			rec: runs.Record{
				Status:    "Succeeded",
				TestLevel: "",
				CreatedAt: now.Add(-1 * 24 * time.Hour),
			},
			wantEligible: false,
			wantReason:   "required tests not run",
		},
		{
			name: "ineligible: Status Failed",
			rec: runs.Record{
				Status:    "Failed",
				TestLevel: "RunLocalTests",
				CreatedAt: now.Add(-1 * 24 * time.Hour),
			},
			wantEligible: false,
			wantReason:   "not a successful validation",
		},
		{
			name: "ineligible: already quick-deployed (correction 2, MANDATORY)",
			rec: runs.Record{
				Status:          "Succeeded",
				TestLevel:       "RunLocalTests",
				CreatedAt:       now.Add(-1 * 24 * time.Hour),
				QuickDeployedAt: now.Add(-1 * time.Hour),
			},
			wantEligible: false,
			wantReason:   "already quick-deployed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotEligible, gotReason := runs.QuickDeployEligible(tt.rec, now)
			if gotEligible != tt.wantEligible {
				t.Errorf("eligible = %v, want %v", gotEligible, tt.wantEligible)
			}
			if gotReason != tt.wantReason {
				t.Errorf("reason = %q, want %q", gotReason, tt.wantReason)
			}
		})
	}
}
