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

// OwnerRepo derives HOST-QUALIFIED "host/owner/repo" from originURL (the
// configured `origin` remote's URL) — a thin wrapper over parseOrigin
// (design.md's "owner/repo helpers" decision). The host MUST stay in the
// result: the pr-provenance signature payload binds to this string, and a
// host-stripped "owner/repo" would let a marker signed for
// github.com/org/repo verify unchanged on an Enterprise host sharing the
// same org/repo/branch/runID. Casing is preserved exactly as parsed —
// lowercasing (owner/repo is GitHub-case-insensitive) happens in
// provenance.Sign, not here. An unrecognized origin form returns ok=false,
// never an error: callers degrade gracefully (e.g. pr-provenance's Compose
// writes a footer-only body, no marker, rather than a marker whose
// signature could not be bound to a real repository).
func OwnerRepo(originURL string) (ownerRepo string, ok bool) {
	host, org, repo, ok := parseOrigin(originURL)
	if !ok {
		return "", false
	}
	return host + "/" + org + "/" + repo, true
}

// prURLPattern matches a PR URL's ANCHORED shape:
// https://<host>/<owner>/<repo>/pull/<n>(/), tolerating an optional trailing
// slash. This is a DIFFERENT shape than an origin remote URL (no
// git@/ssh:// forms, a mandatory /pull/<n> suffix), so it is a distinct
// pattern from parseOrigin's — a PR URL rejects parseOrigin's own
// `$`-anchored regexes (design.md's "owner/repo helpers" decision).
var prURLPattern = regexp.MustCompile(`^https://([^/]+)/([^/]+)/([^/]+)/pull/([0-9]+)/?$`)

// ParsePRURL derives HOST-QUALIFIED "host/owner/repo" from a
// `deploydeck pr verify <url>` argument — the verify-side counterpart of
// OwnerRepo's host-qualification: dropping the host here would let a marker
// signed for one host verify unchanged against a PR URL on a different host
// sharing the same org/repo/branch/runID. Any non-matching shape (a repo
// root, an issues URL, a non-numeric PR "number", or plain garbage) returns
// ok=false, never an error — the caller reports a clear degraded message
// rather than a raw parse failure.
func ParsePRURL(prURL string) (ownerRepo string, ok bool) {
	m := prURLPattern.FindStringSubmatch(prURL)
	if m == nil {
		return "", false
	}
	return m[1] + "/" + m[2] + "/" + m[3], true
}
