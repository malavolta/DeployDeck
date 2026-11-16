package app

// Golden pins for validationBody (run-failure-detail task 1.0.2). Taken BEFORE the
// reportSections extraction: the extraction must keep this output byte-identical.

import (
	"strings"
	"testing"

	"github.com/malavolta/DeployDeck/internal/salesforce"
)

func goldenModels() map[string]Model {
	out := map[string]Model{}
	full := Model{state: StateFailed, jobID: "0Af1", reportPath: "/r/report-002.json"}
	full.report = salesforce.DeployReport{
		Status: "Failed", NumberComponentsTotal: 5, NumberComponentsDeployed: 3, NumberComponentErrors: 2,
		NumberTestsTotal: 4, NumberTestsCompleted: 4, NumberTestErrors: 1,
		ComponentFailures: []salesforce.ComponentFailure{
			{Component: "A", Type: "ApexClass", Message: "boom\x1b[31m", FileName: "classes/A.cls", LineNumber: 3, ColumnNumber: 2, ProblemType: "Error"},
			{Component: "B", Type: "ApexTrigger", Message: "warn", ProblemType: "Warning"},
		},
		TestFailures:         []salesforce.TestFailure{{Class: "T", Method: "m", Message: "assert", StackTrace: "Class.T.m: line 1\nClass.T.x: line 2"}},
		CodeCoverageWarnings: []salesforce.CodeCoverageWarning{{Message: "Average test coverage across all Apex Classes and Triggers is 66%, at least 75% required."}},
		CodeCoverage:         []salesforce.CodeCoverageResult{{Name: "Z", NumLocations: 10, NumLocationsNotCovered: 10}},
		FlowCoverageWarnings: []salesforce.FlowCoverageWarning{{FlowName: "F", Message: "low"}},
		ErrorMessage:         "org down", ErrorStatusCode: "UNKNOWN",
	}
	out["failed-full"] = full
	ok := Model{state: StateSucceeded, jobID: "0Af2", report: salesforce.DeployReport{Status: "Succeeded", NumberComponentsTotal: 2, NumberComponentsDeployed: 2, NumberTestsTotal: 1, NumberTestsCompleted: 1}}
	out["succeeded"] = ok
	out["failed-fallback"] = Model{state: StateFailed, report: salesforce.DeployReport{Status: "Failed"}}
	out["canceled"] = Model{state: StateFailed, report: salesforce.DeployReport{Status: "Canceled", CanceledByName: "ana"}, reportPath: "/r/report-001.json"}
	out["polling"] = Model{state: StateValidationPolling, jobID: "0Af3", reportPath: "/r/x", report: salesforce.DeployReport{Status: "InProgress", NumberComponentsTotal: 9, NumberComponentsDeployed: 1}}
	return out
}

var validationBodyGolden = map[string]string{
	"canceled":        "  Estado: Canceled\n  Componentes: 0/0 (errores: 0)\n  Tests: 0/0 (errores: 0)\n\n  Cancelado por: ana\n\n  Reporte completo: /r/report-001.json\n",
	"failed-fallback": "  Estado: Failed\n  Componentes: 0/0 (errores: 0)\n  Tests: 0/0 (errores: 0)\n\n  [XX] La validación falló sin detalle estructurado.\n",
	"failed-full":     "\n  Job: 0Af1\n  Estado: Failed\n  Componentes: 3/5 (errores: 2)\n  Tests: 4/4 (errores: 1)\n\n  Errores de metadata:\n  [XX] A (ApexClass): boom[31m classes/A.cls:3:2\n  [!!] B (ApexTrigger): warn\n\n  Tests fallidos:\n  - T.m: assert\n    Class.T.m: line 1\n\n  Cobertura de código:\n  - [cobertura global] Average test coverage across all Apex Classes and Triggers is 66%, at least 75% required.\n  - Z: 0%\n  - flujo: F: low\n\n  [XX] Error (UNKNOWN): org down\n\n  Reporte completo: /r/report-002.json\n",
	"polling":         "\n  Job: 0Af3\n  Estado: InProgress\n  Componentes: 1/9 (errores: 0)\n  Tests: 0/0 (errores: 0)\n",
	"succeeded":       "\n  Job: 0Af2\n  Estado: Succeeded\n  Componentes: 2/2 (errores: 0)\n  Tests: 1/1 (errores: 0)\n",
}

func TestValidationBody_GoldenPinned(t *testing.T) {
	for name, m := range goldenModels() {
		want, ok := validationBodyGolden[name]
		if !ok {
			t.Fatalf("no golden for %q", name)
		}
		if got := m.validationBody(); got != want {
			t.Errorf("%s: validationBody drifted\n got: %q\nwant: %q", name, got, want)
		}
	}
}

// --- run-failure-detail: result-screen headline (task 1.5.1) ----------------

func resultModel(state State, r salesforce.DeployReport) Model {
	m := New(Deps{})
	m.state = state
	m.report = r
	return m
}

func TestResultScreen_HeadlineAboveCountersOnFailedAndCanceled(t *testing.T) {
	for _, tc := range []struct {
		name string
		r    salesforce.DeployReport
		want string
	}{
		{"failed coverage", coverageReport(), motivoCoverage},
		{"canceled", salesforce.DeployReport{Status: "Canceled", CanceledByName: "ana"}, "Cancelado por ana"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v := resultModel(StateFailed, tc.r).viewValidationResult()
			h, c := strings.Index(v, tc.want), strings.Index(v, "Componentes:")
			if h < 0 || c < 0 || h > c {
				t.Errorf("headline must precede counters (h=%d c=%d):\n%s", h, c, v)
			}
		})
	}
}

func TestResultScreen_NoHeadlineWhileNotTerminalOrSucceeded(t *testing.T) {
	for _, status := range []string{"Queued", "InProgress", "Pending", "Succeeded", "SucceededPartial"} {
		r := coverageReport()
		r.Status = status
		state := StateSucceeded
		if status == "Queued" || status == "InProgress" || status == "Pending" {
			state = StateFailed // timeout lands on StateFailed with a non-terminal last report
		}
		v := resultModel(state, r).viewValidationResult()
		if strings.Contains(v, "Falló por") {
			t.Errorf("%s must not show a failure headline:\n%s", status, v)
		}
	}
}

func TestResultScreen_TimeoutKeepsLineAndAddsNoHeadline(t *testing.T) {
	timeout := "Timeout: la validación no alcanzó un estado terminal a tiempo."
	// A timeout only happens while the last report is non-terminal, so the
	// classifier yields no failure headline; only the timeout line shows.
	m := resultModel(StateFailed, salesforce.DeployReport{Status: "InProgress"})
	m.timedOut = true
	v := m.viewValidationResult()
	if !strings.Contains(v, timeout) || strings.Contains(v, "Falló por") {
		t.Errorf("want timeout line and no headline:\n%s", v)
	}

	none := resultModel(StateFailed, salesforce.DeployReport{})
	none.timedOut = true
	v = none.viewValidationResult()
	if !strings.Contains(v, timeout) || strings.Contains(v, "Falló") || strings.Contains(v, "En curso") {
		t.Errorf("timeout without report: only the timeout line:\n%s", v)
	}
}
