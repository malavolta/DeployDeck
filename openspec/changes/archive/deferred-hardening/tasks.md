# Tasks: Deferred Hardening — L-1 + D1 + D2

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | ~260-320 (3 prod files ~50 lines, 3 test files ~230 lines) |
| 400-line budget risk | Low |
| Chained PRs recommended | No |
| Suggested split | single PR |
| Delivery strategy | ask-on-risk |
| Chain strategy | pending |

Decision needed before apply: No
Chained PRs recommended: No
Chain strategy: pending
400-line budget risk: Low

### Suggested Work Units

| Unit | Goal | Likely PR | Focused test command | Runtime harness | Rollback boundary |
|------|------|-----------|----------------------|-----------------|-------------------|
| 1 | L-1 protected-from-prune skip | single PR | `go test ./internal/runs/... -run Prune` | N/A — pure unit, no runtime harness needed | `internal/runs/retention.go` + its test additions |
| 2 | D1 in-progress entry guard | single PR | `go test ./internal/app/... -run TestKeyMainMenu_BlocksStandalone -short` | N/A — `Model.Update` unit call, no live git needed | `keyMainMenu`'s two new guard blocks in `keys.go` |
| 3 | D2 sanitized identity + mode render | single PR | `go test ./internal/app/... -run 'DeltaSourceSelect|ViewRunHistory' -short` | `go test ./internal/app/... -run TestE2E_StandaloneDelta -short=false` (real `sf sgd`, see design Deviation 2) | `confirmDeltaSourceSelect`'s `Ticket` line + `viewRunHistory`'s render branch |

Each group below builds + vets + tests independently. No org.

## Group 1 — L-1 (`internal/runs`, pure-unit)

- [x] 1.1 RED — `internal/runs/retention_test.go`: add `TestIsProtectedFromPrune` table: `{JobID:"0Af1",Status:"InProgress"}`→true; `{Phase:"cherry-pick"}`→true; `{Phase:"git-conflict"}`→true; `{JobID:"0Af1",Status:"Succeeded"}`→false; `{Mode:"delta"}` (no JobID/Phase)→false. Satisfies `run-retention`: "Non-Terminal Or Resumable Runs Are Never Pruned" (predicate).
- [x] 1.2 RED — same file: add `TestSelectPruneCandidates_ProtectsResumableRuns` table (new func, existing `TestSelectPruneCandidates` untouched): in-flight non-terminal run outside keepLast/keepDays→KEPT; unfinished-cherry-pick run outside window→KEPT; terminal old run outside window→PRUNED (unchanged); recent/count-kept run→KEPT regardless of status. Satisfies all 4 `run-retention` scenarios.
- [x] 1.3 GREEN — `internal/runs/retention.go`: add local `terminalStatuses` map `{Succeeded,SucceededPartial,Failed,Canceled}` (comment: mirrors `salesforce.terminalStatuses`, `report.go:94-99`, source of truth, never imported) + `isProtectedFromPrune(rec Record) bool`: `(rec.JobID != "" && !terminalStatuses[rec.Status]) || rec.Phase=="cherry-pick" || rec.Phase=="git-conflict"` (comment: phase strings mirror `app.isCherryPickPhase`, `update.go:416-417`).
- [x] 1.4 GREEN — same file: insert `if isProtectedFromPrune(rec) { continue }` as the FIRST check inside `selectPruneCandidates`'s loop, before `keptByCount`/`keptByAge`; count/age math byte-identical for all other records.
- [x] 1.5 Verify — `go build ./internal/runs/... && go vet ./internal/runs/... && go test ./internal/runs/...` green; confirm no new import of `internal/salesforce` or `internal/app` in `internal/runs`.

## Group 2 — D1 (`internal/app`, app-level)

- [x] 2.1 RED — `internal/app/main_menu_test.go`: add `TestKeyMainMenu_BlocksStandaloneEntryWhileInProgress`: `m := mainMenuModel(1)` (delta entry), `m.repoState.InProgress = true`; call `m.keyMainMenu(keyPress("enter"))` directly (need `cmd`, not `advance`); assert `State()==StateMainMenu`, `standaloneMode==""`, `notice != ""`, `cmd == nil`. Repeat for `mainMenuModel(2)` (validate entry). Satisfies `standalone-modes`: "Entry blocked with an in-progress cherry-pick".
- [x] 2.2 RED — same test: with `InProgress == false`, cursor 1 → `State()==StateDeltaSourceSelect`, `standaloneMode=="delta"`, `cmd != nil`; cursor 2 → `State()==StatePackageSelect`, `standaloneMode=="validate"` (regression). Satisfies "Entry proceeds normally with no operation in progress".
- [x] 2.3 RED — same test: cursor 0 ("Promocionar ticket") with `InProgress==true` still reaches `StateTicketInput` (Promote entry unaffected, per design).
- [x] 2.4 GREEN — `internal/app/keys.go` `keyMainMenu`: in `case StateDeltaSourceSelect` (`:433`) and `case StatePackageSelect` (`:440`), BEFORE setting `standaloneMode`/state/firing cmd, add `if m.repoState.InProgress { m.notice = "<actionable message>"; return m, nil }`. Zero `m.repoState` (fail-open) falls through unchanged. (Deviation: notice text kept in English, matching the majority of existing `m.notice` strings in this file rather than Spanish.)
- [x] 2.5 Verify — `go build ./internal/app/... && go vet ./internal/app/... && go test ./internal/app/... -short` green.

