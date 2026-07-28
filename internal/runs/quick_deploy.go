package runs

import "time"

// quickDeployWindow is Salesforce's quick-deploy eligibility window: a
// validated run can only be quick-deployed within 10 days of its original
// validation (quick-deploy spec: "Eligibility Detection For Quick Deploy").
const quickDeployWindow = 10 * 24 * time.Hour

// requiredTestsRanWhitelist is the positive whitelist of TestLevel values
// that satisfy a production deploy's test-coverage gate (design.md ADR-1).
// Anything NOT in this whitelist — including "", "RunSpecifiedTests", and
// "NoTestRun" — is treated as required-tests-NOT-ran, fail-safe.
var requiredTestsRanWhitelist = map[string]bool{
	"RunLocalTests":    true,
	"RunAllTestsInOrg": true,
}

// requiredTestsRan reports whether testLevel is one of the whitelisted
// values that DeployDeck can trust to satisfy the required-test gate.
func requiredTestsRan(testLevel string) bool {
	return requiredTestsRanWhitelist[testLevel]
}

// QuickDeployEligible reports whether rec is eligible for a Salesforce quick
// deploy as of now, mirroring internal/app's isRePromoteEligible idiom: a
// pure, exported predicate over a Record (quick-deploy spec: "Eligibility
// Detection For Quick Deploy"). Eligible if and only if ALL of:
//   - rec.Status is "Succeeded" or "SucceededPartial"
//   - rec.TestLevel indicates the required tests ran (requiredTestsRan)
//   - rec's age (now - rec.CreatedAt) is under Salesforce's 10-day window
//   - rec has never already been quick-deployed (rec.QuickDeployedAt is zero)
//
// reason explains the first failing predicate when eligible is false, one
// of "not a successful validation", "required tests not run", "older than
// 10 days", or "already quick-deployed". reason is "" when eligible is true.
func QuickDeployEligible(rec Record, now time.Time) (eligible bool, reason string) {
	if rec.Status != "Succeeded" && rec.Status != "SucceededPartial" {
		return false, "not a successful validation"
	}
	if !requiredTestsRan(rec.TestLevel) {
		return false, "required tests not run"
	}
	if now.Sub(rec.CreatedAt) >= quickDeployWindow {
		return false, "older than 10 days"
	}
	if !rec.QuickDeployedAt.IsZero() {
		return false, "already quick-deployed"
	}
	return true, ""
}
