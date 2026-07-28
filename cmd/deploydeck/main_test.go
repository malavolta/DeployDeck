package main

import (
	"context"
	"errors"
	"testing"

	"deploydeck/internal/version"
)

// TestDefaultCheckUpdate_DegradesOnError proves defaultCheckUpdate degrades
// to (false, "", non-nil error) on any check failure — here a pre-canceled
// context, so the composition seam is proven WITHOUT hitting the real
// network (update-notification: Silent Skip on Check Failure).
func TestDefaultCheckUpdate_DegradesOnError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	hasUpdate, latest, err := defaultCheckUpdate(ctx)

	if err == nil {
		t.Fatal("expected a non-nil error from a pre-canceled context, got nil")
	}
	if hasUpdate {
		t.Errorf("hasUpdate = true, want false on check failure")
	}
	if latest != "" {
		t.Errorf("latest = %q, want empty string on check failure", latest)
	}
}

// TestDecideUpdate covers the pure decision logic defaultCheckUpdate
// delegates to after fetching: given a fetch error, decideUpdate degrades
// silently; given a successful fetch, it defers to update.HasNewer's
// current-vs-latest comparison (including the "dev" no-nag default).
func TestDecideUpdate(t *testing.T) {
	orig := version.Version
	defer func() { version.Version = orig }()

	tests := []struct {
		name          string
		current       string
		latest        string
		fetchErr      error
		wantHasUpdate bool
		wantLatest    string
		wantErr       bool
	}{
		{
			name:          "fetch error degrades silently",
			current:       "1.0.0",
			latest:        "",
			fetchErr:      errors.New("boom"),
			wantHasUpdate: false,
			wantLatest:    "",
			wantErr:       true,
		},
		{
			name:          "latest newer than current",
			current:       "1.0.0",
			latest:        "1.1.0",
			fetchErr:      nil,
			wantHasUpdate: true,
			wantLatest:    "1.1.0",
			wantErr:       false,
		},
		{
			name:          "latest equal to current",
			current:       "1.0.0",
			latest:        "1.0.0",
			fetchErr:      nil,
			wantHasUpdate: false,
			wantLatest:    "1.0.0",
			wantErr:       false,
		},
		{
			name:          "latest older than current",
			current:       "1.5.0",
			latest:        "1.0.0",
			fetchErr:      nil,
			wantHasUpdate: false,
			wantLatest:    "1.0.0",
			wantErr:       false,
		},
		{
			name:          "dev build never nags",
			current:       "dev",
			latest:        "9.9.9",
			fetchErr:      nil,
			wantHasUpdate: false,
			wantLatest:    "9.9.9",
			wantErr:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			version.Version = tt.current

			hasUpdate, latest, err := decideUpdate(tt.latest, tt.fetchErr)

			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if hasUpdate != tt.wantHasUpdate {
				t.Errorf("hasUpdate = %v, want %v", hasUpdate, tt.wantHasUpdate)
			}
			if latest != tt.wantLatest {
				t.Errorf("latest = %q, want %q", latest, tt.wantLatest)
			}
		})
	}
}
