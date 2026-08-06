# Exploration — deploy-gate

A per-environment DEPLOY GATE. For environments opted-in via the YAML config (e.g. INT, FULL),
before DeployDeck performs the real deployment (quick-deploy) it MUST verify the run's PR meets 4
conditions and block otherwise. User-approved shape (3 product decisions taken up-front):
- Gate runs LOCALLY inside DeployDeck (cooperative control — bypassable, same honesty class as
  provenance; a hard gate would be branch protection + Actions, out of scope).
- Approval requirement = `minApprovals` configurable per environment (default 1), counting only
  approvals from a configured approver list.
- Validation record = DeployDeck posts an automatic comment to the PR after a successful CheckOnly.

The 4 gate conditions: (1) approved by ≥ minApprovals of the approver list; (2) no unresolved
review threads; (3) a DeployDeck validation comment present (the audit record AND the gate signal);
(4) valid provenance signature (PR created by DeployDeck). Verified against the code.

## Current state

1. **Real deploy step is quick-deploy — confirmed, no alternative path.** `salesforce.Client`
   (`internal/salesforce/client.go:19-61`) has 8 methods; the only real-deploy primitive is
   `QuickDeploy` (`internal/salesforce/quick.go:41-68`, `sf project deploy quick --job-id --target-org`),
   re-executing a validated CheckOnly job as a real deploy. Fires from `keyQuickDeploy` "enter"
   (`internal/app/keys.go:1432-1511`); point of no return `keys.go:1477`
   (`return m, m.quickDeployCmd()`). Gated today by: in-flight guard (1435-1441), `QuickDeployEligible`
   re-check (1455-1458), `quickDeployExecAllowed` (`keys.go:1355-1357`), typed `"DESPLEGAR"`
   (1464-1467). **Gate insertion: between the word-check (1464-1467) and dispatch (1477).**
2. **Config has two keying conventions.** `Config.Branches` (`config.go:103-105`) keys by logical
   env name; `Config.Sandboxes map[string]SandboxConfig` (`config.go:107-109`) keys by branch/glob,
   resolved by `SandboxFor` (`internal/config/sandbox_for.go:11-27`, exact then `path.Match`).
   `Destination.Branch` (`git/target_selection.go:33`) confirms "INT" is a branch. → new
   `Gates map[string]GateConfig` mirrors `Sandboxes` keying (branch/glob). `QuickDeployConfig`
   (`config.go:68-79`, default-off, zero-value-safe) is the structural precedent; `Validate()`
   (`config/validate.go:19-59`) is where new checks (minApprovals≥1, non-empty approvers) go.
