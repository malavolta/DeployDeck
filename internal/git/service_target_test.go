package git_test

import (
	"context"
	"testing"

	"deploydeck/internal/exec"
	"deploydeck/internal/git"
)

// TestService_BranchExists_TableDriven proves a destination branch that
// exists locally, one that exists only as an origin remote-tracking
// branch, and one that exists nowhere are each resolved correctly (HU-004
// AC: "Dado que la rama destino no existe, cuando el usuario intenta
// continuar, entonces se bloquea el flujo").
func TestService_BranchExists_TableDriven(t *testing.T) {
	dir := newTempRepo(t)
	runner := exec.NewOSRunner()
	svc := git.New(runner)

	// A branch that exists locally AND is pushed to origin.
	runGit(t, runner, dir, "checkout", "-b", "UAT", "main")
	runGit(t, runner, dir, "push", "origin", "UAT")

	// A branch that exists ONLY as a remote-tracking ref: create it, push
	// it, then delete the local branch (checking out main first, since a
	// branch cannot be deleted while checked out).
	runGit(t, runner, dir, "checkout", "-b", "Release/Julio2026", "main")
	runGit(t, runner, dir, "push", "origin", "Release/Julio2026")
	runGit(t, runner, dir, "checkout", "main")
	runGit(t, runner, dir, "branch", "-D", "Release/Julio2026")

	tests := []struct {
		name   string
		branch string
		want   bool
	}{
		{name: "exists locally and remotely", branch: "UAT", want: true},
		{name: "exists only as a remote-tracking branch", branch: "Release/Julio2026", want: true},
		{name: "does not exist anywhere", branch: "STAGING-DOES-NOT-EXIST", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := svc.BranchExists(context.Background(), dir, tt.branch)
			if err != nil {
				t.Fatalf("BranchExists(%q): unexpected error: %v", tt.branch, err)
			}
			if got != tt.want {
				t.Errorf("BranchExists(%q) = %v, want %v", tt.branch, got, tt.want)
			}
		})
	}
}

// TestService_RemoteHead_TableDriven proves the destination branch's
// remote HEAD is resolved via `git rev-parse origin/<target>` (HU-004 AC:
// "Mostrar HEAD remoto de la rama destino"), and that a target absent from
// origin resolves ok=false rather than an error.
func TestService_RemoteHead_TableDriven(t *testing.T) {
	dir := newTempRepo(t)
	runner := exec.NewOSRunner()
	svc := git.New(runner)

	runGit(t, runner, dir, "checkout", "-b", "UAT", "main")
	writeAndCommit(t, runner, dir, "uat.txt", "uat\n", "chore: seed UAT")
	runGit(t, runner, dir, "push", "origin", "UAT")
	uatHead := trimNewline(string(runGit(t, runner, dir, "rev-parse", "origin/UAT").Stdout))

	mainHead := trimNewline(string(runGit(t, runner, dir, "rev-parse", "origin/main").Stdout))

	tests := []struct {
		name       string
		target     string
		wantSHA    string
		wantExists bool
	}{
		{name: "UAT has a known remote HEAD", target: "UAT", wantSHA: uatHead, wantExists: true},
		{name: "main has a known remote HEAD", target: "main", wantSHA: mainHead, wantExists: true},
		{name: "target absent from origin resolves ok=false", target: "NO-SUCH-TARGET", wantExists: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sha, ok, err := svc.RemoteHead(context.Background(), dir, tt.target)
			if err != nil {
				t.Fatalf("RemoteHead(%q): unexpected error: %v", tt.target, err)
			}
			if ok != tt.wantExists {
				t.Fatalf("RemoteHead(%q) ok = %v, want %v", tt.target, ok, tt.wantExists)
			}
			if tt.wantExists && sha != tt.wantSHA {
				t.Errorf("RemoteHead(%q) sha = %q, want %q", tt.target, sha, tt.wantSHA)
			}
		})
	}
}

// TestService_BranchExists_CustomBranchName proves BranchExists correctly
// validates an arbitrary user-typed custom destination branch — one that
// is NOT among config.Config.Branches at all — rejecting it when absent
// and accepting it when present (HU-004 AC: "Permitir rama custom
// validando que exista local o remota"). This exercises the exact same
// BranchExists primitive 8.3/8.4 already proved for configured
// destinations; this test's own value is proving the check has no
// dependency on the branch being a "known" environment name.
func TestService_BranchExists_CustomBranchName(t *testing.T) {
	dir := newTempRepo(t)
	runner := exec.NewOSRunner()
	svc := git.New(runner)

	runGit(t, runner, dir, "checkout", "-b", "hotfix/custom-PROJ-99", "main")
	runGit(t, runner, dir, "push", "origin", "hotfix/custom-PROJ-99")

	exists, err := svc.BranchExists(context.Background(), dir, "hotfix/custom-PROJ-99")
	if err != nil {
		t.Fatalf("BranchExists: unexpected error: %v", err)
	}
	if !exists {
		t.Errorf("expected the custom branch to be accepted as existing")
	}

	rejected, err := svc.BranchExists(context.Background(), dir, "hotfix/never-created")
	if err != nil {
		t.Fatalf("BranchExists: unexpected error: %v", err)
	}
	if rejected {
		t.Errorf("expected a custom branch that was never created to be rejected")
	}
}
