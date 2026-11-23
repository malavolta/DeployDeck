package main

import (
	"errors"
	"testing"
)

// TestDecideUpdate covers the pure decision logic newCheckUpdate
// delegates to after fetching: given a fetch error, decideUpdate degrades
// silently; given a successful fetch, it defers to update.HasNewer's
// current-vs-latest comparison (including the "dev" no-nag default).
func TestDecideUpdate(t *testing.T) {
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
			hasUpdate, latest, err := decideUpdate(tt.current, tt.latest, tt.fetchErr)

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
