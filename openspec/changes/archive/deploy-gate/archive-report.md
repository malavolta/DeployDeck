# Archive Report — deploy-gate

**Status:** archived · **Delivery:** single-PR (size:exception) · **Mode:** strict TDD · **Milestone:** v1.3.0

## Summary

An opt-in, per-environment governance gate DeployDeck enforces LOCALLY between the typed
`DESPLEGAR` confirmation and the real quick-deploy dispatch. For a gated environment (configured
in `deploydeck.yaml`, keyed by branch/glob like `sandboxes`), the deploy is blocked unless the
run's PR satisfies four independently-toggleable conditions:

1. **Approval** — ≥ `minApprovals` (default 1) APPROVED reviews from the configured approver
   handle list; matching is on the immutable GitHub login (normalized `@`/case), only listed
   users count, and a listed user's later CHANGES_REQUESTED negates their approval.
2. **No unresolved review threads** — read via the project's first `gh api graphql` call.
3. **Validation comment** — after a successful CheckOnly, DeployDeck posts (once, dedup-marker
   idempotent) a PR comment with job id, error counts, and aggregate coverage; this comment is
   both the audit record and the condition's signal.
4. **Provenance signature** — the pr-provenance verify path (reused, not modified).

Everything fails CLOSED: no resolvable PR, any `gh`/GraphQL error, dev-build signature, or
truncated thread page blocks the deploy. New pure `internal/gate` package (evaluator, deps
config+provenance only, own boundary test); new `internal/github` reads/writes; a
`StateDeployGateBlocked` screen listing the unmet conditions with no in-app override. Ungated
environments deploy exactly as before with zero new `gh` calls.

Trust model (stated honestly throughout): this is a COOPERATIVE local control — bypassable by
editing config/binary — enforcing team discipline, not a hard security boundary. The hard
equivalent (branch protection + a required CI check) is the documented future path.

## SDD trail

exploration → propose (paused for user OK of the approach) → spec + design (parallel) →
fresh-context design validation (FAIL → reconciled) → tasks (69) → apply (69/69, strict TDD) →
verify (PASS WITH WARNINGS) + full 4R adversarial review (parallel) → consolidated remediation →
archive. Artifact store: OpenSpec (Engram MCP unavailable).

## Design-validation corrections (pre-apply)

The fresh-context validator caught two CRITICAL spec↔design contradictions before any code:
1. The design had inverted the spec's `require*` default-on toggles to opt-OUT `skip*` (different
   YAML keys than the operator was shown) and invented a non-spec 4th toggle. Reconciled to
   `Require* *bool` (nil = default-on, preserving the spec key names) and `MinApprovals *int`
   (nil→1, explicit <1 rejected); `SkipApprovals` dropped (approval always applies when enabled).
2. `gh` read errors on approvals/comments were unmodeled (could crash vs block). All reads now
   fail-closed uniformly via per-condition `Facts.*Err`. Also documented the honest caveat that
   `requireValidationComment` without `requireSignature` is forgeable (recommend pairing).

## Review & remediation

Five parallel reviews: sdd-verify **PASS WITH WARNINGS** (29/29 scenarios tested-green, 69/69
tasks, boundaries hold, pr.go seam restructure verified non-weakening); readability **3 SUGGESTION**;
risk **1 WARNING + 2 SUGGESTION**; reliability **2 SUGGESTION**; resilience **1 CRITICAL + 1
WARNING**. The CRITICAL is the standout — no green suite would have revealed it:

- **CRITICAL (resilience) — gate-check in-flight double-fire.** On a gated Enter, the check runs
  async but the re-fire guard only watched `quickDeployingRunID` (unset until the check returns),
  so re-typing `DESPLEGAR`+Enter during the gh window launched a SECOND check → TWO real
  `sf project deploy quick` for one run, leaving it unmarked (still eligible). Reopened the
  H-1/H-2 double-fire vector. FIXED: the guard now also covers `gateCheckingRunID`.
- **WARNING (resilience) — no timeout/feedback.** The 3-4 sequential `gh` calls ran under
  `context.Background()` with no indicator (looking frozen — what made the re-type realistic).
  FIXED: a "Verificando gate…" in-flight note + a 30s bounded context (a hung `gh` fails-closed).
- **WARNING (risk/verify) — fail-OPEN on >100 threads.** `reviewThreads(first:100)` truncated
  silently, passing the threads condition. FIXED: query `pageInfo.hasNextPage`; truncation →
  error → fail-closed (a governance gate must never fail-open).
- **SUGGESTIONs:** `BestResult` empty-markers zero-value landmine → returns `Mismatch` explicitly;
  `gh api graphql` owner/repo via `-f` (literal) not `-F`; added the non-listed-CHANGES_REQUESTED
  test; readability (`min` shadow → `required`, shared exported `gate.ToggleOn`, Spanish condition
  labels on the block screen, deterministic validation-error ordering).

## Verification

- `go build ./...`, `go vet ./...`, `gofmt -l internal cmd` clean.
- `go test ./... -race -count=1` green across all 15 packages (re-run by the orchestrator after
  remediation).
- Boundaries: `internal/gate` and `internal/provenance` exec-free (own boundary tests green);
  `internal/app` never imports exec; all `gh` in `internal/github`.

## Spec merge note

Merged ADDITIVELY: NEW `deploy-gate` living spec (220 lines, 8 requirements); `quick-deploy`
124→141 (opt-in-execution requirement now gates on the deploy gate + 2 scenarios);
`validation-progress` 159→178 (new terminal-success comment-trigger requirement). Diffstat
+41/−5; no pre-existing requirement lost; no delta markers leaked.

## Follow-up (accepted, non-blocking)

- Hard-gate enforcement (branch protection + required CI check) — the non-cooperative version,
  the documented future path.
- Marker anti-forgery hardening (sign the marker / verify author) — condition-3 stays cooperative;
  condition-4 is the non-forgeable anchor when `requireSignature` is on.
- `>100` unresolved threads now fails-closed (was the deferral); full GraphQL pagination only if a
  real PR is ever observed to exceed 100 threads.
