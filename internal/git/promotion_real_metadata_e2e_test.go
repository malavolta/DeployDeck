package git_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/malavolta/DeployDeck/internal/exec"
	"github.com/malavolta/DeployDeck/internal/git"
)

// TestRealMetadataPromotion_E2E is a HIGH-REALISM promotion end-to-end test
// over the ACTUAL Salesforce metadata fixture in test-e2e-org (real
// AccountService.cls / AccountServiceTest.cls / Status__c custom field, real
// -meta.xml, real package.xml + sfdx-project.json) — a deliberate realism
// upgrade over the synthetic 3-line seeds the other HU-006 e2e uses. It drives
// DeployDeck's REAL promotion engine (git.Service.Discover -> selection ->
// CreatePromotionBranch -> CherryPick -> conflict classification/resolution ->
// gated ContinueCherryPick -> VerifyPromotedContent) end-to-end on real git,
// with NO org needed.
//
//	Part A (always runs; real git + real Salesforce metadata, no org): a clean
//	faithful promotion and a REAL conflict on a REAL .cls file (both branches
//	edit AccountService.cls) that is detected, classified as a text conflict on
//	the real Salesforce path, resolved via the engine's gated-continue path, and
//	verified faithful.
//
//	Part B (opt-in, local only, gated by DEPLOYDECK_E2E_ORG): a NON-DESTRUCTIVE
//	`sf project deploy start --dry-run` of the promoted tree against the
//	connected org, proving the metadata DeployDeck promoted is genuinely
//	deployable. This step runs `sf` DIRECTLY (not through DeployDeck) because
//	deploy/validate is intentionally OUT of DeployDeck's current slice scope —
//	it is the e2e that proves the promotion's OUTPUT is real-deployable.
func TestRealMetadataPromotion_E2E(t *testing.T) {
	// Part A — clean faithful promotion of real Salesforce metadata onto UAT.
	t.Run("PartA clean faithful promotion over real Salesforce metadata", func(t *testing.T) {
		runner := exec.NewOSRunner()
		svc := git.New(runner)

		p := driveRealMetadataPromotion(t, runner, svc, false)

		// A clean multi-commit promotion completed with no cherry-pick in
		// progress and downstream (delta + validation) unblocked.
		if p.outcome.State.InProgress {
			t.Fatalf("expected a clean promotion to complete, got %+v", p.outcome.State)
		}
		if !git.DeltaAndValidationAllowed(p.outcome.State, false) {
			t.Fatalf("a clean completed promotion must allow delta + validation")
		}

		// The promotion branch carries EXACTLY the two selected metadata
		// changes (the real .cls and the real field-meta.xml) and nothing else.
		assertPromotedFileSet(t, runner, p.dir, realClsPath, realFieldPath)

		// Verification reports the promotion faithful: every selected file
		// matches the intended source content and no spurious file was dragged
		// onto the branch.
		verify, err := svc.VerifyPromotedContent(context.Background(), p.dir, "origin/UAT", p.selectedTip, []string{realClsPath, realFieldPath})
		if err != nil {
			t.Fatalf("VerifyPromotedContent error: %v", err)
		}
		if !verify.OK() {
			t.Fatalf("expected a faithful promotion; partial=%v spurious=%v", verify.PartialFiles, verify.SpuriousFiles)
		}

		// The promoted metadata is genuinely the feature content (real assertion
		// on the real file, not just a name check).
		got := readRepoFile(t, p.dir, realClsPath)
		if !bytes.Contains(got, []byte("inactiveAccounts")) {
			t.Fatalf("promoted AccountService.cls should carry the PROJ-100 inactiveAccounts method, got:\n%s", got)
		}
	})

	// Part A — REAL conflict on a REAL Salesforce metadata file.
	t.Run("PartA real conflict on AccountService.cls classified, resolved, continued, verified", func(t *testing.T) {
		runner := exec.NewOSRunner()
		svc := git.New(runner)

		dir := newTempRepo(t)
		_, feature2 := seedRealMetadataRepo(t, runner, dir, true /* diverge UAT on AccountService.cls */)

		result, selected := discoverAndSelectRealMetadata(t, svc, dir)

		if err := svc.CreatePromotionBranch(context.Background(), dir, "UAT", "deploy/PROJ-100-to-UAT"); err != nil {
			t.Fatalf("CreatePromotionBranch error: %v", err)
		}

		contiguous := git.IsContiguousSelection(result.OrderedCommits, selected)
		outcome, err := svc.CherryPick(context.Background(), dir, selected, contiguous)
		if err != nil {
			t.Fatalf("CherryPick error: %v", err)
		}

		// The cherry-pick genuinely STOPPED on a real conflict, classified as a
		// TEXT conflict on the REAL Salesforce .cls path (real porcelain UU).
		if !outcome.State.InProgress {
			t.Fatalf("expected the cherry-pick to stop on a real conflict, got %+v", outcome.State)
		}
		if len(outcome.State.Unmerged) != 1 {
			t.Fatalf("expected exactly one conflicted file, got %+v", outcome.State.Unmerged)
		}
		conflict := outcome.State.Unmerged[0]
		if conflict.Path != realClsPath {
			t.Fatalf("expected the conflict on the real Salesforce class %q, got %q", realClsPath, conflict.Path)
		}
		if conflict.Kind != git.ConflictText {
			t.Fatalf("expected a TEXT conflict on the real .cls, got %v", conflict.Kind)
		}

		// While the conflict is unresolved, the continue-gate is closed and
		// downstream delta/validation is blocked.
		if git.EvaluateContinueGate(outcome.State, nil).Enabled {
			t.Fatalf("continue must be disabled while the real .cls conflict is unresolved")
		}
		if git.DeltaAndValidationAllowed(outcome.State, false) {
			t.Fatalf("an in-progress cherry-pick must block delta + validation")
		}

		// Resolve the real metadata conflict by promoting the selected (feature)
		// version of the class — the intended promoted content — then stage it.
		// This is exactly the engine's text-conflict resolution path (edit the
		// file, `git add`, gated continue), reconciled from the repo on re-read.
		writeFileHelper(t, dir, realClsPath, featureAccountServiceCls)
		runGit(t, runner, dir, "add", realClsPath)

		reread, err := svc.RepoState(context.Background(), dir)
		if err != nil {
			t.Fatalf("RepoState error: %v", err)
		}
		if len(reread.Unmerged) != 0 {
			t.Fatalf("expected the resolution reflected on re-read, still unmerged: %+v", reread.Unmerged)
		}
		if !git.EvaluateContinueGate(reread, nil).Enabled {
			t.Fatalf("continue should be enabled once the real .cls conflict is resolved and staged")
		}

		done, err := svc.ContinueCherryPick(context.Background(), dir)
		if err != nil {
			t.Fatalf("ContinueCherryPick error: %v", err)
		}
		if done.State.InProgress {
			t.Fatalf("expected the sequence to complete after the gated continue, got %+v", done.State)
		}

		// The promotion branch carries EXACTLY the two selected metadata changes.
		assertPromotedFileSet(t, runner, dir, realClsPath, realFieldPath)

		// Verification reports the resolved promotion faithful: no partial, no
		// spurious. (The resolution reproduced the selected source content, so
		// the honest content-equality check passes — this is NOT a weakened
		// assertion; the same verifier reports a partial warning in the sibling
		// cherry_pick_e2e test when a file is resolved DIVERGENTLY.)
		verify, err := svc.VerifyPromotedContent(context.Background(), dir, "origin/UAT", feature2, []string{realClsPath, realFieldPath})
		if err != nil {
			t.Fatalf("VerifyPromotedContent error: %v", err)
		}
		if !verify.OK() {
			t.Fatalf("expected the resolved promotion faithful (no spurious files); partial=%v spurious=%v", verify.PartialFiles, verify.SpuriousFiles)
		}
		if len(verify.SpuriousFiles) != 0 {
			t.Fatalf("expected no spurious files on the real-metadata promotion, got %v", verify.SpuriousFiles)
		}
	})

	// Part B — opt-in, local only: prove the promoted tree is genuinely
	// deployable via a non-destructive real-org dry-run.
	t.Run("PartB sf dry-run validates the promoted tree against the real org", func(t *testing.T) {
		alias := os.Getenv("DEPLOYDECK_E2E_ORG")
		if alias == "" {
			t.Skip("set DEPLOYDECK_E2E_ORG=<alias> to run the opt-in real-org dry-run validation (local only)")
		}

		runner := exec.NewOSRunner()
		svc := git.New(runner)

		// Produce the SAME clean, faithful real-metadata promotion Part A does,
		// through DeployDeck's real engine, and validate THAT promoted tree.
		p := driveRealMetadataPromotion(t, runner, svc, false)
		if p.outcome.State.InProgress {
			t.Fatalf("promotion did not complete before org validation: %+v", p.outcome.State)
		}

		// Non-destructive check-only deploy. --dry-run validates (and would run
		// Apex tests, which we suppress with a non-production-valid test level)
		// WITHOUT saving to the org. NEVER a real deploy; no org mutation.
		testLevel := os.Getenv("DEPLOYDECK_E2E_TESTLEVEL")
		if testLevel == "" {
			testLevel = "NoTestRun"
		}
		sfArgs := []string{
			"project", "deploy", "start",
			"--source-dir", "force-app",
			"--dry-run",
			"--test-level", testLevel,
			"--ignore-conflicts",
			"-o", alias,
			"--json",
		}
		t.Logf("running (non-destructive, out of DeployDeck scope): sf %v in %s", sfArgs, p.dir)

		ctx, cancel := context.WithTimeout(context.Background(), 18*time.Minute)
		defer cancel()

		res, err := runner.Run(ctx, exec.CommandRequest{Name: "sf", Args: sfArgs, Dir: p.dir})
		if err != nil {
			t.Fatalf("sf dry-run failed to start: %v (stderr: %s)", err, res.Stderr)
		}

		raw := res.Stdout
		if len(bytes.TrimSpace(raw)) == 0 {
			raw = res.Stderr
		}
		var parsed sfDeployResult
		if jerr := json.Unmarshal(raw, &parsed); jerr != nil {
			t.Fatalf("could not parse sf --json output (exit %d): %v\nstdout:\n%s\nstderr:\n%s", res.ExitCode, jerr, res.Stdout, res.Stderr)
		}

		failures := hasComponentFailures(parsed.Result.Details.ComponentFailures)
		if res.ExitCode != 0 || !parsed.Result.Success || parsed.Result.NumberComponentErrors > 0 || failures {
			t.Fatalf("sf dry-run did NOT validate the promoted tree (real org error, reported verbatim):\n"+
				"exit=%d topStatus=%d resultStatus=%q success=%v componentErrors=%d\ncomponentFailures: %s\nfull stdout:\n%s\nstderr:\n%s",
				res.ExitCode, parsed.Status, parsed.Result.Status, parsed.Result.Success, parsed.Result.NumberComponentErrors,
				string(parsed.Result.Details.ComponentFailures), res.Stdout, res.Stderr)
		}

		t.Logf("sf dry-run validated the DeployDeck-promoted tree against %s (status=%q, componentErrors=%d) — promoted metadata is deployable",
			alias, parsed.Result.Status, parsed.Result.NumberComponentErrors)
	})
}

