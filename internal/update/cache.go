package update

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// CacheTTL is how long a cached latest-release tag stays fresh.
const CacheTTL = 24 * time.Hour

const cacheFile = "update-check.json"

// Cache wraps a latest-tag fetch with an on-disk 24h cache; Dir "" disables it.
type Cache struct {
	Dir string
	Now func() time.Time
}

type cacheEntry struct {
	CheckedAt time.Time `json:"checked_at"`
	Latest    string    `json:"latest"`
}

// Latest returns the cached tag while fresh (0 <= age < CacheTTL), else
// fetches and caches a non-empty result. Cache problems degrade to a live fetch.
func (c Cache) Latest(ctx context.Context, fetch func(context.Context) (string, error)) (string, error) {
	now := time.Now
	if c.Now != nil {
		now = c.Now
	}
	if c.Dir != "" {
		if data, err := os.ReadFile(filepath.Join(c.Dir, cacheFile)); err == nil {
			var e cacheEntry
			if json.Unmarshal(data, &e) == nil && e.Latest != "" {
				if age := now().Sub(e.CheckedAt); age >= 0 && age < CacheTTL {
					return e.Latest, nil
				}
			}
		}
	}
	latest, err := fetch(ctx)
	if err != nil {
		return "", err
	}
	if c.Dir != "" && latest != "" {
		c.write(cacheEntry{CheckedAt: now(), Latest: latest})
	}
	return latest, nil
}

// write persists e atomically; every error is ignored on purpose.
func (c Cache) write(e cacheEntry) {
	data, err := json.Marshal(e)
	if err != nil || os.MkdirAll(c.Dir, 0o755) != nil {
		return
	}
	tmp, err := os.CreateTemp(c.Dir, cacheFile+".*")
	if err != nil {
		return
	}
	_, werr := tmp.Write(data)
	cerr := tmp.Close()
	if werr != nil || cerr != nil || os.Rename(tmp.Name(), filepath.Join(c.Dir, cacheFile)) != nil {
		_ = os.Remove(tmp.Name())
	}
}
