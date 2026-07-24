package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// FileName is the per-repo config file Load reads from dir.
const FileName = "deploydeck.yaml"

// Load reads deploydeck.yaml from dir, applying defaults for any omitted
// fields (BranchFormat, Runs.KeepLast, Runs.KeepDays).
func Load(dir string) (Config, error) {
	path := filepath.Join(dir, FileName)

	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("config: reading %s: %w", path, err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("config: parsing %s: %w", path, err)
	}

	applyDefaults(&cfg)

	return cfg, nil
}

func applyDefaults(cfg *Config) {
	if cfg.BranchFormat == "" {
		cfg.BranchFormat = DefaultBranchFormat
	}
	if cfg.Runs.KeepLast == 0 {
		cfg.Runs.KeepLast = DefaultRunsKeepLast
	}
	if cfg.Runs.KeepDays == 0 {
		cfg.Runs.KeepDays = DefaultRunsKeepDays
	}
}
