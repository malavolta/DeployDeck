# Archive Report: HU-014 — Push And PR Preparation (`push-pr`)

**Date Archived**: 2026-07-27  
**Status**: Complete and Verified  
**Total Tasks**: 37 (all checked) + 1 post-review fix  
**Final Verification**: PASS  
**Test Result**: `go test -race ./...` Green, `gofmt` and `go vet` Clean

## Executive Summary

HU-014 ships the complete push-and-PR preparation capability to DeployDeck, enabling users to push a validated deploy branch to origin with upstream tracking (`git push -u`), derive and display base/compare/PR suggestion data, and optionally create a GitHub PR via `gh pr create` only after explicit two-step confirmation. The implementation introduces a new `push-pr-preparation` capability, extends `prereq-check` with an informative `gh` availability/auth doctor check, and extends `run-persistence` with PR URL recording. All 37 implementation tasks completed; 1 post-review fix applied (resumed-run empty PromotionBranch). Architecture is sound: no PR can be created without confirmation (invariant verified end-to-end), `gh` client is isolated from app state, git operations are seamed via interfaces, and real git-push integration tested via bare local remote.

## What Shipped

### 1. Push Workflow (`git push -u origin <deploy-branch>`)

**Acceptance Criteria Addressed**: AC1, AC2, AC3

- **Visibility**: Push is offered only after validation succeeds (`StateSucceeded` or `StatePartial`)
- **Confirm-then-Execute**: User must explicitly confirm (key `enter`) before `git push -u origin <deploy-branch>` runs
- **Transport**: Real git push to configured `origin` remote, with upstream tracking set (`-u`)
- **Implementation**: New `git.Service.Push(ctx, dir, branch)` in `internal/git/service_push.go`, tested with temporary Git repos and bare local remotes (round-trip validated)

### 2. PR Preparation Data (Base, Compare, Suggested Title)

**Acceptance Criteria Addressed**: AC3, AC4, AC6

- **Base Branch**: Derived from `TargetBranch` (Salesforce org target)
- **Compare Branch**: `PromotionBranch` (validated deploy branch)
- **Suggested Title**: Format `<ticket> - Promote changes to <target>` (e.g., `HU-014 - Promote changes to production`)
- **Origin-Derived Compare URL**: When `gh` is absent or unauthenticated, the system constructs `https://<host>/<org>/<repo>/compare/<target>...<deploy>` from `origin` remote URL
  - Supports SSH origins: `git@github.com:org/repo.git`, `git@github.ibm.com:org/repo.git`
  - Supports HTTPS origins: `https://github.com/org/repo`, `https://github.ibm.com/org/repo`
  - Supports ssh:// URLs: `ssh://git@host/org/repo.git`
  - Enterprise-safe: derives host from any recognized form (e.g., `github.ibm.com`)
  - Graceful fallback: if origin form is unrecognized, raw origin URL + manual base/compare/title shown instead of a malformed link
- **Implementation**: New `internal/github` package with pure-function `CompareURL(originURL, base, head)` (no side effects, no shell)

### 3. `gh` Availability/Authentication Detection (Informative Check)

**Acceptance Criteria Addressed**: AC5, AC6

- **Three States**: 
  1. `Absent` — `gh` binary not found or `gh auth status` fails completely
  2. `PresentUnauthenticated` — `gh` found but not logged in (non-zero exit code)
  3. `PresentAuthenticated` — `gh` found and logged in (zero exit code)
- **Non-Blocking**: Never blocks the flow; always informative
- **Single Call**: One `gh auth status` invocation per state detection (no repeated calls)
- **Implementation**: 
  - `internal/github.Client.AuthStatus(ctx)` — interface-based with dependency injection (FakeRunner for tests)
  - `internal/prereq/checker_gh.go` — informative PrereqCheck (new requirement in `prereq-check` spec)

### 4. GitHub PR Creation With Two-Step Confirmation

**Acceptance Criteria Addressed**: AC5, AC7

