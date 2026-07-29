# Exploration: HU-018 — Modos Standalone: Delta Y Validacion Sueltas (`standalone-modes`)

Tenth (final) slice, stacked on `quick-deploy`. HU-018 (`docs/HISTORIAS.md:1116-1149`, Fase Futuro / Media / Producto). User scoped it to **build the minimal main menu AS PART of this slice** ("aplicar todo") — HU-018 assumes a `menu principal` that does NOT exist yet. Two standalone modes = near-zero-risk REUSE of existing machinery via minimal-plan population; the `StateMainMenu` prerequisite is the only genuine architectural change.

## Current state (code-anchored)
- **No menu today.** `app.New` starts at `StatePrereqCheck` (`app.go:479`); `onPrereqDone` → resume-detect → either `StateRunHistory` (resume offer) or `StateTicketInput` (the linear-flow hub). No `StateMenu`/`StateMain`.
- **Delta machinery (HU-007) is reusable**: `deltaCmd` (`commands.go:651`, 3 callers in keys.go) runs `delta.Service.Generate` and lands on `StatePackageReview` with the per-type summary via `parsePackageFile` (`commands.go:701`), output under `.deploydeck/manifest/`. **It reads from `m.plan`** (source/base, output) and **hardcodes `to := "HEAD"`** — so "elegir ref actual" is HEAD (display-only confirmation, NOT an arbitrary-ref picker). Document this, don't silently discover it.
- **Validation machinery (HU-010/011) is reusable**: `validateCmd` (`commands.go:728`, 7 callers) runs `salesforce.ValidateDeploy(ValidateRequest{...})` (reads `m.plan` manifest path + `SandboxAlias`) → `StateValidationStart`/`StateValidationPolling` (HU-011 `tea.Tick` poll to terminal). `salesforce.ValidateRequest` (`validate.go`) takes a manifest path directly.
- **Run creation** happens today at BRANCH time (`onBranchCreated`, `update.go:~516`) with plan fields (Ticket/Target/PromotionBranch/Commits/TestLevel/JobID). `Writer.Create(rec, validateRaw)`/`Save` are the persistence seams. Standalone modes have NO branch/ticket/commits.
- **`StatePackageReview`** confirm handler + footer are NOT mode-aware — they continue to validation. Standalone delta needs a small mode-aware branch to "stop after summary".
- **Blast radius**: `Update` has ~148 callers; several tests assert `state == StateTicketInput` right after prereq/`New`. `onResumeDetect` has a state-literal guard `if m.state != StateTicketInput` (`update.go:269`).

## Resolved decisions
1. **Menu = new `StateMainMenu` as the post-prereq no-resume landing.** `onResumeDetect`'s no-resume branch → `StateMainMenu` (was `StateTicketInput`). Entries (keyboard cursor + Enter): **[Promocionar ticket → `StateTicketInput`** (the ENTIRE existing full flow past that point is UNCHANGED)**, Generar delta package → standalone delta, Validar package contra sandbox → standalone validation]**. `q`/`esc` quits. **LOAD-BEARING: update `onResumeDetect`'s guard `!= StateTicketInput` → `!= StateMainMenu`** (`update.go:269`) or HU-013 resume-detection silently regresses. Hide-unimplemented (AC3): the menu builds its entry list from a compile-time list of IMPLEMENTED modes (both implemented here → both shown; the mechanism still supports hiding a future-unimplemented entry — spec it). **Cost/risk**: tests asserting post-prereq==`StateTicketInput` need updating to `StateMainMenu` (or a menu-select step); tests that set `state:` directly are unaffected. Tradeoff vs a hub-reachable menu (key from `StateTicketInput`, zero churn but less faithful to "menu principal"): recommend the landing approach (faithful to the story) but design must handle the guard + quantify the test churn.
2. **Delta standalone**: input = a base-branch picker (reuse `git.ListBranches`/`CandidateBranches`); current ref = HEAD (deltaCmd hardcode — display-only). Populate a MINIMAL `m.plan` (the few fields `deltaCmd` reads: base/source + manifest output) → reuse `deltaCmd` AS-IS → `StatePackageReview` shows the summary → **stop after summary** (mode-aware branch in the review confirm handler; NO cherry-picks, NO validation). Empty-delta reuses HU-007/008's warning.
3. **Validation standalone**: user picks an EXISTING `package.xml` (path input) + a sandbox (from `config.Branches`/sandbox aliases). Pre-check the path exists/parses → actionable error if nonexistent/invalid (do NOT launch). Populate a MINIMAL `m.plan` (`PackageXMLPath`, `SandboxAlias`) → reuse `validateCmd` AS-IS → `StateValidationStart`/`StateValidationPolling` to terminal. The invalid/nonexistent-package variant is an explicit actionable-error scenario.
4. **Run creation for standalone modes**: create a MINIMAL Record via `Writer.Create`/`Save` — standalone delta (manifest path, no jobId), standalone validation (jobId, package path). Add an additive `Record.Mode string` (`json:"mode,omitempty"`; ""=promotion, "delta", "validate") discriminator so history distinguishes them; no SchemaVersion bump. Both create a local run under `.deploydeck/runs/` (AC).
5. **Menu scope = minimal**: 3 keyboard-selectable entries, cursor + Enter, small `viewMainMenu`. No richer chrome.
6. **Scope OUT**: NO new delta/validation behavior (pure reuse of HU-007/008/010/011); NOT a productive deploy tool; no change to the full-promotion flow past `StateTicketInput`. HU-019 already done. Reuse the existing empty-delta warning + an actionable invalid-package error.

