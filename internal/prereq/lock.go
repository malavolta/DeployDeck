// Package prereq implements HU-001 local-prerequisite checks (git/sf/
// sfdx-git-delta presence+version, repo membership, origin, working tree,
// .deploydeck/ gitignore, git hooks interference, commit.gpgsign, Salesforce
// alias validation) plus the single-instance .deploydeck/lock. It is a
// separate package (not internal/app) so both the TUI and the `deploydeck
// doctor` CLI subcommand reuse the exact same checks.
package prereq

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

// maxTakeoverAttempts bounds the remove-then-retry loop used to take over a
// stale lock, so a persistently contended lock fails loudly instead of
// looping forever.
const maxTakeoverAttempts = 5

// LockInfo is the persisted content of .deploydeck/lock.
type LockInfo struct {
	PID       int       `json:"pid"`
	PName     string    `json:"pname"`
	Host      string    `json:"host"`
	StartedAt time.Time `json:"startedAt"`
	CreatedAt time.Time `json:"createdAt"`
}

// ProcessProber reports whether the process identified by pid, started at
// startedAt, is still alive. A pid that currently exists but whose live
// start-time does NOT match startedAt is treated as NOT alive: the pid was
// reused by an unrelated process, so this makes the PID-reuse guard real
// rather than decorative. Production code reads the live start-time via
// darwin sysctl KERN_PROC / `ps -o lstart` (or /proc/<pid>/stat on linux);
// unit tests inject a fake.
type ProcessProber interface {
	Alive(pid int, startedAt time.Time) bool
}

// StartTimeReader is an optional capability a ProcessProber may implement:
// reporting a process's real OS start-time. Acquire uses it to stamp
// LockInfo.StartedAt from the SAME source a later liveness probe compares
// against (`ps -o lstart=`), so the PID-reuse guard compares start-time vs
// start-time instead of start-time vs the acquisition wall clock. A prober
// that does not implement it simply causes Acquire to fall back to the wall
// clock.
type StartTimeReader interface {
	StartTime(pid int) (time.Time, error)
}

// ErrLockHeld is returned when another instance holds .deploydeck/lock.
// SameHost distinguishes a live, probed same-host owner (definitive refusal)
// from a different-host owner (conservative refusal — liveness cannot be
// probed across hosts).
type ErrLockHeld struct {
	Owner    LockInfo
	SameHost bool
}

func (e *ErrLockHeld) Error() string {
	if !e.SameHost {
		return fmt.Sprintf(
			"prereq: lock held by %s (pid %d) on a different host (%s) — refusing conservatively, liveness cannot be verified across hosts",
			e.Owner.PName, e.Owner.PID, e.Owner.Host,
		)
	}
	return fmt.Sprintf("prereq: another instance is active: %s (pid %d)", e.Owner.PName, e.Owner.PID)
}

// Lock guards a single .deploydeck/lock file for single-instance enforcement
// over one repository.
type Lock struct {
	path   string
	self   LockInfo
	prober ProcessProber
	now    func() time.Time
}

// NewLock returns a Lock at path, identifying this process as self
// (PID/PName/Host — StartedAt/CreatedAt are filled in by Acquire) and using
// prober to check a contending owner's liveness.
func NewLock(path string, self LockInfo, prober ProcessProber) *Lock {
	return &Lock{path: path, self: self, prober: prober, now: time.Now}
}

// Path returns the filesystem path this Lock guards.
func (l *Lock) Path() string {
	return l.path
}

// Acquire claims the lock, writing this process's LockInfo to path.
//
//   - No existing lock (or a stale one) → acquired, LockInfo persisted.
//   - Same host + a live owner (per prober) → refuse, *ErrLockHeld{SameHost: true}.
//   - A different host → refuse conservatively without probing, *ErrLockHeld{SameHost: false}.
//
// Claiming is exclusive and atomic: the full LockInfo record is written to a
// temp file, then published via os.Link (which fails with EEXIST rather
// than silently replacing an existing target, unlike os.Rename), so a
// concurrent reader never observes a half-written record and two racing
// acquirers can never both believe they own the lock. Stale takeover
// removes the old record and retries the same exclusive claim in a bounded
// loop — it never renames over an existing lock.
func (l *Lock) Acquire() error {
	self := l.self
	if self.StartedAt.IsZero() {
		self.StartedAt = l.currentStartTime()
	}
	self.CreatedAt = l.now()

	var lastOwner LockInfo
	for attempt := 0; attempt < maxTakeoverAttempts; attempt++ {
		ok, err := tryClaimLock(l.path, self)
		if ok {
			return nil
		}
		if err != nil && !errors.Is(err, fs.ErrExist) {
			return fmt.Errorf("prereq: acquiring lock %s: %w", l.path, err)
		}

		raw, owner, readErr := readLockInfoWithRetry(l.path)
		if readErr != nil {
			// The file vanished between our failed claim and this read
			// (the holder released it, or another racer's takeover already
			// completed) — just retry the claim from scratch.
			continue
		}
		lastOwner = owner

		if owner.Host != self.Host {
			return &ErrLockHeld{Owner: owner, SameHost: false}
		}
		if l.prober.Alive(owner.PID, owner.StartedAt) {
			return &ErrLockHeld{Owner: owner, SameHost: true}
		}

		// Stale: the owner is dead or its pid was reused. Remove the
		// record ONLY if it still matches what we just read (narrows,
		// without fully eliminating, the takeover race window against a
		// concurrent racer that may have already claimed it), then retry
		// the exclusive claim.
		removeLockIfUnchanged(l.path, raw)
	}

	if lastOwner.PID != 0 {
		return &ErrLockHeld{Owner: lastOwner, SameHost: lastOwner.Host == self.Host}
	}
	return fmt.Errorf("prereq: could not acquire lock %s after %d attempts (contended stale takeover)", l.path, maxTakeoverAttempts)
}