## Group 3 — D2 (`internal/app`, app-level)

- [x] 3.1 RED — `internal/app/standalone_delta_test.go`: add `TestSanitizeBranch` table: `"release/1.0"`→`"release-1.0"`; `"main"`→`"main"` (no-op).
- [x] 3.2 RED — same file: extend `deltaSourceSelectModel`/add `TestModel_DeltaSourceSelect_TicketIncludesSanitizedBase`: pick branch `origin/release/1.0` → `nm.Plan().Ticket == "standalone-release-1.0"`. Satisfies `standalone-modes`: "Different base branches produce distinct outputs" (identity half).
- [x] 3.3 RED — `internal/app/run_history_test.go`: add `TestRunPackagePath_DistinctPerBase`: `runPackagePath` for `Ticket="standalone-main"` vs `Ticket="standalone-release-1.0"` (same `Target`) → distinct paths. Satisfies "Different base branches produce distinct outputs" (render half). (Note: this test already passed GREEN before task 3.7's view.go change — `runPackagePath` already discriminated on `Ticket`, which the 3.5 identity fix made distinct per-base; kept as a triangulation/regression pin per the task.)
- [x] 3.4 RED — same file: add `TestViewRunHistory_ModeAwareRender`: `Mode:"delta",Ticket:"standalone-main",Target:"main"` → view contains `"[delta] main"`; `Mode:"validate",ManifestPath:".../pkg/package.xml"` (empty Ticket/Target) → view contains `"[validate] package.xml"` and the detail `Package:` line shows `rec.ManifestPath` directly (not the blank `runPackagePath` `-to-` placeholder); `Mode:""` record (existing `TestViewRunHistory_DetailOnSelection` fixture) unaffected. Satisfies "Standalone runs render with a mode-distinct label". (Deviation: initial assertion used a blanket `!strings.Contains(v, "-to-")` over the whole view, which false-failed on the legitimate `Branch:` line — `deploy/standalone-main-to-main` for delta, and the untouched `deploy/-to-` for validate, since task 3.7 scopes only the row loop + detail `Package:` line, never `Branch:`. Narrowed the assertion to the in-scope `Package:` line.)
- [x] 3.5 GREEN — `internal/app/keys.go`: add `sanitizeBranch(base string) string` (`strings.ReplaceAll(base, "/", "-")`); in `confirmDeltaSourceSelect` (`:499-502`) set `Ticket: "standalone-" + sanitizeBranch(base)` (was `"standalone"`); `TargetBranch` unchanged.
- [x] 3.6 GREEN — `internal/app/standalone_delta_test.go`: update the now-stale literal `"standalone"` assertions to `"standalone-main"` in `TestModel_DeltaSourceSelect_CursorPickAndNormalize` (`:86-87`) and `TestModel_DeltaSourceSelect_Confirm_ResetsStalePlan` (`:125-126`) — base is `"main"` in both fixtures, sanitize is a no-op. Line `:253` (`onDeltaDone` fixture) is untouched — unrelated to `confirmDeltaSourceSelect`.
- [x] 3.7 GREEN — `internal/app/view.go` `viewRunHistory`: row loop (`:849-850`) and detail's `Package:` line (`:858`) — `Mode=="delta"` renders `[delta] <Target>`; `Mode=="validate"` renders `[validate] <basename(ManifestPath)>` and detail shows `rec.ManifestPath` (not `runPackagePath(...)`); `Mode==""` keeps existing columns unchanged. `Branch:` line intentionally untouched (out of task scope).
- [x] 3.8 Verify — `go test ./internal/app/... -run 'TestE2E_StandaloneDelta_TempGitAndRealSgd|TestE2E_StandaloneDelta_EmptyDelta_WarningVariant' -short=false` (real `sf sgd`, per go-testing skill's integration-test gate) PASSED unmodified — asserts only `.deploydeck/manifest` prefix + `Mode`/`ManifestPath`/`TargetBranch`, none of which change (design Deviation 2). No fix needed.
- [x] 3.9 Verify — `go build ./internal/app/... && go vet ./internal/app/... && go test ./internal/app/...` green.

## Group 4 — Docs

Skip with rationale: post-epic follow-up (all 19 HUs already closed), no per-HU doc marker convention applies — same policy as prior deferred slices (`docs/HISTORIAS.md` stays as the adversarial-review record; no new HU entry).
