package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/malavolta/DeployDeck/internal/config"
)

func writeLocateFixture(t *testing.T, dir string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, config.FileName), []byte("branches: {}\n"), 0o644); err != nil {
		t.Fatalf("writing fixture: %v", err)
	}
}

// TestLocate is task 1.1 (RED): Locate walks upward from startDir, bounded
// at stopDir, probing for deploydeck.yaml (design.md ADR-1). Pure —
// t.TempDir() only, no git binary involved; the "git-root bound" scenarios
// simulate the boundary directly via stopDir, since Locate itself never
// resolves it.
func TestLocate(t *testing.T) {
	t.Run("found at startDir", func(t *testing.T) {
		root := t.TempDir()
		writeLocateFixture(t, root)

		dir, found := config.Locate(root, root)

		if !found {
			t.Fatal("expected found=true")
		}
		if dir != root {
			t.Errorf("dir = %q, want %q", dir, root)
		}
	})

	t.Run("found after walking up to the git-root bound", func(t *testing.T) {
		root := t.TempDir()
		writeLocateFixture(t, root)
		sub := filepath.Join(root, "project")
		if err := os.MkdirAll(sub, 0o755); err != nil {
			t.Fatalf("creating subdir: %v", err)
		}

		dir, found := config.Locate(sub, root)

		if !found {
			t.Fatal("expected found=true")
		}
		if dir != root {
			t.Errorf("dir = %q, want %q", dir, root)
		}
	})

	t.Run("found in a subdir with the bound at the git root", func(t *testing.T) {
		root := t.TempDir()
		sub := filepath.Join(root, "project")
		if err := os.MkdirAll(sub, 0o755); err != nil {
			t.Fatalf("creating subdir: %v", err)
		}
		writeLocateFixture(t, sub)

		dir, found := config.Locate(sub, root)

		if !found {
			t.Fatal("expected found=true")
		}
		if dir != sub {
			t.Errorf("dir = %q, want %q", dir, sub)
		}
	})

	t.Run("not found anywhere between startDir and stopDir", func(t *testing.T) {
		root := t.TempDir()
		sub := filepath.Join(root, "project")
		if err := os.MkdirAll(sub, 0o755); err != nil {
			t.Fatalf("creating subdir: %v", err)
		}

		dir, found := config.Locate(sub, root)

		if found {
			t.Fatalf("expected found=false, got dir=%q", dir)
		}
	})

	t.Run("empty stopDir probes startDir only", func(t *testing.T) {
		root := t.TempDir()
		writeLocateFixture(t, root)
		sub := filepath.Join(root, "project")
		if err := os.MkdirAll(sub, 0o755); err != nil {
			t.Fatalf("creating subdir: %v", err)
		}

		dir, found := config.Locate(sub, "")

		if found {
			t.Fatalf("expected found=false (no upward walk with empty stopDir), got dir=%q", dir)
		}
	})

	t.Run("stopDir not an ancestor of startDir probes startDir only", func(t *testing.T) {
		root := t.TempDir()
		writeLocateFixture(t, root)
		unrelated := t.TempDir()
		writeLocateFixture(t, unrelated)

		dir, found := config.Locate(unrelated, root)

		if !found {
			t.Fatal("expected found=true (startDir itself has the config)")
		}
		if dir != unrelated {
			t.Errorf("dir = %q, want %q (must not walk past startDir when stopDir is not an ancestor)", dir, unrelated)
		}
	})
}
