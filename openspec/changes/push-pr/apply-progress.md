# Apply Progress: HU-014 — Push And PR Preparation (`push-pr`)

## Batch

**Batch**: FINAL (Phases 5-8 — `internal/app` state machine + wiring + E2E + verification).
Merges the prior batch (Phases 1-4 — leaf modules + data layer).
**Mode**: Strict TDD
**Delivery**: single-pr, `size:exception` (pre-granted, `review_budget_lines=40000`)
**Scope boundary (this final batch)**: Phase 5 (`internal/app` `StatePushPreparation`),
Phase 6 (main wiring), Phase 7 (consolidated E2E), Phase 8 (final verification). Phases 1-4
were completed and committed in the prior batch and are preserved below unchanged.
**Status**: ALL 37 tasks complete across all 8 phases. Ready for verify.

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

### Phase 5: `internal/app` `StatePushPreparation` (this batch)
- [x] 5.1 [T] RED `push_preparation_test.go`: `keySucceeded` — `p`→`StatePushPreparation`; q/enter/esc quit; Failed/Canceled/Aborted/Error stay quit-only
- [x] 5.2 GREEN: `keys.go` splits `StateSucceeded` into `keySucceeded`; `app.go` adds `StatePushPreparation`, `pushPhase`, `Deps.GH`, push-flow fields
- [x] 5.3 [T] RED: confirm→`pushCmd`→`git.Push(dir,branch)`; push success→`preparePRCmd`→base/compare/`SuggestedTitle` shown
- [x] 5.4 GREEN: `commands.go` `pushCmd`/`preparePRCmd`; `update.go` `onPushDone`/`onPrepDone`; `view.go` `viewPushPreparation` + success footer `p preparar push`
- [x] 5.5 [T] RED (spec invariant): authed `g` reveals confirm gate but `createPRCmd` fires only after explicit `y`; no `gh pr create` without confirmation, none on the fallback path
- [x] 5.6 GREEN: `keys.go` `keyPushPreparation` confirm gate; `commands.go` `createPRCmd`
- [x] 5.7 [T] RED: PR success→`Runs.MarkPRCreated` + URL shown; PR failure→error + manual base/compare/title, flow continues
- [x] 5.8 GREEN: `update.go` `onPrCreated` success/failure branches
- [x] 5.9 [T] RED: gh ABSENT/UNAUTHED→`CompareURL` from `RemoteURL` shown (SSH/HTTPS/Enterprise + unrecognized→raw origin), no PR offered/attempted
- [x] 5.10 GREEN: fallback branch wired in `onPrepDone`/`viewPRData`
- [x] 5.11 `boundary_test.go` (`TestApp_NeverImportsExecSeam`) confirmed green with `Deps.GH` wired

### Phase 6: Wiring (this batch)
- [x] 6.1 GREEN `cmd/deploydeck/main.go`: `github.New(runner)` wired into `Deps.GH` (TUI) + `Checker.GH` (doctor)

### Phase 7: Consolidated E2E [I] (this batch)
- [x] 7.1 [I] `push_pr_e2e_test.go`: temp repo + bare local remote; Succeeded→`p`→real `git push -u` round-trips (upstream asserted)→base/compare/title
- [x] 7.2 [I] 3 gh states via FakeRunner (authed+confirm→`CreatePR`+`PRUrl` recorded; unauthed/absent→compare URL, origin in SSH AND HTTPS + Enterprise; PR-failure→error+manual)
- [x] 7.3 GREEN: no PR without confirmation asserted end-to-end; failed/canceled terminal never offers push (AC2)

### Phase 8: Final Verification (this batch)
- [x] 8.1 `go test -race ./...` all green
- [x] 8.2 `go vet ./...` clean
- [x] 8.3 `gofmt -l .` clean
- [x] 8.4 `boundary_test.go` passes
- [x] 8.5 proposal.md Success Criteria checked off (all 5)

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

### Phase 5-8 deviations

