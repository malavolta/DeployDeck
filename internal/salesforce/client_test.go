package salesforce_test

import (
	"context"
	"strings"
	"testing"

	"github.com/malavolta/DeployDeck/internal/exec"
	"github.com/malavolta/DeployDeck/internal/salesforce"
)

func TestClient_Version_ParsesCLIVersionFromPlainTextOutput(t *testing.T) {
	fr := exec.NewFakeRunner()
	fr.When("sf", []string{"--version"}, exec.CommandResult{
		ExitCode: 0,
		Stdout:   []byte("@salesforce/cli/2.63.6 darwin-arm64 node-v22.11.0\n"),
	})
	client := salesforce.New(fr)

	got, err := client.Version(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.CLIVersion != "2.63.6" {
		t.Fatalf("expected CLIVersion %q, got %q", "2.63.6", got.CLIVersion)
	}
	if !strings.Contains(got.Raw, "darwin-arm64") {
		t.Fatalf("expected Raw to preserve full output, got %q", got.Raw)
	}
}

func TestClient_Version_DifferentVersion(t *testing.T) {
	// Triangulation: a different CLI version/platform must parse to a
	// different CLIVersion, proving parseVersionOutput is real regex
	// extraction, not a hardcoded return.
	fr := exec.NewFakeRunner()
	fr.When("sf", []string{"--version"}, exec.CommandResult{
		ExitCode: 0,
		Stdout:   []byte("@salesforce/cli/1.2.3 linux-x64 node-v18.0.0\n"),
	})
	client := salesforce.New(fr)

	got, err := client.Version(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.CLIVersion != "1.2.3" {
		t.Fatalf("expected CLIVersion %q, got %q", "1.2.3", got.CLIVersion)
	}
}

func TestClient_Plugins_ParsesTopLevelArray(t *testing.T) {
	// `sf plugins --json` returns a top-level JSON ARRAY of oclif plugin
	// objects, NOT the `{"status":0,"result":...}` envelope other sf
	// commands (e.g. `sf org list --json`) use. Verified against the real
	// sf CLI (2.135.7).
	fr := exec.NewFakeRunner()
	fr.When("sf", []string{"plugins", "--json"}, exec.CommandResult{
		ExitCode: 0,
		Stdout: []byte(`[` +
			`{"name":"@salesforce/cli","version":"2.135.7","children":[]},` +
			`{"name":"other-plugin","version":"1.0.0","children":[]}` +
			`]`),
	})
	client := salesforce.New(fr)

	got, err := client.Plugins(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 plugins, got %d", len(got))
	}
	if got[0].Name != "@salesforce/cli" || got[0].Version != "2.135.7" {
		t.Fatalf("unexpected first plugin: %+v", got[0])
	}
}

func TestClient_Plugins_FlattensNestedChildren(t *testing.T) {
	// Triangulation: a plugin nested inside another plugin's "children"
	// array (how sfdx-git-delta can appear in real output) must still
	// surface as a flat Plugin entry, proving Plugins() recursively
	// flattens rather than only reading top-level entries.
	fr := exec.NewFakeRunner()
	fr.When("sf", []string{"plugins", "--json"}, exec.CommandResult{
		ExitCode: 0,
		Stdout: []byte(`[` +
			`{"name":"@salesforce/plugin-org","version":"5.0.10","children":[` +
			`{"name":"sfdx-git-delta","version":"5.35.0","children":[]}` +
			`]}` +
			`]`),
	})
	client := salesforce.New(fr)

	got, err := client.Plugins(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected parent + flattened nested child, got %d: %+v", len(got), got)
	}
	found := false
	for _, p := range got {
		if p.Name == "sfdx-git-delta" && p.Version == "5.35.0" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected nested sfdx-git-delta child to be flattened into results, got %+v", got)
	}
}

func TestClient_Plugins_EmptyListWhenNonePresent(t *testing.T) {
	// Triangulation: different setup (empty top-level array) must produce a
	// different, non-trivial outcome (0 items), proving the empty result
	// comes from real parsing of an empty JSON array, not a fixed stub.
	fr := exec.NewFakeRunner()
	fr.When("sf", []string{"plugins", "--json"}, exec.CommandResult{
		ExitCode: 0,
		Stdout:   []byte(`[]`),
	})
	client := salesforce.New(fr)

	got, err := client.Plugins(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected empty plugin list, got %d", len(got))
	}
}

func TestClient_Plugins_MalformedJSONErrors(t *testing.T) {
	fr := exec.NewFakeRunner()
	fr.When("sf", []string{"plugins", "--json"}, exec.CommandResult{
		ExitCode: 0,
		Stdout:   []byte(`not json`),
	})
	client := salesforce.New(fr)

	_, err := client.Plugins(context.Background())
	if err == nil {
		t.Fatal("expected an error for malformed JSON, got nil")
	}
}

func TestClient_Orgs_ParsesAllFiveCategories(t *testing.T) {
	fr := exec.NewFakeRunner()
	fr.When("sf", []string{"org", "list", "--json"}, exec.CommandResult{
		ExitCode: 0,
		Stdout: []byte(`{"status":0,"result":{` +
			`"nonScratchOrgs":[{"alias":"prod","username":"a@b.com","connectedStatus":"Connected"}],` +
			`"scratchOrgs":[{"alias":"scratch1","username":"c@d.com","connectedStatus":"Active"}],` +
			`"sandboxes":[{"alias":"uat","username":"e@f.com","connectedStatus":"Connected"}],` +
			`"devHubs":[{"alias":"devhub","username":"g@h.com","connectedStatus":"Connected"}],` +
			`"other":[{"alias":"weird","username":"i@j.com","connectedStatus":"Unknown"}]` +
			`}}`),
	})
	client := salesforce.New(fr)

	got, err := client.Orgs(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got.NonScratch) != 1 || got.NonScratch[0].Alias != "prod" {
		t.Fatalf("unexpected NonScratch: %+v", got.NonScratch)
	}
	if len(got.Scratch) != 1 || got.Scratch[0].Alias != "scratch1" {
		t.Fatalf("unexpected Scratch: %+v", got.Scratch)
	}
	if len(got.Sandboxes) != 1 || got.Sandboxes[0].Alias != "uat" {
		t.Fatalf("unexpected Sandboxes: %+v", got.Sandboxes)
	}
	if len(got.DevHubs) != 1 || got.DevHubs[0].Alias != "devhub" {
		t.Fatalf("unexpected DevHubs: %+v", got.DevHubs)
	}
	if len(got.Other) != 1 || got.Other[0].Alias != "weird" {
		t.Fatalf("unexpected Other: %+v", got.Other)
	}
}

func TestClient_Orgs_MalformedJSONErrors(t *testing.T) {
	fr := exec.NewFakeRunner()
	fr.When("sf", []string{"org", "list", "--json"}, exec.CommandResult{
		ExitCode: 0,
		Stdout:   []byte(`{"status":0,"result":`),
	})
	client := salesforce.New(fr)

	_, err := client.Orgs(context.Background())
	if err == nil {
		t.Fatal("expected an error for malformed JSON, got nil")
	}
}

func TestOrgList_FindByAlias_SearchesAllFiveCategories(t *testing.T) {
	list := salesforce.OrgList{
		Sandboxes: []salesforce.Org{{Alias: "uat"}},
		DevHubs:   []salesforce.Org{{Alias: "devhub"}},
	}

	org, ok := list.FindByAlias("devhub")
	if !ok || org.Alias != "devhub" {
		t.Fatalf("expected to find alias %q in DevHubs, got ok=%v org=%+v", "devhub", ok, org)
	}

	_, ok = list.FindByAlias("missing")
	if ok {
		t.Fatal("expected alias search to fail for a non-existent alias")
	}
}
