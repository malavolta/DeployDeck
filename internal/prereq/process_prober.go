package prereq

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"deploydeck/internal/exec"
)

// OSProcessProber checks process liveness and start-time via `ps`, run
// through the shared exec.Runner boundary rather than raw syscalls, so it
// stays consistent with the rest of the codebase's process-execution
// pattern (darwin: `ps -p <pid> -o lstart=`; most POSIX ps implementations
// support the same flag). Per design.md/tasks.md this reader is
// integration/manual, not fake-covered by the unit-level lock tests (real
// process start-times can't be canned meaningfully) — CI and unit tests
// inject a fake ProcessProber instead.
type OSProcessProber struct {
	Runner exec.Runner
}

// NewOSProcessProber returns a ProcessProber backed by runner.
func NewOSProcessProber(runner exec.Runner) *OSProcessProber {
	return &OSProcessProber{Runner: runner}
}

// psStartTimeLayout matches `ps -o lstart=`'s fixed-width output, e.g.
// "Wed Jul 24 10:15:03 2026". The C locale is pinned on the invocation (see
// readLstart) so this English layout is always what `ps` prints, regardless
// of the operator's shell locale.
const psStartTimeLayout = "Mon Jan  2 15:04:05 2006"

// startTimeTolerance absorbs `ps`'s one-second resolution when comparing
// against the sub-second precision LockInfo.StartedAt may carry.
const startTimeTolerance = 2 * time.Second

// Alive reports whether pid is currently running AND its live start-time
// matches startedAt (within startTimeTolerance), so a pid reused by an
// unrelated process is correctly treated as NOT alive (see design.md's
// PID-reuse guard).
//
// Failure direction is FAIL-SAFE: an EMPTY `ps` result (or a non-zero exit)
// means the process was not found → dead → takeover allowed. But a NON-EMPTY
// line we cannot parse means we found the process yet cannot verify its
// start-time; rather than risk taking over a live owner's lock (two
// instances), we treat that as ALIVE and refuse takeover. Locale is pinned
// to C so a localized `ps` layout never triggers that fallback in practice.
func (p *OSProcessProber) Alive(pid int, startedAt time.Time) bool {
	raw, found := p.readLstart(pid)
	if !found {
		return false
	}

	live, err := time.ParseInLocation(psStartTimeLayout, raw, time.Local)
	if err != nil {
		// Present-but-unparseable line: fail safe → assume the owner is
		// still alive and refuse takeover.
		return true
	}

	diff := live.Sub(startedAt)
	if diff < 0 {
		diff = -diff
	}
	return diff <= startTimeTolerance
}

// StartTime reads pid's live OS start-time from the SAME `ps -o lstart=`
// source Alive compares against, so a caller (Lock.Acquire) can stamp its own
// lock record's StartedAt from that exact source — making a later liveness
// probe compare start-time vs start-time, not start-time vs the wall clock at
// acquisition. Returns an error when the process is not found or its line is
// unparseable.
func (p *OSProcessProber) StartTime(pid int) (time.Time, error) {
	raw, found := p.readLstart(pid)
	if !found {
		return time.Time{}, fmt.Errorf("prereq: no start-time for pid %d: process not found", pid)
	}
	live, err := time.ParseInLocation(psStartTimeLayout, raw, time.Local)
	if err != nil {
		return time.Time{}, fmt.Errorf("prereq: parsing ps lstart %q for pid %d: %w", raw, pid, err)
	}
	return live, nil
}

// readLstart runs `ps -p <pid> -o lstart=` through the shared Runner with a
// pinned C locale, returning the trimmed line and whether the process was
// found. A run failure, non-zero exit, or empty output all mean "not found"
// (dead); a non-empty line is returned verbatim for the caller to parse.
func (p *OSProcessProber) readLstart(pid int) (raw string, found bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	result, err := p.Runner.Run(ctx, exec.CommandRequest{
		Name: "ps",
		Args: []string{"-p", strconv.Itoa(pid), "-o", "lstart="},
		// Pin the C locale so `ps -o lstart=` always prints the fixed
		// English layout (psStartTimeLayout). Without this, a localized
		// shell (e.g. Spanish macOS: "lun. 22 jun. 14:13:00 2026") yields an
		// unparseable line — which the fail-safe path would still treat as
		// alive, but pinning the locale restores the exact start-time
		// comparison the PID-reuse guard depends on.
		Env: []string{"LC_ALL=C", "LANG=C"},
	})
	if err != nil || result.ExitCode != 0 {
		return "", false
	}

	raw = strings.TrimSpace(string(result.Stdout))
	if raw == "" {
		return "", false
	}
	return raw, true
}
