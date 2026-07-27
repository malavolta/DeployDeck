# Tasks: HU-014 — Push And PR Preparation (`push-pr`)

## Review Workload Forecast

| Field | Value |
|---|---|
| Estimated changed lines | ~1,100–1,400 (adds+dels) |
| Session review budget (preflight cache) | 40,000 |
| 400-line budget risk | High vs generic 400 default; Low vs session's actual 40,000 budget |
| Chained PRs recommended | No |
| Suggested split | Single PR — fits comfortably (~3% of 40,000) |
| Delivery strategy | single-pr |
| Chain strategy | size-exception (pre-granted via cached `review_budget_lines=40000`; no re-ask) |

Decision needed before apply: No
Chained PRs recommended: No
Chain strategy: size-exception
400-line budget risk: Low

### Suggested Work Units (commits inside the single PR)

| Unit | Goal | PR | Focused test | Runtime harness | Rollback boundary |
|---|---|---|---|---|---|
| 1 | git Push/RemoteURL | PR1 | `go test ./internal/git/... -run 'Push\|RemoteURL'` | temp repo+bare remote, `-short`-skippable | delete `service_push.go`, revert `service_remote.go` |
| 2 | `internal/github` pkg | PR1 | `go test ./internal/github/...` | N/A — pure fn + FakeRunner | `rm -r internal/github` |
| 3 | `runs` PRUrl+MarkPRCreated | PR1 | `go test ./internal/runs/...` | N/A — `t.TempDir` | revert `writer.go` diff |
| 4 | `prereq` gh doctor check | PR1 | `go test ./internal/prereq/...` | N/A — FakeRunner | revert `checker*.go` diff |
| 5 | `app` StatePushPreparation | PR1 | `go test ./internal/app/... -run PushPreparation` | `Model.Update` w/ fake Deps | revert `app/*.go` diff |
| 6 | main wiring + E2E | PR1 | `go test ./... -run E2E` | temp+bare remote + FakeRunner gh (no real org/PR) | revert `main.go`, delete e2e test |

## Phase 1: Git Push & RemoteURL (`internal/git`)
- [x] 1.1 [I] RED `service_push_test.go`: temp repo+bare remote, first push sets upstream, round-trips (AC3)
- [x] 1.2 GREEN `service_push.go`: `Push(ctx,dir,branch)` = `git push -u origin <branch>`, `RepoRoot`+`newRequest` shape
- [x] 1.3 [I] RED `service_remote_test.go`: `RemoteURL` trimmed stdout, SSH+HTTPS origin
- [x] 1.4 GREEN `service_remote.go`: add `RemoteURL(ctx,dir,name)`, sibling of `HasRemote`

## Phase 2: New `internal/github` Package
- [ ] 2.1 [U] RED `compare_url_test.go` table: SSH `github.ibm.com` (Enterprise), HTTPS `github.ibm.com`, github.com SSH+HTTPS, trailing `.git`, `ssh://git@host/org/repo.git`, unrecognized→error (AC6)
- [ ] 2.2 GREEN `compare_url.go`: pure `CompareURL(originURL,base,head)` → `https://<host>/<org>/<repo>/compare/<base>...<head>`
- [ ] 2.3 [U] RED `client_test.go`: `AuthStatus` 3 states via FakeRunner (Runner err→Absent, ExitCode!=0→Unauthed, 0→Authed) (AC5/AC6)
- [ ] 2.4 GREEN `client.go`: `AuthState`, `Client` interface, `New(runner)`, `AuthStatus`
- [ ] 2.5 [U] RED `client_test.go`: `CreatePR` FakeRunner — success→URL parsed+Raw kept; non-zero exit→error+Raw kept; exact arg-slice `pr create --base --head --title --body ""` (AC5/AC7)
- [ ] 2.6 GREEN `client.go`: `CreatePR(ctx,base,head,title)`
- [ ] 2.7 [U] RED+GREEN `SuggestedTitle`: `<ticket> - Promote changes to <target>` (AC4)

## Phase 3: `internal/runs` PRUrl Persistence
- [ ] 3.1 [U] RED `writer_test.go`: `PRUrl` round-trips write/reload; prior `run.json` (no PRUrl) loads zero-valued (run-persistence delta)
- [ ] 3.2 GREEN `writer.go`: `Record.PRUrl` `json:"prUrl,omitempty"`, no schema bump
- [ ] 3.3 [U] RED `writer_test.go`: `MarkPRCreated(runID,prURL)` persists to `run.json`, mirrors `MarkCanceled`
- [ ] 3.4 GREEN `writer.go`: implement `MarkPRCreated`

