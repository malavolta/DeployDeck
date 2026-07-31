package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/malavolta/DeployDeck/internal/config"
	"github.com/malavolta/DeployDeck/internal/exec"
	"github.com/malavolta/DeployDeck/internal/git"
	"github.com/malavolta/DeployDeck/internal/prereq"
	"github.com/malavolta/DeployDeck/internal/salesforce"
)

// This file is the consolidated HU-001 "Test E2E" deliverable
// (docs/HISTORIAS.md:86-92): one harness (a real temp git repo + a fake `sf`
// via FakeRunner) that seeds every documented variant — with/without origin,
// .gitignore with/without .deploydeck/, sfdx-git-delta present/absent,
// versions below/above minVersions, working tree clean/dirty, alias
// present/missing, and lock already taken — and asserts each variant's
// resulting Status BOTH through the assembled Checker.Check() report AND
// through the `deploydeck doctor` exit code (0 vs non-zero).

// liveProber reports the seeded lock owner as alive, so the lock-taken
// variant refuses acquisition.
type liveProber struct{}

func (liveProber) Alive(int, time.Time) bool { return true }

const healthySFVersion = "@salesforce/cli/2.63.6 darwin-arm64 node-v22.11.0\n"

type doctorVariant struct {
	name         string
	withOrigin   bool
	gitignore    string // committed .gitignore content; "" => no .gitignore file
	leaveDirty   bool
	sfVersion    string
	deltaVersion string // "" => sfdx-git-delta plugin absent
	minVersions  map[string]string
	sandboxes    map[string]config.SandboxConfig
	orgAliases   []string
	lockTaken    bool

	targetCheck        string
	wantStatus         prereq.Status
	wantDoctorBlocked  bool
	wantDetailContains []string
	wantFixContains    string
}

func baselineMinVersions() map[string]string {
	return map[string]string{"git": "2.0.0", "sf": "2.0.0", "sfdx-git-delta": "5.0.0"}
}

func baselineSandboxes() map[string]config.SandboxConfig {
	return map[string]config.SandboxConfig{"UAT": {Alias: "uat", TestLevel: "RunLocalTests"}}
}

