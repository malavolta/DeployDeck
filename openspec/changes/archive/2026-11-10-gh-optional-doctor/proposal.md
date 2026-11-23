# Proposal: gh Is Optional — Honest, Fast Doctor and Graceful gh Fallbacks

Delivery phase: Fase 5 Productizacion.

## Intent

The Doctor screen looks frozen for about 33s, then shows a false "gh is installed but not authenticated" warning. The cause is that `github.client.AuthStatus` runs `gh auth status` against every configured host with no timeout, so one unreachable extra host (for example github.ibm.com) both stalls the screen and flips the result. gh is optional, because push uses git. The TUI should say what is degraded without gh, respond quickly, and never block.

## Scope

### In Scope
1. The Doctor gh warning says what happens without gh: PR creation falls back to the compare URL, and the deploy gate and the validation comment are unavailable. It also says push still works. It stays `StatusWarning` and never becomes `StatusBlocking`.
2. `AuthStatus` checks only the repo's host, taken from the `origin` remote (`gh auth status --hostname <host>`), with a timeout of about 5s. A timeout gets its own `AuthState`. It is never reported as "unauthenticated".
3. While checks run, the Doctor screen shows the spinner and "Comprobando prerequisitos…", driven by the existing `spinnerActive()` tick loop.
4. PR screen: when gh is unavailable (absent, unauthenticated or timed out), the screen offers the compare URL directly. It never offers a gh-create option.
5. Deploy gate: when gh is unavailable or times out, the block message names that cause. It must not suggest that approvals are missing.

### Out of Scope
- Making gh required. Changing push.
- Supporting more than one host.
- Changing gate pass/fail semantics. The gate still fails closed.

## Capabilities

### New Capabilities
None.

### Modified Capabilities
- `prereq-check`: host-scoped gh check with a timeout, a separate timeout result, warning text that lists consequences, and a loading indicator.
- `push-pr-preparation`: the timed-out state falls back to the compare URL, with no gh-create option.
- `deploy-gate`: a block caused by gh being unavailable names gh, not approvals.

## Approach

- `internal/github`: add a pure `OriginHost(originURL)` helper (reuses `parseOrigin`). `AuthStatus(ctx, host)` passes `--hostname` when the host is known. Add an `AuthTimedOut` state for when the context deadline is exceeded.
- `internal/prereq/checker_gh.go`: get the host from `Git.RemoteURL(GitRoot, "origin")`, wrap the call in a ~5s `context.WithTimeout`, and map each state to clear warning text.
- `internal/app`: add `StatePrereqCheck && len(m.checks)==0` to `spinnerActive()` and render the loading line in `viewPrereq`. `preparePRCmd` uses the same host and timeout. `viewPRData` treats any non-authenticated state as the compare-URL path. `gateCheckCmd` checks auth first; when gh is unavailable it sets a gate fact so that `gate.Evaluate` reports a single condition naming gh as the cause.
- Boundary rules hold: no `os/exec` or `net/http` in `internal/app`. The footer does not change (the loading state reuses the existing bindings).

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `internal/github/client.go`, `compare_url.go` | Modified | host arg, `AuthTimedOut`, `OriginHost` |
| `internal/prereq/checker_gh.go` | Modified | host, timeout, text |
| `internal/app/update.go`, `view.go`, `commands.go` | Modified | spinner, loading line, PR fallback, gate cause |
| `internal/gate/gate.go` | Modified | gh-unavailable condition |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| No origin, or an unrecognized origin form | Med | Fall back to the current check across all hosts, still with the timeout |
| On a slow network, a 5s timeout reports "timed out" for a valid login | Low | The result says "could not verify", not "unauthenticated" |
| Size goes over the budget | Med | See the forecast in the result |

## Rollback Plan

Revert the single PR commit range. No shared Git state, sandbox, or persisted data changes.

## Dependencies

- gh ≥ 2.x supports `auth status --hostname`.

## Success Criteria

- [ ] The Doctor screen shows the spinner right away, and the gh check finishes within about 5s even if an extra host hangs.
- [ ] When github.com is authenticated, the Doctor shows gh as OK, whatever the state of other hosts.
- [ ] A timeout shows a separate warning and never shows "not authenticated".
- [ ] The PR screen and the gate block message name gh as the cause when gh is unavailable.

## Proposal question round (needs user review)

1. Gate: when gh is unavailable, should the gate block with "gh no disponible" only, or also offer an override? Assumption: block only, no override.
2. Should a timeout during PR preparation fall back silently to the compare URL? Assumption: yes, with a one-line note.
3. Should the gh warning text stay in English (as today) or move to Spanish? Assumption: keep English and change only the wording.
