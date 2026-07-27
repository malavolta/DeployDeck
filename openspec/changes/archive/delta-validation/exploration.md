# Exploration: Delta + Validation (HU-007, HU-008, HU-010, HU-011)

Builds on the archived `foundation-mvp-git` (HU-001..006). Living specs in `openspec/specs/`.

## 1. Scope: IN vs OUT

**IN** (`docs/HISTORIAS.md:1184-1187`, recommended order positions 7-10):
- HU-007 Generar Delta Package (`docs/HISTORIAS.md:440-504`) — wraps `sf sgd source delta`.
- HU-008 Resumir package.xml + destructive changes (`docs/HISTORIAS.md:506-569`).
- HU-010 Validación async (`docs/HISTORIAS.md:628-696`) — `sf project deploy validate --async --json`.
- HU-011 Progreso en vivo (`docs/HISTORIAS.md:698-761`) — poll `sf project deploy report --json`.

**OUT**: HU-009 queue/`DeployRequest` (Tooling API), HU-012 cancel, HU-014 push/PR, HU-015 quick deploy.

**`QueueReview` state = HU-009 → OUT.** State machine (`docs/ARQUITECTURA.md:180-184`):
`DeltaGeneration → PackageReview → QueueReview → ValidationStart → ValidationPolling → Succeeded/Failed/Canceled`.
Since HU-009 is out, `QueueReview` is an inert pass-through this slice (`PackageReview → ValidationStart` on confirm); keep the state name for forward-compat. Explicit design decision.

**Foundation scope edge (verified in code):** `internal/app/app.go:57-59` stops at `StatePickVerification`. The gate `git.DeltaAndValidationAllowed(state, aborted)` (`internal/git/pick_verification.go:129-138`, exposed as `Model.DeltaAllowed()` `internal/app/app.go:152-158`) MUST gate HU-007's entry — reuse, do not re-derive. Living spec `openspec/specs/cherry-pick/spec.md:118-134` confirms: no delta/validation when a cherry-pick fails.

## 2. sfdx-git-delta multi-`--source-dir` SPIKE (blocking HU-007 start)

`EPICA.md:842` + `docs/HISTORIAS.md:458`: verify whether the pinned `sfdx-git-delta` (`minVersions.sfdx-git-delta: "5.0.0"`, `docs/ARQUITECTURA.md:338`) supports repeating `--source-dir`. Two outcomes:
1. Supported → single `sf sgd source delta` call with N `--source-dir` flags.
2. Not → one call per `delta.sourceDirs` entry + merge `package.xml`/`destructiveChanges.xml` (dedupe members, union types).

**SPIKE RESOLVED (empirically, 2026-07-27):** `sfdx-git-delta@6.45.1` installed (≥ minVersions 5.0.0). `sf sgd source delta` DOES support multiple `--source-dir` in ONE invocation — help shows `-s, --source-dir=<value>...` (oclif `...` = repeatable); a real run with `--source-dir force-app --source-dir src2` exited 0 and produced a SINGLE merged `package.xml` (`AccountService` + `B` both under `ApexClass`). → `internal/delta` issues ONE `sf sgd source delta` call with N `--source-dir` flags; the per-dir-and-merge fallback is NOT needed. Observed artifact layout: `<output-dir>/package/package.xml` (destructive under `<output-dir>/destructiveChanges/…` when deletes exist). The real HU-007 `temp+sgd` e2e can now run locally (plugin installed); CI needs the same `sf plugins install sfdx-git-delta`.

## 3. Per-HU acceptance criteria + E2E harness

Legend (`docs/HISTORIAS.md:1248`): `temp`=temp git repo; `sgd`=+real sfdx-git-delta; `sf-fake`=fake Runner canned JSON; `alias`=real org via `DEPLOYDECK_E2E_ORG`; `fs`=filesystem fixtures.

- **HU-007** (`:440-504`): sf sgd source delta → package.xml; deletes → destructiveChanges.xml; empty package → warn + block validation pending future confirm; sgd failure → show output, no validation. Cmd `:477-484`. **E2E `temp+sgd`** (`:498-504`), CI yes (needs plugin). Artifacts under `.deploydeck/manifest/delta/<ticket>-to-<target>`; working tree stays clean.
- **HU-008** (`:506-569`): per-type counts; empty warned; destructive shown separately; sensitive types (`Profile`,`PermissionSet`,`Flow`,`CustomObject`,`CustomField`) warned. `PackageSummary` model `:541-550`. **E2E `fs`** (pure unit, `:563-569`), CI yes.
- **HU-010** (`:628-696`): valid package → jobId; destructive → `--post-destructive-changes`; RunSpecifiedTests → `--tests`; CLI error → msg + raw JSON; jobId → persisted to `.deploydeck/runs/` immediately. Cmds `:665-684`. **E2E `sf-fake/alias`** (`:690-696`), CI partial.
- **HU-011** (`:698-761`): poll `deploy report`; components/tests update live; metadata errors (component/type/message); failed tests (class/method/message); user exits → job stays active, run resumable. Cmd `:735-739`. Terminal states `Succeeded/SucceededPartial/Failed/Canceled` (`:744-749`). **E2E `sf-fake/alias`** (`:755-761`), CI partial.

