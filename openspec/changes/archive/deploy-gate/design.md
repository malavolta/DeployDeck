# Design: Deploy Gate (per-environment governance check before quick-deploy)

## Technical Approach

Additive and default-off. Config grows a `Gates map[string]GateConfig` resolved by `GateFor`
(mirrors `Sandboxes`/`SandboxFor`, D1). A new PURE `internal/gate` package holds the four-condition
evaluator, approver normalization, coverage aggregation, and the validation-comment composer — no
I/O, unit-testable in isolation. All gh reads/writes land as thin single-call `internal/github.Client`
methods (exec boundary unchanged). `internal/app` orchestrates: an async `gateCheckCmd` inserted
between the typed-`DESPLEGAR` word-check (`keys.go:1464-1467`) and dispatch (`keys.go:1477`, D2), a
`StateDeployGateBlocked` screen, and a terminal-success validation-comment post in `onReportDone`.
Provenance is reused (D11): the ranking helper is extracted to `internal/provenance.BestResult` so
`pr verify` and condition-4 share ONE implementation. Maps proposal D1–D11. Milestone v1.3.0.

## Architecture Decisions

| Decision | Choice | Rejected | Rationale |
|---|---|---|---|
| D1 config keying | `Gates map[string]GateConfig`; `GateFor(target)(GateConfig,bool)` exact→`path.Match`, `bool` true only when matched AND `Enabled` | key by env name (`Branches`) | branch/glob mirrors `SandboxFor`; unmatched/disabled both mean "ungated, deploy as today" |
| Default-on toggles | `Require{ResolvedThreads,ValidationComment,Signature} *bool` — nil (omitted) → ON when `Enabled`; explicit `false` → OFF. Approval is NOT a toggle (always on when Enabled) | plain `Require* bool` (zero=false=OFF, wrong default); opt-OUT `Skip*` (contradicts the spec's `require*` YAML the operator writes, and mis-stems/adds a non-spec 4th toggle); Load defaulting | `*bool` nil ≠ false: the pointer's nil zero-value IS "default-on" AND preserves the spec-mandated `require*` key names — the standard Go idiom the earlier revision dismissed with a rationale that only applied to a plain `bool` |
| MinApprovals default | `MinApprovals *int`: nil/omitted → `effectiveMin = 1`; `Validate` rejects an enabled gate whose explicit `MinApprovals < 1`, and rejects an enabled gate with empty `Approvers` (UNCONDITIONALLY — approval always applies when Enabled) | plain `int` (can't distinguish omitted 0 from explicit 0, so can't honor the spec's "reject <1") | `*int` nil = omitted → default 1; an explicit `<1` is a real misconfig the spec says to reject; an empty approver list can never pass |
| Gate package | new PURE `internal/gate`; owns `Facts→Result`, `normalizeLogin`, `AggregateCoverage`, `ValidationComment`, `HasValidationComment`; deps only `config`+`provenance` | put in `internal/app` | screaming-arch capability isolation; pure = exhaustively unit-testable without gh; app maps gh DTOs → gate value types at the boundary |
| Provenance reuse | extract `bestVerifyResult`/`verifyResultRank` → `internal/provenance.BestResult(ownerRepo,head,markers)(Result,runID)`; pr.go keeps a `var bestResult = provenance.BestResult` test seam | duplicate ranking in gate; keep in `package main` (unimportable) | one impl for pr-verify + condition-4 (D11); seam preserves pr_verify_test exit-code mapping under an empty test secret |
| Comment upsert | thin `PostComment` + **app-layer skip-if-present**: list `PRComments`, post only when no body carries the marker prefix | `gh pr comment --edit-last --create-if-none` (version-fragile); REST PATCH (`databaseId` not cleanly exposed by `--json comments`) | never double-posts (D6) with zero dependence on newer-gh flags; first comment stands as the audit record |
| Condition-3 trust | marker-**presence** only (cooperative) | verify comment author identity / sign the marker | whole gate is cooperative (proposal trust model); condition-4's provenance signature is the non-forgeable anchor; author-check buys little (DeployDeck posts as the invoking user) |
| Signature in dev build | condition-4 passes ONLY on `provenance.Verified`; `DevMarker`/`DevVerifier` FAIL-closed with an explanatory detail | treat Dev* as pass | a governance gate must not green-light an unverifiable signature; honest with provenance's dev semantics |
| Threads | `UnresolvedThreadCount` via `gh api graphql` (`reviewThreads(first:100){isResolved}`); error/auth/GraphQL-errors → FAIL-closed (block) | REST (no thread-resolution field); treat-unknown-as-pass | D5; unknown is unsafe for a gate |
| PR resolution | `resolvePRURL`: `rec.PRUrl` → `PRForBranch(RenderBranchName(BranchFormat,Ticket,Target))`; empty → FAIL-closed | trust `rec.PRUrl` only | D4; `PRUrl` frequently unset (standalone/cross-session) |

## Data Flow

    keyQuickDeploy "enter" (DESPLEGAR ok)
      │  GateFor(rec.Target).Enabled?
      ├─ no ─────────────────────────────────────────────► quickDeployCmd (unchanged)
      └─ yes ► gateCheckCmd (async, gh) ─────────► internal/github.Client
                 resolvePRURL(rec)  [rec.PRUrl | PRForBranch(RenderBranchName)]  fail-closed
                 PRReviews(url)            (gh pr view --json latestReviews)
                 UnresolvedThreadCount(url)(gh api graphql reviewThreads.isResolved)  fail-closed
                 PRComments(url)           (gh pr view --json comments)
                 PRDetails+ParseMarkers+provenance.BestResult → Result
                          │  map DTOs → gate.Facts; ANY gh read error (reviews/threads/comments/PRDetails)
                          │  is captured into the matching Facts.*Err → that condition FAILS-closed (never a crash)
      onGateCheckDone ◄── gate.Evaluate(Facts) → Result{Passed,[]Condition}
        ├─ Passed ──────────────────────────────────────► quickDeployCmd (point-of-no-return dispatch)
        └─ blocked ► m.gateConditions; state = StateDeployGateBlocked (view lists unmet + reasons; no override)

    onReportDone (terminal SUCCESS) & GateFor(rec.Target).Enabled && requireValidationComment(on)
      └─ postValidationCommentCmd (async, best-effort) ► resolvePRURL → PRComments →
           if !HasValidationComment: ValidationComment(job,run,report scalars) → PostComment

## File Changes

| File | Action | Description |
|---|---|---|
| `internal/config/config.go` | Modify | add `GateConfig` struct + `Config.Gates map[string]GateConfig` (mirror `QuickDeployConfig`/`Sandboxes`) |
| `internal/config/validate.go` | Modify | per enabled gate: reject an explicit `MinApprovals < 1`; require non-empty `Approvers` (unconditional when `Enabled`) |
| `internal/config/gate_for.go` | Create | `GateFor(target)(GateConfig,bool)` exact→`path.Match` (mirror `sandbox_for.go`) |
| `internal/gate/gate.go` | Create | PURE `Facts`/`Result`/`Condition`/`Evaluate`; `normalizeLogin`; `AggregateCoverage`; `ValidationComment`; `HasValidationComment`; `MarkerPrefix` |
| `internal/github/client.go` | Modify | interface + impls: `PRReviews`, `UnresolvedThreadCount`, `PRComments`, `PostComment`; `Review`/`Comment` structs |
| `internal/github/compare_url.go` | Modify | add `ParsePRURLParts(url)(host,owner,repo string,number int,ok bool)`; `ParsePRURL` delegates (callers unchanged) |
| `internal/provenance/provenance.go` | Modify | add `BestResult` (+`resultRank`), moved from pr.go |
| `cmd/deploydeck/pr.go` | Modify | replace `verifyFn`/`bestVerifyResult`/`verifyResultRank` with `var bestResult = provenance.BestResult` seam |
| `internal/app/app.go` | Modify | `StateDeployGateBlocked`; Model fields `gateCheckingRunID string`, `gateConditions []gate.Condition` (GH dep already present) |
| `internal/app/keys.go` | Modify | branch to `gateCheckCmd` between word-check and dispatch; `keyDeployGateBlocked` (q/esc → StateRunHistory) |
| `internal/app/commands.go` | Modify | `gateCheckCmd`, `postValidationCommentCmd`, `resolvePRURL`, `verifyProvenance` helpers |
| `internal/app/update.go` | Modify | `onGateCheckDone` (pass→dispatch / block→screen); terminal-success comment post; `onPostCommentDone` (best-effort) |
| `internal/app/view.go` | Modify | `viewDeployGateBlocked` — lists unmet conditions + per-condition reason (style.go palette) |

## Interfaces / Contracts

```go
// internal/config
type GateConfig struct {
    Enabled      bool     `yaml:"enabled"`
    Approvers    []string `yaml:"approvers"`
    MinApprovals *int     `yaml:"minApprovals"` // nil/omitted → 1; explicit <1 rejected by Validate
    // default-ON toggles: *bool nil (omitted) → condition ON when Enabled; explicit false → OFF.
    // Approval is NOT toggleable — it always applies when Enabled (minApprovals + approvers).
    RequireResolvedThreads   *bool `yaml:"requireResolvedThreads"`
    RequireValidationComment *bool `yaml:"requireValidationComment"`
    RequireSignature         *bool `yaml:"requireSignature"`
}
func (c Config) GateFor(target string) (GateConfig, bool) // (cfg,true) iff matched AND Enabled

// internal/github  (each = ONE gh call, discrete args, never shell-joined)
type Review  struct { Login, State string }  // latest per author: APPROVED|CHANGES_REQUESTED|...
type Comment struct { Login, Body  string }
func (c *client) PRReviews(ctx, url string) ([]Review, error)          // gh pr view <url> --json latestReviews
func (c *client) UnresolvedThreadCount(ctx, url string) (int, error)   // gh api graphql; err → fail-closed
func (c *client) PRComments(ctx, url string) ([]Comment, error)        // gh pr view <url> --json comments
func (c *client) PostComment(ctx, url, body string) (raw string, err error) // gh pr comment <url> --body <body>
func ParsePRURLParts(prURL string) (host, owner, repo string, number int, ok bool)

// internal/provenance
func BestResult(ownerRepo, headBranch string, markers []Marker) (Result, string)

// internal/gate  (PURE; deps: config, provenance)
const MarkerPrefix = "<!-- deploydeck-validation:"
type ApproverReview struct { Login, State string }
type Facts struct {
    Config          config.GateConfig
    PRResolved      bool
    Reviews         []ApproverReview
    ReviewsErr      error            // gh failure reading approvals → approvals condition fails (fail-closed)
    UnresolvedCount int
    ThreadErr       error            // graphql/auth failure → threads condition fails (fail-closed)
    CommentPresent  bool
    CommentErr      error            // gh failure reading comments → validation-comment condition fails (fail-closed)
    Provenance      provenance.Result
    ProvenanceErr   error            // gh/parse failure fetching PR body → signature condition fails (fail-closed)
}
type Condition struct { Name string; Passed bool; Detail string } // names: pr-resolution|approvals|threads|validation-comment|signature
type Result struct { Passed bool; Conditions []Condition }
func Evaluate(f Facts) Result                       // !PRResolved → single failed pr-resolution; any Facts.*Err fails ITS condition (fail-closed)
func HasValidationComment(bodies []string) bool     // any body contains MarkerPrefix
func ValidationComment(jobID, runID string, componentErrs, testErrs, coveragePct int, coverageKnown bool) (marker, body string)
func AggregateCoverage(totals, notCovered []int) (pct int, known bool) // Σ(total-notCovered)/Σtotal; 0 total → known=false
```

Approver match: `normalizeLogin(s)=ToLower(TrimPrefix(TrimSpace(s),"@"))`; condition-1 passes iff
`count(approvers whose latest state==APPROVED) >= effectiveMin` AND no listed approver's latest
state is `CHANGES_REQUESTED` (changes-requested negates). Marker:
`<!-- deploydeck-validation: job:<jobId> run:<runId> -->`.

## Testing Strategy (strict TDD — RED first)

| Layer | What | Where |
|---|---|---|
| Unit config | parse `Gates` (`*bool`/`*int` nil vs explicit); `GateFor` exact/glob/no-match/disabled→false; `Validate` rejects an enabled gate with explicit `minApprovals<1` or empty `approvers`; `require*` nil→on, explicit-false→off | `config/*_test.go` |
| Unit github (FakeRunner) | `PRReviews` maps latestReviews json; `UnresolvedThreadCount` counts `isResolved:false`, graphql-error→err; `PRComments` maps bodies; `PostComment` arg slice + raw; `ParsePRURLParts` number+parts, delegated `ParsePRURL` unchanged | `github/*_test.go` |
| Unit gate (pure) | approvals: count/normalize(`@`,case)/`effectiveMin` default-1/changes-requested-negates, `ReviewsErr`→fail-closed; threads `>0` blocks, `ThreadErr`→fail-closed; comment present/absent, `CommentErr`→fail-closed; signature Verified-pass, Mismatch/Dev*-fail, `ProvenanceErr`→fail-closed; `!PRResolved`→fail-closed; `AggregateCoverage`(Σ, 0-total); `ValidationComment` marker+body; `HasValidationComment` | `gate/gate_test.go` |
| Unit provenance | `BestResult` ranking any-of (real test secret) | `provenance_test.go` |
| Churn | pr.go `bestResult` seam — `TestRunPRVerify_AnyOfClassification` (per-marker `verifyFn` sig-discriminating fake) is RESTRUCTURED, not renamed: the sig-based any-of ranking proof moves to `provenance_test.go` (real secret); `pr_verify_test` keeps only exit-code mapping via the coarse `bestResult` seam | `pr_verify_test.go`, `provenance_test.go` |
| App (integration) | gated env: block dispatches NO quick deploy + `StateDeployGateBlocked` lists unmet reasons; PASS falls through to quickDeployCmd; ungated deploys as today; gate-block screen under Ascii TestMain; a gh read error mid-check blocks (never crashes) | `app/deploy_gate_test.go` |
| App | `resolvePRURL`: `rec.PRUrl` used when present; empty → `PRForBranch(RenderBranchName)`; neither resolves → fail-closed block | `app/deploy_gate_test.go` |
| App | terminal-success on gate-enabled env upserts comment (absent→post, present→skip, dedup never double-posts); ungated env posts none | `app/deploy_gate_test.go` |

## Threat Matrix

| Boundary | Applicability | Design response | RED test |
|---|---|---|---|
| PR commands / arg composition | Applicable — `gh pr view`/`gh pr comment`/`gh api graphql` | discrete `[]string` args, url/branch/body/owner/repo/number as separate elements, NEVER shell-joined or interpolated | assert exact arg slices per method |
| Outward comment write | Applicable — first write besides `CreatePR` | body = report scalars + non-secret marker (NO provenance secret embedded); idempotent skip-if-present | body carries no secret; re-post is a no-op |
| GraphQL / auth degrade | Applicable — first `gh api graphql` | error / non-zero / GraphQL `errors` / malformed → threads condition FAILS (fail-closed, block) | graphql-error → gate blocks |
| Marker forgeability (condition-3) | Applicable — presence-based signal | ACCEPTED cooperative: any commenter can echo the marker; condition-4's provenance signature is the non-forgeable anchor. CAVEAT: with `requireValidationComment:on` + `requireSignature:off` (a config the independent toggles allow), condition-3 has NO backstop — purely cooperative. Config docs recommend pairing the two; NOT hard-coupled, to preserve per-condition flexibility | non-DeployDeck comment carrying the marker satisfies condition-3 (asserted behavior); with signature on, a forged provenance still blocks via condition-4; with signature off, the caveat above applies |
| Git repo selection | N/A | every gh call is url-/arg-addressed (`<url>`, explicit owner/repo/number); no `-C`, no cwd authority | — |
| Commit / push state | N/A | change reads PR state and posts a comment; no staging, commit, or push | — |
| Documentation-like paths | N/A | no file classification/execution | — |

## Migration / Rollout

No migration. `Gates` is zero-value-safe and default-off: an env with no gate entry (or `enabled:false`)
behaves exactly as today; existing `deploydeck.yaml`/`run.json` load unchanged (NO SchemaVersion bump).
Revert is localized — remove the `gateCheckCmd` branch (`keys.go`), the `onReportDone` comment post, the
new `github.Client` methods, and `internal/gate`; the `internal/salesforce` exec surface is untouched
throughout. Single PR, strict TDD, `size:exception` pre-approved (proposal Delivery Note).

## Open Questions

- [x] **Comment editing mechanism** → RESOLVED: thin `PostComment` + app-layer skip-if-present.
  `--edit-last --create-if-none` (gh-version-fragile) and REST PATCH (`databaseId` not cleanly exposed
  by `--json comments`) rejected. Re-validation does NOT rewrite the first comment (audit record stands);
  content-refresh on re-validation deferred.
- [x] **Condition-3 anti-forgery** → RESOLVED: stays cooperative (marker-presence only); condition-4's
  provenance signature is the non-forgeable anchor. Documented caveat: `requireValidationComment` WITHOUT
  `requireSignature` has no backstop (config docs recommend pairing them; not hard-coupled). Deferred
  hardening: sign the marker / verify author.
- [x] **Gate evaluator placement** → RESOLVED: new pure `internal/gate` package; gh exec stays in
  `internal/github`; provenance ranking extracted to `internal/provenance.BestResult`.
- [x] **`reviewThreads(first:100)` pagination** → RESOLVED: fail-closed on truncation, not full
  pagination. `pageInfo.hasNextPage` is now requested alongside the first page; `hasNextPage==true`
  returns an error (undeterminable thread-resolution status) rather than the truncated partial count —
  correct and far cheaper than looping for a gate that only ever needs "zero vs. more".
