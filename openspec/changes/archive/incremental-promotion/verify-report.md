# Verification Report: Incremental Promotion (reuse-on-collision)

**Change**: `incremental-promotion`
**Branch**: `feat/incremental-promotion` (applied, not committed)
**Mode**: OpenSpec (file-based), Strict TDD ACTIVE
**Verdict**: **PASS WITH WARNINGS**
**Summary**: 0 CRITICAL / 1 WARNING / 2 SUGGESTION. Every spec requirement and scenario is
implemented and covered by a test that passed at runtime. The load-bearing FastForwardBranch
safety property (diverged = data, never force-push/rewrite) is proven against a real git repo.

---

## Completeness

| Dimension | Result |
|---|---|
| Tasks complete (tasks.md) | 26/26 checked |
| Tasks match code state | Yes — every file in the apply "Files Changed" table exists with the described symbols |
| TDD Cycle Evidence present | Yes — full table in apply-progress.md |
| Delta specs verified | 4/4 (incremental-promotion, promotion-branch, push-pr-preparation, run-persistence) |

## Build / Test / Static Evidence

| Command | Exit | Result |
|---|---|---|
| `PATH=/usr/local/go/bin:$PATH go test ./... -race -count=1` | 0 | ok across all 13 packages |
| `go vet ./...` | 0 | clean |
| `gofmt -l internal` | 0 | clean (no files listed) |

- `test_output_hash` (sha256): `d32876e123598dec61463fda342af872bd3db8fd15e73478fedc363dc0bc5f8f`
- Packages: cmd/deploydeck, internal/{ai,app,config,delta,exec,git,github,prereq,runs,salesforce,update,version} — all `ok` with `-race`.
- `internal/app` 30.3s, `internal/git` 62.5s (real-temp-repo integration/E2E).

---

## Spec Compliance Matrix (requirement → runtime-proven test)

### capability: incremental-promotion

| Requirement / Scenario | Evidence | Status |
|---|---|---|
| Branch Collision Offers Reuse/Recreate/Cancel; no dead-end | `onBranchCreated` routes `ErrPromotionBranchExists`→`StateBranchCollision` (update.go:571); `viewBranchCollision` renders r/d/c | PASS |
| — Collision presents all three choices | `TestOnBranchCreated_ErrPromotionBranchExists_RoutesToStateBranchCollision`, `TestViewBranchCollision_ShowsThreeChoices` | PASS |
| — Cancel leaves branch + PR untouched | `keyBranchCollision` c/esc → `StatePlanPreview`, cmd==nil (no git call) — `TestKeyBranchCollision_R_D_C` c/esc subtests | PASS |
| — Delete & recreate replaces the branch fresh | `deleteAndRecreateBranchCmd` (DeleteLocalBranch→CreatePromotionBranch) — `TestKeyBranchCollision_R_D_C` "d" | PASS* (see W1) |
| Layered Already-On-Branch Detection (ancestry→trailer→cherry) | `FilterNotOnBranch` composes IsAncestor→trailers→Cherry`=='-'` in order (service_reuse.go:128-142) | PASS |
| — Ancestry-merged commit detected | `TestService_FilterNotOnBranch_LayeredClassification` (ancestorSHA excluded) | PASS |
| — Reapplied-under-different-SHA detected via trailer | same test, `cherry-pick -x` trailerSourceSHA excluded | PASS |
| Not-Yet-Present Commits Filtered Before Cherry-Pick | `reuseBranchCmd`→`FilterNotOnBranch`; only remainder passed to `cherryPickCmd` | PASS |
| — Only new commits are cherry-picked | E2E: 2 selected, A already present, only B picked (`b.cls` present, tip=B) | PASS |
| Empty-After-Filter Shows Explicit Notice (no raw pick error) | `onReuseReady` len(remaining)==0 → notice + `StatePushPreparation` (update.go:654) | PASS |
| — All present shows clear notice | `TestOnReuseReady` "empty remainder shows a notice" | PASS |
| Diverged Remote Blocks With Clear Error | `FastForwardBranch` FFDiverged (data) → `onReuseReady`→`StateError`, no push | PASS |
| — Remote strictly ahead is fast-forwarded | `TestService_FastForwardBranch_TableDriven` "remote strictly ahead" → FFFastForwarded via `merge --ff-only` | PASS |
| — Diverged surfaces clear error, no push/overwrite | see Safety Judgment below | PASS |
| Reuse Targets The Prior Run For The Same Branch | `runs.FindRunForBranch` over `m.runs` (update.go:666) | PASS |
| — Locates matching prior run | `TestFindRunForBranch` match; `TestOnReuseReady` mutate-in-place | PASS |
| — No prior run = fresh increment target (no error) | `onReuseReady` else-branch seeds fresh run → `StateCherryPicking`; `TestOnReuseReady` "no matching prior run", `TestFindRunForBranch` no-match | PASS |

