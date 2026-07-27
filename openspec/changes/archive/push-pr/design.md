# Design: HU-014 — Preparar Push Y PR (`push-pr`)

## Technical Approach

Follows `docs/ARQUITECTURA.md` verbatim (`App → GHSvc → Exec`). A NEW `internal/github`
sibling module over `exec.Runner` (mirrors `internal/delta`/`internal/salesforce`) owns
gh detection, `CreatePR`, and a PURE `CompareURL` normalizer. Push stays in `internal/git`
(`Push`, `RemoteURL`). `internal/app` gains `StatePushPreparation`, entered by a distinct `p`
key from a now-own-handler `StateSucceeded`; it composes the injected `git.Service` +
`github.Client` and NEVER execs (boundary_test holds). `run-persistence` grows `PRUrl`
additively; `prereq-check` gains an informative gh check.

## Architecture Decisions

### Decision: gh lives in a new module; push stays in git
**Choice**: `internal/github` for gh; `git.Service.Push`/`RemoteURL` for git.
**Alternatives**: push in a github module; gh inside salesforce shim.
**Rationale**: gh is a distinct external binary → its own Runner-backed sibling (delta precedent).
Push is a git operation → belongs with `fetchOrigin`/`HasRemote`, reusing the non-interactive `newRequest`.

### Decision: gh 3-state via non-zero-exit-as-data
**Choice**: one `gh auth status`; `AuthStatus(ctx) AuthState` (total, no error return).
Runner error → `AuthAbsent`; nil err + `ExitCode!=0` → `AuthUnauthenticated`; `ExitCode==0` → `AuthAuthenticated`.
**Alternatives**: `gh --version` + separate auth probe.
**Rationale**: the Runner contract already distinguishes start-failure from non-zero exit; one call covers all 3.

### Decision: CompareURL derives host from origin (pure)
**Choice**: package-level pure func parsing SSH/HTTPS/`ssh://`, stripping `.git`; unrecognized → error.
**Rationale**: user's gh is Enterprise (`github.ibm.com`); hardcoding github.com breaks it. Purity → the load-bearing table test.

### Decision: success screen offers push; StatePushPreparation holds the sub-flow
**Choice**: `terminalState` still returns `StateSucceeded`, but it leaves the shared quit-only handler for `keySucceeded`
(`p`→`StatePushPreparation`; `q`/`enter`/`esc` quit). `Failed`/`Canceled`/`Aborted`/`Error` stay quit-only.
**Rationale**: mirrors HU-012's `c`-gated sub-flow; satisfies AC "push offered only after success, not primary after failure".

### Decision: CreatePR only on explicit confirm; Raw always preserved
**Choice**: `g` reveals the exact `gh pr create` command; a second explicit key fires it. Raw kept on success AND failure.
**Rationale**: never auto-create a real PR; failure must degrade to manual data, not crash.

## Data Flow

    StateSucceeded ──p──▶ StatePushPreparation (pushPhase field)
      keySucceeded          │ confirm ─▶ Git.Push(dir, branch)
                            │ success ─▶ Git.RemoteURL(origin) + GH.AuthStatus
      CompareURL(origin,base,head) ┐             │
                            authed? ─ no ─▶ show CompareURL + manual data (flow ends)
                            └ yes ─▶ g ─▶ confirm ─▶ GH.CreatePR(base,head,title)
                                   ok ─▶ Runs.MarkPRCreated(runID,url) + show URL
                                   err ─▶ show error + manual base/compare/title (survives)

App never execs — `git.Service` and `github.Client` hold the exec seam.

## File Changes

| File | Action | Description |
|------|--------|-------------|
| `internal/github/client.go` | Create | `Client` interface, `client` impl, `New(runner)`, `AuthStatus`, `CreatePR`, `SuggestedTitle` |
| `internal/github/compare_url.go` | Create | pure `CompareURL(originURL, base, head)` normalizer |
| `internal/git/service_remote.go` | Modify | + `RemoteURL(ctx,dir,name)` (trimmed stdout of `git remote get-url`) |
| `internal/git/service_push.go` | Create | `Push(ctx,dir,branch)` = `git push -u origin <branch>` (RepoRoot + exit-check, `fetchOrigin` shape) |
| `internal/runs/writer.go` | Modify | + `Record.PRUrl` (`omitempty`, no schema bump) + `MarkPRCreated(runID,prURL)` |
| `internal/prereq/checker.go` + `checker_gh.go` + `checker_check.go` | Modify/Create | `Checker.GH` field (nil-guarded) + informative `CheckGH` wired into `Check()` |
| `internal/app/app.go` | Modify | `Deps.GH`; `StatePushPreparation`; `pushPhase`/`originURL`/`compareURL`/`authState`/`prURL`/`prErr` fields |
| `internal/app/keys.go` | Modify | pull `StateSucceeded` into `keySucceeded`; add `keyPushPreparation` |
| `internal/app/commands.go` | Modify | `pushCmd`, `preparePRCmd` (RemoteURL+AuthStatus), `createPRCmd` + messages |
| `internal/app/update.go` | Modify | `onPushDone`, `onPrepDone`, `onPrCreated` reducers |
| `internal/app/view.go` | Modify | `viewPushPreparation`; success footer offers `p` |
| `cmd/deploydeck/main.go` | Modify | construct `github.New(runner)`; wire `Deps.GH` + `Checker.GH` |