func TestHU001_Doctor_E2E_ConsolidatedVariants(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping HU-001 doctor E2E: it shells out to a real git binary")
	}

	variants := []doctorVariant{
		{
			name:         "all prerequisites pass",
			withOrigin:   true,
			gitignore:    ".deploydeck/\n",
			sfVersion:    healthySFVersion,
			deltaVersion: "5.35.0",
			minVersions:  baselineMinVersions(),
			sandboxes:    baselineSandboxes(),
			orgAliases:   []string{"uat"},
			targetCheck:  "instance lock",
			wantStatus:   prereq.StatusOK,
		},
		{
			name:              "missing origin blocks",
			withOrigin:        false,
			gitignore:         ".deploydeck/\n",
			sfVersion:         healthySFVersion,
			deltaVersion:      "5.35.0",
			minVersions:       baselineMinVersions(),
			sandboxes:         baselineSandboxes(),
			orgAliases:        []string{"uat"},
			targetCheck:       "origin remote",
			wantStatus:        prereq.StatusBlocking,
			wantDoctorBlocked: true,
		},
		{
			name:              ".deploydeck not gitignored blocks",
			withOrigin:        true,
			gitignore:         "*.log\n",
			sfVersion:         healthySFVersion,
			deltaVersion:      "5.35.0",
			minVersions:       baselineMinVersions(),
			sandboxes:         baselineSandboxes(),
			orgAliases:        []string{"uat"},
			targetCheck:       ".deploydeck/ gitignore",
			wantStatus:        prereq.StatusBlocking,
			wantDoctorBlocked: true,
		},
		{
			name:               "sfdx-git-delta plugin absent blocks with install command",
			withOrigin:         true,
			gitignore:          ".deploydeck/\n",
			sfVersion:          healthySFVersion,
			deltaVersion:       "", // absent
			minVersions:        baselineMinVersions(),
			sandboxes:          baselineSandboxes(),
			orgAliases:         []string{"uat"},
			targetCheck:        "sfdx-git-delta plugin",
			wantStatus:         prereq.StatusBlocking,
			wantDoctorBlocked:  true,
			wantFixContains:    "install",
			wantDetailContains: []string{"sfdx-git-delta"},
		},
		{
			name:               "sfdx-git-delta below minimum blocks",
			withOrigin:         true,
			gitignore:          ".deploydeck/\n",
			sfVersion:          healthySFVersion,
			deltaVersion:       "5.10.0",
			minVersions:        map[string]string{"git": "2.0.0", "sf": "2.0.0", "sfdx-git-delta": "5.30.0"},
			sandboxes:          baselineSandboxes(),
			orgAliases:         []string{"uat"},
			targetCheck:        "sfdx-git-delta version",
			wantStatus:         prereq.StatusBlocking,
			wantDoctorBlocked:  true,
			wantDetailContains: []string{"5.10.0", "5.30.0"},
		},
		{
			name:               "git below minimum blocks",
			withOrigin:         true,
			gitignore:          ".deploydeck/\n",
			sfVersion:          healthySFVersion,
			deltaVersion:       "5.35.0",
			minVersions:        map[string]string{"git": "99.0.0"},
			sandboxes:          baselineSandboxes(),
			orgAliases:         []string{"uat"},
			targetCheck:        "git version",
			wantStatus:         prereq.StatusBlocking,
			wantDoctorBlocked:  true,
			wantDetailContains: []string{"99.0.0"},
		},
		{
			name:               "sf below minimum blocks",
			withOrigin:         true,
			gitignore:          ".deploydeck/\n",
			sfVersion:          healthySFVersion,
			deltaVersion:       "5.35.0",
			minVersions:        map[string]string{"sf": "99.0.0"},
			sandboxes:          baselineSandboxes(),
			orgAliases:         []string{"uat"},
			targetCheck:        "sf version",
			wantStatus:         prereq.StatusBlocking,
			wantDoctorBlocked:  true,
			wantDetailContains: []string{"99.0.0"},
		},
		{
			name:              "dirty working tree blocks",
			withOrigin:        true,
			gitignore:         ".deploydeck/\n",
			leaveDirty:        true,
			sfVersion:         healthySFVersion,
			deltaVersion:      "5.35.0",
			minVersions:       baselineMinVersions(),
			sandboxes:         baselineSandboxes(),
			orgAliases:        []string{"uat"},
			targetCheck:       "working tree",
			wantStatus:        prereq.StatusBlocking,
			wantDoctorBlocked: true,
		},
		{
			name:               "missing alias blocks naming the alias",
			withOrigin:         true,
			gitignore:          ".deploydeck/\n",
			sfVersion:          healthySFVersion,
			deltaVersion:       "5.35.0",
			minVersions:        baselineMinVersions(),
			sandboxes:          baselineSandboxes(),
			orgAliases:         []string{}, // uat not present
			targetCheck:        "sandbox alias (UAT)",
			wantStatus:         prereq.StatusBlocking,
			wantDoctorBlocked:  true,
			wantDetailContains: []string{"uat"},
		},
		{
			name:               "lock already taken blocks naming the owner",
			withOrigin:         true,
			gitignore:          ".deploydeck/\n",
			sfVersion:          healthySFVersion,
			deltaVersion:       "5.35.0",
			minVersions:        baselineMinVersions(),
			sandboxes:          baselineSandboxes(),
			orgAliases:         []string{"uat"},
			lockTaken:          true,
			targetCheck:        "instance lock",
			wantStatus:         prereq.StatusBlocking,
			wantDoctorBlocked:  true,
			wantDetailContains: []string{"deploydeck-other", "999"},
		},
	}

	for _, v := range variants {
		t.Run(v.name, func(t *testing.T) {
			dir := newDoctorRepo(t, v)
			sfRunner := newSFRunner(v)
			cfg := config.Config{MinVersions: v.minVersions, Sandboxes: v.sandboxes}

			// A pre-seeded, live-owned lock for the lock-taken variant; a
			// fresh free lock path otherwise. Kept OUTSIDE the repo working
			// tree so it never interferes with the working-tree check.
			takenLockPath := filepath.Join(t.TempDir(), "lock")
			if v.lockTaken {
				seedLiveLock(t, takenLockPath)
			}

			// (1) Assert the Status THROUGH the assembled Checker.Check().
			checkerA := buildDoctorChecker(dir, sfRunner, cfg, variantLock(t, v, takenLockPath))
			checks, err := checkerA.Check(context.Background())
			if err != nil {
				t.Fatalf("Check() returned a hard error: %v", err)
			}

			target := findE2ECheck(t, checks, v.targetCheck)
			if target.Status != v.wantStatus {
				t.Fatalf("variant %q: check %q Status = %q, want %q (%+v)", v.name, v.targetCheck, target.Status, v.wantStatus, target)
			}
			for _, sub := range v.wantDetailContains {
				if !strings.Contains(target.Detail, sub) {
					t.Fatalf("variant %q: expected check %q detail to contain %q, got %q", v.name, v.targetCheck, sub, target.Detail)
				}
			}
			if v.wantFixContains != "" && !strings.Contains(target.FixCommand, v.wantFixContains) {
				t.Fatalf("variant %q: expected check %q FixCommand to contain %q, got %q", v.name, v.targetCheck, v.wantFixContains, target.FixCommand)
			}
			if gotBlocked := anyBlocking(checks); gotBlocked != v.wantDoctorBlocked {
				t.Fatalf("variant %q: Check() overall blocking = %v, want %v", v.name, gotBlocked, v.wantDoctorBlocked)
			}

			// (2) Assert the SAME outcome THROUGH the deploydeck doctor exit
			// code (0 vs non-zero). A fresh lock instance avoids colliding
			// with checkerA's own acquisition above.
			deps := Deps{NewChecker: func(string) (*prereq.Checker, error) {
				return buildDoctorChecker(dir, sfRunner, cfg, variantLock(t, v, takenLockPath)), nil
			}}
			cmd := newRootCmd(deps)
			cmd.SetArgs([]string{"doctor"})
			cmd.SetOut(io.Discard)
			cmd.SetErr(io.Discard)

			doctorErr := cmd.Execute()
			if v.wantDoctorBlocked {
				if doctorErr == nil {
					t.Fatalf("variant %q: expected doctor to exit non-zero, got nil", v.name)
				}
				if !errors.Is(doctorErr, errDoctorBlocked) {
					t.Fatalf("variant %q: expected doctor error to be errDoctorBlocked, got %v", v.name, doctorErr)
				}
			} else if doctorErr != nil {
				t.Fatalf("variant %q: expected doctor to exit 0 (nil), got %v", v.name, doctorErr)
			}
		})
	}
}

