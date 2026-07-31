package ai

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// TestDoctor_BoundsHangingEndpoint verifies Doctor applies its own deadline so a
// configured-but-unresponsive AI endpoint cannot hang `deploydeck doctor` or the
// TUI-startup prereq check indefinitely (review-risk WARNING: the doctor path was
// invoked with an unbounded context.Background() and a Timeout-less http.Client).
func TestDoctor_BoundsHangingEndpoint(t *testing.T) {
	block := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-block // never responds until cleanup unblocks it
	}))
	// LIFO cleanup: unblock the handler (registered last → runs first) BEFORE
	// srv.Close (registered first → runs last), so Close does not itself hang.
	t.Cleanup(srv.Close)
	t.Cleanup(func() { close(block) })

	restore := doctorProbeTimeout
	doctorProbeTimeout = 50 * time.Millisecond
	t.Cleanup(func() { doctorProbeTimeout = restore })

	c := New(srv.URL, "any-model", &http.Client{})

	done := make(chan DoctorState, 1)
	go func() { done <- c.Doctor(context.Background()) }()

	select {
	case got := <-done:
		if got != Unreachable {
			t.Fatalf("hanging endpoint: got %v, want Unreachable", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Doctor did not return within 2s — it hangs on a non-responding endpoint")
	}
}