- **PR confirm gate is a two-step `g` → `y` (design-faithful)**: design.md's data flow is
  explicit — `authed? └ yes ─▶ g ─▶ confirm ─▶ GH.CreatePR` and the decision "`g` reveals the
  exact `gh pr create` command; a second explicit key fires it." Implemented exactly that:
  `g` (on `pushReady`, authed only) reveals the confirm gate (`pushPRConfirm`), and an
  explicit `y` there is the SOLE trigger of `createPRCmd`. This satisfies the spec invariant
  ("no PR without explicit confirmation") with a hard, testable gate: on the compare-fallback
  path `g` is inert, so `CreatePR` is structurally unreachable there. The mockup's
  always-visible `gh pr create` command line is preserved (shown on `pushReady`).
- **`viewValidationResult` footer is conditional on `StateSucceeded`**: the shared
  Succeeded/Failed/Canceled result view now appends `p preparar push` ONLY for
  `StateSucceeded` (AC1/AC2), leaving Failed/Canceled quit-only. No new view function was
  needed for the success footer; `viewPushPreparation`/`viewPRData` are the new HU-014 screens.
- **Push-failure UX**: a non-zero push keeps the user on `pushConfirm` with `pushErr` shown
  (flow survives, `p` retries, `q` quits) — matching design.md's Open Question default
  ("retry key + q, flow survives"). No terminal error state on push failure.
- No other deviations — Phase 5-8 match design.md's File Changes, Interfaces/Contracts,
  Data Flow, and Testing Strategy. `AuthStatus(ctx) AuthState` (total) and
  `CreatePR(ctx,base,head,title) (url, raw string, err error)` were consumed exactly as the
  batch prompt clarified.

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
| 7 | app `StatePushPreparation` (keys/reducers/commands/view) | `push_preparation_test.go` (6 funcs: keySucceeded split, push confirm→push, invariant no-PR-without-confirm, PR success records URL, PR failure manual data, fallback compare URL); `go vet` → `undefined: StatePushPreparation` | app.go/keys.go/commands.go/update.go/view.go; `go test ./internal/app/ -run PushPreparation\|Succeeded_OwnHandler` PASS | None — mirrors HU-012's `c`-gated typed-confirm sub-flow shape |
| 8 | Consolidated E2E `[I]` | `push_pr_e2e_test.go` (real push round-trip + 3 gh states + PR-failure + AC2 no-push-on-failure); driven test-first against the Phase-5 state machine | `go test ./internal/app/ -run TestHU014_PushPR_E2E -v` PASS (all subtests) | None |

Phase 6 (`cmd/deploydeck/main.go` DI) is mechanical wiring with no RED (task 6.1 is
explicitly "no RED"); covered by `go test ./cmd/... -race` PASS and the existing doctor E2E.

## Work Unit Evidence

