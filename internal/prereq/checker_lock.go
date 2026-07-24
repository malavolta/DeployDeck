package prereq

import (
	"context"
	"errors"
	"fmt"
)

// CheckLock attempts to acquire the single-instance lock (HU-001:
// "Single-Instance Lock Acquisition"). A nil Lock skips acquisition and
// reports OK, so the other checks remain independently exercisable.
func (c *Checker) CheckLock(ctx context.Context) (PrereqCheck, error) {
	if c.Lock == nil {
		return PrereqCheck{Name: "instance lock", Status: StatusOK, Detail: "lock acquisition skipped (no lock configured)"}, nil
	}

	if err := c.Lock.Acquire(); err != nil {
		var held *ErrLockHeld
		if errors.As(err, &held) {
			return PrereqCheck{
				Name:       "instance lock",
				Status:     StatusBlocking,
				Detail:     held.Error(),
				FixCommand: fmt.Sprintf("wait for pid %d (%s) to exit, or remove %s once you have confirmed it is stale", held.Owner.PID, held.Owner.PName, c.Lock.Path()),
			}, nil
		}
		var corrupt *ErrLockCorrupt
		if errors.As(err, &corrupt) {
			return PrereqCheck{
				Name:       "instance lock",
				Status:     StatusBlocking,
				Detail:     fmt.Sprintf("the lock file %s is corrupt or unreadable, so no owning process can be identified", corrupt.Path),
				FixCommand: fmt.Sprintf("rm %s  # remove the corrupt lock after confirming no deploydeck instance is running", corrupt.Path),
			}, nil
		}
		return PrereqCheck{}, fmt.Errorf("prereq: acquiring lock: %w", err)
	}

	return PrereqCheck{Name: "instance lock", Status: StatusOK, Detail: fmt.Sprintf("lock acquired at %s", c.Lock.Path())}, nil
}
