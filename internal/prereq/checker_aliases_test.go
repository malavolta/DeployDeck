package prereq_test

import (
	"context"
	"strings"
	"testing"

	"deploydeck/internal/config"
	"deploydeck/internal/exec"
	"deploydeck/internal/prereq"
	"deploydeck/internal/salesforce"
)

func TestChecker_CheckAliases_MissingAlias_BlocksThatSandbox(t *testing.T) {
	fr := exec.NewFakeRunner()
	fr.When("sf", []string{"org", "list", "--json"}, exec.CommandResult{
		ExitCode: 0,
		Stdout: []byte(`{"status":0,"result":{
			"sandboxes":[{"alias":"uat","username":"a@b.com"}]
		}}`),
	})

	cfg := config.Config{
		Sandboxes: map[string]config.SandboxConfig{
			"UAT":         {Alias: "uat", TestLevel: "RunLocalTests"},
			"Integration": {Alias: "int-missing", TestLevel: "RunLocalTests"},
		},
	}
	checker := &prereq.Checker{Git: nil, SF: salesforce.New(fr), Config: cfg}

	checks, err := checker.CheckAliases(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(checks) != 2 {
		t.Fatalf("expected 2 alias checks, got %d: %+v", len(checks), checks)
	}

	uatCheck := findCheck(t, checks, "sandbox alias (UAT)")
	if uatCheck.Status != prereq.StatusOK {
		t.Fatalf("expected the present alias to be OK, got %+v", uatCheck)
	}

	intCheck := findCheck(t, checks, "sandbox alias (Integration)")
	if intCheck.Status != prereq.StatusBlocking {
		t.Fatalf("expected the missing alias to block that sandbox, got %+v", intCheck)
	}
	if !strings.Contains(intCheck.Detail, "int-missing") {
		t.Fatalf("expected detail to name the missing alias, got %q", intCheck.Detail)
	}
}

func TestChecker_CheckAliases_AliasFoundInNonDefaultCategory_OK(t *testing.T) {
	// Triangulation: the alias lives in devHubs, not sandboxes/nonScratchOrgs
	// — proves the five-category scan is really composed end-to-end, not
	// just checking a default bucket.
	fr := exec.NewFakeRunner()
	fr.When("sf", []string{"org", "list", "--json"}, exec.CommandResult{
		ExitCode: 0,
		Stdout: []byte(`{"status":0,"result":{
			"devHubs":[{"alias":"my-devhub","username":"a@b.com"}]
		}}`),
	})

	cfg := config.Config{
		Sandboxes: map[string]config.SandboxConfig{
			"DevHub": {Alias: "my-devhub", TestLevel: "NoTestRun"},
		},
	}
	checker := &prereq.Checker{SF: salesforce.New(fr), Config: cfg}

	checks, err := checker.CheckAliases(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	check := findCheck(t, checks, "sandbox alias (DevHub)")
	if check.Status != prereq.StatusOK {
		t.Fatalf("expected an alias found in devHubs to be OK, got %+v", check)
	}
}