## Phase 4: `internal/prereq` gh Doctor Check
- [ ] 4.1 [U] RED `checker_gh_test.go`: `CheckGH` absent/unauth/authed, always informative/non-blocking; existing checker tests stay green with `Checker.GH` nil (prereq-check delta)
- [ ] 4.2 GREEN: `checker.go` add `Checker.GH` (nil-guarded); `checker_gh.go` `CheckGH`; wire into `checker_check.go` `Check()`

## Phase 5: `internal/app` StatePushPreparation
- [ ] 5.1 [T] RED `keys_test.go`: `keySucceeded` — `p`→`StatePushPreparation`; `q`/`enter`/`esc` quit; Failed/Canceled/Aborted/Error stay quit-only (AC1/AC2)
- [ ] 5.2 GREEN: `keys.go` splits `StateSucceeded` out of the shared quit-only case; `app.go` adds `StatePushPreparation`, `Deps.GH`, push-flow fields
- [ ] 5.3 [T] RED `commands_test.go`/`update_test.go`: confirm→`pushCmd`→`git.Push(dir,branch)`; push success→`preparePRCmd` (`RemoteURL`+`AuthStatus`)→base/compare/`SuggestedTitle` shown (AC3/AC4)
- [ ] 5.4 GREEN: `commands.go` `pushCmd`,`preparePRCmd`; `update.go` `onPushDone`,`onPrepDone`; `view.go` `viewPushPreparation` + success footer offers `p`
- [ ] 5.5 [T] RED: gh AUTHED offers PR data but `createPRCmd` fires ONLY after a second explicit confirm key — invariant test: no `gh pr create` without explicit confirmation (AC5, spec invariant)
- [ ] 5.6 GREEN: `keys.go` `keyPushPreparation` confirm gate; `commands.go` `createPRCmd`
- [ ] 5.7 [T] RED: PR success→`Runs.MarkPRCreated` called + URL shown; PR failure→error+manual base/compare/title, flow continues (AC5/AC7)
- [ ] 5.8 GREEN: `update.go` `onPrCreated` success/failure branches
- [ ] 5.9 [T] RED: gh ABSENT/UNAUTHED→shows `CompareURL` from `RemoteURL`, no PR offered, no gh call attempted (AC6)
- [ ] 5.10 GREEN: wire ABSENT/UNAUTHED branch in `onPrepDone`
- [ ] 5.11 Re-run `boundary_test.go` (`TestApp_NeverImportsExecSeam`) — confirm still green with `Deps.GH` wired

## Phase 6: Wiring
- [ ] 6.1 GREEN `cmd/deploydeck/main.go`: construct `github.New(runner)`, wire `Deps.GH` + `Checker.GH` (mechanical DI, no RED)

## Phase 7: Consolidated E2E [I]
- [ ] 7.1 [I] RED `push_pr_e2e_test.go`: temp repo+bare local remote; drive Succeeded→`p`→real `git push -u` round-trips→base/compare/title
- [ ] 7.2 [I] extend: 3 gh states via FakeRunner (authed→`CreatePR`+URL recorded; unauthed/absent→compare URL, origin in SSH AND HTTPS forms); PR-failure→error+manual data
- [ ] 7.3 GREEN: close any integration gaps until 7.1/7.2 pass; assert no PR without confirmation end-to-end

## Phase 8: Final Verification
- [ ] 8.1 `go test -race ./...` all green
- [ ] 8.2 `go vet ./...` clean
- [ ] 8.3 `gofmt -l .` clean
- [ ] 8.4 `boundary_test.go` passes
- [ ] 8.5 Check off proposal.md Success Criteria

## Constraints (apply throughout)
- `internal/app` never execs directly — `git.Service`/`github.Client` hold the seam
- No PR is ever created without explicit confirmation
- `CreatePR` body is always `--body ""`
- OUT OF SCOPE, no tasks: local-model PR title/description, HU-016 (`SourceRunID`), HU-017 (`Cleanup`), HU-015 (quick deploy), CLI push/PR subcommand
