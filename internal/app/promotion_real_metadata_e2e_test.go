package app

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	execpkg "deploydeck/internal/exec"
	"deploydeck/internal/git"
	"deploydeck/internal/prereq"
)

// TestRealMetadataPromotion_AppFlow_E2E drives DeployDeck's internal/app Model
// state machine over the REAL test-e2e-org Salesforce metadata (real
// AccountService.cls + Status__c custom field), proving the full
// PrereqCheck -> TicketInput -> Discovery -> Selection -> TargetSelection ->
// PlanPreview -> BranchCreation -> CherryPicking -> PickVerification path works
// end-to-end on genuine Salesforce metadata (not synthetic 3-line seeds), with
// no org needed. It composes the real git.Service against a temp clone seeded
// from the fixture and asserts the promotion verifies faithful at the scope
// edge.
func TestRealMetadataPromotion_AppFlow_E2E(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test shells out to real git")
	}
	local := setupRealMetadataFlowRepo(t)

	deps := Deps{
		Git:    git.New(execpkg.NewOSRunner()),
		Config: flowConfig(),
		Dir:    local,
	}

	m := New(deps)
	if m.State() != StatePrereqCheck {
		t.Fatalf("start state = %v", m.State())
	}

	// PrereqCheck -> StateMainMenu (HU-018 landing), fed directly like flow_e2e.
	m = advance(t, m, prereqDoneMsg{checks: []prereq.PrereqCheck{{Name: "git", Status: prereq.StatusOK}}})
	if m.State() != StateMainMenu {
		t.Fatalf("after prereq: %v", m.State())
	}

	// Main menu -> full flow: Enter on the default "Promocionar ticket" entry
	// (cursor 0) enters the UNCHANGED promotion flow at StateTicketInput.
	m = advance(t, m, keyPress("enter"))
	if m.State() != StateTicketInput {
		t.Fatalf("Enter on Promocionar should reach StateTicketInput: %v", m.State())
	}

	// TicketInput -> Discovery.
	m = typeString(m, "PROJ-100")
	m = advance(t, m, keyPress("enter"))
	if m.State() != StateCommitDiscovery {
		t.Fatalf("after ticket enter: %v", m.State())
	}

	// Real discovery over origin/UAT..origin/feature/PROJ-100 -> Selection.
	m = advance(t, m, run(t, m.discoverCmd()))
	if m.State() != StateCommitSelection {
		t.Fatalf("after discovery: %v (err=%v)", m.State(), m.Err())
	}
	if len(m.items) != 2 {
		t.Fatalf("discovery should find 2 real-metadata commits, got %d", len(m.items))
	}

	// Confirm selection -> TargetSelection.
	m = advance(t, m, keyPress("enter"))
	if m.State() != StateTargetSelection {
		t.Fatalf("after selection confirm: %v (notice=%q)", m.State(), m.notice)
	}

	// Confirm target (UAT) -> PlanPreview.
	m = advance(t, m, keyPress("enter"))
	if m.State() != StatePlanPreview {
		t.Fatalf("after target confirm: %v (notice=%q)", m.State(), m.notice)
	}
	if m.branchName != "deploy/PROJ-100-to-UAT" {
		t.Fatalf("branch name = %q", m.branchName)
	}

	// Confirm plan -> BranchCreation, then real fetch + branch creation.
	m = advance(t, m, keyPress("enter"))
	if m.State() != StateBranchCreation {
		t.Fatalf("after plan confirm: %v", m.State())
	}
	m = advance(t, m, run(t, m.branchCreateCmd()))
	if m.State() != StateCherryPicking {
		t.Fatalf("after branch creation: %v (err=%v)", m.State(), m.Err())
	}
	if m.Plan().PromotionBranch != "deploy/PROJ-100-to-UAT" {
		t.Fatalf("promotion branch not registered: %q", m.Plan().PromotionBranch)
	}

	// Real cherry-pick of the real metadata (clean) -> pending verify.
	m = advance(t, m, run(t, m.cherryPickCmd()))
	if m.State() != StateCherryPicking {
		t.Fatalf("after cherry-pick, expected to stay in CherryPicking pending verify, got %v (err=%v)", m.State(), m.Err())
	}

	// Real post-pick verification -> PickVerification (scope edge).
	m = advance(t, m, run(t, m.verifyCmd()))
	if m.State() != StatePickVerification {
		t.Fatalf("after verify: %v (err=%v)", m.State(), m.Err())
	}
	if !m.Verification().OK() {
		t.Fatalf("faithful real-metadata promotion should verify OK; warnings: %v", m.Verification().Warnings())
	}
	if !m.DeltaAllowed() {
		t.Fatalf("a clean, non-aborted completion should allow delta downstream")
	}
	if !strings.Contains(m.View(), "origin/UAT") {
		t.Errorf("verification view should name the promotion target")
	}
}

