package github_test

import (
	"testing"

	"github.com/malavolta/DeployDeck/internal/github"
)

// TestCompareURL is task 2.1 (RED): a load-bearing table test covering every
// recognized origin form — including the ENTERPRISE host github.ibm.com,
// since the user's own gh targets that host, not github.com — plus the
// graceful-failure path for an unrecognized origin (HU-014 AC6).
func TestCompareURL(t *testing.T) {
	tests := []struct {
		name      string
		originURL string
		base      string
		head      string
		want      string
		wantErr   bool
	}{
		{
			name:      "github.com SSH",
			originURL: "git@github.com:org/repo.git",
			base:      "main",
			head:      "deploy/PROJ-1-to-main",
			want:      "https://github.com/org/repo/compare/main...deploy/PROJ-1-to-main",
		},
		{
			name:      "github.com HTTPS",
			originURL: "https://github.com/org/repo",
			base:      "UAT",
			head:      "deploy/PROJ-1-to-UAT",
			want:      "https://github.com/org/repo/compare/UAT...deploy/PROJ-1-to-UAT",
		},
		{
			name:      "github.com HTTPS with trailing .git",
			originURL: "https://github.com/org/repo.git",
			base:      "main",
			head:      "feature",
			want:      "https://github.com/org/repo/compare/main...feature",
		},
		{
			name:      "ssh:// form with trailing .git",
			originURL: "ssh://git@github.com/org/repo.git",
			base:      "main",
			head:      "deploy/PROJ-4-to-main",
			want:      "https://github.com/org/repo/compare/main...deploy/PROJ-4-to-main",
		},
		{
			// Enterprise SSH host — MUST derive github.ibm.com, not a
			// hardcoded github.com. This is the load-bearing case: the
			// user's own gh instance targets this host.
			name:      "Enterprise SSH host (github.ibm.com)",
			originURL: "git@github.ibm.com:org/repo.git",
			base:      "main",
			head:      "deploy/PROJ-2-to-main",
			want:      "https://github.ibm.com/org/repo/compare/main...deploy/PROJ-2-to-main",
		},
		{
			// Enterprise HTTPS host — same host-derivation requirement.
			name:      "Enterprise HTTPS host (github.ibm.com)",
			originURL: "https://github.ibm.com/org/repo",
			base:      "main",
			head:      "deploy/PROJ-3-to-main",
			want:      "https://github.ibm.com/org/repo/compare/main...deploy/PROJ-3-to-main",
		},
		{
			name:      "unrecognized origin form fails gracefully",
			originURL: "not-a-valid-origin-url",
			base:      "main",
			head:      "feature",
			wantErr:   true,
		},
		{
			name:      "empty origin fails gracefully",
			originURL: "",
			base:      "main",
			head:      "feature",
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := github.CompareURL(tt.originURL, tt.base, tt.head)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("CompareURL(%q) expected an error, got %q", tt.originURL, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("CompareURL(%q): unexpected error: %v", tt.originURL, err)
			}
			if got != tt.want {
				t.Fatalf("CompareURL(%q) = %q, want %q", tt.originURL, got, tt.want)
			}
		})
	}
}

// TestOwnerRepo is task 2.1 (RED): pr-provenance's thin wrapper over
// parseOrigin, exercising the same SSH/HTTPS/ssh:// origin forms
// TestCompareURL already covers, plus the junk-origin degrade.
//
// HOST-QUALIFIED (remediation): OwnerRepo now returns "host/owner/repo", not
// bare "owner/repo" — a signature payload built from a host-STRIPPED
// ownerRepo would let a marker signed for github.com/org/repo verify on an
// Enterprise host sharing the same org/repo/branch/runID. Casing is
// otherwise preserved exactly as parsed (lowering happens in
// provenance.Sign, not here).
func TestOwnerRepo(t *testing.T) {
	tests := []struct {
		name      string
		originURL string
		want      string
		wantOK    bool
	}{
		{"github.com SSH", "git@github.com:org/repo.git", "github.com/org/repo", true},
		{"github.com HTTPS", "https://github.com/org/repo", "github.com/org/repo", true},
		{"ssh:// form with trailing .git", "ssh://git@github.com/org/repo.git", "github.com/org/repo", true},
		{"Enterprise SSH host", "git@github.ibm.com:org/repo.git", "github.ibm.com/org/repo", true},
		{"host-qualified, owner/repo casing preserved as parsed", "git@github.com:Org/Repo.git", "github.com/Org/Repo", true},
		{"junk origin", "not-a-valid-origin-url", "", false},
		{"empty origin", "", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := github.OwnerRepo(tt.originURL)
			if ok != tt.wantOK {
				t.Fatalf("OwnerRepo(%q) ok = %v, want %v", tt.originURL, ok, tt.wantOK)
			}
			if ok && got != tt.want {
				t.Fatalf("OwnerRepo(%q) = %q, want %q", tt.originURL, got, tt.want)
			}
		})
	}
}

// TestParsePRURL is task 2.1 (RED): ParsePRURL derives owner/repo from a PR
// URL's ANCHORED shape (https://<host>/<owner>/<repo>/pull/<n>) — a
// different shape than an origin remote, so parseOrigin's own patterns
// must NOT be reused as-is (design's "owner/repo helpers" decision).
//
// HOST-QUALIFIED (remediation): ParsePRURL now returns "host/owner/repo",
// not bare "owner/repo" — the verify-side counterpart of OwnerRepo's fix,
// so a marker copied to a PR on a different host with the same org/repo/
// branch/runID fails verification instead of matching.
func TestParsePRURL(t *testing.T) {
	tests := []struct {
		name   string
		prURL  string
		want   string
		wantOK bool
	}{
		{"github.com PR URL", "https://github.com/org/repo/pull/42", "github.com/org/repo", true},
		{"Enterprise host PR URL", "https://github.ibm.com/org/repo/pull/7", "github.ibm.com/org/repo", true},
		{"trailing slash", "https://github.com/org/repo/pull/42/", "github.com/org/repo", true},
		{"arbitrary enterprise host", "https://ghe.corp/org/repo/pull/7", "ghe.corp/org/repo", true},
		{"non-PR path (repo root)", "https://github.com/org/repo", "", false},
		{"non-PR path (issues)", "https://github.com/org/repo/issues/42", "", false},
		{"non-numeric PR number", "https://github.com/org/repo/pull/abc", "", false},
		{"empty URL", "", "", false},
		{"garbage", "not-a-url-at-all", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := github.ParsePRURL(tt.prURL)
			if ok != tt.wantOK {
				t.Fatalf("ParsePRURL(%q) ok = %v, want %v", tt.prURL, ok, tt.wantOK)
			}
			if ok && got != tt.want {
				t.Fatalf("ParsePRURL(%q) = %q, want %q", tt.prURL, got, tt.want)
			}
		})
	}
}
