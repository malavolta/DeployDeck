# Design: Standalone Modes — Delta & Validation Without Full Promotion (HU-018)

## Technical Approach

`StateMainMenu` becomes the post-prereq no-resume landing. Both standalone modes populate a MINIMAL `m.plan` with EXACTLY the fields the reused command reads, then call `deltaCmd`/`validateCmd` unchanged. All external work still routes through the existing `deps.Git`/`deps.Delta`/`deps.SF` seams (`boundary_test.go` holds — no new exec; only file reads via the existing `parsePackageFile`). New surface is UI states + one additive `Record` discriminator. The exploration's 6 decisions are held; two refinements are flagged below.

## Architecture Decisions

### ADR-1 — Menu placement + `onResumeDetect` guard

| Aspect | Choice | Rejected | Rationale |
|---|---|---|---|
| Landing | `New`→`StatePrereqCheck` unchanged; `onPrereqDone` lands `StateMainMenu` (was `StateTicketInput`, `update.go:123`) | Hub-key from `StateTicketInput` (zero churn) | Faithful to "menú principal"; entry point per story |
| Resume guard | `onResumeDetect` guard `!= StateTicketInput` → `!= StateMainMenu` (`update.go:269`) **and** no-resume branch `m.state = StateTicketInput` → `StateMainMenu` (`update.go:287`) | Leave guard | LOAD-BEARING: async `resumeDetectMsg` now lands while on `StateMainMenu`; old guard would treat the landing as "user advanced" and no-op → HU-013 resume silently dies |
| Menu entries | `Promocionar ticket`→`StateTicketInput` (full flow UNCHANGED); `Generar delta`→delta; `Validar package`→validation. `q`/`esc` quit | — | Minimal 3-entry cursor+Enter |
| Hide-unimplemented (AC3) | Package-level `menuEntries []{label,target,implemented bool}`; view/keys iterate the `implemented==true` subset; cursor indexes the FILTERED slice | Runtime feature flag | Compile-time list; both entries `true` here, mechanism still hides a future one |
| Resume-decline (REFINEMENT) | `q`/`esc` on `StateRunHistory` → `StateMainMenu` (was `StateTicketInput`) | Keep `StateTicketInput` | Otherwise the hub is unreachable after a declined resume. Flagged — beyond exploration's literal text |

### ADR-2 — Minimal `m.plan` field sets (EXACT reads)

| Mode | Command reads (verified) | Minimal plan set | Notes |
|---|---|---|---|
| Delta | `deltaCmd` (`commands.go:651`): `plan.Ticket` (output-dir name only), `plan.TargetBranch` (→ `from="origin/"+target`); `to="HEAD"` HARDCODED (display-only, documented limitation, not a bug) | `Ticket="standalone"`, `TargetBranch=<picked base, "origin/" stripped>` | `SourceDirs`/ignores come from `cfg`, not plan |
| Validate | `validateCmd` (`commands.go:728`, reuse branch): `plan.PackageXMLPath`→`ManifestPath`, `plan.DestructiveChangesPath`→`PostDestructivePath`, `plan.SandboxAlias`→`TargetOrg`, `plan.TestLevel` | `PackageXMLPath=<path>`, `SandboxAlias=<alias>`, `TestLevel=<SandboxConfig.TestLevel>` | Reuse branch does NOT read `Ticket`/`Target` (only the fallback does) |

Base-branch picker reuses `git.ListBranches` (async `standaloneBranchesCmd`→`standaloneBranchesMsg`, mirroring `onDeployBranches`). Sandbox pick is built in-memory from `cfg.Sandboxes[*].Alias`+`.TestLevel` (no exec).

### ADR-3 — `StatePackageReview` mode-aware stop

| Aspect | Choice | Rationale |
|---|---|---|
| Mechanism | New `Model.standaloneMode string` (`""`/`"delta"`/`"validate"`), set on menu dispatch | A field, not a state fork — keeps the single review screen; full flow (`standaloneMode==""`) continues to `StateQueueReview` UNCHANGED |
| Delta stop | `keyPackageReview`: when `standaloneMode=="delta"`, `enter` is a no-op (no `confirmPackageReview`); `viewPackageReview` footer drops "Enter validar" | No cherry-picks, no validation; summary is terminal |
| Empty delta | Existing `m.summary.Empty` warning renders as-is (informational only; no override needed — nothing to gate) | Pure HU-007/008 reuse |

