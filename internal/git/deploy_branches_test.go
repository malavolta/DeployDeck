package git

// deploy_branches_test.go pins parseDeployBranches — the PURE parse half of
// ListDeployBranches (design.md's "deploy/* lister — single-pass" section):
// no git process runs here, only raw `git for-each-ref
// --format='%(refname:short)|%(committerdate:iso-strict)'` output already
// captured as a []byte. Table-driven, white-box (package git, not git_test)
// since parseDeployBranches is unexported.

import (
	"reflect"
	"testing"
	"time"
)

// TestParseDeployBranches_StripsOriginAndSetsPushed is task 1.13 (RED):
// parseDeployBranches strips the "origin/" prefix off remote-tracking
// entries, correlates a local head with its remote-tracking counterpart by
// short name, and sets Pushed when a remote-tracking entry was seen —
// exactly the ref-existence signal BranchExists/revParseVerify already use
// elsewhere in this package.
func TestParseDeployBranches_StripsOriginAndSetsPushed(t *testing.T) {
	mustParse := func(t *testing.T, s string) time.Time {
		t.Helper()
		ts, err := time.Parse(time.RFC3339, s)
		if err != nil {
			t.Fatalf("test setup: parsing %q as RFC3339: %v", s, err)
		}
		return ts
	}

	tests := []struct {
		name string
		raw  string
		want []DeployBranch
	}{
		{
			name: "local-only branch is not pushed",
			raw:  "deploy/DD-1|2026-07-01T10:00:00Z\n",
			want: []DeployBranch{{Name: "deploy/DD-1", LastCommit: mustParse(t, "2026-07-01T10:00:00Z"), Pushed: false}},
		},
		{
			name: "pushed branch correlates local head with remote and strips origin/",
			raw:  "deploy/DD-2|2026-07-02T10:00:00Z\norigin/deploy/DD-2|2026-07-02T10:00:00Z\n",
			want: []DeployBranch{{Name: "deploy/DD-2", LastCommit: mustParse(t, "2026-07-02T10:00:00Z"), Pushed: true}},
		},
		{
			name: "remote-only branch (local deleted) still strips origin/ and reports pushed",
			raw:  "origin/deploy/DD-3|2026-07-03T10:00:00Z\n",
			want: []DeployBranch{{Name: "deploy/DD-3", LastCommit: mustParse(t, "2026-07-03T10:00:00Z"), Pushed: true}},
		},
		{
			name: "blank lines around and between entries are skipped",
			raw:  "\n\ndeploy/DD-4|2026-07-04T10:00:00Z\n\n",
			want: []DeployBranch{{Name: "deploy/DD-4", LastCommit: mustParse(t, "2026-07-04T10:00:00Z"), Pushed: false}},
		},
		{
			name: "empty input yields no branches",
			raw:  "",
			want: nil,
		},
		{
			name: "malformed line (no separator) is skipped, valid lines still parse",
			raw:  "not-a-valid-line-without-separator\ndeploy/DD-5|2026-07-05T10:00:00Z\n",
			want: []DeployBranch{{Name: "deploy/DD-5", LastCommit: mustParse(t, "2026-07-05T10:00:00Z"), Pushed: false}},
		},
		{
			name: "multiple independent branches are all reported",
			raw:  "deploy/DD-6|2026-07-06T10:00:00Z\ndeploy/DD-7|2026-07-07T10:00:00Z\norigin/deploy/DD-7|2026-07-07T10:00:00Z\n",
			want: []DeployBranch{
				{Name: "deploy/DD-6", LastCommit: mustParse(t, "2026-07-06T10:00:00Z"), Pushed: false},
				{Name: "deploy/DD-7", LastCommit: mustParse(t, "2026-07-07T10:00:00Z"), Pushed: true},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseDeployBranches([]byte(tt.raw))
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseDeployBranches(%q) = %+v, want %+v", tt.raw, got, tt.want)
			}
		})
	}
}
