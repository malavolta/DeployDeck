package app

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/malavolta/DeployDeck/internal/salesforce"
)

const orgCoverageMsg = "Average test coverage across all Apex Classes and Triggers is 66%, at least 75% test coverage is required."

func coverageReport() salesforce.DeployReport {
	return salesforce.DeployReport{
		Status:               "Failed",
		CodeCoverageWarnings: []salesforce.CodeCoverageWarning{{Message: orgCoverageMsg}},
		CodeCoverage: []salesforce.CodeCoverageResult{
			{Name: "NebulaDemoQueueable", NumLocations: 9, NumLocationsNotCovered: 9},
			{Name: "Covered", NumLocations: 10, NumLocationsNotCovered: 0},
			{Name: "NebulaDemoService", NumLocations: 23, NumLocationsNotCovered: 23},
			{Name: "NoLocations", NumLocations: 0},
		},
	}
}

func TestClassify_PriorityTable(t *testing.T) {
	meta := []salesforce.ComponentFailure{{Component: "A", Type: "ApexClass", Message: "bad", ProblemType: "Error"}}
	tests := []salesforce.TestFailure{{Class: "T", Method: "m", Message: "x"}}
	cov := []salesforce.CodeCoverageWarning{{Message: orgCoverageMsg}}
	cases := []struct {
		name string
		r    salesforce.DeployReport
		kind failureKind
		want string
	}{
		{"canceled outranks everything", salesforce.DeployReport{Status: "Canceled", CanceledByName: "ana", TestFailures: tests}, causeCanceled, "Cancelado por ana"},
		{"canceled without name", salesforce.DeployReport{Status: "Canceled"}, causeCanceled, "Cancelado"},
		{"metadata outranks tests", salesforce.DeployReport{Status: "Failed", ComponentFailures: meta, TestFailures: tests, CodeCoverageWarnings: cov}, causeMetadata, "Falló por errores de metadata"},
		{"metadata via component error count", salesforce.DeployReport{Status: "Failed", NumberComponentErrors: 2, TestFailures: tests}, causeMetadata, "Falló por errores de metadata"},
		{"warning-only component failure is not metadata", salesforce.DeployReport{Status: "Failed", ComponentFailures: []salesforce.ComponentFailure{{ProblemType: "Warning"}}, TestFailures: tests}, causeTests, "Falló por tests"},
		{"tests outrank coverage", salesforce.DeployReport{Status: "Failed", TestFailures: tests, CodeCoverageWarnings: cov}, causeTests, "Falló por tests"},
		{"tests via error counter", salesforce.DeployReport{Status: "Failed", NumberTestErrors: 3}, causeTests, "Falló por tests: 3"},
		{"coverage outranks org error", salesforce.DeployReport{Status: "Failed", CodeCoverageWarnings: cov, ErrorMessage: "x"}, causeCoverage, "Falló por cobertura: 66% global < 75%"},
		{"org error", salesforce.DeployReport{Status: "Failed", ErrorMessage: "UNABLE_TO_LOCK_ROW", ErrorStatusCode: "X"}, causeOrgError, "Falló: UNABLE_TO_LOCK_ROW"},
		{"generic", salesforce.DeployReport{Status: "Failed"}, causeGeneric, "La validación falló sin detalle estructurado"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			v := classify(tc.r)
			if v.kind != tc.kind {
				t.Fatalf("kind = %v, want %v", v.kind, tc.kind)
			}
			if !v.isFailure() {
				t.Error("must be a failure verdict")
			}
			if !strings.Contains(v.headline, mark("XX")) || !strings.Contains(v.headline, tc.want) {
				t.Errorf("headline = %q, want mark+%q", v.headline, tc.want)
			}
		})
	}
}