### promotion-branch (MODIFIED)

| Requirement / Scenario | Evidence | Status |
|---|---|---|
| Existing Temp Branch Collision Handling → reuse/recreate/cancel, no dead-end | as incremental-promotion R1 above | PASS* (W1) |
| — Existing branch offers reuse/recreate/cancel | `TestViewBranchCollision_ShowsThreeChoices` | PASS |
| — Collision never dead-ends silently | `onBranchCreated` never routes the collision to StateError | PASS |

### push-pr-preparation (MODIFIED)

| Requirement / Scenario | Evidence | Status |
|---|---|---|
| Check open PR first; skip create; else offer create; record URL | `preparePRCmd` gated `reusing && auth==Authenticated` → `PRForBranch`; `onPrepDone` gates create | PASS |
| — Authenticated + confirm creates PR (no open PR) | existing push-pr create suite (unchanged) | PASS |
| — No PR without confirmation | existing behavior (unchanged) | PASS |
| — Explicitly accepted AI title used | `effectiveTitle` gates on `aiAccepted` (app.go:672) — pre-existing ai-pr-summary, unchanged | PASS |
| — Open PR already exists → skip create, show URL | `TestPreparePRCmd_Reusing_SkipsCreateWhenPROpen` (prCreate calls==0) + E2E Property 3 | PASS |
| — PR lookup unavailable degrades to manual flow | guard `auth==Authenticated` short-circuits PRForBranch; `TestPreparePRCmd_NotReusing_NeverCallsPRForBranch` proves the reuse gate | PASS (see S1) |
| — Closed/merged falls through to create + notice | `TestPreparePRCmd_Reusing_ClosedFallsThroughToCreateWithNotice`; `onPrepDone` notice (update.go:1191) | PASS |

### run-persistence (ADDED)

| Requirement / Scenario | Evidence | Status |
|---|---|---|
| Incremental Append Mutates Same Run In Place | `onReuseReady` mutate branch (update.go:666-674) | PASS |
| — Reuse appends to existing run record | `TestOnReuseReady` mutate-in-place + E2E (reload asserts Commits [A,B], PickTotal 2) | PASS |
| — CreatedAt and RunID survive | both tests assert `rec.CreatedAt.Equal(created)` and `rec.RunID==prior.RunID` | PASS |
| — Increment does not set SourceRunID | `TestOnReuseReady` asserts `rec.SourceRunID==""` | PASS |
| — Prior-slice run.json still loads after mutation | both tests `writer.Load(RunID)` succeeds, reflects extended commits | PASS |

---

## Safety Judgment — FastForwardBranch (load-bearing)

**CONFIRMED SAFE.** The diverged case is returned purely as `FFDiverged` DATA and no code path
force-pushes or rewrites either tip:

- `FastForwardBranch` (service_reuse.go:48) performs an IsAncestor-layered classification via TWO
  `merge-base --is-ancestor` checks BEFORE any mutation. `git merge --ff-only` runs ONLY in the
  `localAncestorOfRemote && !remoteAncestorOfLocal` case (origin is a strict descendant of local —
  a genuine fast-forward). Equal, local-ahead, and diverged are each resolved from the two ancestry
  checks alone with NO merge/push call.
- The file contains no `push`, no `--force`, no `reset`/`update-ref`. `reuseBranchCmd` hard-stops on
  `FFDiverged` and returns before `FilterNotOnBranch`; `onReuseReady` maps it to `StateError` with a
  clear message and fires no follow-up command.
- Runtime proof: `TestService_FastForwardBranch_TableDriven` "diverged" seeds a REAL divergence
  (remote-only + local-only commits), asserts the seeding actually diverged both tips, then asserts
  both `rev-parse branch` and `rev-parse origin/branch` are BYTE-IDENTICAL before vs. after the call.
  This is a genuine behavioral safety proof, not a tautology.

---

## Deviations from Design — Conformance Check

| Deviation (apply-progress) | Assessment |
|---|---|
| `reuseBranchCmd` uses `sourceRef = m.source.Name` (task said `.SHA`, which does not exist on `git.Branch`) | CONFORMANT / non-behavioral. `git.Branch` exposes only `Name`/`Remote`; `Name` is a valid ref for `Cherry`'s head and degrades to `""` identically when unresolved, matching `FilterNotOnBranch`'s documented empty-sourceRef contract. |
| Added display-only `prExisting bool` (not in design) for `viewPRData` wording | CONFORMANT / non-behavioral. Set in `onPrepDone` only on the open-PR branch; affects only rendered text ("PR existente"). No test-contract or control-flow impact. |

## Boundary Invariant

