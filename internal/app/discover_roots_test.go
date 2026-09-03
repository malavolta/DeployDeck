package app

import (
	"testing"

	"github.com/malavolta/DeployDeck/internal/config"
	"github.com/malavolta/DeployDeck/internal/git"
)

// TestModel_DiscoverCmd_NestedDeps_RunsGitAtGitRoot is config-validation-wiring
// task 8.1 (W-3, explicitly PARTIAL): proves discoverCmd — ONE of ~13
// git-backed call sites in this package — runs every git invocation at
// deps.GitRoot, not deps.ProjectDir, in a nested layout where the two
// differ.
//
// roots_guard_test.go only proves deps.Dir is never read outside
// normalizeRoots; it says nothing about which of the THREE resolved roots a
// given call site threads through afterward, so a site that (mistakenly)
// read ProjectDir for a git call would pass that guard equally. This test
// closes that gap for one representative site, not all of them — see
// design.md's "Secondary Items — Concrete Shapes" (W-3) for the remaining
// coverage this deliberately leaves open, revisited only if a second
// root-conflation defect surfaces at a non-deltaCmd, non-discoverCmd site.
func TestModel_DiscoverCmd_NestedDeps_RunsGitAtGitRoot(t *testing.T) {
	gitRoot := "/repo"
	projectDir := gitRoot + "/project"
	ticket := "PROJ-1"

	// candidateFakeRunner (source_confirm_test.go) cans exactly the git
	// calls a message+branch-only pass-1 Discover issues (ticket only, no
	// target/source): empty local/remote branch lists resolve to
	// resolveDegrade, so no second (ranged) Discover call follows.
	fr := candidateFakeRunner(t, gitRoot, ticket, "", "")

	m := New(Deps{
		Git: git.New(fr),
		// GitRoot/ProjectDir/ArtifactsRoot set explicitly and DISTINCT —
		// the nested layout this test exists to prove discoverCmd handles
		// correctly (Dir left unset entirely, so normalizeRoots's fallback
		// never engages).
		GitRoot:       gitRoot,
		ProjectDir:    projectDir,
		ArtifactsRoot: projectDir,
		Config:        config.Config{},
	})
	m.ticket = ticket

	run(t, m.discoverCmd())

	if len(fr.Calls) == 0 {
		t.Fatal("expected discoverCmd to issue at least one git call")
	}
	for i, call := range fr.Calls {
		if call.Dir != gitRoot {
			t.Errorf("call %d (%s %v) ran with Dir = %q, want the git root %q — not the (deliberately distinct) project dir %q", i, call.Name, call.Args, call.Dir, gitRoot, projectDir)
		}
	}
}
