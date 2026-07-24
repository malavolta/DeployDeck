package prereq_test

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"deploydeck/internal/prereq"
)

// fakeProber is a table-driven test double for prereq.ProcessProber. It
// never touches a real OS process, keeping lock tests fast and
// deterministic.
type fakeProber struct {
	alive func(pid int, startedAt time.Time) bool
}

func (f fakeProber) Alive(pid int, startedAt time.Time) bool {
	return f.alive(pid, startedAt)
}

func writeLockFile(t *testing.T, path string, info prereq.LockInfo) {
	t.Helper()
	data, err := json.Marshal(info)
	if err != nil {
		t.Fatalf("marshal seed lock: %v", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write seed lock: %v", err)
	}
}

func TestLock_Acquire_TableDriven(t *testing.T) {
	startedAt := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)

	tests := []struct {
		name        string
		existing    *prereq.LockInfo
		alive       func(pid int, startedAt time.Time) bool
		wantErr     bool
		wantSubstr  string
		wantNoAlive bool // asserts the prober is never consulted (different host)
	}{
		{
			name:     "no existing lock succeeds",
			existing: nil,
			wantErr:  false,
		},
		{
			name:     "same host alive owner refuses naming pname and pid",
			existing: &prereq.LockInfo{PID: 999, PName: "deploydeck-old", Host: "test-host", StartedAt: startedAt},
			alive: func(pid int, s time.Time) bool {
				return pid == 999 && s.Equal(startedAt)
			},
			wantErr:    true,
			wantSubstr: "deploydeck-old",
		},
		{
			name:     "dead owner is a stale takeover, acquire succeeds",
			existing: &prereq.LockInfo{PID: 999, PName: "deploydeck-old", Host: "test-host", StartedAt: startedAt},
			alive: func(pid int, s time.Time) bool {
				return false
			},
			wantErr: false,
		},
		{
			name:     "pid-reused (start-time mismatch) is a stale takeover, acquire succeeds",
			existing: &prereq.LockInfo{PID: 999, PName: "deploydeck-old", Host: "test-host", StartedAt: startedAt},
			alive: func(pid int, s time.Time) bool {
				// A live PID 999 exists, but its real start-time does not
				// match what the lock file recorded: the PID was reused by
				// an unrelated process. The owner is gone.
				return pid == 999 && !s.Equal(startedAt)
			},
			wantErr: false,
		},
		{
			name:        "different host refuses conservatively without probing",
			existing:    &prereq.LockInfo{PID: 999, PName: "deploydeck-old", Host: "other-host", StartedAt: startedAt},
			wantErr:     true,
			wantSubstr:  "other-host",
			wantNoAlive: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "lock")
			if tt.existing != nil {
				writeLockFile(t, path, *tt.existing)
			}

			probed := false
			alive := tt.alive
			prober := fakeProber{alive: func(pid int, s time.Time) bool {
				probed = true
				if alive == nil {
					t.Fatalf("prober should not have been consulted for %q", tt.name)
					return false
				}
				return alive(pid, s)
			}}

			self := prereq.LockInfo{PID: 1234, PName: "deploydeck", Host: "test-host"}
			lock := prereq.NewLock(path, self, prober)

			err := lock.Acquire()

			if tt.wantErr && err == nil {
				t.Fatalf("expected an error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("expected no error, got: %v", err)
			}
			if tt.wantErr && tt.wantSubstr != "" && !strings.Contains(err.Error(), tt.wantSubstr) {
				t.Fatalf("expected error to mention %q, got: %v", tt.wantSubstr, err)
			}
			if tt.wantNoAlive && probed {
				t.Fatalf("expected the prober NOT to be consulted for a different-host lock")
			}

			if !tt.wantErr {
				data, err := os.ReadFile(path)
				if err != nil {
					t.Fatalf("expected a persisted lock file after successful acquire: %v", err)
				}
				var got prereq.LockInfo
				if err := json.Unmarshal(data, &got); err != nil {
					t.Fatalf("persisted lock file did not parse as valid JSON: %v (content: %s)", err, data)
				}
				if got.PID != self.PID || got.PName != self.PName {
					t.Fatalf("expected persisted lock to record self (pid %d, %s), got %+v", self.PID, self.PName, got)
				}
			}
		})
	}
}

