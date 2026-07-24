package prereq

import (
	"context"
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
// "Wed Jul 24 10:15:03 2026".
const psStartTimeLayout = "Mon Jan  2 15:04:05 2006"

// startTimeTolerance absorbs `ps`'s one-second resolution when comparing
// against the sub-second precision LockInfo.StartedAt may carry.
const startTimeTolerance = 2 * time.Second

// Alive reports whether pid is currently running AND its live start-time
// matches startedAt (within startTimeTolerance), so a pid reused by an
// unrelated process is correctly treated as NOT alive (see design.md's
// PID-reuse guard).
func (p *OSProcessProber) Alive(pid int, startedAt time.Time) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	result, err := p.Runner.Run(ctx, exec.CommandRequest{
		Name: "ps",
		Args: []string{"-p", strconv.Itoa(pid), "-o", "lstart="},
	})
	if err != nil || result.ExitCode != 0 {
		return false
	}

	raw := strings.TrimSpace(string(result.Stdout))
	if raw == "" {
		return false
	}

	live, err := time.ParseInLocation(psStartTimeLayout, raw, time.Local)
	if err != nil {
		// Unparseable output: be conservative and assume it's a different
		// process rather than trusting a match we can't verify.
		return false
	}

	diff := live.Sub(startedAt)
	if diff < 0 {
		diff = -diff
	}
	return diff <= startTimeTolerance
}
