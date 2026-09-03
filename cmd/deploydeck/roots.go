package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/malavolta/DeployDeck/internal/config"
	"github.com/malavolta/DeployDeck/internal/exec"
	"github.com/malavolta/DeployDeck/internal/git"
	"github.com/malavolta/DeployDeck/internal/github"
	"github.com/malavolta/DeployDeck/internal/prereq"
	"github.com/malavolta/DeployDeck/internal/salesforce"
)

// roots is the immutable quadruple design.md's Resolution Sequence
// produces: the true git repository top level, the SFDX project root, the
// artifacts root .deploydeck/ actually lives under, and the directory
// deploydeck.yaml was actually LOCATED in. Every downstream consumer
// (app.Deps, prereq.Checker) reads the ONE root it semantically needs from
// this quadruple instead of a single conflated directory.
//
// ConfigDir (config-validation-wiring ADR-3) is a genuinely distinct fact,
// not a redundant alias for one of the other three: config.Locate searches
// UPWARD from cwd, so in a nested layout with an explicit projectDir,
// ConfigDir can differ in VALUE from GitRoot, ProjectDir and ArtifactsRoot
// all at once.
type roots struct {
	GitRoot       string
	ProjectDir    string
	ArtifactsRoot string
	ConfigDir     string
}

// resolveRoots implements design.md's Resolution Sequence, always called
// with the RAW cwd — never re-derived from an already-resolved root (ADR-5:
// re-resolving from projectDir can miss a sibling configDir when
// deploydeck.yaml and projectDir diverge).
//
//	os.Getwd() -> cwd
//	git.RepoRoot(ctx, cwd)
//	  err (not inside a git repo) -> gitRoot = cwd ; stopDir = cwd (probe cwd only)
//	  ok                          -> gitRoot = <toplevel> ; stopDir = gitRoot
//	config.Locate(cwd, stopDir) -> configDir (found), or cwd (not found — Load
//	  then emits today's EXACT "config: reading <cwd>/deploydeck.yaml" error,
//	  never a confusing "not found while searching upward" message)
//	config.Load(configDir) -> cfg
//	cfg.ProjectRoot(gitRoot, configDir) -> projectDir, or an actionable error
//	  (absolute/".." projectDir), aborting HERE — before any command runs,
//	  since resolveRoots is a pure resolution step with no side effects
//	artifactsRoot = projectDir (design.md ADR-2)
func resolveRoots(ctx context.Context, g *git.Service, cwd string) (roots, config.Config, error) {
	// Canonicalise the cwd FIRST, before anything compares paths as strings.
	// `git rev-parse --show-toplevel` reports the symlink-RESOLVED path while
	// os.Getwd() reports the LOGICAL one, so on a symlinked checkout the two
	// differ textually even though they name the same directory. Left
	// unresolved, config.Locate's lexical bound check (isAncestorOrSelf) would
	// see stopDir as unrelated to startDir, collapse the search to startDir
	// alone, and silently disable the upward deploydeck.yaml search — the
	// operator would be told the config is missing while it sits one level up
	// inside the same repository. Canonicalising here puts every subsequent
	// comparison in one path space.
	//
	// A failure degrades to the raw cwd rather than aborting: EvalSymlinks
	// errors when the path does not exist, and that condition is already
	// reported far more usefully downstream (by RepoRoot's "not inside a git
	// repository" check, or by Load's own read error).
	if resolved, err := filepath.EvalSymlinks(cwd); err == nil {
		cwd = resolved
	}

	gitRoot := cwd
	stopDir := cwd
	if top, err := g.RepoRoot(ctx, cwd); err == nil {
		gitRoot = top
		stopDir = top
	}

	configDir := cwd
	if found, ok := config.Locate(cwd, stopDir); ok {
		configDir = found
	}

	cfg, err := config.Load(configDir)
	if err != nil {
		return roots{}, config.Config{}, err
	}

	projectDir, err := cfg.ProjectRoot(gitRoot, configDir)
	if err != nil {
		return roots{}, config.Config{}, fmt.Errorf("resolving projectDir: %w", err)
	}

	return roots{
		GitRoot:       gitRoot,
		ProjectDir:    projectDir,
		ArtifactsRoot: projectDir, // ADR-2: .deploydeck/ must never relocate
		ConfigDir:     configDir,  // config-validation-wiring ADR-3: computed above, now returned instead of discarded
	}, cfg, nil
}

// newChecker composes a *prereq.Checker from an ALREADY-RESOLVED cfg/roots —
// pure composition, no resolution of its own (mirrors design.md's
// signature). Kept as a package-level func, not a method, so
// defaultRunTUI's app.Deps.NewChecker closure can capture a fixed (cfg, r)
// pair without re-deriving anything (ADR-5).
func newChecker(cfg config.Config, r roots) (*prereq.Checker, error) {
	runner := exec.NewOSRunner()

	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown-host"
	}
	self := prereq.LockInfo{PID: os.Getpid(), PName: "deploydeck", Host: hostname}
	// The lock lives under the ARTIFACTS root, never the git root — it must
	// not relocate for any existing install (design.md ADR-2).
	lock := prereq.NewLock(filepath.Join(r.ArtifactsRoot, ".deploydeck", "lock"), self, prereq.NewOSProcessProber(runner))

	return &prereq.Checker{
		GitRoot:       r.GitRoot,
		ArtifactsRoot: r.ArtifactsRoot,
		Git:           git.New(runner),
		SF:            salesforce.New(runner),
		Config:        cfg,
		// ConfigPath composes from r.ConfigDir (config-validation-wiring
		// ADR-3), never GitRoot or ProjectDir, so CheckConfig's FixCommand
		// names the file config.Locate actually found — a bare filename
		// would be ambiguous the moment ConfigDir diverges from the other
		// roots (nested layout with an explicit projectDir).
		ConfigPath: filepath.Join(r.ConfigDir, config.FileName),
		Lock:       lock,
		// GH backs the informative, non-blocking gh doctor check (HU-014).
		GH: github.New(runner),
		// AI backs the informative, non-blocking AI model doctor check
		// (ai-pr-summary); nil (composeAIClient's degrade) when cfg.AI is
		// disabled/absent, mirroring GH's nil-skip discipline.
		AI: composeAIClient(cfg),
	}, nil
}