// promotionResult bundles what a driven real-metadata promotion produced: the
// temp repo dir (checked out on the promotion branch, working tree == promoted
// content), the selected commits, the selected tip SHA (for verification), and
// the cherry-pick outcome.
type promotionResult struct {
	dir         string
	selected    []git.DiscoveredCommit
	selectedTip string
	outcome     git.PickOutcome
}

// driveRealMetadataPromotion seeds a temp git repo from the REAL test-e2e-org
// Salesforce fixture and drives DeployDeck's real promotion engine end-to-end
// through the CLEAN (non-conflicting) path: Discover the PROJ-100 commits,
// build + validate the selection, CreatePromotionBranch off origin/UAT, and
// CherryPick the selected metadata commits. divergeUAT=false here (the clean
// variant); the conflict variant is driven inline in its own subtest so its
// resolution steps stay visible.
func driveRealMetadataPromotion(t *testing.T, runner exec.Runner, svc *git.Service, divergeUAT bool) promotionResult {
	t.Helper()

	dir := newTempRepo(t)
	_, feature2 := seedRealMetadataRepo(t, runner, dir, divergeUAT)

	result, selected := discoverAndSelectRealMetadata(t, svc, dir)

	if err := svc.CreatePromotionBranch(context.Background(), dir, "UAT", "deploy/PROJ-100-to-UAT"); err != nil {
		t.Fatalf("CreatePromotionBranch error: %v", err)
	}

	contiguous := git.IsContiguousSelection(result.OrderedCommits, selected)
	outcome, err := svc.CherryPick(context.Background(), dir, selected, contiguous)
	if err != nil {
		t.Fatalf("CherryPick error: %v", err)
	}

	return promotionResult{dir: dir, selected: selected, selectedTip: feature2, outcome: outcome}
}