func buildDoctorChecker(dir string, sfRunner exec.Runner, cfg config.Config, lock *prereq.Lock) *prereq.Checker {
	return &prereq.Checker{
		Dir:    dir,
		Git:    git.New(exec.NewOSRunner()),
		SF:     salesforce.New(sfRunner),
		Config: cfg,
		Lock:   lock,
	}
}

func variantLock(t *testing.T, v doctorVariant, takenLockPath string) *prereq.Lock {
	t.Helper()
	self := prereq.LockInfo{PID: 4321, PName: "deploydeck", Host: "test-host"}
	if v.lockTaken {
		return prereq.NewLock(takenLockPath, self, liveProber{})
	}
	return prereq.NewLock(filepath.Join(t.TempDir(), "lock"), self, fakeAliveProber{})
}

func seedLiveLock(t *testing.T, path string) {
	t.Helper()
	owner := prereq.LockInfo{
		PID:       999,
		PName:     "deploydeck-other",
		Host:      "test-host",
		StartedAt: time.Now().Add(-time.Minute),
	}
	data, err := json.Marshal(owner)
	if err != nil {
		t.Fatalf("marshal seed lock: %v", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("seed live lock: %v", err)
	}
}

func newSFRunner(v doctorVariant) *exec.FakeRunner {
	fr := exec.NewFakeRunner()
	fr.When("sf", []string{"--version"}, exec.CommandResult{ExitCode: 0, Stdout: []byte(v.sfVersion)})
	fr.When("sf", []string{"plugins", "--json"}, exec.CommandResult{ExitCode: 0, Stdout: []byte(pluginsJSON(v.deltaVersion))})
	fr.When("sf", []string{"org", "list", "--json"}, exec.CommandResult{ExitCode: 0, Stdout: []byte(orgListJSON(v.orgAliases))})
	return fr
}

func pluginsJSON(deltaVersion string) string {
	// `sf plugins --json` returns a top-level array of oclif plugin objects,
	// NOT the `{"status":0,"result":...}` envelope `sf org list --json` and
	// `sf --version` use.
	if deltaVersion == "" {
		return `[]`
	}
	return fmt.Sprintf(`[{"name":"sfdx-git-delta","version":%q,"children":[]}]`, deltaVersion)
}

func orgListJSON(aliases []string) string {
	if len(aliases) == 0 {
		return `{"status":0,"result":{}}`
	}
	items := make([]string, 0, len(aliases))
	for _, a := range aliases {
		items = append(items, fmt.Sprintf(`{"alias":%q,"username":"%s@example.com"}`, a, a))
	}
	return fmt.Sprintf(`{"status":0,"result":{"sandboxes":[%s]}}`, strings.Join(items, ","))
}

func newDoctorRepo(t *testing.T, v doctorVariant) string {
	t.Helper()
	runner := exec.NewOSRunner()
	dir := t.TempDir()

	gitCmd(t, runner, dir, "init", "-b", "main")
	gitCmd(t, runner, dir, "config", "user.name", "DeployDeck Test")
	gitCmd(t, runner, dir, "config", "user.email", "deploydeck-test@example.com")

	if v.withOrigin {
		remote := t.TempDir()
		gitCmd(t, runner, remote, "init", "--bare")
		gitCmd(t, runner, dir, "remote", "add", "origin", remote)
	}

	if v.gitignore != "" {
		if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte(v.gitignore), 0o644); err != nil {
			t.Fatalf("seed .gitignore: %v", err)
		}
		gitCmd(t, runner, dir, "add", ".gitignore")
	}

	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("# deploydeck temp repo\n"), 0o644); err != nil {
		t.Fatalf("seed README: %v", err)
	}
	gitCmd(t, runner, dir, "add", "README.md")
	gitCmd(t, runner, dir, "commit", "-m", "chore: initial commit")

	if v.leaveDirty {
		if err := os.WriteFile(filepath.Join(dir, "dirty.txt"), []byte("uncommitted change\n"), 0o644); err != nil {
			t.Fatalf("seed dirty file: %v", err)
		}
	}

	return dir
}

func gitCmd(t *testing.T, runner exec.Runner, dir string, args ...string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, err := runner.Run(ctx, exec.CommandRequest{
		Name: "git",
		Args: args,
		Dir:  dir,
		Env:  []string{"GIT_EDITOR=true", "GIT_TERMINAL_PROMPT=0", "GIT_PAGER=cat"},
	})
	if err != nil {
		t.Fatalf("git %v failed to run: %v (stderr: %s)", args, err, result.Stderr)
	}
	if result.ExitCode != 0 {
		t.Fatalf("git %v exited %d: %s", args, result.ExitCode, result.Stderr)
	}
}

func findE2ECheck(t *testing.T, checks []prereq.PrereqCheck, name string) prereq.PrereqCheck {
	t.Helper()
	for _, c := range checks {
		if c.Name == name {
			return c
		}
	}
	t.Fatalf("expected a check named %q, got: %+v", name, checks)
	return prereq.PrereqCheck{}
}

func anyBlocking(checks []prereq.PrereqCheck) bool {
	for _, c := range checks {
		if c.Status == prereq.StatusBlocking {
			return true
		}
	}
	return false
}
