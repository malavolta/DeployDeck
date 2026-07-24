package prereq

import "context"

// nameGitRepository is the PrereqCheck.Name CheckRepository uses for
// repo-membership, shared here so Check() can decide whether the
// repo-dependent checks (working tree, gitignore, hooks, gpgsign) are safe
// to run at all.
const nameGitRepository = "git repository"

// Check runs every configured local-prerequisite check — git/sf/
// sfdx-git-delta versions, repo membership + origin, working tree,
// .deploydeck/ gitignore, git hooks, commit.gpgsign, Salesforce aliases,
// and the single-instance lock — aggregating them into one report (HU-001:
// "Prerequisite Check Execution And Reporting").
//
// Repo-dependent checks (working tree, gitignore, hooks, gpgsign) are
// skipped when Dir is not inside a git repository — the repo-membership
// check already reports that as blocking, and running the others would
// only surface the same root cause redundantly, or error where a check
// does not itself degrade gracefully (unlike CheckVersions).
func (c *Checker) Check(ctx context.Context) ([]PrereqCheck, error) {
	var all []PrereqCheck

	versionChecks, err := c.CheckVersions(ctx)
	if err != nil {
		return nil, err
	}
	all = append(all, versionChecks...)

	repoChecks, err := c.CheckRepository(ctx)
	if err != nil {
		return nil, err
	}
	all = append(all, repoChecks...)

	if insideRepo(repoChecks) {
		wtCheck, err := c.CheckWorkingTree(ctx)
		if err != nil {
			return nil, err
		}
		all = append(all, wtCheck)

		giCheck, err := c.CheckGitignore(ctx)
		if err != nil {
			return nil, err
		}
		all = append(all, giCheck)

		hooksCheck, err := c.CheckHooks(ctx)
		if err != nil {
			return nil, err
		}
		all = append(all, hooksCheck)

		gpgCheck, err := c.CheckGpgSign(ctx)
		if err != nil {
			return nil, err
		}
		all = append(all, gpgCheck)
	}

	aliasChecks, err := c.CheckAliases(ctx)
	if err != nil {
		return nil, err
	}
	all = append(all, aliasChecks...)

	lockCheck, err := c.CheckLock(ctx)
	if err != nil {
		return nil, err
	}
	all = append(all, lockCheck)

	return all, nil
}

func insideRepo(repoChecks []PrereqCheck) bool {
	for _, check := range repoChecks {
		if check.Name == nameGitRepository {
			return check.Status == StatusOK
		}
	}
	return false
}
