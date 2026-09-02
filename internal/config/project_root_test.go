package config_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/malavolta/DeployDeck/internal/config"
)

// TestConfig_ProjectRoot is task 1.3 (RED): ProjectRoot composes the SFDX
// project root from gitRoot and the configured ProjectDir (design.md
// "Interfaces / Contracts"). Empty ProjectDir defaults to configDir
// (directory-resolution spec: "projectDir Defaults To The Config File's
// Directory"); an absolute or ".."-bearing value is rejected on this
// resolution path (directory-resolution spec: "projectDir Configuration
// Key").
func TestConfig_ProjectRoot(t *testing.T) {
	gitRoot := filepath.FromSlash("/repo")
	configDir := filepath.FromSlash("/repo/nested/config")

	t.Run("empty ProjectDir defaults to configDir", func(t *testing.T) {
		cfg := config.Config{}

		got, err := cfg.ProjectRoot(gitRoot, configDir)

		if err != nil {
			t.Fatalf("ProjectRoot() returned unexpected error: %v", err)
		}
		if got != configDir {
			t.Errorf("ProjectRoot() = %q, want %q", got, configDir)
		}
	})

	t.Run("relative ProjectDir joins onto gitRoot", func(t *testing.T) {
		cfg := config.Config{ProjectDir: "up_saln0001_giss_salesforce"}

		got, err := cfg.ProjectRoot(gitRoot, configDir)

		if err != nil {
			t.Fatalf("ProjectRoot() returned unexpected error: %v", err)
		}
		want := filepath.Join(gitRoot, "up_saln0001_giss_salesforce")
		if got != want {
			t.Errorf("ProjectRoot() = %q, want %q", got, want)
		}
	})

	t.Run("absolute ProjectDir is rejected naming the offending value", func(t *testing.T) {
		cfg := config.Config{ProjectDir: filepath.FromSlash("/etc/passwd")}

		_, err := cfg.ProjectRoot(gitRoot, configDir)

		if err == nil {
			t.Fatal("expected ProjectRoot() to return an error, got nil")
		}
		if !strings.Contains(err.Error(), cfg.ProjectDir) {
			t.Errorf("error %q does not name the offending value %q", err.Error(), cfg.ProjectDir)
		}
	})

	t.Run("ProjectDir containing a .. segment is rejected naming the offending value", func(t *testing.T) {
		cfg := config.Config{ProjectDir: filepath.FromSlash("../escape")}

		_, err := cfg.ProjectRoot(gitRoot, configDir)

		if err == nil {
			t.Fatal("expected ProjectRoot() to return an error, got nil")
		}
		if !strings.Contains(err.Error(), cfg.ProjectDir) {
			t.Errorf("error %q does not name the offending value %q", err.Error(), cfg.ProjectDir)
		}
	})

	t.Run("ProjectDir with a nested .. segment is rejected", func(t *testing.T) {
		cfg := config.Config{ProjectDir: filepath.FromSlash("project/../../escape")}

		_, err := cfg.ProjectRoot(gitRoot, configDir)

		if err == nil {
			t.Fatal("expected ProjectRoot() to return an error, got nil")
		}
	})
}