### ADR-4 — `Record` growth (run-persistence, additive)

| Field | Type/tag | Set by | Rationale |
|---|---|---|---|
| `Mode` (required) | `string` `json:"mode,omitempty"` (`""`=promotion/`"delta"`/`"validate"`) | both modes | History discriminator; omitempty → old `run.json` loads unchanged; NO SchemaVersion bump |
| `ManifestPath` (REFINEMENT) | `string` `json:"manifestPath,omitempty"` | delta=generated `package.xml`, validate=input path | Task requires the path be captured; `Record` has no path slot today. Same additive contract. Flagged — proposal named only `Mode` |

Creation, mirroring `onBranchCreated` (best-effort, nil-`Runs` skip):
- **Delta** — in `onDeltaDone` when `standaloneMode=="delta"` (after `RegisterDeltaArtifacts`): `Writer.Save` a record (`RunID="delta-"+base+"-"+ts`, `Mode="delta"`, `ManifestPath=plan.PackageXMLPath`, no `JobID`).
- **Validate** — on sandbox confirm, BEFORE firing `validateCmd`: `Writer.Save` a minimal record (`RunID="validate-"+alias+"-"+ts`, `Mode="validate"`, `ManifestPath`, `Alias`, `TestLevel`) and set `m.runID`. `validateCmd`'s existing reuse branch then `Load`s it and MERGES `JobID`+`Phase="validating"` (Mode/ManifestPath survive the round-trip). `validateCmd` stays AS-IS.

### ADR-5 — Test-migration rule (~148 `Update` callers)

| Test shape | Rule | Files |
|---|---|---|
| Constructs `Model{state: StateTicketInput}` directly | UNAFFECTED — do not touch | most of `resume_test`, `branch_cleanup_test`, `transitions_test`, etc. |
| Drives `New()`→`prereqDoneMsg`, asserts landing | Change assertion `StateTicketInput`→`StateMainMenu` | `prereq_test.go:82`, `original_branch_test.go:104-105`, `resume_test.go:385,435` |
| E2E that continues INTO the full flow after prereq | Insert one menu-select step (`Enter` on `Promocionar ticket`) to reach `StateTicketInput` | `flow_e2e_test.go:136`, `promotion_real_metadata_e2e_test.go:43` |
| Resume-decline asserts `StateTicketInput` | Change to `StateMainMenu` (ADR-1 refinement) | `run_history_test.go:176`, `run_history_e2e_test.go:89` |

`sdd-apply` applies exactly this table — do not blanket-rename `StateTicketInput`.

## Data Flow

    Prereq ─→ onPrereqDone ─→ StateMainMenu ←── resumeDetect (guard: !=StateMainMenu)
      │                          │  │  └─ resumable? → StateRunHistory ──(q/esc)─┘
      │      Promocionar ─────────┘  │
      │      ticket → StateTicketInput (full flow UNCHANGED)
      ├─ Delta:   StateDeltaSourceSelect ─(ListBranches)→ minimal plan → deltaCmd
      │           → onDeltaDone(save Mode=delta) → StatePackageReview → STOP
      └─ Validate: StatePackageSelect(path pre-check) → StateSandboxSelect
                  → save Mode=validate (sets runID) → validateCmd → StateValidationStart/Polling

## File Changes

