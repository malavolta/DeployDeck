package salesforce

import (
	"errors"
	"fmt"
)

// ErrMissingProjectDir is returned when an `sf project deploy ...` call is
// made without a working directory.
//
// Every `sf project` subcommand resolves its project from its own working
// directory: it requires sfdx-project.json in cwd. internal/exec treats an
// EMPTY CommandRequest.Dir as "inherit the process cwd", so an omitted
// directory does not fail — it silently runs the command wherever the
// operator happened to launch DeployDeck from. That is exactly the
// git-root/project-root/cwd conflation the directory-resolution change
// removes, and it is most expensive precisely here, on the commands that
// deploy.
//
// Rejecting it up front makes the mistake unrepresentable rather than
// latent: a caller that forgets the directory gets a loud, local error
// instead of a deployment aimed at an unintended directory.
var ErrMissingProjectDir = errors.New("salesforce: SFDX project directory is required")

// requireProjectDir guards the four `sf project deploy ...` entry points
// (validate, report, quick, cancel). It returns a wrapped
// ErrMissingProjectDir — wrapped, so errors.Is still matches while the
// message names the offending command — and callers MUST return before
// running anything, so no process is ever started with an inherited cwd.
func requireProjectDir(command, dir string) error {
	if dir == "" {
		return fmt.Errorf("%w: %s needs the directory holding sfdx-project.json", ErrMissingProjectDir, command)
	}
	return nil
}