## New capability + surface
- **NEW spec domain `standalone-modes`** (menu + delta standalone + validation standalone + run creation). Modified (additive delta): `run-persistence` (`Record.Mode`). The entry-point change (post-prereq → `StateMainMenu`) touches startup/resume-detection — spec the resume-detection-still-works invariant.
- New: `StateMainMenu` + `viewMainMenu` + menu key handling; a standalone-delta input screen (`StateDeltaSourceSelect`?) + minimal-plan → `deltaCmd`; a standalone-validation input screen (`StatePackageSelect`?) + minimal-plan → `validateCmd`; `Record.Mode`; the `onResumeDetect` guard fix; a mode-aware `StatePackageReview` stop-after-summary branch.

## ACs → testability (`docs/HISTORIAS.md:1137-1149`; temp git + sgd for delta in CI, fake sf/alias for validation; both create runs)
- delta mode on a hand-prepared branch → generates `package.xml` under `.deploydeck/manifest/` + per-type summary, NO cherry-picks; creates a run.
- validation mode on an existing `package.xml` → launches + polls to terminal like the full flow; creates a run.
- menu entries appear + operational when implemented; the hide-unimplemented mechanism (AC3).
- **Variants**: validation with nonexistent/invalid `package.xml` → actionable error, does NOT launch; delta with empty delta → reuses HU-007/008 warning.
- **Resume-detection regression guard** (from risk 1): after prereq with a resumable run present, resume is still offered despite the new `StateMainMenu` landing.
- CI: partial (delta in CI with temp git + sgd; validation with fake/alias).

## Risks
- **[LOAD-BEARING] `onResumeDetect` guard** (`update.go:269`): must change `!= StateTicketInput` → `!= StateMainMenu` or HU-013 resume-detection silently dies. Explicit regression test required.
- **Test churn / blast radius**: `Update` ~148 callers; tests asserting post-prereq == `StateTicketInput` break when the landing becomes `StateMainMenu`. Design must plan the migration (many tests set `state:` directly and are unaffected; the promotion-flow drivers need a menu-select step or to start at `StateTicketInput`). This is the biggest cost of the slice.
- **`deltaCmd` `to:="HEAD"` hardcode**: "elegir ref actual" is HEAD, not an arbitrary picker — a documented limitation, not a bug.
- **`StatePackageReview` not mode-aware**: needs a careful branch so standalone delta stops after summary WITHOUT breaking the full-flow continue-to-validation path.
- **plan-coupling**: `deltaCmd`/`validateCmd` read `m.plan.*`; minimal-plan population must set EXACTLY the fields they read (enumerate them in design) — an omitted field → wrong/empty behavior.
- Biggest architectural change of any slice (new entry point) — batch the apply (menu + guard first, then each mode, then e2e) and adversarially review the entry-point/resume interaction.

## Ready for Proposal: yes
Design must lock: the menu-placement approach (landing vs hub-key) + the `onResumeDetect` guard fix + the test-migration plan; the EXACT `m.plan` fields `deltaCmd`/`validateCmd` read (for minimal-plan population); the `StatePackageReview` mode-aware stop; `Record.Mode`.
