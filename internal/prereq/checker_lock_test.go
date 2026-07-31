package prereq_test

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/malavolta/DeployDeck/internal/prereq"
)

// TestChecker_CheckLock_NilLock_SkipsWithOK confirms a nil Lock keeps the
// other checks independently exercisable.
func TestChecker_CheckLock_NilLock_SkipsWithOK(t *testing.T) {
	checker := &prereq.Checker{}
	check, err := checker.CheckLock(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if check.Status != prereq.StatusOK {
		t.Fatalf("expected a nil lock to report OK (skipped), got %+v", check)
	}
}

// TestChecker_CheckLock_HeldByLiveOwner_BlocksNamingProcess is GAP B: a live
// same-host owner (ErrLockHeld) must map to a StatusBlocking PrereqCheck that
// names the owning pid AND pname, with a non-empty FixCommand.
func TestChecker_CheckLock_HeldByLiveOwner_BlocksNamingProcess(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "lock")
	startedAt := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	writeLockFile(t, path, prereq.LockInfo{PID: 777, PName: "deploydeck-owner", Host: "test-host", StartedAt: startedAt})

	prober := fakeProber{alive: func(int, time.Time) bool { return true }}
	self := prereq.LockInfo{PID: 4321, PName: "deploydeck", Host: "test-host"}
	checker := &prereq.Checker{Lock: prereq.NewLock(path, self, prober)}

	check, err := checker.CheckLock(context.Background())
	if err != nil {
		t.Fatalf("expected a blocking check, not a hard error: %v", err)
	}
	if check.Status != prereq.StatusBlocking {
		t.Fatalf("expected a held lock to block, got %+v", check)
	}
	if !strings.Contains(check.Detail, "deploydeck-owner") {
		t.Fatalf("expected the detail to name the owning process, got %q", check.Detail)
	}
	if !strings.Contains(check.Detail, strconv.Itoa(777)) {
		t.Fatalf("expected the detail to name the owning pid 777, got %q", check.Detail)
	}
	if check.FixCommand == "" {
		t.Fatal("expected a non-empty FixCommand for a held lock")
	}
}

// TestChecker_CheckLock_CorruptLock_BlocksWithRemoveFixCommand is H3: a
// corrupt/zero-length lock must surface as a distinct StatusBlocking check
// with an actionable remove FixCommand, not a misleading generic error.
func TestChecker_CheckLock_CorruptLock_BlocksWithRemoveFixCommand(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "lock")
	if err := os.WriteFile(path, []byte("corrupt-not-json"), 0o644); err != nil {
		t.Fatalf("seed corrupt lock: %v", err)
	}

	prober := fakeProber{alive: func(int, time.Time) bool { return false }}
	self := prereq.LockInfo{PID: 4321, PName: "deploydeck", Host: "test-host"}
	checker := &prereq.Checker{Lock: prereq.NewLock(path, self, prober)}

	check, err := checker.CheckLock(context.Background())
	if err != nil {
		t.Fatalf("expected a blocking check, not a hard error: %v", err)
	}
	if check.Status != prereq.StatusBlocking {
		t.Fatalf("expected a corrupt lock to block, got %+v", check)
	}
	if !strings.Contains(check.FixCommand, path) {
		t.Fatalf("expected the FixCommand to reference the corrupt lock path %q, got %q", path, check.FixCommand)
	}
	if !strings.Contains(strings.ToLower(check.Detail), "corrupt") {
		t.Fatalf("expected the detail to explain the lock is corrupt, got %q", check.Detail)
	}
}

// TestChecker_CheckLock_Free_AcquiresOK confirms a free lock is acquired and
// reported OK.
func TestChecker_CheckLock_Free_AcquiresOK(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "lock")

	prober := fakeProber{alive: func(int, time.Time) bool { return false }}
	self := prereq.LockInfo{PID: 4321, PName: "deploydeck", Host: "test-host"}
	checker := &prereq.Checker{Lock: prereq.NewLock(path, self, prober)}

	check, err := checker.CheckLock(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if check.Status != prereq.StatusOK {
		t.Fatalf("expected a free lock to be acquired OK, got %+v", check)
	}
	if _, statErr := os.Stat(path); statErr != nil {
		t.Fatalf("expected the lock file to be created on successful acquire: %v", statErr)
	}
}
