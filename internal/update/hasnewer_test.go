package update_test

import (
	"testing"

	"deploydeck/internal/update"
)

// TestHasNewer proves the hand-rolled semver comparison used by the
// non-blocking update notice: newer/older/equal detection, tolerant of a
// leading "v", never nags on the "dev" default build, and never panics on a
// malformed version string (HU-019, update-notification: Semver-Based
// Newer-Version Detection + Silent Skip discipline).
func TestHasNewer(t *testing.T) {
	tests := []struct {
		name    string
		current string
		latest  string
		want    bool
	}{
		{name: "latest patch is newer", current: "1.0.0", latest: "1.1.0", want: true},
		{name: "equal versions are not newer", current: "1.0.0", latest: "1.0.0", want: false},
		{name: "latest is older", current: "1.1.0", latest: "1.0.0", want: false},
		{name: "leading v prefix on both sides is stripped", current: "v1.0.0", latest: "v1.1.0", want: true},
		{name: "dev current never nags", current: "dev", latest: "1.0.0", want: false},
		{name: "malformed latest never panics, treated as not-newer", current: "1.0.0", latest: "not-a-version", want: false},
		{name: "malformed current never panics, treated as not-newer", current: "not-a-version", latest: "1.0.0", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := update.HasNewer(tt.current, tt.latest); got != tt.want {
				t.Errorf("HasNewer(%q, %q) = %v, want %v", tt.current, tt.latest, got, tt.want)
			}
		})
	}
}
