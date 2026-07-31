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