## 4. Module extensions (verified against code — extend, don't rewrite)

- **`internal/salesforce`** (`client.go:1-6` documents the Fase-3 extension path): add `validate.go` (`ValidateDeploy` → `sf project deploy validate --async --json`, envelope-decoded to a jobId struct) and `report.go` (`ReportDeploy(jobID)` → `sf project deploy report --json`). Extend the `Client` interface; `New(runner)` unchanged. Reuse `decodeEnvelope` (`client.go:86-102`).
- **`internal/delta`** (NEW, `docs/ARQUITECTURA.md:100-106`): runs `sf sgd source delta` over `exec.Runner` directly (sgd is its own CLI, sibling of the salesforce shim, `ARQUITECTURA.md:30-40`); locates artifacts; parses package.xml + destructiveChanges.xml; summarizes metadata per type.
- **`internal/config`** (gap: `config.go:40-63` lacks `Delta` + `PollIntervalSeconds`): add `delta.{outputDir,sourceDirs,ignoreFile,ignoreDestructiveFile}` and `pollIntervalSeconds` (documented `ARQUITECTURA.md:317-329`) + `Validate()` rules.
- **`internal/git.DeploymentPlan`** (`deployment_plan.go:11-32`): add delta-artifact-path fields (`PackageXMLPath`, `DestructiveChangesPath`) — HU-007 AC `docs/HISTORIAS.md:463`.
- **`internal/app`** (`app.go:66-86`): add `Deps.Delta`, new `State` constants `DeltaGeneration → PackageReview → (QueueReview inert) → ValidationStart → ValidationPolling → Succeeded/Failed/Canceled`, wiring past `StatePickVerification`.
- **`internal/runs`** (NEW/minimal): see open decision #2.

## 5. Build order + dependencies

1. HU-007 (delta gen) first — HU-008 and HU-010 consume its package/destructive paths.
2. HU-008 (summary) — pure parsing of HU-007 artifacts; buildable/unit-testable independently of the sgd spike (fixtures, `fs`).
3. HU-010 (async validate) — consumes package/destructive paths as `--manifest`/`--post-destructive-changes`; uses HU-004's `TargetBranch`/`SandboxAlias`/`TestLevel` from `DeploymentPlan`.
4. HU-011 (polling) — depends on HU-010's jobId.

Foundation deps (all built): `internal/exec.Runner` (sole seam + `FakeRunner`), `internal/git` (`DeltaAndValidationAllowed` gate, `DeploymentPlan`), `internal/config`, `internal/salesforce`.

## 6. Testability (strict TDD)

- **Pure unit**: package.xml/destructiveChanges.xml parsing (HU-008 core); validate/report JSON envelope parsing; poll-state transitions (terminal set detection); config Delta/poll load+validate; DeploymentPlan delta-path fields.
- **Integration (real sgd, temp repo, `-short`-skippable)**: HU-007 `temp+sgd` — needs the plugin installed.
- **Real-org e2e (opt-in `DEPLOYDECK_E2E_ORG`, non-destructive CheckOnly)**: `sf project deploy validate --async`/`report` against a personal org (e.g. `AM-DEV-EDITION`) polling to terminal. Follow the exact convention in `internal/prereq/real_org_e2e_test.go` (env-gate + `t.Skip`, `NewOSRunner`, never a real deploy, personal alias only per `ARQUITECTURA.md:373`).

## 7. Open decisions for sdd-propose

1. **sgd multi-`--source-dir`**: spike outcome decides single-call-repeated-flag vs N-calls-and-merge. Resolve (install plugin + probe) or design both paths.
2. **Run persistence gap**: HU-010/011 ACs require `.deploydeck/runs/` but `internal/runs` doesn't exist (it's HU-013, later). Decide: build a MINIMAL run-record writer scoped to jobId+status+raw JSON under `.deploydeck/runs/<run-id>/`, deferring full history/list/retention/resume to HU-013; OR pull forward a slice of HU-013. Building nothing leaves the ACs unmet.
3. **Polling policy**: `pollIntervalSeconds` config + retry/timeout on transient errors (`ARQUITECTURA.md:425` "Polling infinito | Timeout, estados terminales, cancelación por contexto") — fixed interval + hard timeout + ctx cancel.
4. **State-machine extension**: new `State` constants past `StatePickVerification`; `QueueReview` inert pass-through vs omitted; `Deps.Delta`.
5. **Empty-package handling**: HU-007 empty → warn + block validation "salvo confirmación futura" (`:470`) — dedicated confirm state or a flag on `PackageReview`.

## Recommendation

Resolve the sgd spike immediately (install `sfdx-git-delta`, probe `--source-dir` repeatability). Parallel-track HU-008's pure parsing (fixtures, no plugin dep) while the spike resolves. Resolve the run-persistence scope (#2) explicitly in propose before tasking HU-010 — it's the one gap that isn't "extend an existing module."

## Risks

- Spike outcome could resize HU-007 (fallback = per-dir + merge).
- `internal/runs` absent → HU-010/011 ACs reference a dir with no writer unless scoped here.
- Real-org e2e is CheckOnly but touches a live sandbox queue → personal alias only, never shared.
- `QueueReview` behavior ambiguity → decide once (design), not twice.

## Ready for Proposal: yes.