// discoverAndSelectRealMetadata runs DeployDeck's real discovery over
// origin/UAT..origin/feature/PROJ-100, builds the HU-003 selection model, and
// returns the discovery result plus the selected DiscoveredCommits (in
// topological order). It asserts both PROJ-100 commits are discovered and
// selectable.
func discoverAndSelectRealMetadata(t *testing.T, svc *git.Service, dir string) (git.DiscoverResult, []git.DiscoveredCommit) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	result, err := svc.Discover(ctx, dir, git.DiscoverOptions{
		Ticket: "PROJ-100",
		Target: "UAT",
		Source: "feature/PROJ-100",
	})
	if err != nil {
		t.Fatalf("Discover error: %v", err)
	}
	if len(result.OrderedCommits) != 2 {
		t.Fatalf("expected 2 discovered commits in origin/UAT..origin/feature/PROJ-100, got %d (%+v)", len(result.OrderedCommits), result.OrderedCommits)
	}

	items := git.NewCommitSelectionItems(result.OrderedCommits, "PROJ-100")
	for _, it := range items {
		if it.Disabled {
			t.Fatalf("expected every real-metadata commit selectable, got disabled: %q (%s)", it.Subject, it.Reason)
		}
	}
	if err := git.ValidateSelection(items); err != nil {
		t.Fatalf("ValidateSelection error: %v", err)
	}

	var selected []git.DiscoveredCommit
	for _, it := range items {
		if it.Selected {
			selected = append(selected, it.DiscoveredCommit)
		}
	}
	if len(selected) != 2 {
		t.Fatalf("expected 2 selected commits, got %d", len(selected))
	}
	return result, selected
}