// setupRealMetadataFlowRepo builds a bare remote seeded from the REAL
// test-e2e-org Salesforce fixture with main, UAT, and a feature/PROJ-100 branch
// carrying two ticket commits that edit REAL metadata (AccountService.cls +
// Status__c custom field) so the promotion onto UAT is clean and faithful, then
// returns a local clone whose origin/UAT and origin/feature/PROJ-100 refs are
// real.
func setupRealMetadataFlowRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	remote := filepath.Join(root, "remote.git")
	seed := filepath.Join(root, "seed")
	local := filepath.Join(root, "local")

	gitRun(t, root, "init", "--bare", "-b", "main", remote)
	gitRun(t, root, "init", "-b", "main", seed)
	gitRun(t, seed, "config", "user.name", "ana")
	gitRun(t, seed, "config", "user.email", "ana@example.com")

	copyRealFixtureInto(t, seed)
	gitRun(t, seed, "add", "-A")
	gitRun(t, seed, "commit", "-m", "chore: import real Salesforce metadata fixture (test-e2e-org)")

	gitRun(t, seed, "checkout", "-b", "UAT")
	gitRun(t, seed, "checkout", "-b", "feature/PROJ-100")

	writeFile(t, seed, "force-app/main/default/classes/AccountService.cls", realFeatureAccountServiceCls)
	gitRun(t, seed, "add", "-A")
	gitRun(t, seed, "commit", "-m", "PROJ-100: add inactiveAccounts filter to AccountService")

	writeFile(t, seed, "force-app/main/default/objects/Account/fields/Status__c.field-meta.xml", realFeatureStatusField)
	gitRun(t, seed, "add", "-A")
	gitRun(t, seed, "commit", "-m", "PROJ-100: widen Status__c to 80 chars and relabel")

	gitRun(t, seed, "remote", "add", "origin", remote)
	gitRun(t, seed, "push", "origin", "main", "UAT", "feature/PROJ-100")

	gitRun(t, root, "clone", remote, local)
	gitRun(t, local, "config", "user.name", "ana")
	gitRun(t, local, "config", "user.email", "ana@example.com")

	return local
}

// copyRealFixtureInto copies the real Salesforce metadata (sfdx-project.json,
// manifest/, force-app/) from test-e2e-org into dst, resolved relative to this
// test file.
func copyRealFixtureInto(t *testing.T, dst string) {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot resolve the test file path to locate the test-e2e-org fixture")
	}
	root := filepath.Join(filepath.Dir(thisFile), "..", "..", "test-e2e-org")

	for _, rel := range []string{"sfdx-project.json", "manifest", "force-app"} {
		src := filepath.Join(root, rel)
		info, err := os.Stat(src)
		if err != nil {
			t.Fatalf("stat fixture %s: %v", src, err)
		}
		if !info.IsDir() {
			copyRealFile(t, src, filepath.Join(dst, rel))
			continue
		}
		werr := filepath.WalkDir(src, func(p string, d os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			relPath, rerr := filepath.Rel(src, p)
			if rerr != nil {
				return rerr
			}
			target := filepath.Join(dst, rel, relPath)
			if d.IsDir() {
				return os.MkdirAll(target, 0o755)
			}
			copyRealFile(t, p, target)
			return nil
		})
		if werr != nil {
			t.Fatalf("copy fixture dir %s: %v", src, werr)
		}
	}
}

func copyRealFile(t *testing.T, src, dst string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(dst), err)
	}
	data, err := os.ReadFile(src)
	if err != nil {
		t.Fatalf("read fixture %s: %v", src, err)
	}
	if err := os.WriteFile(dst, data, 0o644); err != nil {
		t.Fatalf("write %s: %v", dst, err)
	}
}

// realFeatureAccountServiceCls is the PROJ-100 source version of the real Apex
// class (adds a second valid filter method).
const realFeatureAccountServiceCls = `public with sharing class AccountService {
    // Devuelve las cuentas activas e inactivas mediante el campo custom Status__c.
    public static List<Account> activeAccounts(List<Account> accounts) {
        List<Account> result = new List<Account>();
        for (Account acc : accounts) {
            if (acc.Status__c == 'Active') {
                result.add(acc);
            }
        }
        return result;
    }

    // PROJ-100: complementary filter for inactive accounts.
    public static List<Account> inactiveAccounts(List<Account> accounts) {
        List<Account> result = new List<Account>();
        for (Account acc : accounts) {
            if (acc.Status__c != 'Active') {
                result.add(acc);
            }
        }
        return result;
    }
}
`

// realFeatureStatusField is the PROJ-100 source version of the real
// custom-field metadata (widens to 80 chars, relabels).
const realFeatureStatusField = `<?xml version="1.0" encoding="UTF-8"?>
<CustomField xmlns="http://soap.sforce.com/2006/04/metadata">
    <fullName>Status__c</fullName>
    <label>Account Status</label>
    <type>Text</type>
    <length>80</length>
    <required>false</required>
    <trackHistory>false</trackHistory>
</CustomField>
`
