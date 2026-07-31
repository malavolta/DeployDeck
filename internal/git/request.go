package git

import "github.com/malavolta/DeployDeck/internal/exec"

// nonInteractiveEnv is layered onto every git CommandRequest built by
// newRequest so `git cherry-pick --continue` (and any other git command)
// never opens an editor, prompts for terminal input, or pages output and
// hangs. See ARQUITECTURA.md's execution-wrapper rules.
var nonInteractiveEnv = []string{
	"GIT_EDITOR=true",
	"GIT_TERMINAL_PROMPT=0",
	"GIT_PAGER=cat",
}

// newRequest builds a git CommandRequest rooted at dir, carrying the
// non-interactive env on every call. This is the single place that
// constructs git exec requests so the non-interactive contract holds by
// construction for every Service method, current and future.
func newRequest(dir string, args ...string) exec.CommandRequest {
	return exec.CommandRequest{
		Name: "git",
		Args: args,
		Dir:  dir,
		Env:  append([]string(nil), nonInteractiveEnv...),
	}
}
