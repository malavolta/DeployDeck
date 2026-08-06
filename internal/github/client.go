package github

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/malavolta/DeployDeck/internal/exec"
)

// AuthState classifies gh's availability/authentication, derived from ONE
// `gh auth status` invocation (design.md's "gh 3-state via
// non-zero-exit-as-data" decision): a Runner error (the binary is missing
// or cannot even start) means AuthAbsent, a nil error with a non-zero exit
// means AuthUnauthenticated, and a zero exit means AuthAuthenticated.
type AuthState int

const (
	AuthAbsent AuthState = iota
	AuthUnauthenticated
	AuthAuthenticated
)

// Client is the gh CLI shim: auth-status detection and PR creation.
type Client interface {
	// AuthStatus runs ONE `gh auth status` and classifies the result. It is
	// TOTAL — no error return — because every outcome (missing binary,
	// present-unauthenticated, present-authenticated) is itself a valid,
	// informative AuthState the caller branches on directly, never a
	// failure mode requiring separate error handling.
	AuthStatus(ctx context.Context) AuthState
	// CreatePR runs `gh pr create --base <base> --head <head> --title
	// <title> --body <body>` (fully non-interactive — --body forces it,
	// even when body is "" — args passed as a discrete slice, NEVER
	// through a shell, so a multi-line body is passed through safely). On
	// success it returns the created PR's URL parsed from stdout. Raw (the
	// command's combined stdout+stderr) is preserved on BOTH success and
	// failure so a caller can show it after a failed creation (HU-014
	// AC7). CreatePR NEVER runs unless the caller has already obtained
	// explicit user confirmation — this package enforces the non-shell,
	// non-interactive argument shape; the confirm gate itself lives in the
	// caller (internal/app's StatePushPreparation).
	CreatePR(ctx context.Context, base, head, title, body string) (url, raw string, err error)
	// PRForBranch runs `gh pr view <branch> --json url,state` (incremental-
	// promotion design: "PR detection") to discover whether a PR already
	// exists for branch, exit-code-as-data: a found PR parses url/open from
	// its JSON state (OPEN -> open=true; CLOSED/MERGED -> open=false, url
	// still set — the caller falls through to a normal create + notice); no
	// PR for the branch (gh's own non-zero exit, e.g. "no pull requests
	// found") degrades to url="", open=false, err=nil — never an error, so
	// the caller's normal create path is unaffected. A genuine Runner
	// failure (gh missing/cannot start) still surfaces as err.
	PRForBranch(ctx context.Context, branch string) (url string, open bool, err error)
	// PRDetails runs `gh pr view <url> --json headRefName,body` (pr-
	// provenance's `deploydeck pr verify <url>`) to fetch the fields needed
	// to re-check a marker. UNLIKE PRForBranch's non-zero-exit-as-data
	// (verify has no create-fallthrough to degrade into), a Runner error, a
	// non-zero exit, OR malformed JSON here ALL return err — never a
	// zero-valued result silently treated as data (design.md's "PRDetails
	// degrade" decision).
	PRDetails(ctx context.Context, url string) (headBranch, body string, err error)
	// PRReviews runs `gh pr view <url> --json latestReviews` (deploy-gate's
	// approval condition) and maps each entry's author.login/state into a
	// Review — the LATEST review per author, exactly what gh's own
	// `latestReviews` field already reports. A Runner error, non-zero exit,
	// or malformed JSON all return an error (fail-closed: the caller's
	// approval condition treats a read failure as FAILED, never as zero
	// reviews silently passing).
	PRReviews(ctx context.Context, url string) ([]Review, error)
	// UnresolvedThreadCount runs ONE `gh api graphql` querying the PR's
	// review threads (`reviewThreads(first:100){isResolved}`) and counts the
	// entries with isResolved==false (deploy-gate's unresolved-threads
	// condition). A Runner error, non-zero exit, a GraphQL `errors`
	// envelope, or malformed JSON all return an error — thread-resolution
	// status "cannot be determined" fails closed (design.md's "Threads"
	// decision), never treated as zero/unknown-but-passing. An unparseable
	// url returns an error WITHOUT ever calling gh.
	UnresolvedThreadCount(ctx context.Context, url string) (int, error)
	// PRComments runs `gh pr view <url> --json comments` (deploy-gate's
	// validation-comment condition, and its dedup skip-if-present check) and
	// maps each entry's author.login/body into a Comment. A Runner error,
	// non-zero exit, or malformed JSON all return an error.
	PRComments(ctx context.Context, url string) ([]Comment, error)
	// PostComment runs `gh pr comment <url> --body <body>` — args passed as
	// a discrete slice, NEVER through a shell, so a multi-line body (the
	// deploy-gate validation comment's human-readable detail) is passed
	// through safely. Raw (combined stdout+stderr) is preserved on BOTH
	// success and failure.
	PostComment(ctx context.Context, url, body string) (raw string, err error)
}

