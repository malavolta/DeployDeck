package update

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// ownerRepo identifies the GitHub repository whose Releases API is queried
// for the latest tag. It is a PLACEHOLDER — set at activation checklist
// step (a) once the real module path / origin repository exists
// (docs/ACTIVATION-CHECKLIST.md).
const ownerRepo = "OWNER/REPO" // PLACEHOLDER — set at activation checklist step (a)

// Checker looks up the latest DeployDeck release from GitHub. BaseURL is
// injectable so tests can point it at an httptest.Server instead of the
// real GitHub API.
type Checker struct {
	// BaseURL is the GitHub API root, e.g. "https://api.github.com" in
	// production. Tests inject an httptest.Server URL.
	BaseURL string
	// HTTPClient performs the request. Callers should set a client with a
	// reasonable transport; Latest itself relies on the passed context for
	// bounding the request duration.
	HTTPClient *http.Client
}

// releaseResponse is the subset of GitHub's Releases API response this
// package needs.
type releaseResponse struct {
	TagName string `json:"tag_name"`
}

// Latest fetches the latest release's tag name from
// BaseURL/repos/<ownerRepo>/releases/latest. It returns an error on any
// transport failure, context cancellation/timeout, or non-2xx response
// (including 401 for a private repository) — callers treat every error
// identically per update-notification's Silent Skip discipline.
func (c Checker) Latest(ctx context.Context) (string, error) {
	url := c.BaseURL + "/repos/" + ownerRepo + "/releases/latest"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("update: building request: %w", err)
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("update: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("update: unexpected status %d from %s", resp.StatusCode, url)
	}

	var rel releaseResponse
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return "", fmt.Errorf("update: decoding response: %w", err)
	}

	return rel.TagName, nil
}
