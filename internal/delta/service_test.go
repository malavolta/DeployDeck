package delta_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/malavolta/DeployDeck/internal/delta"
	"github.com/malavolta/DeployDeck/internal/exec"
)

// TestService_Generate_ComposesArgs_TableDriven proves Generate composes
// exactly ONE `sf sgd source delta` invocation (threat-matrix
// "PR/argument composition" row): --from/--to/--output-dir/--generate-delta
// always present, one repeated --source-dir per configured entry (in
// order), and --ignore-file/--ignore-destructive-file added only when
// configured. CommandRequest.Dir is always req.Dir (the repo root the
// caller resolved), never a trusted cwd (threat-matrix "Git repo
// selection" row).
func TestService_Generate_ComposesArgs_TableDriven(t *testing.T) {
	tests := []struct {
		name     string
		req      func(outputDir string) delta.Request
		wantTail func(outputDir string) []string
	}{
		{
			name: "single source dir, no ignore files",
			req: func(outputDir string) delta.Request {
				return delta.Request{
					Dir: "/repo", From: "origin/UAT", To: "HEAD",
					OutputDir: outputDir, SourceDirs: []string{"force-app"},
				}
			},
			wantTail: func(outputDir string) []string {
				return []string{
					"--source-dir", "force-app",
				}
			},
		},
		{
			name: "multiple source dirs repeat --source-dir in order",
			req: func(outputDir string) delta.Request {
				return delta.Request{
					Dir: "/repo", From: "origin/UAT", To: "HEAD",
					OutputDir: outputDir, SourceDirs: []string{"force-app", "unpackaged"},
				}
			},
			wantTail: func(outputDir string) []string {
				return []string{
					"--source-dir", "force-app",
					"--source-dir", "unpackaged",
				}
			},
		},
		{
			name: "ignore-file and ignore-destructive-file added only when configured",
			req: func(outputDir string) delta.Request {
				return delta.Request{
					Dir: "/repo", From: "origin/UAT", To: "HEAD",
					OutputDir: outputDir, SourceDirs: []string{"force-app"},
					IgnoreFile: ".sgdignore", IgnoreDestructiveFile: ".sgdignoredestructive",
				}
			},
			wantTail: func(outputDir string) []string {
				return []string{
					"--source-dir", "force-app",
					"--ignore-file", ".sgdignore",
					"--ignore-destructive-file", ".sgdignoredestructive",
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			outputDir := filepath.Join(t.TempDir(), "out")
			req := tt.req(outputDir)

			wantArgs := append([]string{
				"sgd", "source", "delta",
				"--from", req.From, "--to", req.To,
				"--output-dir", outputDir, "--generate-delta",
			}, tt.wantTail(outputDir)...)

			runner := exec.NewFakeRunner()
			runner.When("sf", wantArgs, exec.CommandResult{ExitCode: 0, Stdout: []byte("Success\n")})

			svc := delta.New(runner)
			if _, err := svc.Generate(context.Background(), req); err != nil {
				t.Fatalf("Generate() unexpected error: %v", err)
			}

			if len(runner.Calls) != 1 {
				t.Fatalf("expected exactly ONE sgd invocation, got %d: %+v", len(runner.Calls), runner.Calls)
			}
			call := runner.Calls[0]
			if call.Dir != req.Dir {
				t.Errorf("CommandRequest.Dir = %q, want %q (repo root, never a trusted cwd)", call.Dir, req.Dir)
			}
			if call.Name != "sf" {
				t.Errorf("CommandRequest.Name = %q, want %q", call.Name, "sf")
			}
		})
	}
}

// TestService_Generate_DiscoversArtifacts proves artifact discovery per
// design.md: package/package.xml is ALWAYS returned (the path is
// constructed, not conditioned on a disk check), while
// destructiveChanges/destructiveChanges.xml is returned only when sgd
// actually produced it.
func TestService_Generate_DiscoversArtifacts(t *testing.T) {
	newReqAndArgs := func(t *testing.T) (delta.Request, []string, string) {
		t.Helper()
		outputDir := filepath.Join(t.TempDir(), "out")
		req := delta.Request{Dir: "/repo", From: "origin/UAT", To: "HEAD", OutputDir: outputDir, SourceDirs: []string{"force-app"}}
		args := []string{
			"sgd", "source", "delta",
			"--from", "origin/UAT", "--to", "HEAD",
			"--output-dir", outputDir, "--generate-delta",
			"--source-dir", "force-app",
		}
		return req, args, outputDir
	}

	t.Run("destructiveChanges.xml present is located", func(t *testing.T) {
		req, args, outputDir := newReqAndArgs(t)
		writeFixtureFile(t, filepath.Join(outputDir, "destructiveChanges", "destructiveChanges.xml"), "<Package/>")

		runner := exec.NewFakeRunner()
		runner.When("sf", args, exec.CommandResult{ExitCode: 0, Stdout: []byte("Success\n")})

		got, err := delta.New(runner).Generate(context.Background(), req)
		if err != nil {
			t.Fatalf("Generate() unexpected error: %v", err)
		}

		wantPackage := filepath.Join(outputDir, "package", "package.xml")
		if got.PackageXMLPath != wantPackage {
			t.Errorf("PackageXMLPath = %q, want %q (always located, not disk-checked)", got.PackageXMLPath, wantPackage)
		}
		wantDestructive := filepath.Join(outputDir, "destructiveChanges", "destructiveChanges.xml")
		if got.DestructiveChangesPath != wantDestructive {
			t.Errorf("DestructiveChangesPath = %q, want %q", got.DestructiveChangesPath, wantDestructive)
		}
		if got.Raw == "" {
			t.Error("expected Raw to capture the sgd command output")
		}
	})

	t.Run("destructiveChanges.xml absent leaves DestructiveChangesPath empty", func(t *testing.T) {
		req, args, outputDir := newReqAndArgs(t)
		// Deliberately nothing written to outputDir: package.xml's path must
		// still be returned (it is never disk-checked), only
		// destructiveChanges.xml's absence is disk-checked.
		_ = outputDir

		runner := exec.NewFakeRunner()
		runner.When("sf", args, exec.CommandResult{ExitCode: 0, Stdout: []byte("Success\n")})

		got, err := delta.New(runner).Generate(context.Background(), req)
		if err != nil {
			t.Fatalf("Generate() unexpected error: %v", err)
		}
		if got.PackageXMLPath == "" {
			t.Error("expected a non-empty PackageXMLPath even when nothing was pre-written to disk")
		}
		if got.DestructiveChangesPath != "" {
			t.Errorf("expected empty DestructiveChangesPath when sgd produced no destructive changes, got %q", got.DestructiveChangesPath)
		}
	})
}

// TestService_Generate_SgdFailureReturnsErrorWithRaw_NoArtifacts proves a
// non-zero sgd exit returns a zero Result and an error carrying the raw
// stdout/stderr — no artifact paths are ever returned for a failed run, so
// callers can never accidentally trigger downstream validation against a
// package sgd never (successfully) produced.
func TestService_Generate_SgdFailureReturnsErrorWithRaw_NoArtifacts(t *testing.T) {
	outputDir := filepath.Join(t.TempDir(), "out")
	req := delta.Request{Dir: "/repo", From: "origin/UAT", To: "HEAD", OutputDir: outputDir, SourceDirs: []string{"force-app"}}
	args := []string{
		"sgd", "source", "delta",
		"--from", "origin/UAT", "--to", "HEAD",
		"--output-dir", outputDir, "--generate-delta",
		"--source-dir", "force-app",
	}

	runner := exec.NewFakeRunner()
	runner.When("sf", args, exec.CommandResult{
		ExitCode: 2,
		Stderr:   []byte("Error (2): Parsing --from"),
	})

	got, err := delta.New(runner).Generate(context.Background(), req)
	if err == nil {
		t.Fatal("expected an error when sgd exits non-zero")
	}
	if !strings.Contains(err.Error(), "Parsing --from") {
		t.Errorf("expected the error to carry sgd's raw output, got: %v", err)
	}
	if got != (delta.Result{}) {
		t.Errorf("expected a zero Result on sgd failure, got %+v", got)
	}
}

func writeFixtureFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("failed to create dir for %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("failed to write fixture %s: %v", path, err)
	}
}