// Review is one PR reviewer's LATEST review state (deploy-gate's approval
// condition), mapped from `gh pr view --json latestReviews`. State is one
// of GitHub's review states, e.g. "APPROVED", "CHANGES_REQUESTED",
// "COMMENTED".
type Review struct {
	Login string
	State string
}

// Comment is one PR (issue) comment, mapped from `gh pr view --json
// comments` (deploy-gate's validation-comment condition and dedup check).
type Comment struct {
	Login string
	Body  string
}

// client is the Runner-backed Client implementation.
type client struct {
	runner exec.Runner
}

// New returns a Client backed by runner.
func New(runner exec.Runner) Client {
	return &client{runner: runner}
}

// AuthStatus implements Client.AuthStatus.
func (c *client) AuthStatus(ctx context.Context) AuthState {
	result, err := c.runner.Run(ctx, exec.CommandRequest{
		Name: "gh",
		Args: []string{"auth", "status"},
	})
	if err != nil {
		return AuthAbsent
	}
	if result.ExitCode != 0 {
		return AuthUnauthenticated
	}
	return AuthAuthenticated
}

// CreatePR implements Client.CreatePR.
func (c *client) CreatePR(ctx context.Context, base, head, title, body string) (string, string, error) {
	req := exec.CommandRequest{
		Name: "gh",
		Args: []string{"pr", "create", "--base", base, "--head", head, "--title", title, "--body", body},
	}

	result, err := c.runner.Run(ctx, req)
	if err != nil {
		return "", "", fmt.Errorf("github: running gh pr create: %w", err)
	}

	raw := combineOutput(result.Stdout, result.Stderr)
	if result.ExitCode != 0 {
		return "", raw, fmt.Errorf("github: gh pr create exited %d: %s", result.ExitCode, raw)
	}

	return strings.TrimSpace(string(result.Stdout)), raw, nil
}

// prView is the shape `gh pr view <branch> --json url,state` prints.
type prView struct {
	URL   string `json:"url"`
	State string `json:"state"`
}

// PRForBranch implements Client.PRForBranch.
func (c *client) PRForBranch(ctx context.Context, branch string) (string, bool, error) {
	req := exec.CommandRequest{
		Name: "gh",
		Args: []string{"pr", "view", branch, "--json", "url,state"},
	}

	result, err := c.runner.Run(ctx, req)
	if err != nil {
		return "", false, fmt.Errorf("github: running gh pr view: %w", err)
	}
	if result.ExitCode != 0 {
		// No PR for this branch (or gh itself reported an error) — data, not
		// a failure: the caller degrades to its own normal create path.
		return "", false, nil
	}

	var parsed prView
	if err := json.Unmarshal(result.Stdout, &parsed); err != nil {
		return "", false, fmt.Errorf("github: parsing gh pr view output: %w", err)
	}

	return parsed.URL, parsed.State == "OPEN", nil
}

// prDetails is the shape `gh pr view <url> --json headRefName,body` prints.
type prDetails struct {
	HeadRefName string `json:"headRefName"`
	Body        string `json:"body"`
}

