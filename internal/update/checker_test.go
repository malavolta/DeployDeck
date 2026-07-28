package update_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"deploydeck/internal/update"
)

// TestChecker_Latest_Success proves Latest decodes the GitHub Releases
// "latest" endpoint's tag_name into the returned version string (HU-019,
// update-notification: Semver-Based Newer-Version Detection — the input
// side of the comparison).
func TestChecker_Latest_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"tag_name":"v1.2.3"}`))
	}))
	defer srv.Close()

	c := update.Checker{BaseURL: srv.URL, HTTPClient: srv.Client()}
	got, err := c.Latest(context.Background())
	if err != nil {
		t.Fatalf("Latest() unexpected error: %v", err)
	}
	if got != "v1.2.3" {
		t.Fatalf("Latest() = %q, want %q", got, "v1.2.3")
	}
}

// TestChecker_Latest_NonOK proves a non-2xx response (here 401, matching
// the story's private-repo unauthorized case) is surfaced as an error
// rather than a bogus version string (update-notification: Silent Skip on
// Check Failure is enforced by the caller treating any error identically —
// this proves Latest itself reports the failure).
func TestChecker_Latest_NonOK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	c := update.Checker{BaseURL: srv.URL, HTTPClient: srv.Client()}
	_, err := c.Latest(context.Background())
	if err == nil {
		t.Fatal("Latest() expected a non-nil error on a 401 response, got nil")
	}
}

// TestChecker_Latest_Timeout proves Latest respects context cancellation: a
// handler that sleeps past a short context timeout must return an error,
// not hang (update-notification: Non-Blocking Startup Check — the seam a
// bounded context.WithTimeout depends on).
func TestChecker_Latest_Timeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"tag_name":"v1.2.3"}`))
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	c := update.Checker{BaseURL: srv.URL, HTTPClient: srv.Client()}
	_, err := c.Latest(ctx)
	if err == nil {
		t.Fatal("Latest() expected a non-nil error when the context times out, got nil")
	}
}
