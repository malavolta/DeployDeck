# Tasks: gh Is Optional — Honest, Fast Doctor and Graceful gh Fallbacks

Strict TDD: every GREEN task is preceded by its RED test task (test written and seen failing first).
Budget: ~550 changed lines total (tests + code). Tests stay focused: one table test per behavior, no redundant triangulation.

## Accepted spec/design mismatch

Spec "Doctor Screen Loading Indicator" is satisfied only for the first load. On retry (`r`) the design KEEPS the
previous check results and shows NO loading line (loading line is gated on `len(m.checks)==0`), so the footer
(Continue visibility) does not change. This deviation is ACCEPTED; do not add a retry loading state or clear `m.checks`.

## Language rules

- Gate condition label stays Spanish: `gh-availability` -> "disponibilidad de gh" (consistent with `conditionLabelES`).
- Doctor warning `Detail` is English. Loading text is Spanish per spec: "Comprobando prerequisitos…".
- Gate cause strings (from app) stay Spanish, e.g. "gh no disponible", "gh no autenticado", "no se pudo verificar gh".

## Phase 1: github client seam (~90 lines)

- [x] 1.1 RED: `internal/github/client_test.go` (and `compare_url_test.go`) — table tests: `AuthStatus` args with host (`auth status --hostname=<h>`) and without (`auth status`); host `-x`/whitespace rejected (host-less probe); already-expired ctx (`context.WithDeadline(past)`) -> `AuthTimedOut` not `AuthUnauthenticated`; `OriginHost` for SSH, HTTPS, `ssh://`, Enterprise host, unrecognized form (ok=false), `https://--help/o/r` (ok=false). (~50)
- [x] 1.2 GREEN: `internal/github/client.go` — append `AuthTimedOut`; `AuthStatus(ctx, host)` adds single token `--hostname=<h>`; classify `ctx.Err()!=nil || errors.Is(err, DeadlineExceeded)` as TimedOut. `internal/github/compare_url.go` — `OriginHost` over `parseOrigin` + flag-safety check. Update existing callers' signatures to compile (pass `""` temporarily where later phases replace it). (~40)

## Phase 2: prereq CheckGH (~110 lines)

Depends on Phase 1.

- [x] 2.1 RED: `internal/prereq/checker_gh_test.go` — origin-scoped args (`git@github.com:org/repo.git` -> `--hostname=github.com`); no origin / `https://--help/o/r` -> host-less probe; timeout -> `StatusWarning`, Detail has "could not verify" and not "not authenticated"; absent/unauth/timed-out Detail mentions compare URL fallback, deploy gate + validation comment unavailable, push still works; never `StatusBlocking`; ctx-recording runner asserts deadline <= now+`GHTimeout`. (~65)
- [x] 2.2 GREEN: `internal/prereq/checker.go` — `GHTimeout time.Duration` (zero = 5s). `internal/prereq/checker_gh.go` — nil-safe `Git` origin lookup via `OriginHost`, `context.WithTimeout`, 4-state English consequence text. (~45)

## Phase 3: gate pure fact (~60 lines)

Independent of Phases 1-2 (parallel with them).

- [x] 3.1 RED: `internal/gate/gate_test.go` — `Facts.GHUnavailable != ""` yields exactly one failed condition `gh-availability` and no approvals/threads/comment/signature conditions; empty value leaves existing output unchanged. (~30)
- [x] 3.2 GREEN: `internal/gate/gate.go` — `Facts.GHUnavailable string`; short-circuit in `Evaluate` before `PRResolved`. (~30)

## Phase 4: app commands — PR timeout + gate auth-first (~140 lines)

Depends on Phases 1 and 3.

- [x] 4.1 RED: `internal/app/commands_test.go` — runner stub returning `context.DeadlineExceeded`: `preparePRCmd` yields compare URL result flagged as timed-out; nil gh client -> Absent; `gateCheckCmd` with absent/unauth/timed-out sets `Facts.GHUnavailable` (distinct Spanish cause per state) and does not call `resolvePRURL`; authenticated leaves facts unchanged. (~70)
- [x] 4.2 GREEN: `internal/app/commands.go` — `ghAuthTimeout` const (5s); `ghAuth(parent, gh, originURL) github.AuthState` (nil client -> Absent, host via `OriginHost`, timeout); `preparePRCmd` reads origin first then uses `ghAuth`; `gateCheckCmd` reads origin via nil-safe `m.deps.Git`, calls `ghAuth` before `resolvePRURL`, maps non-Authenticated to `Facts.GHUnavailable`. (~70)

## Phase 5: app views (PR note + gate label) (~70 lines)

Depends on Phase 4.

- [x] 5.1 RED: `internal/app/view_test.go` — PR screen with timed-out prep shows a one-line "could not verify gh" note, compare URL offered, no `gh pr create` option in view/footer, no "not authenticated" wording; gate block screen for each of the 3 causes shows one condition labelled "disponibilidad de gh" naming gh, with no approvals-missing text and no override action. Existing footer parity/pin tests unchanged. (~40)
- [x] 5.2 GREEN: `internal/app/view.go` — timeout note in `viewPRData`; `"gh-availability": "disponibilidad de gh"` in `conditionLabelES`; render cause detail on the gate block screen. (~30)

## Phase 6: Doctor loading indicator (~80 lines)

Independent of Phases 1-5 (parallel-safe; touches different app files from Phase 4/5 except `view.go` — sequence after 5.2 to avoid conflicts).

- [x] 6.1 RED: `internal/app/update_test.go` / `app_test.go` / `view_test.go` — `spinnerActive` true in `StatePrereqCheck` with `len(m.checks)==0` and false once results exist (tick loop stops); `viewPrereq` shows spinner + "Comprobando prerequisitos…" only while empty; footer identical to existing Doctor footer; `Init()` returns non-nil cmd only when `NewChecker != nil`; no double tick loop. (~45)
- [x] 6.2 GREEN: `internal/app/app.go` — `New` sets `spinning=true` and `Init` batches `spinnerCmd()` when `NewChecker != nil`; `internal/app/update.go` — `spinnerActive`: `StatePrereqCheck && len(m.checks)==0`; `internal/app/view.go` — loading line in `viewPrereq`. (~35)

## Phase 7: Verification (~0 new lines)

- [x] 7.1 Run `go build ./... && go vet ./... && go test ./...` (mind the Go PATH quirk in memory); confirm existing footer parity, pin and `internal/prereq` tests stay green.
- [x] 7.2 Check changed-line count against the ~550 budget (`git diff --shortstat`); stop and report if > ~600.

## Line estimate summary

| Phase | Tests | Code | Total |
|---|---|---|---|
| 1 github seam | 50 | 40 | ~90 |
| 2 prereq | 65 | 45 | ~110 |
| 3 gate | 30 | 30 | ~60 |
| 4 app commands | 70 | 70 | ~140 |
| 5 app views | 40 | 30 | ~70 |
| 6 Doctor loading | 45 | 35 | ~80 |
| **Total** | **300** | **250** | **~550** |

Memory note: agent line forecasts historically undercount ~1.6x; if apply tracks over ~550, raise the size exception before continuing.

## Parallelism

- Sequential chain: 1 -> 2; 1 + 3 -> 4 -> 5 -> 6 (6 shares `view.go` with 5).
- Parallel-safe: Phase 3 with Phases 1-2. Phase 6's logic is independent but is ordered last to avoid `view.go` conflicts under a single writer.
