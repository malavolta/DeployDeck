package git

import (
	"context"
	"fmt"
	"strings"
)

// Branch is a local or remote-tracking branch reference.
type Branch struct {
	// Name is the short ref name: "main" for a local branch,
	// "origin/main" for a remote-tracking branch.
	Name   string
	Remote bool
}

// ListBranches returns local and remote-tracking branches for the
// repository containing dir. Symbolic refs such as "origin/HEAD" are
// excluded.
func (s *Service) ListBranches(ctx context.Context, dir string) ([]Branch, error) {
	root, err := s.RepoRoot(ctx, dir)
	if err != nil {
		return nil, err
	}

	return s.branchesByPattern(ctx, root, "")
}

// CandidateBranches returns local and remote-tracking branches whose name
// contains ticket (HU-002: "Buscar ramas remotas y locales que contengan
// el ticket"), via `git branch --list '*<ticket>*'` for each of local and
// remote-tracking refs — the same underlying listing ListBranches uses,
// scoped with a glob pattern. A pushed branch present both locally and as
// its origin/ counterpart is collapsed into a SINGLE candidate
// (dedupeByLocalName) so a logical branch is never counted twice — scoped to
// CandidateBranches ONLY; ListBranches/branchesByPattern stay unmodified,
// since the standalone-delta base picker requires local+remote as separate
// rows.
func (s *Service) CandidateBranches(ctx context.Context, dir, ticket string) ([]Branch, error) {
	root, err := s.RepoRoot(ctx, dir)
	if err != nil {
		return nil, err
	}

	branches, err := s.branchesByPattern(ctx, root, "*"+ticket+"*")
	if err != nil {
		return nil, err
	}
	return dedupeByLocalName(branches), nil
}

// dedupeByLocalName collapses a local branch and its origin/ remote-tracking
// twin (e.g. "X" and "origin/X") into a single entry, keeping the bare/local
// form. Genuinely distinct branches are never collapsed, and first-seen
// order is preserved. Scoped to CandidateBranches only — never called by
// ListBranches/branchesByPattern.
func dedupeByLocalName(branches []Branch) []Branch {
	local := make(map[string]bool, len(branches))
	for _, b := range branches {
		if !b.Remote {
			local[b.Name] = true
		}
	}

	deduped := make([]Branch, 0, len(branches))
	for _, b := range branches {
		if b.Remote && local[strings.TrimPrefix(b.Name, "origin/")] {
			continue
		}
		deduped = append(deduped, b)
	}
	return deduped
}

// branchesByPattern lists local and remote-tracking branches already
// resolved to root, optionally scoped to a `git branch --list` glob
// pattern ("" lists everything, matching prior ListBranches behavior).
// Symbolic refs such as "origin/HEAD" are always excluded.
func (s *Service) branchesByPattern(ctx context.Context, root, pattern string) ([]Branch, error) {
	local, err := s.branchNames(ctx, root, false, pattern)
	if err != nil {
		return nil, err
	}
	remote, err := s.branchNames(ctx, root, true, pattern)
	if err != nil {
		return nil, err
	}

	branches := make([]Branch, 0, len(local)+len(remote))
	for _, name := range local {
		branches = append(branches, Branch{Name: name, Remote: false})
	}
	for _, name := range remote {
		if strings.HasSuffix(name, "/HEAD") {
			continue
		}
		branches = append(branches, Branch{Name: name, Remote: true})
	}

	return branches, nil
}

func (s *Service) branchNames(ctx context.Context, root string, remote bool, pattern string) ([]string, error) {
	args := []string{"branch", "--format=%(refname:short)"}
	if remote {
		args = append(args, "-r")
	}
	if pattern != "" {
		args = append(args, "--list", pattern)
	}

	req := newRequest(root, args...)
	result, err := s.runner.Run(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("git: listing branches (remote=%v) in %s: %w", remote, root, err)
	}
	if result.ExitCode != 0 {
		return nil, fmt.Errorf("git: listing branches (remote=%v) in %s: %s", remote, root, strings.TrimSpace(string(result.Stderr)))
	}

	var names []string
	for _, line := range strings.Split(string(result.Stdout), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		names = append(names, line)
	}
	return names, nil
}
