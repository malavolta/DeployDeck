// Package delta wraps sfdx-git-delta (the `sf sgd source delta` plugin
// command) over internal/exec.Runner, producing HU-007's delta package
// (package.xml, and destructiveChanges.xml when metadata was deleted) from
// two git refs. It is a sibling of internal/salesforce, not an extension of
// it: sgd is a distinct external command with its own flags and output
// shape, so keeping it separate keeps the read-only sf shim thin (see
// design.md's "delta placement" decision).
package delta

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"deploydeck/internal/exec"
)

// Request is one delta-generation invocation's parameters.
type Request struct {
	// Dir is the repository root `sf sgd source delta` runs in (the
	// CommandRequest's Dir) — always an explicit, already-resolved root,
	// never a trusted process cwd (see design.md's Threat Matrix "Git repo
	// selection" row).
	Dir string
	// From and To are the two git refs compared, passed through literally
	// (e.g. "origin/UAT", "HEAD") — never re-derived here.
	From string
	To   string
	// OutputDir is where sgd writes its package/ and destructiveChanges/
	// subfolders. Generate creates it if missing: sgd itself errors
	// ("No directory found at <dir>") when --output-dir does not already
	// exist.
	OutputDir string
	// SourceDirs are passed as one repeated --source-dir flag per entry —
	// a single sgd invocation merges all of them into one package (the
	// spike verified 6.45.1 supports a repeatable --source-dir; see
	// design.md's "sgd multi-dir" decision).
	SourceDirs []string
	// IgnoreFile and IgnoreDestructiveFile are optional paths to
	// gitignore-style files, passed through as sgd's --ignore-file and
	// --ignore-destructive-file flags (both take a file path, verified via
	// `sf sgd source delta --help`) when non-empty.
	IgnoreFile            string
	IgnoreDestructiveFile string
}

// Result is a successful delta generation's output.
type Result struct {
	// PackageXMLPath is always set: sgd always writes package/package.xml
	// under OutputDir on a successful run, even when the delta is empty
	// (zero <types> elements).
	PackageXMLPath string
	// DestructiveChangesPath is set only when sgd produced
	// destructiveChanges/destructiveChanges.xml (deleted metadata present
	// in the diff); "" otherwise.
	DestructiveChangesPath string
	// Raw is the sgd command's captured stdout+stderr, kept for
	// diagnostics.
	Raw string
}

// Service generates delta packages via `sf sgd source delta`, backed by a
// Runner.
type Service struct {
	runner exec.Runner
}

// New returns a Service backed by runner.
func New(runner exec.Runner) *Service {
	return &Service{runner: runner}
}

// Generate runs ONE `sf sgd source delta` invocation comparing req.From to
// req.To and discovers the resulting artifacts. A runner error or a
// non-zero sgd exit returns a zero Result and an error carrying the raw
// sgd output — no artifact paths are ever returned for a failed run, so
// callers never run downstream Salesforce validation against a package sgd
// never (successfully) produced.
func (s *Service) Generate(ctx context.Context, req Request) (Result, error) {
	if err := os.MkdirAll(req.OutputDir, 0o755); err != nil {
		return Result{}, fmt.Errorf("delta: creating output dir %s: %w", req.OutputDir, err)
	}

	cmdReq := exec.CommandRequest{
		Name: "sf",
		Args: buildArgs(req),
		Dir:  req.Dir,
	}

	result, err := s.runner.Run(ctx, cmdReq)
	if err != nil {
		return Result{}, fmt.Errorf("delta: running sf sgd source delta: %w", err)
	}

	raw := combineOutput(result.Stdout, result.Stderr)
	if result.ExitCode != 0 {
		return Result{}, fmt.Errorf("delta: sf sgd source delta exited %d: %s", result.ExitCode, raw)
	}

	return discoverArtifacts(req.OutputDir, raw), nil
}

// buildArgs composes the single sgd invocation:
//
//	sf sgd source delta --from <From> --to <To> --output-dir <OutputDir> --generate-delta
//	  (--source-dir X)... [--ignore-file <path>] [--ignore-destructive-file <path>]
func buildArgs(req Request) []string {
	args := []string{
		"sgd", "source", "delta",
		"--from", req.From,
		"--to", req.To,
		"--output-dir", req.OutputDir,
		"--generate-delta",
	}
	for _, dir := range req.SourceDirs {
		args = append(args, "--source-dir", dir)
	}
	if req.IgnoreFile != "" {
		args = append(args, "--ignore-file", req.IgnoreFile)
	}
	if req.IgnoreDestructiveFile != "" {
		args = append(args, "--ignore-destructive-file", req.IgnoreDestructiveFile)
	}
	return args
}

func combineOutput(stdout, stderr []byte) string {
	raw := string(stdout)
	if len(stderr) > 0 {
		if raw != "" {
			raw += "\n"
		}
		raw += string(stderr)
	}
	return raw
}

// discoverArtifacts locates sgd's fixed output layout under outputDir:
// package/package.xml (its path is always returned — never conditioned on
// a disk check, since a successful sgd run always writes it) and
// destructiveChanges/destructiveChanges.xml (returned only when present on
// disk). destructiveChanges/package.xml — sgd also writes this alongside
// destructiveChanges.xml — is deliberately never referenced: the additive
// package.xml lives ONLY under package/.
func discoverArtifacts(outputDir, raw string) Result {
	packagePath := filepath.Join(outputDir, "package", "package.xml")
	destructivePath := filepath.Join(outputDir, "destructiveChanges", "destructiveChanges.xml")

	if _, err := os.Stat(destructivePath); err != nil {
		destructivePath = ""
	}

	return Result{
		PackageXMLPath:         packagePath,
		DestructiveChangesPath: destructivePath,
		Raw:                    raw,
	}
}