func TestClassify_NonFailureStatuses(t *testing.T) {
	cases := []struct {
		status, want string
		kind         failureKind
	}{
		{"Succeeded", mark("OK") + " Validación exitosa", causeNone},
		{"SucceededPartial", mark("!!") + " Resultado parcial", causeNone},
		{"Queued", mark("i") + " En curso", causeNone},
		{"InProgress", mark("i") + " En curso", causeNone},
		{"Pending", mark("i") + " En curso", causeNone},
	}
	for _, tc := range cases {
		// Even with failure-looking data, a non-failed status never yields a failure headline.
		r := salesforce.DeployReport{Status: tc.status, TestFailures: []salesforce.TestFailure{{Class: "T"}}}
		v := classify(r)
		if v.kind != tc.kind || v.isFailure() || v.headline != tc.want {
			t.Errorf("%s: got %+v, want headline %q", tc.status, v, tc.want)
		}
	}
}

func TestClassify_CoverageHeadlineAndHint(t *testing.T) {
	v := classify(coverageReport())
	if v.kind != causeCoverage {
		t.Fatalf("kind = %v", v.kind)
	}
	if v.headline != mark("XX")+" Falló por cobertura: 66% global < 75%" {
		t.Errorf("headline = %q", v.headline)
	}
	for _, want := range []string{"incluye todas las clases Apex de la org", "NebulaDemoQueueable", "NebulaDemoService"} {
		if !strings.Contains(v.hint, want) {
			t.Errorf("hint missing %q: %q", want, v.hint)
		}
	}
	if strings.Contains(v.hint, "Covered") || strings.Contains(v.hint, "NoLocations") {
		t.Errorf("hint must list only 0%% classes with executable locations: %q", v.hint)
	}
}

func TestClassify_CoverageFallbackWhenUnparseable(t *testing.T) {
	r := coverageReport()
	r.CodeCoverageWarnings = []salesforce.CodeCoverageWarning{{Message: "coverage is bad"}}
	v := classify(r)
	if v.headline != mark("XX")+" Falló por cobertura de código" {
		t.Errorf("headline = %q", v.headline)
	}
}

func TestClassify_CoverageZeroListCappedAtFive(t *testing.T) {
	r := coverageReport()
	r.CodeCoverage = nil
	for i := 0; i < 8; i++ {
		r.CodeCoverage = append(r.CodeCoverage, salesforce.CodeCoverageResult{Name: fmt.Sprintf("Cls%d", i), NumLocations: 4, NumLocationsNotCovered: 4})
	}
	hint := classify(r).hint
	if !strings.Contains(hint, "Cls4") || strings.Contains(hint, "Cls5") || !strings.Contains(hint, "+3 más") {
		t.Errorf("want 5 names then +3 más, got %q", hint)
	}
}

func TestClassify_SanitizesOrgText(t *testing.T) {
	r := salesforce.DeployReport{Status: "Failed", ErrorMessage: "boom\x1b[31m\nsecond"}
	v := classify(r)
	if strings.ContainsAny(v.headline, "\x1b\n") || !strings.Contains(v.headline, "boom") {
		t.Errorf("headline not sanitized: %q", v.headline)
	}
	r = coverageReport()
	r.CodeCoverage[0].Name = "Evil\x1b[2JClass"
	if h := classify(r).hint; strings.ContainsAny(h, "\x1b\n") {
		t.Errorf("hint not sanitized: %q", h)
	}
}

// Real persisted report: 66% org coverage, trailing CLI warning text.
func TestClassify_RealCoverageFixture(t *testing.T) {
	raw, err := os.ReadFile("testdata/report_coverage_trailing.json")
	if err != nil {
		t.Fatal(err)
	}
	r, err := salesforce.ParseDeployReport(raw)
	if err != nil {
		t.Fatal(err)
	}
	v := classify(r)
	if !strings.Contains(v.headline, "Falló por cobertura: 66% global < 75%") {
		t.Errorf("headline = %q", v.headline)
	}
	if !strings.Contains(v.hint, "NebulaDemoQueueable") || !strings.Contains(v.hint, "NebulaDemoService") {
		t.Errorf("hint = %q", v.hint)
	}
}
