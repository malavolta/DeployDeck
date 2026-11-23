# Design: gh Is Optional — Honest, Fast Doctor and Graceful gh Fallbacks

## Technical Approach

Change the single gh seam (`github.Client.AuthStatus`) to accept a host and to classify a cancelled/expired context as a new `AuthTimedOut` state. Every caller (prereq `CheckGH`, app `preparePRCmd`, app `gateCheckCmd`) derives the host from `origin` with the pure `github.OriginHost` and bounds the call with a 5s `context.WithTimeout`. The PR screen already routes every non-authenticated state to the compare URL (`view.go:985/1028`, `keymap.go:291`, `keys.go:454`); it only gains a timeout note. The gate gets one new pure fact that short-circuits `Evaluate` with a single gh condition. The Doctor loading line reuses the spinner tick loop.

## Architecture Decisions

| Decision | Options | Tradeoff | Choice |
|---|---|---|---|
| Host plumbing | (a) `AuthStatus(ctx, host)` (b) client reads origin itself | (b) couples github to git | **(a)**. There is one implementation and three callers, and no hand-written fakes exist. |
| Timeout classification | (a) `ctx.Err()!=nil \|\| errors.Is(err, DeadlineExceeded)` on Runner error (b) a timer in the client | (b) duplicates the OSRunner kill logic | **(a)**. OSRunner already returns the error with `ctx.Err()` set (`os_runner.go:68`). |
| Who owns the timeout | (a) each caller wraps ctx (b) a client field | (b) hides the deadline from tests | **(a)**. `prereq.Checker.GHTimeout` (zero means 5s) and an app `ghAuthTimeout` const. Tests drive an already-expired ctx and need no sleeps. |
| `--hostname` arg shape | `--hostname <h>` vs `--hostname=<h>` | A separate token lets a host such as `--help` be parsed as a flag | **`--hostname=<h>`** single token. `OriginHost` rejects a host that starts with `-` or contains whitespace, and the check then falls back to the host-less probe. |
| Gate cause | (a) `Facts.GHUnavailable string` (cause), checked before `PRResolved` (b) a per-condition Err | (b) makes the screen say "approvals failed" | **(a)**. gate stays pure (no github import). App maps AuthState to a Spanish cause. The new condition `gh-availability` gets the label "disponibilidad de gh". |
| Spinner seeding | (a) `New` sets `spinning=true` and `Init` batches `spinnerCmd()` when `NewChecker!=nil` (b) Init seeds without the flag | (b) risks a double tick loop if a spinner state is entered before the loop dies | **(a)**. When `NewChecker` is nil (unit tests), behavior is unchanged. |
| Retry (`r`) loading | clear `m.checks` vs keep | Clearing changes the footer (Continue visibility) | **Keep**. The loading line shows only on the first load (`len(m.checks)==0`), so the footer does not change. |

## Data Flow

    origin URL ──git.RemoteURL──> github.OriginHost ──host──┐
    parent ctx ──WithTimeout(5s)──────────────────────────────┤
                                                           v
                     gh.AuthStatus(ctx, host) -> Absent|Unauth|Auth|TimedOut
        CheckGH: consequence text (Warning/OK)
        preparePRCmd: auth -> prepDoneMsg (compare URL + timeout note)
        gateCheckCmd: auth != Auth -> Facts.GHUnavailable -> Evaluate -> 1 condition

The app helper `ghAuth(parent, gh, originURL) github.AuthState` covers the nil client (returns Absent), the host and the timeout. Both app commands use it. `gateCheckCmd` reads origin via `m.deps.Git` (nil-safe) and calls `ghAuth` before `resolvePRURL`.

## File Changes

| File | Action | Description |
|---|---|---|
| `internal/github/client.go` | Modify | `AuthTimedOut` const; `AuthStatus(ctx, host)` adds `--hostname=`; timeout classification |
| `internal/github/compare_url.go` | Modify | `OriginHost(originURL) (string, bool)` over `parseOrigin` plus a flag-safety check |
| `internal/prereq/checker.go` | Modify | `GHTimeout time.Duration` field |
| `internal/prereq/checker_gh.go` | Modify | origin host (nil-safe `Git`), timeout, 4-state English consequence text |
| `internal/gate/gate.go` | Modify | `Facts.GHUnavailable`; short-circuit in `Evaluate` |
| `internal/app/commands.go` | Modify | `ghAuthTimeout`, `ghAuth` helper; `preparePRCmd` reads origin first; `gateCheckCmd` auth-first |
| `internal/app/app.go` | Modify | `New` arms `spinning`; `Init` seeds `spinnerCmd` |
| `internal/app/update.go` | Modify | `spinnerActive`: `StatePrereqCheck && len(m.checks)==0` |
| `internal/app/view.go` | Modify | loading line in `viewPrereq`; timeout note in `viewPRData`; label in `conditionLabelES` |

## Interfaces / Contracts

```go
const ( AuthAbsent AuthState = iota; AuthUnauthenticated; AuthAuthenticated; AuthTimedOut )
AuthStatus(ctx context.Context, host string) AuthState // host=="" -> `gh auth status`
func OriginHost(originURL string) (host string, ok bool)
type Facts struct { GHUnavailable string; /* existing fields */ } // non-empty => single failed "gh-availability"
```

The `AuthTimedOut` value is appended, so the existing ordinals do not change.

## Testing Strategy (Strict TDD, RED first)

| Layer | What | Approach |
|---|---|---|
| github unit | args with and without host; `-x` host rejected; expired ctx gives TimedOut, not Unauth; `OriginHost` for SSH/HTTPS/ssh:// and unrecognized forms | FakeRunner plus `context.WithDeadline(past)` |
| prereq unit | origin-scoped args; no origin falls back to host-less; timeout Warning text without "not authenticated"; consequence text in all 3 non-auth states; never Blocking; deadline ≤ now+GHTimeout | FakeRunner; a ctx-recording runner stub; existing tests stay green because an unmatched git call gives host "" |
| gate unit | GHUnavailable gives exactly 1 failed condition and no approvals text; empty value leaves output unchanged | pure table tests |
| app unit | `spinnerActive` while loading; loading line; Init cmd non-nil only with `NewChecker`; PR timeout note and no gh-create footer; gate block screen names gh for each cause | Runner stub returning `context.DeadlineExceeded`; existing footer parity and pin tests unchanged |

## Threat Matrix

| Boundary | Applicability | Design response | Planned RED tests |
|---|---|---|---|
| Documentation-like paths | N/A: no file classification | — | — |
| Git repository selection | N/A: reuses the existing `RemoteURL(GitRoot,"origin")`, no new selector | — | — |
| Commit state | N/A: no commits | — | — |
| Push state | N/A: push unchanged | — | — |
| PR commands (gh arg composition) | Applicable: the host comes from the user-controlled origin | Discrete `--hostname=<h>` token, no shell; hosts starting with `-` or containing whitespace are rejected and the check falls back to the host-less probe | `AuthStatus` args for a normal host; `OriginHost("https://--help/o/r")` returns ok=false; CheckGH with that origin runs a host-less probe |

## Migration / Rollout

No migration required.

## Open Questions

None blocking.