| Unit | Focused test command and exact result | Runtime harness command/scenario and exact result | Rollback boundary |
|---|---|---|---|
| 1: git Push/RemoteURL | `go test ./internal/git/... -run 'Push\|RemoteURL' -v` → `PASS` (6/6 subtests, incl. failure paths) | Real `git` via `exec.NewOSRunner()` against a temp repo + a **bare local remote** (`newTempRepo` helper): first push round-trips to `origin/<branch>` and sets upstream tracking (asserted via `git status --short --branch`) | `rm internal/git/service_push.go internal/git/service_push_test.go internal/git/service_remote_test.go`; `git checkout -- internal/git/service_remote.go` |
| 2: `internal/github` pkg | `go test ./internal/github/... -v` → `PASS` (15/15 cases: 8 CompareURL table rows, 3 AuthStatus states, 3 CreatePR cases, 1 SuggestedTitle) | N/A — pure function + `exec.FakeRunner`; no real `gh` binary is ever invoked (design constraint: never create a real PR) | `rm -r internal/github` |
| 3: `runs` PRUrl+MarkPRCreated | `go test ./internal/runs/... -v` → `PASS` (22/22, all pre-existing + 4 new) | N/A — `t.TempDir()`-backed real filesystem read/write, no external process | `git diff internal/runs/writer.go internal/runs/writer_test.go` shows an isolated additive diff; revert both files |
| 4: `prereq` gh doctor check | `go test ./internal/prereq/... -v` → `PASS` (all pre-existing + 5 new, incl. nil-GH regression) | N/A — `exec.FakeRunner`; nil-guard proven directly (`&prereq.Checker{}` with `GH` unset) | `rm internal/prereq/checker_gh.go internal/prereq/checker_gh_test.go`; `git checkout -- internal/prereq/checker.go internal/prereq/checker_check.go` |
| 5: app `StatePushPreparation` | `go test ./internal/app/ -run 'PushPreparation\|Succeeded_OwnHandler\|BoundaryStillHolds' -count=1` → `ok` (7 test funcs, incl. spec-invariant no-PR-without-confirm and boundary) | `Model.Update` with `git.New(FakeRunner)` + `github.New(FakeRunner)` fakes; real integration is the Phase-7 E2E | `git checkout -- internal/app/{app,keys,commands,update,view}.go`; `rm internal/app/push_preparation_test.go` |
| 6: cmd wiring | `go test ./cmd/... -count=1` → `ok` (existing doctor/root E2E green with `Checker.GH` wired) | Real `github.New(NewOSRunner())` composed in `defaultRunTUI`/`defaultChecker` | `git checkout -- cmd/deploydeck/main.go` |
| 7: consolidated E2E | `go test ./internal/app/ -run TestHU014_PushPR_E2E -v -count=1` → `PASS` (all subtests) | Real `git push -u origin <branch>` round-tripping to a **bare local remote** (upstream tracking asserted); 3 gh states via `exec.FakeRunner` (real `gh pr create` NEVER run) | `rm internal/app/push_pr_e2e_test.go` |

## Issues Found

None. `internal/salesforce`'s existing `Client`/`client`/`New(runner) Client` shape and
`internal/delta`'s `combineOutput` helper were reused as direct precedent for
`internal/github`'s `client`/`CreatePR` Raw handling.

## Final Verification (whole change, this batch)

```
export PATH="/usr/local/go/bin:$PATH" && go build ./... && go vet ./... && gofmt -l . && go test -race ./...
```

Result: all packages `ok` under `-race` (cmd/deploydeck, internal/app, internal/config,
internal/delta, internal/exec, internal/git, internal/github, internal/prereq, internal/runs,
internal/salesforce). `gofmt -l .` produced no output (clean). No `go vet` findings.
`TestApp_NeverImportsExecSeam` PASS (app imports `internal/github`, never `os/exec` /
`internal/exec` directly).

## Commits (this final batch)

- `feat(app): push-preparation state and PR flow` — Phase 5 (app.go/keys.go/commands.go/
  update.go/view.go + `push_preparation_test.go`)
- `feat(cmd): wire github client into TUI and doctor` — Phase 6 (main.go)
- `test(app): consolidated push-pr e2e` — Phase 7 (`push_pr_e2e_test.go`)

## Remaining Tasks

None. All 37 tasks across Phases 1-8 are complete.

## Workload / PR Boundary

- Mode: single PR, `size:exception` (pre-granted, 40,000-line session budget)
- Final work unit: Phases 5-8 of 8 (app state machine + wiring + E2E + verification)
- Boundary: starts from the Phases 1-4 leaf modules (all green, prior batch); ends with the
  full HU-014 flow reachable end-to-end (Succeeded → `p` → push → PR/compare) and green under
  `-race`. Rollback of this batch = revert the 3 commits above; Phases 1-4 stay intact.
- Estimated review budget impact: this batch adds ~520 changed lines (app state machine +
  two test files + main wiring) atop the prior ~812, ~1.3k total — well inside 40,000.

## Status

37/37 tasks complete (Phases 1-8, all 8 phases). All 5 proposal Success Criteria checked off.
Ready for verify.
