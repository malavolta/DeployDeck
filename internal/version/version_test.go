package version_test

import (
	"strings"
	"testing"

	"github.com/malavolta/DeployDeck/internal/version"
)

// TestString proves internal/version.String() formats the ldflags-injected
// build identity: it defaults to "dev" when the binary is built without any
// -X injection, and reports Version/Commit/Date once they are set (HU-019,
// release-versioning: Version Variables Package). Each subtest saves and
// restores the package-level vars, so this test must never run with
// t.Parallel.
func TestString(t *testing.T) {
	// Confirm the pristine package default before any subtest mutates it.
	if version.Version != "dev" {
		t.Fatalf("pristine default Version = %q, want %q", version.Version, "dev")
	}

	tests := []struct {
		name     string
		version  string
		commit   string
		date     string
		want     string
		contains []string
	}{
		{
			name:    "default build (no ldflags) reports dev",
			version: "dev",
			commit:  "",
			date:    "",
			want:    "dev",
		},
		{
			name:     "ldflags-injected version reports version, commit, and date",
			version:  "1.2.3",
			commit:   "abc",
			date:     "2026-01-01",
			contains: []string{"1.2.3", "abc", "2026-01-01"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			origVersion, origCommit, origDate := version.Version, version.Commit, version.Date
			defer func() {
				version.Version, version.Commit, version.Date = origVersion, origCommit, origDate
			}()

			version.Version = tt.version
			version.Commit = tt.commit
			version.Date = tt.date

			got := version.String()
			if tt.want != "" && got != tt.want {
				t.Errorf("String() = %q, want %q", got, tt.want)
			}
			for _, want := range tt.contains {
				if !strings.Contains(got, want) {
					t.Errorf("String() = %q, want it to contain %q", got, want)
				}
			}
		})
	}
}