**HOLDS.** The only new process execution introduced by this change lives in the git/github layers:
`internal/git/service_reuse.go` (`git merge --ff-only`, `git log`) and `internal/github/client.go`
(`gh pr view --json url,state`). `internal/app` performs zero direct exec (grep finds only boundary
comments; no `runner.Run`/`newRequest`/`os/exec` in app or runs). Pre-existing exec in
salesforce/delta/prereq and the cmd/main editor handoff are out of scope for this change.

---

## TDD Compliance (Strict)

| Check | Result | Details |
|---|---|---|
| TDD Evidence reported | PASS | Full TDD Cycle Evidence table in apply-progress.md |
| All tasks have tests | PASS | Every behavior maps to a `Test*` in one of 6 new test files |
| RED confirmed (tests exist) | PASS | All 6 test files exist on disk |
| GREEN confirmed (tests pass) | PASS | All packages `ok` under `-race -count=1` |
| Triangulation adequate | PASS | Table-driven: FastForward (4 cases +absent), FilterNotOnBranch (layered+degrade), PRForBranch (5), FindRunForBranch (3), onReuseReady (5) |
| Safety net for modified files | PASS | Existing `onBranchCreated`/push-prep suites re-run clean per apply-progress |

**TDD Compliance**: 6/6 checks passed.

## Test Layer Distribution

| Layer | Tests | Files |
|---|---|---|
| Unit | ParseCherryPickTrailers (4), FindRunForBranch (3), PRForBranch (5 FakeRunner), collision-routing/keys/view | trailers_test, finder_test, pr_for_branch_test, branch_collision_test |
| Integration (real temp git) | FastForwardBranch (5), FilterNotOnBranch (2), reuse call-order, onReuseReady mutate | service_reuse_test, branch_collision_test |
| E2E (real git + fake gh) | reuse-appends-only-new + same-RunID + skip-create-when-open | incremental_promotion_e2e_test |

## Assertion Quality

All assertions verify real behavior against a real temp git repo or a FakeRunner with concrete value
checks (tips, persisted record fields, call ordering, gh-create call count). No tautologies, no
smoke-only tests, no ghost loops. The one loop in `TestPreparePRCmd_NotReusing` is a valid negative
existence check (fails if a `gh pr view` call is present; `fr.Calls` is non-empty by construction).
**Assertion quality**: All assertions verify real behavior.

**Linter (go vet)**: no errors. **gofmt**: clean.

---

## Issues

### CRITICAL
None.

### WARNING

**W1 — Delete & recreate can dead-end to `StateError` on a pure origin-only collision.**
`deleteAndRecreateBranchCmd` (commands.go:709) calls `DeleteLocalBranch` first, which runs
`git branch -D -- <name>` and returns an error (exit 1, "branch not found") when the branch exists
ONLY on origin with no local copy. That error is not `ErrPromotionBranchExists`, so `onBranchCreated`
routes it to `StateError` rather than back to `StateBranchCollision`. The code comment at
commands.go:699-701 asserts an origin-only branch "re-enters StateBranchCollision," which holds only
when a LOCAL branch also exists (local delete succeeds, then CreatePromotionBranch re-collides on the
origin copy → `ErrPromotionBranchExists` → collision). Impact is bounded: the collision decision
still always offers three choices, and both `reuse` and `cancel` work correctly for origin-only
branches; only a user explicitly choosing `d` on a branch with no local copy sees the raw git error.
Behavior is safe (no data loss, no remote clobber — consistent with the never-force-delete-remote
design). Recommend either tolerating a missing-local-branch delete (proceed to recreate) or updating
the comment to match actual behavior. Non-blocking for archive.

### SUGGESTION

**S1 — No dedicated test for `reusing && unauthenticated` PR-lookup degrade.** The
push-pr-preparation "PR lookup unavailable degrades" scenario is satisfied by the `auth==Authenticated`
guard, but the reuse-path unauth degrade is proven only indirectly (`TestPreparePRCmd_NotReusing` +
existing auth-degrade suite). A `reusing=true` + `AuthAbsent/Unauthenticated` case would close the gap.

**S2 — Exact "5 commits, 2 already present, 3 remain" scenario numbers untested.** The filter
property is proven with 3-commit (1 remain) and E2E 2-commit (1 remain) shapes; the literal 5/2/3
numeric scenario is covered by property, not by exact reproduction. Purely cosmetic.

---

## Final Verdict

**PASS WITH WARNINGS.** All 4 delta specs, every requirement, and every scenario are implemented and
covered by tests that passed at runtime under `-race`. The FastForwardBranch never-clobber safety
property is proven behaviorally. Both noted deviations are conformant and non-behavioral. One WARNING
(W1) and two SUGGESTIONs are recorded; none block archive. Recommend proceeding to `sdd-archive`
(optionally addressing W1 first).