// currentStartTime returns this process's real OS start-time as reported by
// the prober's start-time source (the SAME `ps -o lstart=` a future
// acquirer's liveness probe reads), so the persisted StartedAt is exactly
// what that probe will compare against. This defeats the skew between when
// this process actually started and when it finally acquires the lock —
// CheckLock runs LAST, after slow git/sf checks, so the acquisition wall
// clock can be many seconds past the true start-time and exceed the liveness
// tolerance. Falls back to the wall clock only when the prober cannot report
// a start-time (e.g. a cross-platform prober without the seam, or a test
// double that does not implement it).
func (l *Lock) currentStartTime() time.Time {
	if reader, ok := l.prober.(StartTimeReader); ok {
		if start, err := reader.StartTime(l.self.PID); err == nil && !start.IsZero() {
			return start
		}
	}
	return l.now()
}

// Release removes the lock file, but ONLY if its persisted PID still
// matches this Lock's self.PID — it never removes a lock owned by a
// different process. Removing an already-absent lock is not an error.
func (l *Lock) Release() error {
	_, owner, err := readLockInfo(l.path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("prereq: reading lock %s for release: %w", l.path, err)
	}
	if owner.PID != l.self.PID {
		return nil
	}
	if err := os.Remove(l.path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("prereq: removing lock %s: %w", l.path, err)
	}
	return nil
}

// tryClaimLock attempts to atomically create path holding info's full JSON
// content. It writes the content to a temp file in the same directory, then
// publishes it via os.Link: Link fails with EEXIST if path already exists
// (never silently replacing it, unlike Rename), so exactly one caller wins
// when two race to claim the same path, and the content is fully formed the
// instant path becomes discoverable — there is no empty-file window.
func tryClaimLock(path string, info LockInfo) (bool, error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return false, fmt.Errorf("prereq: creating lock directory %s: %w", dir, err)
	}

	data, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		return false, fmt.Errorf("prereq: encoding lock info: %w", err)
	}

	tmp, err := os.CreateTemp(dir, ".lock-tmp-*")
	if err != nil {
		return false, fmt.Errorf("prereq: creating temp lock file: %w", err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return false, fmt.Errorf("prereq: writing temp lock file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return false, fmt.Errorf("prereq: closing temp lock file: %w", err)
	}

	if err := os.Link(tmpPath, path); err != nil {
		return false, err
	}
	return true, nil
}

// readLockInfo reads and parses path, returning the raw bytes alongside the
// parsed LockInfo so callers can compare-before-remove.
func readLockInfo(path string) ([]byte, LockInfo, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, LockInfo{}, err
	}
	var info LockInfo
	if err := json.Unmarshal(data, &info); err != nil {
		return data, LockInfo{}, err
	}
	return data, info, nil
}

// readLockInfoWithRetry re-reads once on a transient parse failure (e.g. a
// reader racing a concurrent writer) before giving up. A missing file is
// not retried — it unambiguously means "no lock right now".
func readLockInfoWithRetry(path string) ([]byte, LockInfo, error) {
	raw, info, err := readLockInfo(path)
	if err == nil {
		return raw, info, nil
	}
	if errors.Is(err, fs.ErrNotExist) {
		return nil, LockInfo{}, err
	}
	return readLockInfo(path)
}

// removeLockIfUnchanged removes path only if its current content still
// equals expected, so a racer that already took over (or whose owner
// released normally) between our read and this call is never clobbered.
func removeLockIfUnchanged(path string, expected []byte) {
	current, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(current, expected) {
		return
	}
	_ = os.Remove(path)
}
