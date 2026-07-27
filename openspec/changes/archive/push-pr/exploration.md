# Exploration: HU-014 — Preparar Push Y PR (`push-pr`)

Builds on archived `foundation-mvp-git`, `delta-validation`, `deploy-queue`, `run-history`. 16 living specs. Architecture already anticipated HU-014 — mostly a matter of following `docs/ARQUITECTURA.md` verbatim.

## Current state (verified)
- State machine ends at `StateSucceeded`/`StateFailed`/`StateCanceled` (`app.go:93-100`), all rendered by one `viewValidationResult()` + one quit-only key handler (`keys.go:40-46`). Nothing past this. **No `internal/github` package, no `Push` method, no gh code anywhere** (grep-confirmed).
- `internal/git` has `HasRemote` (`service_remote.go:8-24`, checks exit code only) but no `Push` and no remote-URL getter. `DeploymentPlan` already has `Ticket`/`TargetBranch`/`PromotionBranch` (the deploy branch).
- `internal/runs.Record` has no `PRUrl` yet — additive per the HU-013 precedent (all new fields `omitempty`, no SchemaVersion bump). `MarkCanceled` is the precedent for a "record a terminal external result + companion file" writer method.
- `openspec/specs/prereq-check/spec.md:102`: the informative `gh` availability/auth doctor check was DEFERRED to HU-014 — this slice owns it.
- Architecture pre-answers the structure: `App → GHSvc[internal/github] → Exec` (`ARQUITECTURA.md:33,39`); push is `internal/git`'s responsibility ("Ejecuta push." `:80`); gh detection / `gh pr create` / compare-URL in `internal/github` (`:93-98`); state diagram `ValidationSucceeded → PushPreparation → Cleanup` (`:188,194`, Cleanup=HU-017 OUT); `RunRecord` already lists `PRUrl` (`:248`).

## Scope IN/OUT
IN (`docs/HISTORIAS.md:896-965`): offer push ONLY after a successful validation (`StateSucceeded`, never Failed/Canceled); `git push -u origin <deploy-branch>` on explicit confirm (show cmd first); show base/compare/suggested-title (`<ticket> - Promote changes to <target>`); detect `gh` (3 states: absent / present-unauth / present-authed); if authed → offer `gh pr create --base <target> --head <deploy> --title "..."` with explicit confirmation, record PR URL; if absent/unauth → show compare URL derived from `origin`; PR failure → show error + manual data; persist `PRUrl` on the run.
OUT: the "Idea Futura (Opcional)" local-model PR title/description (`docs/HISTORIAS.md:985` "Fuera del MVP de HU-014" — CONFIRMED out); HU-016 re-promote (`SourceRunID`); HU-017 cleanup (`Cleanup` state); HU-015 quick deploy.

