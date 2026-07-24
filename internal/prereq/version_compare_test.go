package prereq

import "testing"

func TestCompareVersions_TableDriven(t *testing.T) {
	tests := []struct {
		name string
		a    string
		b    string
		want int
	}{
		{name: "equal versions", a: "2.43.0", b: "2.43.0", want: 0},
		{name: "lower major", a: "1.9.0", b: "2.0.0", want: -1},
		{name: "higher patch", a: "2.43.1", b: "2.43.0", want: 1},
		{name: "numeric segment comparison, not lexical", a: "2.9.0", b: "2.10.0", want: -1},
		{name: "leading v prefix ignored", a: "v5.35.0", b: "5.34.0", want: 1},
		{name: "shorter version treated as zero-padded", a: "2.43", b: "2.43.0", want: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := compareVersions(tt.a, tt.b)
			if got != tt.want {
				t.Fatalf("compareVersions(%q, %q) = %d, want %d", tt.a, tt.b, got, tt.want)
			}
		})
	}
}
