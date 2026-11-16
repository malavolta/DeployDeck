package salesforce_test

import (
	"context"
	"os"
	"reflect"
	"testing"

	"github.com/malavolta/DeployDeck/internal/exec"
	"github.com/malavolta/DeployDeck/internal/salesforce"
)

const parseEnvelope = `{"status":1,"result":{"status":"Failed","numberComponentErrors":1,
"details":{"componentFailures":[{"fullName":"A","componentType":"ApexClass","problem":"bad","problemType":"Error"}],"runTestResult":{}}}}`

func TestParseDeployReport_Tolerance(t *testing.T) {
	cases := []struct {
		name    string
		raw     string
		wantErr bool
		status  string
	}{
		{"clean envelope", parseEnvelope, false, "Failed"},
		{"trailing CLI text", parseEnvelope + "\n\n Warning: update available\n", false, "Failed"},
		{"leading noise before first brace", "npm notice something\n" + parseEnvelope, false, "Failed"},
		{"empty status is an error", `{"status":1,"result":{"status":""}}`, true, ""},
		{"error envelope without status", `{"status":1,"name":"X","message":"boom"}`, true, ""},
		{"garbage", "not json at all", true, ""},
		{"empty", "", true, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := salesforce.ParseDeployReport([]byte(tc.raw))
			if (err != nil) != tc.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tc.wantErr)
			}
			if got.Status != tc.status {
				t.Errorf("Status = %q, want %q", got.Status, tc.status)
			}
		})
	}
}

func TestParseDeployReport_MappingParityWithReportDeploy(t *testing.T) {
	raw := parseEnvelope + "\n trailing\n"
	fr := exec.NewFakeRunner()
	fr.When("sf", []string{"project", "deploy", "report", "--job-id", "J", "--target-org", "O", "--json"},
		exec.CommandResult{Stdout: []byte(parseEnvelope)})
	want, err := salesforce.New(fr).ReportDeploy(context.Background(), "J", "O", "/repo")
	if err != nil {
		t.Fatal(err)
	}
	got, err := salesforce.ParseDeployReport([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	want.Raw, got.Raw = "", ""
	if !reflect.DeepEqual(got, want) {
		t.Errorf("mapping differs\n got: %+v\nwant: %+v", got, want)
	}
}

func TestParseDeployReport_RealCoverageFixture(t *testing.T) {
	raw, err := os.ReadFile("testdata/report_coverage_trailing.json")
	if err != nil {
		t.Fatal(err)
	}
	got, err := salesforce.ParseDeployReport(raw)
	if err != nil {
		t.Fatalf("fixture with trailing CLI text must parse: %v", err)
	}
	if got.Status != "Failed" || len(got.CodeCoverageWarnings) != 1 || got.CodeCoverageWarnings[0].Name != "" {
		t.Errorf("unexpected report: %+v", got)
	}
	if len(got.CodeCoverage) != 6 {
		t.Errorf("want 6 coverage rows, got %d", len(got.CodeCoverage))
	}
}
