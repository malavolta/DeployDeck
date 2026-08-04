package runs

import "github.com/malavolta/DeployDeck/internal/git"

// FindRunForBranch returns the newest record among records whose rendered
// branch name — via the SAME git.RenderBranchName(format, rec.Ticket,
// rec.Target) correlation selectOrphans/resolveMergeTarget already use in
// internal/app — matches branchName (incremental-promotion spec: "Reuse
// Targets The Prior Run For The Same Branch"). records need NOT be
// pre-sorted: every matching record is compared by CreatedAt and the newest
// wins, so a caller passing an unsorted/partial snapshot still gets the
// correct target. ok is false when no record renders to branchName — the
// caller then proceeds without a prior run to increment (spec: "No prior
// run found is treated as a fresh increment target").
func FindRunForBranch(records []Record, format, branchName string) (Record, bool) {
	var best Record
	found := false
	for _, rec := range records {
		if git.RenderBranchName(format, rec.Ticket, rec.Target) != branchName {
			continue
		}
		if !found || rec.CreatedAt.After(best.CreatedAt) {
			best = rec
			found = true
		}
	}
	return best, found
}