// PRDetails implements Client.PRDetails.
func (c *client) PRDetails(ctx context.Context, url string) (string, string, error) {
	req := exec.CommandRequest{
		Name: "gh",
		Args: []string{"pr", "view", url, "--json", "headRefName,body"},
	}

	result, err := c.runner.Run(ctx, req)
	if err != nil {
		return "", "", fmt.Errorf("github: running gh pr view: %w", err)
	}
	if result.ExitCode != 0 {
		return "", "", fmt.Errorf("github: gh pr view exited %d: %s", result.ExitCode, strings.TrimSpace(string(result.Stderr)))
	}

	var parsed prDetails
	if err := json.Unmarshal(result.Stdout, &parsed); err != nil {
		return "", "", fmt.Errorf("github: parsing gh pr view output: %w", err)
	}

	return parsed.HeadRefName, parsed.Body, nil
}

// prReviewsView is the shape `gh pr view <url> --json latestReviews` prints.
type prReviewsView struct {
	LatestReviews []struct {
		Author struct {
			Login string `json:"login"`
		} `json:"author"`
		State string `json:"state"`
	} `json:"latestReviews"`
}

// PRReviews implements Client.PRReviews.
func (c *client) PRReviews(ctx context.Context, url string) ([]Review, error) {
	req := exec.CommandRequest{
		Name: "gh",
		Args: []string{"pr", "view", url, "--json", "latestReviews"},
	}

	result, err := c.runner.Run(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("github: running gh pr view: %w", err)
	}
	if result.ExitCode != 0 {
		return nil, fmt.Errorf("github: gh pr view exited %d: %s", result.ExitCode, strings.TrimSpace(string(result.Stderr)))
	}

	var parsed prReviewsView
	if err := json.Unmarshal(result.Stdout, &parsed); err != nil {
		return nil, fmt.Errorf("github: parsing gh pr view output: %w", err)
	}

	reviews := make([]Review, 0, len(parsed.LatestReviews))
	for _, r := range parsed.LatestReviews {
		reviews = append(reviews, Review{Login: r.Author.Login, State: r.State})
	}
	return reviews, nil
}

// unresolvedThreadsQuery fetches up to the first 100 review threads'
// resolution status for one PR, plus pageInfo.hasNextPage so a truncated
// result (deploy-gate's "Unresolved-Threads" fail-closed-on-truncation
// decision, design.md's resolved Open Question) can be detected and
// rejected rather than silently undercounted. Full pagination (looping on
// hasNextPage) is deliberately NOT implemented — fail-closed-on-truncation
// is correct and far cheaper for a gate that only ever needs to know
// "zero vs. more".
const unresolvedThreadsQuery = `query($owner: String!, $repo: String!, $number: Int!) {
  repository(owner: $owner, name: $repo) {
    pullRequest(number: $number) {
      reviewThreads(first: 100) {
        nodes {
          isResolved
        }
        pageInfo {
          hasNextPage
        }
      }
    }
  }
}`