| File | Action | Change |
|---|---|---|
| `internal/app/app.go` | Modify | Add `State` consts `StateMainMenu`, `StateDeltaSourceSelect`, `StatePackageSelect`, `StateSandboxSelect`; `Model` fields `standaloneMode`, `menuCursor`, `branchList []git.Branch`, `branchCursor`, `packagePath`, `sandboxList []string`, `sandboxCursor`; package `menuEntries` |
| `internal/app/update.go` | Modify | `onPrereqDone`→`StateMainMenu`; `onResumeDetect` guard+no-resume→`StateMainMenu`; `onDeltaDone` delta-run save; add `onStandaloneBranches`; resume-decline→`StateMainMenu` |
| `internal/app/commands.go` | Modify | Add `standaloneBranchesCmd`/`standaloneBranchesMsg`; reuse `parsePackageFile` for path pre-check. `deltaCmd`/`validateCmd`/`parsePackageFile` UNCHANGED |
| `internal/app/keys.go` | Modify | Dispatch + `keyMainMenu`/`keyDeltaSourceSelect`/`keyPackageSelect`/`keySandboxSelect`; mode-aware `keyPackageReview` |
| `internal/app/view.go` | Modify | Switch + `viewMainMenu`/`viewDeltaSourceSelect`/`viewPackageSelect`/`viewSandboxSelect`; mode-aware footer in `viewPackageReview` |
| `internal/runs/writer.go` | Modify | Additive `Record.Mode`, `Record.ManifestPath` (omitempty) |

## Interfaces / Contracts

- `standaloneMode` gates the ONLY behavioral fork in the shared review screen; `""` preserves the full flow verbatim.
- Base picker normalizes remote refs: `Branch{Name:"origin/main",Remote:true}` → `TargetBranch="main"` (so `deltaCmd`'s `"origin/"+target` resolves correctly).
- Validate path pre-check: `parsePackageFile(path,false)`; on error set actionable `m.notice`, STAY on `StatePackageSelect`, do NOT fire `validateCmd`.

## Testing Strategy

| Layer | What | Approach |
|---|---|---|
| Unit | Menu entry list + hide-unimplemented + cursor/Enter routing | `Model.Update` with `tea.KeyMsg` |
| Unit | **Resume regression (MANDATORY)**: resumable run present → still offered after `StateMainMenu` landing (guard fix) | drive `New()`→`prereqDoneMsg`→`resumeDetectMsg`, assert `StateRunHistory` |
| Unit | Delta: base pick→minimal plan→`deltaCmd`→summary→STOP (no queue/validate); run `Mode="delta"`; empty-delta warning | `Model.Update` + fake Delta/Git |
| Unit | Validate: path+sandbox→minimal plan→`validateCmd`→`StateValidationStart`; run `Mode="validate"`; nonexistent/invalid path→actionable error, NOT launched | `Model.Update` + fake SF |
| Unit | `Record.Mode`/`ManifestPath` round-trip + backward-compat (old `run.json` loads) | `t.TempDir()` writer |
| E2E | Consolidated: temp git+`sgd` (delta) and fake `sf`/alias (validate); both create runs under `.deploydeck/runs/`; NO real org; keep `testing.Short()`-skippable | `advance` driver |
| Boundary | `boundary_test.go` still passes (no new exec) | existing |

## Threat Matrix

| Boundary | Applicability | Design response | RED test |
|---|---|---|---|
| Documentation-like paths | N/A — `package.xml` read as data (`os.ReadFile`+XML parse), never executed/classified | — | — |
| Git repository selection | Applicable — picker + `deltaCmd` run against existing `deps.Dir` (git resolves `RepoRoot` itself); remote refs normalized | Reuse `deps.Dir`; strip `origin/` prefix | delta picker with `origin/<base>` → `from="origin/<base>"` |
| Manifest path input (validate) | Applicable — user path → `--manifest` via `internal/exec` arg-vector (no shell) | Pre-check exists/parses before launch; arg-vector prevents injection | nonexistent + malformed `package.xml` → actionable error, no launch |
| Commit state | N/A — delta is read-only `origin/<base>..HEAD`; validate is check-only | — | — |
| Push state | N/A — no push in standalone modes | — | — |
| PR commands | N/A — no PR automation | — | — |

## Migration / Rollout

No data migration. `Record` growth is omitempty-additive (no SchemaVersion bump) — pre-HU-018 `run.json` loads unchanged. Rollback: revert landing+guard to `StateTicketInput`, drop the new states + `Mode`/`ManifestPath`; no shared git/sandbox state mutated.

## Open Questions

- [ ] Confirm resume-decline→`StateMainMenu` (ADR-1 refinement) is accepted vs. keeping `StateTicketInput`.
- [ ] Confirm adding `Record.ManifestPath` (ADR-4 refinement) vs. persisting `Mode` only per the proposal.