3. **GitHub reads — 4 exist, none cover approvals/threads/comment-post.** `github.Client`
   (`client.go:26-63`): `AuthStatus, CreatePR, PRForBranch, PRDetails`. Approvals:
   `gh pr view <url> --json reviews,latestReviews,reviewDecision` (login + APPROVED/CHANGES_REQUESTED)
   over the existing REST `--json` mechanism. Unresolved threads: gh `--json` has NO thread-resolution
   field → requires `gh api graphql` with `reviewThreads(first:N){ nodes { isResolved } }` (the
   project's first GraphQL call); needs owner/repo/PR-number, but `ParsePRURL` (`compare_url.go:94-99`)
   captures then DISCARDS the PR number (`m[4]`) — extend it. Posting: new `gh pr comment <url> --body`,
   same discrete-args shape as `CreatePR` (`client.go:91-108`). No comment method exists yet.
4. **PR↔run linkage is unreliable at quick-deploy time.** `Record.PRUrl` (`runs/writer.go:60-65`)
   is set only by `MarkPRCreated` (`writer.go:311-322`) after `createPRCmd` in the SAME session.
   `validate`/`delta` standalone runs never touch PR → `PRUrl==""`; even promotion runs can reach
   quick-deploy without a PR that session. Reuse `selectOrphans`' pattern (`commands.go:367-390`):
   re-derive the branch via `git.RenderBranchName(format, rec.Ticket, rec.Target)` and fall back to
   `PRForBranch(ctx, branch)` (`client.go:45-54`) when `rec.PRUrl==""`.
5. **Provenance reuse — sufficient, no new primitive.** `ParsePRURL(url)→ownerRepo`
   (`compare_url.go:94-99`); `PRDetails(ctx,url)→(headBranch,body,err)` (`client.go:55-62`);
   `provenance.ParseMarkers(body)→[]Marker` (`provenance.go:134-144`);
   `provenance.Verify(ownerRepo,headBranch,runID,sig)→Result` (`provenance.go:153-164`).
   `cmd/deploydeck/pr.go:86-127` (`runPRVerify` + `bestVerifyResult` 137-148) is the exact reusable
   composition.
6. **Validation→comment seam: `onReportDone` terminal branch.** `update.go:1105-1162`: on
   `IsTerminal(status)` (1151) → `terminalState` folds Succeeded/SucceededPartial into StateSucceeded
   (`app.go:778-787`). Available: `m.report` (`salesforce.DeployReport`, `report.go:87-129`) —
   component/test error counts, `CodeCoverage []CodeCoverageResult` per-class `Percent()`
   (`report.go:57-63`, NO aggregate %), `m.jobID` (SF job id) distinct from `m.runID` (DeployDeck id).
7. **State insertion — precedents.** `StateBranchCollision` (`keys.go:364-380`) is an async
   pre-action gate; `StateQuickDeploy` has an in-flight guard `quickDeployingRunID` (`app.go:516-528`).
   New gate = async `gateCheckCmd` (mirror `quickDeployCmd` async shape `commands.go:1271-1281`) +
   a gate sub-state on `StateQuickDeploy`, re-checked at the point of no return like the existing
   `QuickDeployEligible` re-check (1455-1458).
8. **Exec boundary clean.** All gh behind `internal/github.Client` via `Deps.GH`; new approvals/threads
   reads + comment-post land as new Client methods; `internal/salesforce` untouched (gate uses
   `m.report` already in Model); `internal/app` renders/decides only.

## Key decisions for propose/design

- **Config keying**: branch/glob (mirror Sandboxes+SandboxFor), gate lookup at deploy time via
  `rec.Target`.
- **PR resolution**: `rec.PRUrl` first, fallback `PRForBranch` on the re-derived branch; FAIL-CLOSED
  when no PR is found (mirror `IsProductionTarget` fail-closed, `target_selection.go:105-129`).
- **Threads**: GraphQL required; specify auth/error handling for the first `gh api graphql` call.
- **Comment content**: `DeployReport` has no aggregate coverage % — compute from `CodeCoverage`
  (sum covered / sum total) or omit; decide.
- **Idempotency**: posting is the first write gh call besides `CreatePR` — use a hidden dedup marker
  (like provenance's HTML comment) so retries/resumes never double-post; possibly UPDATE not re-post.
- **Approver matching**: normalize handles (strip `@`, case-insensitive login compare).
- **Gate failure UX**: block the deploy and list the unmet conditions; no in-app override (a
  cooperative gate does not need one — it is bypassable at the config/binary layer anyway).
- **Comment scope**: post the validation comment only when the target env has the gate enabled
  (avoid noise on ungated envs).

## Risks

- Unresolved threads needs `gh api graphql` — heavier/different error shape; zero GraphQL precedent.
- Approver-handle matching (case, `@`, bot/team accounts) — no normalization precedent.
- Cooperative-control honesty: bypassable local gate, same class as provenance's extractable secret
  — state as accepted, not solved.
- Comment-posting is a new outward/write action with no idempotency precedent in the gh usage.
- `PRUrl` frequently unset (standalone/cross-session) — gate correctness depends on the `PRForBranch`
  fallback, not just `rec.PRUrl`.
- Config-keying ambiguity (Branches vs Sandboxes) must be resolved explicitly.

Artifact store: OpenSpec (Engram MCP unavailable).
