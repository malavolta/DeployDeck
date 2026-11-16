# Verification Report — gh-optional-doctor

**Verdict**: PASS WITH WARNINGS (0 CRITICAL, 2 WARNING, 4 SUGGESTION)
**Mode**: openspec, Strict TDD (go test)
**Size**: 20 files, +497/−28 = 525 changed lines outside openspec (budget ~550)

## Evidence

- `go build ./...`, `go vet ./...`: clean
- `go test -count=1 ./...`: 15/15 packages ok
- `gofmt -l internal cmd`: empty
- Coverage of changed functions: AuthStatus, OriginHost, safeHost, CheckGH, gate.Evaluate, ghAuth, ghUnavailableCause, spinnerActive, New, Init at 100%; all others >= 84%.

## Real-world check (agenforce-demo, gh 2.96.0, origin github.com)

| Probe | Exit | Duration |
|---|---|---|
| `gh auth status --hostname=github.com` (new) | 0 | 0.69s |
| `gh auth status` (old, unscoped) | 1 | 30.67s (github.ibm.com timeout) |

Host scoping removes both the stall and the false "unauthenticated" warning.

## Spec compliance

7 requirements, 28 scenarios across prereq-check, push-pr-preparation and deploy-gate: 28 COMPLIANT, 0 FAILING, 0 UNTESTED.

## Focus areas

- Timeout maps to `AuthTimedOut`, never unauthenticated (`internal/github/client.go:136-139`).
- `OriginHost`/`safeHost` reject empty, leading `-` and whitespace hosts; fallback is a host-less timed probe.
- Gate fails closed with one gh condition, no further gh calls (`internal/gate/gate.go:131-136`, `internal/app/commands.go:1687-1696`).
- Footer parity unchanged; spinner active only on first prereq load; internal/app boundary holds.

## Issues

**WARNING**
- W1: apply-report.md lacks a per-task TDD cycle evidence table (downgraded from CRITICAL: all behavior verified at runtime).
- W2: `internal/app/view.go:186-188` shows the loading line based only on `len(m.checks)==0`; with `NewChecker==nil` (non-production wiring) it would be static. Not reachable in production.

**SUGGESTION**
- S1: CheckGH test where the host-less probe stalls but the scoped probe succeeds.
- S2: Assert the PR footer for `AuthTimedOut` has no create binding.
- S3: Cover `reusing=true` with timed-out gh.
- S4: Comment that the origin lookup shares the gh timeout budget.

## Next

sdd-archive.