- **Prerequisite**: Only offered when `gh` is `PresentAuthenticated`
- **Two-Step Invariant**:
  1. User sees base/compare/PR title and confirm prompt (first explicit action)
  2. User must press a dedicated confirm key (second explicit action, e.g., key `p` for "PR") to trigger `gh pr create`
  3. No PR is ever created without both steps (invariant validated end-to-end in E2E test)
- **Command Construction**: `gh pr create --base <target> --head <deploy> --title "<title>" --body ""`
  - Base: `TargetBranch` (Salesforce deployment target)
  - Head: `PromotionBranch` (local validated branch)
  - Title: auto-suggested per AC4
  - Body: always empty (`--body ""`)
- **Success Path**: PR URL captured from `gh` output, shown to user, and recorded in run persistence via `Runs.MarkPRCreated(runID, prURL)`
- **Failure Path**: `gh pr create` errors shown with manual base/compare/title data; flow continues (doesn't abort)
- **Implementation**:
  - `internal/github.Client.CreatePR(ctx, base, head, title)` — returns `(url string, raw string, error)`
  - Raw output preserved for debugging even on non-zero exit
  - `internal/app/commands.go` `createPRCmd` — only reachable after explicit TUI confirmation key in `StatePushPreparation`
  - `internal/app/keys.go` — new `keyPushPreparation` gate ensures second confirm key blocks PR create until pressed

### 5. PR URL Persistence

**Acceptance Criteria Addressed**: AC5

- **Record Field**: `Record.PRUrl` (JSON tag: `"prUrl,omitempty"`)
- **No Schema Bump**: Backward compatible; older `run.json` files load cleanly with `PRUrl` zero-valued
- **Writer Method**: `Writer.MarkPRCreated(runID, prURL)` — persists to `run.json`, mirrors the shape of `MarkCanceled`
- **Round-Trip**: Full write/reload cycle preserves `PRUrl` unchanged
- **Implementation**: 
  - `internal/runs/writer.go` — new `Record.PRUrl` field and `MarkPRCreated` method
  - No `SchemaVersion` bump (additive growth)

### 6. Reachable State for Push Preparation

**Architecture Detail**:

- **New State**: `StatePushPreparation` — exists only after successful validation
- **Transition**: Reachable only from `StateSucceeded` (or `StatePartial`) via explicit key press (e.g., `p` for "push")
- **Failed/Canceled/Aborted/Error**: These states remain "quit-only" — no push or PR offered
- **Single Confirm Key**: Pressing `p` from `StateSucceeded` enters `StatePushPreparation` (does not execute push immediately)
- **Second Confirm**: Inside `StatePushPreparation`, user must press `enter` to confirm push; only then `git.Push` runs
- **PR Offer**: After push succeeds, if `gh` is `PresentAuthenticated`, the user may press another key (e.g., `p` again) to create the PR, which requires a third explicit confirm

## Capabilities: 1 New + 2 Modified

| Capability | Action | Details |
|---|---|---|
| `push-pr-preparation` | **Created** | New spec at `openspec/specs/push-pr-preparation/spec.md` — full lifecycle of push, PR data prep, gh detection, and PR creation. 7 requirements, each with 1–4 scenarios. |
| `prereq-check` | **Modified** | Added 1 new requirement (+ 4 scenarios) for informative `gh` CLI availability/auth check. Design Notes updated to note that the check is now implemented by HU-014. |
| `run-persistence` | **Modified** | Added 1 new requirement (+ 3 scenarios) for `Record.PRUrl` field and `Writer.MarkPRCreated` method. Backward-compatible additive growth. |

## Key Decisions

### 1. **New `internal/github` Sibling Package** (vs. Extending `internal/git`)

- Rationale: `git.Service` handles Git operations; `github.Client` handles GitHub/gh operations. Separation of concerns keeps concerns cleanly isolated.
- `github.Client` is interface-based, injectable with a FakeRunner for testing (no real `gh` calls in unit tests).
- Pure-function `CompareURL` has no side effects, no shell invocation, fully testable with table-driven tests.

### 2. **`git.Service.Push` + `git.RemoteURL` in `internal/git`**

- `Push(ctx, dir, branch)` executes `git push -u origin <branch>` and returns error or nil
- `RemoteURL(ctx, dir, name)` retrieves the fetch URL of a named remote (e.g., `origin`), with trimmed stdout
- Both tested with temporary repos and bare local remotes (round-trip validated)

### 3. **Pure-Function Host Derivation** (`CompareURL`)

- No shell invocation, no `github.Client` dependency
- Regex-based SSH/HTTPS/ssh:// URL parsing
- Enterprise-safe: supports any domain (e.g., `github.ibm.com`, `gitlab.internal`)
- Graceful fallback on unrecognized origin form (raw URL + manual data, never a malformed link)

### 4. **New `StatePushPreparation` State** (Reachable Only After Success)

- Clear separation of concerns: `StateSucceeded` is terminal; pressing `p` enters the new transient `StatePushPreparation` state
- Prevents accidental push from failure states
- Each action (push, PR creation) requires explicit confirm

### 5. **Two-Step PR Confirmation Invariant**

- First step: user sees data and decides to proceed (explicit confirm key, e.g., `p`)
- Second step: system shows `gh pr create` command and user confirms (explicit key, e.g., `enter`)
- No PR can be created with a single action
- Verified end-to-end in E2E test; adversarial review found and this implementation prevents accidental PR creation

### 6. **No Local PR Title/Description Model** (Deferred to HU-016)

- Suggested title is hard-coded format: `<ticket> - Promote changes to <target>`
- Local model, description template, multi-line edit, and other customizations are future work (HU-016)
- Current implementation supports user's own PR customization via GitHub UI after creation

### 7. **Runner Injection for `gh` Detection**

- `github.New(runner)` accepts a `Runner` interface (method: `Run(ctx, cmd, args) (stdout, stderr string, err error)`)
- Unit tests use `FakeRunner` with table-driven inputs (3 states: Absent, Unauthed, Authed)
- E2E tests use real runner (but fake `gh` binary to avoid real org/PR creation)

## Review Findings Fixed

### Finding: Resumed-Run with Empty `PromotionBranch`

**Severity**: Low (informative discovery, not a blocking issue)

**Issue**: When resuming a run, the `PromotionBranch` field could be empty in the re-instantiated `Model`, causing a nil-pointer or empty-value error if the push-preparation code tried to use it without validation.

**Fix Applied**: 
- After run load during resume, the `Model` is re-instantiated with the persisted `PromotionBranch` from `Record.Phase` and `Record.Commits`
- Added explicit nil-check in `preparePRCmd` to validate `PromotionBranch` is not empty before deriving PR data
- Test scenario added: resume a past run and verify push-preparation state is re-entered with correct branch data

**Verification**: Unit tests + E2E test for resume flow; all pass.

## Test Posture

### Unit Tests

- **`internal/git/service_push_test.go`**: `Push` with temp Git repos and bare local remotes; real `git push -u` round-trip; upstream tracking verified
- **`internal/git/service_remote_test.go`**: `RemoteURL` with SSH + HTTPS origins; output trimming validated
- **`internal/github/compare_url_test.go`**: Table-driven URL parsing (SSH enterprise, HTTPS enterprise, github.com, trailing `.git`, ssh:// URLs, unrecognized forms); all branches covered
- **`internal/github/client_test.go`**: `AuthStatus` and `CreatePR` with FakeRunner; 3 auth states, success/failure/error branches; raw output preservation
- **`internal/prereq/checker_gh_test.go`**: `CheckGH` with FakeRunner; 3 states always informative/non-blocking; existing checker tests remain green with nil `Checker.GH`
- **`internal/runs/writer_test.go`**: `Record.PRUrl` round-trip; prior `run.json` (no PRUrl) loads cleanly; `MarkPRCreated` persists to `run.json`
- **`internal/app/keys_test.go`**: `keySucceeded` routes `p` to `StatePushPreparation`; other keys stay quit-only; Failed/Canceled/Aborted/Error remain quit-only
- **`internal/app/commands_test.go`** + **`update_test.go`**: `pushCmd` → `git.Push(dir, branch)`; push success → `preparePRCmd` (RemoteURL + AuthStatus) → base/compare/title shown; gh authed offers PR but second confirm gate prevents exec until key pressed
- **`internal/app/boundary_test.go`**: `TestApp_NeverImportsExecSeam` — confirms no direct exec, only via `git.Service`/`github.Client` seams

### Integration Tests

- **`push_pr_e2e_test.go`**:
  - **Round-trip**: Temp repo + bare local remote; drive `Succeeded` → press `p` → real `git push -u origin <branch>` round-trips → verify upstream set
  - **Base/Compare/Title**: After push success, verify `TargetBranch`, `PromotionBranch`, and suggested title shown
  - **3 `gh` States via FakeRunner**:
    - Authed: `CreatePR` runs, URL captured, recorded; verify no PR without confirmation
    - Unauthed: Compare URL derived from origin (SSH and HTTPS forms), no PR offered, no `gh` call attempted
    - Absent: Compare URL fallback, no PR offered
  - **PR Failure**: `gh pr create` fails → error + manual data shown, flow continues
  - **Origin Forms**: SSH (github.com, github.ibm.com), HTTPS, ssh://, Enterprise-safe parsing
  - **Invariant**: No PR created without explicit two-step confirmation (verified end-to-end)

### Code Quality

- **`go test -race ./...`**: All green (no race conditions)
- **`go vet ./...`**: Clean (no vet warnings)
- **`gofmt -l .`**: No formatting issues
- **No new boundary violations**: `TestApp_NeverImportsExecSeam` passes with `Deps.GH` wired

### Test Coverage Summary

- **37 planned tasks**: All checked
- **1 post-review fix**: Resumed-run PromotionBranch nil-check added and tested
- **E2E scenarios**: 6 (basic round-trip, 3 gh states, PR failure, resume with correct state)
- **Adversarial review**: Found 1 LOW (resumed-run empty PromotionBranch), now fixed; confirmed invariant is sound

## Merged Artifacts

### Files Synced to Living Specs

1. **`openspec/specs/push-pr-preparation/spec.md`** (NEW)
   - Created from change's delta (which was a full spec, not a delta format)
   - 7 requirements (push offer conditions, explicit-confirm push, base/compare/title, `gh` detection, PR creation with confirm, compare URL fallback, PR failure handling)
   - 20 scenarios (3–4 per requirement)

2. **`openspec/specs/prereq-check/spec.md`** (MODIFIED)
   - Added 1 new requirement: `gh` CLI availability/auth informative check (4 scenarios)
   - Updated Design Notes: replaced "intentionally deferred to HU-014" with "implemented by HU-014" note

3. **`openspec/specs/run-persistence/spec.md`** (MODIFIED)
   - Added 1 new requirement: PR URL recorded on run (3 scenarios)
   - `Record.PRUrl` field, `MarkPRCreated` method, backward-compatibility scenarios

### Change Artifacts Archived

- **Proposal**: `openspec/changes/push-pr/proposal.md` (moved to archive)
- **Specs**: `openspec/changes/push-pr/specs/{push-pr-preparation,prereq-check,run-persistence}/spec.md` (merged to living; folder archived)
- **Design**: `openspec/changes/push-pr/design.md` (archived)
- **Tasks**: `openspec/changes/push-pr/tasks.md` (archived; 37/37 complete)
- **Verify Report**: Implicit (no separate file; final PASS verified in task tracker and test suite)

## What Remains OUT (Future Work)

### HU-015: Quick Deploy (Parallel with HU-014, Not Dependent)

- Scope: Add `--quick` flag to skip summary/confirmation when deploying to dev sandboxes
- Status: Not included in HU-014; independent story

### HU-016: Local Model PR Title/Description

- Scope: Allow user to customize PR title and description in TUI before `gh pr create` runs
- Current State: HU-014 offers hard-coded suggested title only
- Design Consideration: May need TUI multiline editor, optional field support
- Status: Deferred to HU-016

### HU-017: Cleanup (Explicit Post-Deploy Cleanup)

- Scope: Add explicit option to delete `.deploydeck/runs/<run-id>` and local promotion branch after a successful deployment + PR
- Current State: Cleanup is manual or via retention policy only
- Status: Deferred to HU-017

### HU-018: Multiple Sandbox Deployments in One Run (Complex Orchestration)

- Scope: Deploy to multiple sandboxes in sequence, each with its own validation, push, and PR
- Current State: HU-014 handles single push + PR per run
- Status: Deferred to HU-018

### HU-019: Release Pipeline Automation

- Scope: Extend PR-to-release workflow (e.g., auto-merge on CI pass, auto-tag, auto-deploy to prod)
- Current State: Out of scope; PR creation is manual, merge is manual
- Status: Deferred to HU-019

### CLI Subcommands (Out of Scope for HU-014)

- `deploydeck push <branch>` — manual push without TUI
- `deploydeck pr create <base> <head>` — manual PR creation via CLI
- Status: Not planned for current slice; TUI-only in HU-014

## Specification Conformance

All acceptance criteria from `docs/HISTORIAS.md:HU-014` met:

| AC | Title | Status | Evidence |
|---|---|---|---|
| AC1 | Push offered only after successful validation | ✅ | `keySucceeded` test + E2E |
| AC2 | Failed validation does not offer push | ✅ | State machine keeps Failed/Canceled/Aborted/Error quit-only |
| AC3 | Explicit-confirm push with upstream tracking | ✅ | `service_push_test.go` round-trip + E2E real bare remote |
| AC4 | Base/compare/title shown after push success | ✅ | `commands_test.go` + E2E |
| AC5 | PR creation after explicit confirmation + URL recorded | ✅ | Two-step confirm invariant tested; `MarkPRCreated` persists URL |
| AC6 | Compare URL derived from origin (Enterprise-safe) | ✅ | `compare_url_test.go` table (SSH/HTTPS/Enterprise) + E2E |
| AC7 | PR creation failure shows error + manual data, flow continues | ✅ | `client_test.go` failure branch + E2E |

## Transition to Production

### Pre-Commit Checklist

- [x] All 37 tasks checked off in `tasks.md`
- [x] `go test -race ./...` green
- [x] `go vet ./...` clean
- [x] `gofmt -l .` clean
- [x] `boundary_test.go` passes (no new exec seams)
- [x] Proposal.md success criteria checked off
- [x] E2E tests pass (no real org, no real PR, bare local remote only)
- [x] Adversarial review: 1 LOW hole (resumed-run empty PromotionBranch) found and fixed

### Deployment Notes

1. **No Database Migrations**: `Record.PRUrl` is additive; existing `run.json` files load cleanly
2. **No Config Changes Required**: `gh` detection is automatic (no new config flags)
3. **Backward Compatible**: Older runs resume correctly; push + PR flow is opt-in (user-initiated via TUI)
4. **Rollback**: Delete `internal/github` package, revert `internal/git/service_push.go`, revert `internal/runs/writer.go`, revert `internal/prereq/checker_gh.go`, revert `internal/app/{keys,commands,update,view}.go` diffs

## Handoff Summary

- **Source of Truth Updated**: `openspec/specs/{push-pr-preparation,prereq-check,run-persistence}/spec.md` now authoritative
- **Change Folder**: Ready to be archived to `openspec/changes/archive/2026-07-27-push-pr/`
- **Code Ready**: All tests pass, all tasks complete, adversarial review findings fixed
- **No Blockers**: Clean bill of health for merge

---

**Archived By**: SDD Archive Phase (push-pr)  
**Archive Date**: 2026-07-27  
**Next Steps**: Merge to `main`, deploy, and proceed with HU-015 or HU-016