// reviewThreadsResponse is `gh api graphql`'s JSON envelope for
// unresolvedThreadsQuery: a top-level `errors` array present (even
// alongside null `data`) signals a GraphQL-level failure (e.g. "Could not
// resolve to a PullRequest") distinct from a Runner/exit-code failure.
type reviewThreadsResponse struct {
	Data struct {
		Repository struct {
			PullRequest struct {
				ReviewThreads struct {
					Nodes []struct {
						IsResolved bool `json:"isResolved"`
					} `json:"nodes"`
					PageInfo struct {
						HasNextPage bool `json:"hasNextPage"`
					} `json:"pageInfo"`
				} `json:"reviewThreads"`
			} `json:"pullRequest"`
		} `json:"repository"`
	} `json:"data"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

// UnresolvedThreadCount implements Client.UnresolvedThreadCount.
func (c *client) UnresolvedThreadCount(ctx context.Context, url string) (int, error) {
	_, owner, repo, number, ok := ParsePRURLParts(url)
	if !ok {
		return 0, fmt.Errorf("github: could not parse owner/repo/number from PR URL %q", url)
	}

	req := exec.CommandRequest{
		Name: "gh",
		Args: []string{
			"api", "graphql",
			"-f", "query=" + unresolvedThreadsQuery,
			// owner/repo use -f (raw string, literal) — NOT -F (typed):
			// gh's -F interprets a leading "@" as "read the value from a
			// file" and a leading ":" as a variable placeholder reference, so
			// an owner/repo literally starting with either would be silently
			// mishandled as -F (remediation-pass risk SUGGESTION). number
			// legitimately needs -F's Int typing (a GraphQL Int variable),
			// so it stays -F.
			"-f", "owner=" + owner,
			"-f", "repo=" + repo,
			"-F", "number=" + strconv.Itoa(number),
		},
	}

	result, err := c.runner.Run(ctx, req)
	if err != nil {
		return 0, fmt.Errorf("github: running gh api graphql: %w", err)
	}
	if result.ExitCode != 0 {
		return 0, fmt.Errorf("github: gh api graphql exited %d: %s", result.ExitCode, strings.TrimSpace(string(result.Stderr)))
	}

	var parsed reviewThreadsResponse
	if err := json.Unmarshal(result.Stdout, &parsed); err != nil {
		return 0, fmt.Errorf("github: parsing gh api graphql output: %w", err)
	}
	if len(parsed.Errors) > 0 {
		return 0, fmt.Errorf("github: gh api graphql returned an error: %s", parsed.Errors[0].Message)
	}
	if parsed.Data.Repository.PullRequest.ReviewThreads.PageInfo.HasNextPage {
		// Fail-closed on truncation (deploy-gate spec's Unresolved-Threads
		// Condition, design.md's resolved Open Question): the first 100
		// threads alone can never prove "zero unresolved" when more threads
		// exist beyond the page — returning a partial count would let the
		// gate fail-OPEN on a PR truncated at exactly this boundary.
		return 0, fmt.Errorf("github: more than 100 review threads on the PR; cannot verify all are resolved")
	}

	unresolved := 0
	for _, n := range parsed.Data.Repository.PullRequest.ReviewThreads.Nodes {
		if !n.IsResolved {
			unresolved++
		}
	}
	return unresolved, nil
}

// prCommentsView is the shape `gh pr view <url> --json comments` prints.
type prCommentsView struct {
	Comments []struct {
		Author struct {
			Login string `json:"login"`
		} `json:"author"`
		Body string `json:"body"`
	} `json:"comments"`
}

// PRComments implements Client.PRComments.
func (c *client) PRComments(ctx context.Context, url string) ([]Comment, error) {
	req := exec.CommandRequest{
		Name: "gh",
		Args: []string{"pr", "view", url, "--json", "comments"},
	}

	result, err := c.runner.Run(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("github: running gh pr view: %w", err)
	}
	if result.ExitCode != 0 {
		return nil, fmt.Errorf("github: gh pr view exited %d: %s", result.ExitCode, strings.TrimSpace(string(result.Stderr)))
	}

	var parsed prCommentsView
	if err := json.Unmarshal(result.Stdout, &parsed); err != nil {
		return nil, fmt.Errorf("github: parsing gh pr view output: %w", err)
	}

	comments := make([]Comment, 0, len(parsed.Comments))
	for _, cm := range parsed.Comments {
		comments = append(comments, Comment{Login: cm.Author.Login, Body: cm.Body})
	}
	return comments, nil
}

// PostComment implements Client.PostComment.
func (c *client) PostComment(ctx context.Context, url, body string) (string, error) {
	req := exec.CommandRequest{
		Name: "gh",
		Args: []string{"pr", "comment", url, "--body", body},
	}

	result, err := c.runner.Run(ctx, req)
	if err != nil {
		return "", fmt.Errorf("github: running gh pr comment: %w", err)
	}

	raw := combineOutput(result.Stdout, result.Stderr)
	if result.ExitCode != 0 {
		return raw, fmt.Errorf("github: gh pr comment exited %d: %s", result.ExitCode, raw)
	}

	return raw, nil
}

// combineOutput joins stdout and stderr the same way internal/delta's own
// combineOutput does, so Raw always carries every line gh printed
// regardless of which stream it used.
func combineOutput(stdout, stderr []byte) string {
	raw := string(stdout)
	if len(stderr) > 0 {
		if raw != "" {
			raw += "\n"
		}
		raw += string(stderr)
	}
	return raw
}

// SuggestedTitle composes HU-014's suggested PR title:
// "<ticket> - Promote changes to <target>" (HU-014 AC4).
func SuggestedTitle(ticket, target string) string {
	return fmt.Sprintf("%s - Promote changes to %s", ticket, target)
}