## Resolved decisions (from the 6 open decisions + the probe)
1. **Push in `internal/git`** (new `Push(ctx,dir,branch) error` = `git push -u origin <branch>`, same non-interactive-env/exit-check shape as `fetchOrigin`). gh + compare-URL in a NEW `internal/github` sibling module over `exec.Runner` (like `internal/delta`/`internal/salesforce`). Add `git.Service.RemoteURL(ctx,dir,name) (string,error)` (sibling of `HasRemote`, returns trimmed stdout of `git remote get-url`).
2. **Compare-URL = a PURE normalizer that DERIVES host/org/repo from the origin URL** — NOT hardcoded to github.com. IMPORTANT (probe): the user's `gh` is authed to **`github.ibm.com` (GitHub Enterprise)**, so the compare host must come from origin. Handle SSH `git@<host>:org/repo(.git)`, HTTPS `https://<host>/org/repo(.git)`, and `ssh://git@<host>/...`; output `https://<host>/<org>/<repo>/compare/<target>...<deploy>`; on an unrecognized form, FAIL GRACEFULLY (show the raw origin URL + the manual base/compare/title, never a malformed link).
3. **gh detection = a single `gh auth status`** via the Runner non-zero-exit-as-data contract: Runner error (nil result) = gh binary MISSING; nil error + `ExitCode!=0` = installed-but-UNAUTHED; `ExitCode==0` = AUTHED. No `gh --version` / no minVersions entry.
4. **`github.Client.CreatePR(ctx, base, head, title) (url, raw, err)`** = `gh pr create ...` with EXPLICIT confirmation (never automatic), fully non-interactive flags (no TTY assumptions), Raw preserved on success AND failure (failure → error + manual data, flow survives).
5. **New literal `StatePushPreparation`**, auto-entered from `Succeeded`/`SucceededPartial` only; `StateFailed`/`StateCanceled`/`StateAborted`/`StateError` stay quit-only. Pull `StateSucceeded` out of the shared quit-only key handler into its own handler (mirrors HU-012's `c`→`StateCancelConfirm` gated-sub-flow idiom). Holds: confirm→`git.Push`; base/compare/title display; gh branch (auth-detected `gh pr create` with confirm, or compare-URL fallback). `internal/app` NEVER execs directly (boundary test holds).
6. **`PRUrl` additive on `Record`** (`json:"prUrl,omitempty"`, no schema bump); persist via a new `Writer.MarkPRCreated(runID, prURL)` (symmetry with `MarkCanceled`).
7. **TUI-only** (no push/PR CLI subcommand this slice).
8. **Informative gh doctor check** (closes the HU-001→HU-014 deferral): add an informative, non-blocking `gh` availability/auth check to `prereq-check`/doctor (modified delta). Small.

## Acceptance criteria (`docs/HISTORIAS.md:922-930`)
1 successful validation→push offered; 2 failed→push not primary; 3 confirm→`git push -u origin <branch>`; 4 push OK→base/compare/title; 5 gh authed+confirm→`gh pr create`→URL; 6 gh absent/unauth→compare URL, flow continues; 7 PR fails→error + manual data.

## E2E harness (`docs/HISTORIAS.md:958-964`, CI-safe, NO org)
`temp + gh-fake`: a temp git repo + a **bare local remote** (`git init --bare`, added as origin) so real `git push -u` round-trips + `exec.FakeRunner` canned `gh auth status`/`gh pr create` for the 3 gh states + origin set to SSH and HTTPS forms to exercise compare-URL normalization. `gh-fake` = the existing `FakeRunner` (same as sf-fake), NOT a new fake type. **Real `gh pr create` is NEVER run in tests** (it would create a REAL PR on github.ibm.com) — fake it. Real read-only `gh auth status` could be an opt-in smoke, but the required coverage is fake.

## Testability (strict TDD)
Pure-unit: compare-URL normalization (SSH/HTTPS/ssh://, trailing `.git`, unrecognized→graceful) — the load-bearing table test incl. an ENTERPRISE host (github.ibm.com) case; title generation; gh-state branching (FakeRunner error vs ExitCode). Integration (real git, temp+bare-remote, `-short`-skippable): `git.Push`/`RemoteURL`. FakeRunner: `CreatePR`. `Model.Update` TUI transitions for `StatePushPreparation` (inject fake git.Service + github.Client via Deps).

## Risks
- Compare-URL edge cases beyond SSH/HTTPS (Enterprise hosts [github.ibm.com — real for this user], `ssh://`, no `.git`) — MUST derive host from origin + fail gracefully, not guess.
- `gh pr create` may prompt interactively in some configs → compose with fully explicit flags, no TTY assumptions (same non-interactive discipline as git).
- State-machine switch blast radius (`app.go`/`view.go`/`keys.go`/`update.go`) — moderate, well-precedented.
- Doing HU-014 before HU-016 (docs' recommended order) — harmless/additive; note it.

## Ready for Proposal: yes (follow ARQUITECTURA Approach A; resolve the decisions above — all low-risk defaults).
