package git

import "testing"

// TestTicketFromBranch covers the branch->ticket auto-suggest matching used
// to pre-fill StateTicketInput (app package) from the current branch name.
func TestTicketFromBranch(t *testing.T) {
	tests := []struct {
		name     string
		branch   string
		patterns []string
		want     string
	}{
		{
			name:     "matches a configured pattern",
			branch:   "DEMO-2-mi-cambio",
			patterns: []string{"DEMO-[0-9]+"},
			want:     "DEMO-2",
		},
		{
			name:     "no pattern matches",
			branch:   "feature/no-ticket-here",
			patterns: []string{"DEMO-[0-9]+"},
			want:     "",
		},
		{
			name:     "invalid pattern is skipped without panicking",
			branch:   "DEMO-2-mi-cambio",
			patterns: []string{"[invalid(", "DEMO-[0-9]+"},
			want:     "DEMO-2",
		},
		{
			name:     "first of multiple matching patterns wins",
			branch:   "DEMO-2-ABC-9-mi-cambio",
			patterns: []string{"DEMO-[0-9]+", "ABC-[0-9]+"},
			want:     "DEMO-2",
		},
		{
			name:     "empty branch",
			branch:   "",
			patterns: []string{"DEMO-[0-9]+"},
			want:     "",
		},
		{
			name:     "no patterns configured",
			branch:   "DEMO-2-mi-cambio",
			patterns: nil,
			want:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := TicketFromBranch(tt.branch, tt.patterns)
			if got != tt.want {
				t.Errorf("TicketFromBranch(%q, %v) = %q, want %q", tt.branch, tt.patterns, got, tt.want)
			}
		})
	}
}
