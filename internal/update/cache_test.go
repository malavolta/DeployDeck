package update_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/malavolta/DeployDeck/internal/update"
)

var cacheNow = time.Date(2026, 11, 5, 12, 0, 0, 0, time.UTC)

func writeCache(t *testing.T, dir, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "update-check.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func entry(latest string, age time.Duration) string {
	return `{"checked_at":"` + cacheNow.Add(-age).Format(time.RFC3339) + `","latest":"` + latest + `"}`
}

func TestCache_Latest(t *testing.T) {
	tests := []struct {
		name        string
		body        string // initial cache file, "" for none
		fetchLatest string
		fetchErr    error
		want        string
		wantFetches int
		wantFile    bool
	}{
		{"fresh", entry("v1.2.0", time.Hour), "v1.3.0", nil, "v1.2.0", 0, true},
		{"stale", entry("v1.2.0", 25*time.Hour), "v1.3.0", nil, "v1.3.0", 1, true},
		{"future", entry("v1.2.0", -time.Hour), "v1.3.0", nil, "v1.3.0", 1, true},
		{"corrupt", "{nope", "v1.3.0", nil, "v1.3.0", 1, true},
		{"missing", "", "v1.3.0", nil, "v1.3.0", 1, true},
		{"fetch error", "", "", errors.New("boom"), "", 1, false},
		{"empty latest", "", "", nil, "", 1, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			if tt.body != "" {
				writeCache(t, dir, tt.body)
			}
			n := 0
			fetch := func(context.Context) (string, error) { n++; return tt.fetchLatest, tt.fetchErr }
			c := update.Cache{Dir: dir, Now: func() time.Time { return cacheNow }}
			got, err := c.Latest(context.Background(), fetch)
			if (err != nil) != (tt.fetchErr != nil) || got != tt.want || n != tt.wantFetches {
				t.Fatalf("got %q, %v, fetches %d; want %q, fetches %d", got, err, n, tt.want, tt.wantFetches)
			}
			_, statErr := os.Stat(filepath.Join(dir, "update-check.json"))
			if (statErr == nil) != tt.wantFile {
				t.Fatalf("cache file exists = %v, want %v", statErr == nil, tt.wantFile)
			}
			if tt.wantFetches == 1 && tt.want != "" { // refreshed: next call is served from the file
				if again, _ := c.Latest(context.Background(), fetch); again != tt.want || n != 1 {
					t.Fatalf("second call got %q, fetches %d; want cached %q", again, n, tt.want)
				}
			}
		})
	}
}

func TestCache_Latest_NoWriteCases(t *testing.T) {
	blocker := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	for name, dir := range map[string]string{"unwritable dir": filepath.Join(blocker, "sub"), "no dir": ""} {
		t.Run(name, func(t *testing.T) {
			n := 0
			c := update.Cache{Dir: dir, Now: func() time.Time { return cacheNow }}
			for i := 0; i < 2; i++ {
				got, err := c.Latest(context.Background(), func(context.Context) (string, error) { n++; return "v1.3.0", nil })
				if err != nil || got != "v1.3.0" {
					t.Fatalf("got %q, %v; want v1.3.0, nil", got, err)
				}
			}
			if n != 2 {
				t.Fatalf("fetches = %d, want 2 (always live)", n)
			}
		})
	}
}
