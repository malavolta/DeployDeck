// Package github wraps the gh CLI over internal/exec.Runner — a sibling of
// internal/delta and internal/salesforce, since gh is a distinct external
// binary with its own flags and output shape (see design.md's "gh lives in
// a new module" decision). It owns gh detection/auth-status, PR creation,
// and a PURE compare-URL normalizer. HU-014's push-pr-preparation composes
// this Client; internal/app never execs gh directly.
package github

import (
	"fmt"
	"regexp"
)

// Origin URL patterns CompareURL recognizes. All three derive the SAME
// (host, org, repo) shape; the trailing ".git" suffix (group 4, unused) and
// an optional trailing slash are both tolerated in every form.
var (
	sshOriginPattern    = regexp.MustCompile(`^git@([^:/]+):([^/]+)/([^/]+?)(\.git)?/?$`)
	httpsOriginPattern  = regexp.MustCompile(`^https://([^/]+)/([^/]+)/([^/]+?)(\.git)?/?$`)
	sshURLOriginPattern = regexp.MustCompile(`^ssh://git@([^/]+)/([^/]+)/([^/]+?)(\.git)?/?$`)
)

// CompareURL derives a GitHub-style compare URL from originURL — the
// configured `origin` remote's URL (see git.Service.RemoteURL) — plus base
// and head branch names:
//
//	https://<host>/<org>/<repo>/compare/<base>...<head>
//
// It recognizes SSH (`git@<host>:org/repo(.git)`), HTTPS
// (`https://<host>/org/repo(.git)`), and `ssh://git@<host>/org/repo(.git)`
// origin forms, deriving the HOST FROM ORIGIN rather than hardcoding
// github.com — DeployDeck's own `gh` targets an Enterprise host
// (github.ibm.com), and hardcoding github.com would silently break that
// (design.md's "CompareURL derives host from origin" decision). It is PURE:
// no I/O, no network, no environment reads. An unrecognized origin form
// returns an error so the caller can fail gracefully — raw origin URL plus
// manual base/compare/title — instead of building a malformed link.
func CompareURL(originURL, base, head string) (string, error) {
	host, org, repo, ok := parseOrigin(originURL)
	if !ok {
		return "", fmt.Errorf("github: unrecognized origin URL form: %q", originURL)
	}

	return fmt.Sprintf("https://%s/%s/%s/compare/%s...%s", host, org, repo, base, head), nil
}

// parseOrigin tries each recognized origin pattern in turn and returns the
// first match's derived host/org/repo.
func parseOrigin(originURL string) (host, org, repo string, ok bool) {
	for _, pattern := range []*regexp.Regexp{sshOriginPattern, httpsOriginPattern, sshURLOriginPattern} {
		if m := pattern.FindStringSubmatch(originURL); m != nil {
			return m[1], m[2], m[3], true
		}
	}
	return "", "", "", false
}