// seedRealMetadataRepo copies the REAL test-e2e-org Salesforce metadata into
// the temp repo, commits it as the main baseline, branches UAT (the promotion
// target) and feature/PROJ-100 (the source), and lays down two ticket-tagged
// feature commits that edit REAL metadata: (1) adds a method to
// AccountService.cls and (2) widens/relabels the Status__c custom field. When
// divergeUAT is true, UAT is advanced with a DIVERGENT edit to the SAME
// AccountService.cls header so the cherry-pick genuinely conflicts on real
// Salesforce metadata. Everything is pushed to origin (so origin/<branch> refs
// exist for Discover/CreatePromotionBranch). Leaves the repo on UAT and returns
// the two feature commit SHAs in topological order.
func seedRealMetadataRepo(t *testing.T, runner exec.Runner, dir string, divergeUAT bool) (feature1, feature2 string) {
	t.Helper()

	copyFixtureInto(t, dir)
	runGit(t, runner, dir, "add", "-A")
	runGit(t, runner, dir, "commit", "-m", "chore: import real Salesforce metadata fixture (test-e2e-org)")
	runGit(t, runner, dir, "push", "origin", "main")

	// UAT target branch off main.
	runGit(t, runner, dir, "checkout", "-b", "UAT", "main")
	if divergeUAT {
		writeFileHelper(t, dir, realClsPath, uatAccountServiceCls)
		runGit(t, runner, dir, "add", realClsPath)
		runGit(t, runner, dir, "commit", "-m", "UAT: maintenance edit to AccountService header")
	}
	runGit(t, runner, dir, "push", "origin", "UAT")

	// feature/PROJ-100 source branch off main with two real-metadata commits.
	runGit(t, runner, dir, "checkout", "-b", "feature/PROJ-100", "main")

	writeFileHelper(t, dir, realClsPath, featureAccountServiceCls)
	runGit(t, runner, dir, "add", realClsPath)
	runGit(t, runner, dir, "commit", "-m", "PROJ-100: add inactiveAccounts filter to AccountService")
	feature1 = trimNewline(string(runGit(t, runner, dir, "rev-parse", "HEAD").Stdout))

	writeFileHelper(t, dir, realFieldPath, featureStatusField)
	runGit(t, runner, dir, "add", realFieldPath)
	runGit(t, runner, dir, "commit", "-m", "PROJ-100: widen Status__c to 80 chars and relabel")
	feature2 = trimNewline(string(runGit(t, runner, dir, "rev-parse", "HEAD").Stdout))

	runGit(t, runner, dir, "push", "origin", "feature/PROJ-100")

	runGit(t, runner, dir, "checkout", "UAT")
	return feature1, feature2
}

// assertPromotedFileSet asserts the promotion branch changed EXACTLY want
// (relative to origin/UAT) — no more, no fewer files.
func assertPromotedFileSet(t *testing.T, runner exec.Runner, dir string, want ...string) {
	t.Helper()
	out := string(runGit(t, runner, dir, "diff", "--name-only", "origin/UAT", "HEAD").Stdout)
	got := map[string]bool{}
	for _, line := range bytes.Split([]byte(out), []byte("\n")) {
		if s := trimNewline(string(line)); s != "" {
			got[s] = true
		}
	}
	if len(got) != len(want) {
		t.Fatalf("promotion should carry exactly %d files %v, got %d: %v", len(want), want, len(got), out)
	}
	for _, w := range want {
		if !got[w] {
			t.Fatalf("expected the promotion to carry %q, got:\n%s", w, out)
		}
	}
}