func TestLock_Acquire_SameHostAliveOwner_ErrorIsTypedErrLockHeld(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "lock")
	startedAt := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	writeLockFile(t, path, prereq.LockInfo{PID: 555, PName: "deploydeck", Host: "test-host", StartedAt: startedAt})

	prober := fakeProber{alive: func(pid int, s time.Time) bool { return true }}
	self := prereq.LockInfo{PID: 1234, PName: "deploydeck", Host: "test-host"}
	lock := prereq.NewLock(path, self, prober)

	err := lock.Acquire()
	if err == nil {
		t.Fatal("expected an error for a live same-host owner")
	}

	var held *prereq.ErrLockHeld
	if !errors.As(err, &held) {
		t.Fatalf("expected *prereq.ErrLockHeld, got %T: %v", err, err)
	}
	if held.Owner.PID != 555 {
		t.Fatalf("expected ErrLockHeld.Owner.PID == 555, got %d", held.Owner.PID)
	}
	if !held.SameHost {
		t.Fatal("expected ErrLockHeld.SameHost == true")
	}
}

func TestLock_StaleTakeoverRace_ExactlyOneWinner(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "lock")

	staleStartedAt := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	writeLockFile(t, path, prereq.LockInfo{PID: 111, PName: "deploydeck-old", Host: "test-host", StartedAt: staleStartedAt})

	// Only the original stale owner (pid 111) is dead; any other pid
	// observed (i.e. a freshly-claimed lock by one of the two racing
	// acquirers) is alive, so a racer that loses must see a live winner and
	// refuse rather than also winning.
	prober := fakeProber{alive: func(pid int, s time.Time) bool {
		return pid != 111
	}}

	selves := []prereq.LockInfo{
		{PID: 2001, PName: "deploydeck-a", Host: "test-host"},
		{PID: 2002, PName: "deploydeck-b", Host: "test-host"},
	}

	results := make([]error, 2)
	var wg sync.WaitGroup
	var start sync.WaitGroup
	start.Add(1)

	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			start.Wait()
			lock := prereq.NewLock(path, selves[i], prober)
			results[i] = lock.Acquire()
		}(i)
	}
	start.Done()
	wg.Wait()

	successCount := 0
	winnerPID := -1
	for i, err := range results {
		if err == nil {
			successCount++
			winnerPID = selves[i].PID
		}
	}
	if successCount != 1 {
		t.Fatalf("expected exactly 1 winner of the stale-lock race, got %d (results=%v)", successCount, results)
	}

	for _, err := range results {
		if err == nil {
			continue
		}
		var held *prereq.ErrLockHeld
		if !errors.As(err, &held) {
			t.Fatalf("expected the loser's error to be *ErrLockHeld, got %T: %v", err, err)
		}
		if held.Owner.PID != winnerPID {
			t.Fatalf("expected the loser to re-read the winner's LockInfo (pid %d), got pid %d", winnerPID, held.Owner.PID)
		}
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("expected a persisted lock file after the race: %v", err)
	}
	var got prereq.LockInfo
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("persisted lock file did not parse as valid JSON after the race: %v (content: %s)", err, data)
	}
	if got.PID != winnerPID {
		t.Fatalf("expected the persisted lock to belong to the winner (pid %d), got pid %d", winnerPID, got.PID)
	}
}

func TestLock_Acquire_NeverObservablePartiallyWritten(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "lock")

	self := prereq.LockInfo{PID: 4242, PName: "deploydeck", Host: "test-host"}
	prober := fakeProber{alive: func(int, time.Time) bool { return false }}
	lock := prereq.NewLock(path, self, prober)

	var wg sync.WaitGroup
	stop := make(chan struct{})
	violations := 0
	var mu sync.Mutex

	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
			}
			data, err := os.ReadFile(path)
			if err != nil {
				continue // not created yet — a valid observation
			}
			var got prereq.LockInfo
			if jsonErr := json.Unmarshal(data, &got); jsonErr != nil || got.PID == 0 {
				mu.Lock()
				violations++
				mu.Unlock()
			}
		}
	}()

	if err := lock.Acquire(); err != nil {
		t.Fatalf("expected acquire to succeed, got: %v", err)
	}
	close(stop)
	wg.Wait()

	if violations > 0 {
		t.Fatalf("observed %d partially-written/unparseable reads of the lock file; LockInfo must never be observable half-written", violations)
	}
}