## Interfaces / Contracts

```go
// internal/github
type AuthState int
const ( AuthAbsent AuthState = iota; AuthUnauthenticated; AuthAuthenticated )
type Client interface {
    AuthStatus(ctx context.Context) AuthState                                  // one `gh auth status`
    CreatePR(ctx context.Context, base, head, title string) (url, raw string, err error) // arg-slice, non-interactive, Raw kept
}
func New(runner exec.Runner) Client
func CompareURL(originURL, base, head string) (string, error) // pure
func SuggestedTitle(ticket, target string) string            // "<ticket> - Promote changes to <target>"
```

CompareURL parsing → `https://<host>/<org>/<repo>/compare/<base>...<head>`:

| form | example | host | org/repo |
|------|---------|------|----------|
| SSH | `git@github.ibm.com:org/repo.git` | github.ibm.com | org/repo |
| HTTPS | `https://github.com/org/repo(.git)` | github.com | org/repo |
| ssh:// | `ssh://git@host/org/repo(.git)` | host | org/repo |
| unrecognized | — | — | → error (caller shows raw origin + manual data) |

`CreatePR` args: `pr create --base <base> --head <head> --title <title> --body ""` (`--body` forces non-interactive).

## Testing Strategy

| Layer | What to Test | Approach |
|-------|-------------|----------|
| Unit | `CompareURL` SSH/HTTPS/ssh:///`.git`/unrecognized incl. Enterprise `github.ibm.com` | table-driven, pure |
| Unit | `SuggestedTitle`; `AuthStatus` 3-state | table; FakeRunner (missing=Runner error, unauth=ExitCode!=0, authed=0) |
| Unit | `CreatePR` arg-slice + Raw on success AND non-zero exit | FakeRunner `Calls`/canned result |
| Integration | `git.Push`/`RemoteURL` on temp repo + bare local remote; upstream set on first push | real git, `-short`-skippable (`t.TempDir`) |
| Unit | `StatePushPreparation` transitions: `p`→push, push-done→PR-data, gh-branching, explicit-confirm gate, PR-fail fallback, `MarkPRCreated` | `Model.Update` with fake `git.Service`+`github.Client` |

Real `gh pr create` is NEVER run in tests (would create a real PR on github.ibm.com).

## Threat Matrix

| Boundary | Cases | Applicability | Design response | Planned RED tests |
|---|---|---|---|---|
| Documentation-like paths | requirements.txt, README.sh | N/A: no file-path classification/execution added | — | — |
| Git repository selection | `git -C`, relative, absolute | Applicable | `Push`/`RemoteURL` resolve `RepoRoot(dir)`; `newRequest` sets `Dir`, never trusts cwd | integration runs with explicit temp dir ≠ cwd |
| Commit state | staged, `commit -a`, empty index | N/A: push/PR never stage or commit | — | — |
| Push state | tracking branch, first push, explicit refspec | Applicable | `push -u origin <branch>`, branch as discrete arg, `GIT_TERMINAL_PROMPT=0` | integration: first push round-trips to bare remote, upstream set |
| PR commands | explicit `--head`, env prefix, composed | Applicable | explicit `--base`/`--head`/`--title`/`--body`, args as slice (no shell), explicit confirm gate, Raw on success+failure | FakeRunner asserts exact args + Raw; `Update` requires confirm before `createPRCmd` |

## Migration / Rollout

No migration. `PRUrl` is `omitempty` — old `run.json` files load unchanged. Push is the only
external side effect; rollback = `git push origin --delete <branch>` (no PR exists without explicit confirm).

## Open Questions

- [ ] `CreatePR` body: empty `--body ""` vs a minimal generated body. Default: empty (non-interactive, explicit title only).
- [ ] Push-failure UX beyond AC scope: allow retry vs quit. Default: retry key + `q`, flow survives.
