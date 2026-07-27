# Apply Progress: HU-014 — Push And PR Preparation (`push-pr`)

## Batch

**Batch**: 1 of N (Phases 1-4 — leaf modules + data layer)
**Mode**: Strict TDD
**Delivery**: single-pr, `size:exception` (pre-granted, `review_budget_lines=40000`)
**Scope boundary**: Phases 1-4 only. Phase 5 (`internal/app` `StatePushPreparation`), Phase 6
(main wiring), Phase 7 (consolidated E2E) and Phase 8 (final verification) are explicitly
OUT of scope for this batch and remain untouched.

## Completed Tasks

### Phase 1: Git Push & RemoteURL (`internal/git`)
- [x] 1.1 [I] RED `service_push_test.go`
- [x] 1.2 GREEN `service_push.go`: `Push(ctx,dir,branch)`
- [x] 1.3 [I] RED `service_remote_test.go`
- [x] 1.4 GREEN `service_remote.go`: `RemoteURL(ctx,dir,name)`

### Phase 2: New `internal/github` Package
- [x] 2.1 [U] RED `compare_url_test.go` (Enterprise + github.com + ssh:// + trailing-`.git` + unrecognized table)
- [x] 2.2 GREEN `compare_url.go`: pure `CompareURL(originURL,base,head)`
- [x] 2.3 [U] RED `client_test.go`: `AuthStatus` 3-state
- [x] 2.4 GREEN `client.go`: `AuthState`, `Client`, `New(runner)`, `AuthStatus`
- [x] 2.5 [U] RED `client_test.go`: `CreatePR` success/failure + arg-slice assertion
- [x] 2.6 GREEN `client.go`: `CreatePR(ctx,base,head,title)`
- [x] 2.7 [U] RED+GREEN `SuggestedTitle`

### Phase 3: `internal/runs` PRUrl Persistence
- [x] 3.1 [U] RED `writer_test.go`: `PRUrl` round-trip + backward-compat
- [x] 3.2 GREEN `writer.go`: `Record.PRUrl` (`omitempty`, no schema bump)
- [x] 3.3 [U] RED `writer_test.go`: `MarkPRCreated`
- [x] 3.4 GREEN `writer.go`: `MarkPRCreated`

### Phase 4: `internal/prereq` gh Doctor Check
- [x] 4.1 [U] RED `checker_gh_test.go`: absent/unauth/authed + nil-guard regression
- [x] 4.2 GREEN: `Checker.GH` field, `checker_gh.go` `CheckGH`, wired into `Check()`

## Deviations From Design

- **`AuthStatus` signature**: the orchestrator's task prompt paraphrased this as
  `AuthStatus(ctx) (State, error)`, but design.md's authoritative Interfaces/Contracts
  section and its own Architecture Decision are explicit and unambiguous: `AuthStatus(ctx
  context.Context) AuthState` — TOTAL, no error return ("gh 3-state via
  non-zero-exit-as-data" decision). Implemented per design.md (no error return). This is
  a documentation inconsistency in the batch prompt, not a design ambiguity — flagging it
  so the discrepancy is visible, not silently resolved.
- **`CreatePR` Raw type**: the orchestrator's task prompt suggested `raw []byte`; design.md's
  Interfaces/Contracts block specifies `(url, raw string, err error)`. Implemented per
  design.md (`raw string`), consistent with `internal/delta`'s own `combineOutput` returning
  `string`.
- No other deviations — implementation matches design.md's File Changes, Interfaces, and
  Testing Strategy tables exactly for Phases 1-4.

## TDD Cycle Evidence

Strict TDD Mode is active. Every unit below was written test-first (RED, confirmed via
`go vet`/`go build` failing on the missing symbol or `no non-test Go files`), then made to
pass (GREEN) with the minimal implementation, matching existing sibling-package
conventions (`internal/delta`, `internal/salesforce`, `MarkCanceled`).

| # | Task | RED (test written first, confirmed failing) | GREEN (implementation, confirmed passing) | REFACTOR |
|---|---|---|---|---|
| 1 | git `Push` | `service_push_test.go`; `go vet` → `svc.Push undefined` | `service_push.go`; `go test -run Push` PASS | None needed — matches `fetchOrigin`/`newRequest` shape exactly |
| 2 | git `RemoteURL` | `service_remote_test.go`; same RED batch as `Push` (single `go vet` pass) | `service_remote.go` addition; `go test -run RemoteURL` PASS | None — sibling of `HasRemote`, same shape |
| 3 | `github.CompareURL` | `compare_url_test.go`; `go vet` → `no non-test Go files in internal/github` | `compare_url.go`; `go test ./internal/github/...` PASS (8/8 table cases incl. Enterprise) | None |
| 4 | `github.AuthStatus`/`CreatePR`/`SuggestedTitle` | `client_test.go`; `go vet` → `undefined: github.AuthState` | `client.go`; `go test ./internal/github/...` PASS (all cases) | None |
| 5 | `runs.Record.PRUrl` + `MarkPRCreated` | appended to `writer_test.go`; `go vet` → `unknown field PRUrl` | `writer.go` additive field + method; `go test ./internal/runs/...` PASS | None — mirrors `MarkCanceled` exactly |
| 6 | `prereq.CheckGH` + `Checker.GH` | `checker_gh_test.go`; `go vet` → `checker.CheckGH undefined` | `checker.go` field + `checker_gh.go` + wiring in `checker_check.go`; `go test ./internal/prereq/...` PASS | None — mirrors `CheckLock`'s nil-guard pattern |

## Work Unit Evidence

| Unit | Focused test command and exact result | Runtime harness command/scenario and exact result | Rollback boundary |
|---|---|---|---|
| 1: git Push/RemoteURL | `go test ./internal/git/... -run 'Push\|RemoteURL' -v` → `PASS` (6/6 subtests, incl. failure paths) | Real `git` via `exec.NewOSRunner()` against a temp repo + a **bare local remote** (`newTempRepo` helper): first push round-trips to `origin/<branch>` and sets upstream tracking (asserted via `git status --short --branch`) | `rm internal/git/service_push.go internal/git/service_push_test.go internal/git/service_remote_test.go`; `git checkout -- internal/git/service_remote.go` |
| 2: `internal/github` pkg | `go test ./internal/github/... -v` → `PASS` (15/15 cases: 8 CompareURL table rows, 3 AuthStatus states, 3 CreatePR cases, 1 SuggestedTitle) | N/A — pure function + `exec.FakeRunner`; no real `gh` binary is ever invoked (design constraint: never create a real PR) | `rm -r internal/github` |
| 3: `runs` PRUrl+MarkPRCreated | `go test ./internal/runs/... -v` → `PASS` (22/22, all pre-existing + 4 new) | N/A — `t.TempDir()`-backed real filesystem read/write, no external process | `git diff internal/runs/writer.go internal/runs/writer_test.go` shows an isolated additive diff; revert both files |
| 4: `prereq` gh doctor check | `go test ./internal/prereq/... -v` → `PASS` (all pre-existing + 5 new, incl. nil-GH regression) | N/A — `exec.FakeRunner`; nil-guard proven directly (`&prereq.Checker{}` with `GH` unset) | `rm internal/prereq/checker_gh.go internal/prereq/checker_gh_test.go`; `git checkout -- internal/prereq/checker.go internal/prereq/checker_check.go` |

## Issues Found

None. `internal/salesforce`'s existing `Client`/`client`/`New(runner) Client` shape and
`internal/delta`'s `combineOutput` helper were reused as direct precedent for
`internal/github`'s `client`/`CreatePR` Raw handling.

## Final Verification (this batch)

```
export PATH="/usr/local/go/bin:$PATH" && go build ./... && go vet ./... && gofmt -l . && go test -race ./...
```

Result: all packages `ok` (cmd/deploydeck, internal/app, internal/config, internal/delta,
internal/exec, internal/git, internal/github, internal/prereq, internal/runs,
internal/salesforce). `gofmt -l .` produced no output (clean). No `go vet` findings.

## Remaining Tasks (next batch)

- [ ] Phase 5: `internal/app` `StatePushPreparation` (tasks 5.1-5.11)
- [ ] Phase 6: `cmd/deploydeck/main.go` wiring (task 6.1)
- [ ] Phase 7: Consolidated E2E (tasks 7.1-7.3)
- [ ] Phase 8: Final verification + proposal.md success criteria (tasks 8.1-8.5)

## Workload / PR Boundary

- Mode: single PR, `size:exception` (pre-granted, 40,000-line session budget)
- Current work unit: Phases 1-4 of 8 (leaf modules + data layer)
- Boundary: starts from the 4 archived HU-014-adjacent slices (`internal/{exec,config,git,
  salesforce,prereq,app,delta,runs}` + `cmd/deploydeck`, all green); ends with every Phase
  1-4 task green, no `internal/app` or `cmd/deploydeck` changes
- Estimated review budget impact: ~812 changed lines (git diff --stat across the 4 commits
  in this batch) of the ~1,100-1,400 total forecast — well inside the 40,000 session budget

## Status

17/37 tasks complete (Phases 1-4 of 8). Ready for next batch (Phase 5 onward).