// readRepoFile reads a repo-relative file from the working tree.
func readRepoFile(t *testing.T, dir, rel string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	return data
}

// fixtureE2ERoot resolves the absolute path of the real test-e2e-org fixture
// relative to THIS test file (via runtime.Caller), independent of the test's
// working directory.
func fixtureE2ERoot(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot resolve the test file path to locate the test-e2e-org fixture")
	}
	root := filepath.Join(filepath.Dir(thisFile), "..", "..", "test-e2e-org")
	if _, err := os.Stat(filepath.Join(root, "sfdx-project.json")); err != nil {
		t.Fatalf("real Salesforce fixture not found at %s: %v", root, err)
	}
	return root
}

// copyFixtureInto copies the REAL Salesforce metadata (sfdx-project.json,
// manifest/, force-app/) from test-e2e-org into dst.
func copyFixtureInto(t *testing.T, dst string) {
	t.Helper()
	root := fixtureE2ERoot(t)
	for _, rel := range []string{"sfdx-project.json", "manifest", "force-app"} {
		copyPath(t, filepath.Join(root, rel), filepath.Join(dst, rel))
	}
}

// copyPath copies a file or directory (recursively) from src to dst.
func copyPath(t *testing.T, src, dst string) {
	t.Helper()
	info, err := os.Stat(src)
	if err != nil {
		t.Fatalf("stat fixture %s: %v", src, err)
	}
	if !info.IsDir() {
		copyOneFile(t, src, dst)
		return
	}
	err = filepath.WalkDir(src, func(p string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, relErr := filepath.Rel(src, p)
		if relErr != nil {
			return relErr
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		copyOneFile(t, p, target)
		return nil
	})
	if err != nil {
		t.Fatalf("copy fixture dir %s: %v", src, err)
	}
}

// copyOneFile copies a single file, creating parent directories.
func copyOneFile(t *testing.T, src, dst string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(dst), err)
	}
	data, err := os.ReadFile(src)
	if err != nil {
		t.Fatalf("read fixture file %s: %v", src, err)
	}
	if err := os.WriteFile(dst, data, 0o644); err != nil {
		t.Fatalf("write %s: %v", dst, err)
	}
}

// sfDeployResult is the subset of `sf project deploy start --json` output this
// test asserts on.
type sfDeployResult struct {
	Status int `json:"status"`
	Result struct {
		Success               bool   `json:"success"`
		Status                string `json:"status"`
		NumberComponentErrors int    `json:"numberComponentErrors"`
		Details               struct {
			ComponentFailures json.RawMessage `json:"componentFailures"`
		} `json:"details"`
	} `json:"result"`
}

// hasComponentFailures reports whether a raw componentFailures JSON value
// (which sf may emit as null, an object, or an array) carries any failure.
func hasComponentFailures(raw json.RawMessage) bool {
	trimmed := bytes.TrimSpace(raw)
	switch string(trimmed) {
	case "", "null", "[]", "{}":
		return false
	default:
		return true
	}
}

// Real Salesforce metadata paths (repo-relative, forward-slash — git's form).
const (
	realClsPath   = "force-app/main/default/classes/AccountService.cls"
	realFieldPath = "force-app/main/default/objects/Account/fields/Status__c.field-meta.xml"
)

// featureAccountServiceCls is the PROJ-100 source version of the real Apex
// class: it rewrites the header comment and adds a second, valid filter method.
const featureAccountServiceCls = `public with sharing class AccountService {
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

// uatAccountServiceCls is a DIVERGENT UAT edit to the SAME header line the
// feature commit rewrites, so a cherry-pick genuinely conflicts (real UU) on
// this real Salesforce class.
const uatAccountServiceCls = `public with sharing class AccountService {
    // [UAT] Filtra cuentas por el campo Status__c (mantenimiento).
    public static List<Account> activeAccounts(List<Account> accounts) {
        List<Account> result = new List<Account>();
        for (Account acc : accounts) {
            if (acc.Status__c == 'Active') {
                result.add(acc);
            }
        }
        return result;
    }
}
`

// featureStatusField is the PROJ-100 source version of the real custom-field
// metadata: it widens the field to 80 chars and relabels it.
const featureStatusField = `<?xml version="1.0" encoding="UTF-8"?>
<CustomField xmlns="http://soap.sforce.com/2006/04/metadata">
    <fullName>Status__c</fullName>
    <label>Account Status</label>
    <type>Text</type>
    <length>80</length>
    <required>false</required>
    <trackHistory>false</trackHistory>
</CustomField>
`
