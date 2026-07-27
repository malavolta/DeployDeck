# Proposal: HU-014 — Preparar Push Y PR (`push-pr`)

## Intent

After a SUCCESSFUL validation, DeployDeck stops at a quit-only result screen: the validated deploy branch is never pushed and the reviewer has no PR entry point, so the promotion flow never closes. HU-014 closes it by offering push + PR preparation from a successful terminal state — WITHOUT replacing the formal review process. Builds on the 4 archived slices; `docs/ARQUITECTURA.md` already anticipates this (`App → GHSvc → Exec`, `PushPreparation` state, `RunRecord.PRUrl`).

## Scope

### In Scope (ACs `HISTORIAS.md:922-930`)
- Offer push ONLY after `Succeeded`/`SucceededPartial` (never Failed/Canceled/Aborted/Error).
- Explicit confirm → `git push -u origin <deploy-branch>` (show command first).
- Push OK → show base/compare + suggested title `<ticket> - Promote changes to <target>`.
- Detect `gh` (3 states: absent / present-unauth / authed) via one `gh auth status`.
- gh authed + explicit confirm → `gh pr create --base <target> --head <deploy> --title`; record PR URL.
- gh absent/unauth → compare URL derived from `origin` (host NOT hardcoded); flow continues.
- PR failure → show error + manual base/compare/title; flow survives.
- Informative, non-blocking `gh` doctor check (closes the HU-001→HU-014 deferral).
- Persist `PRUrl` on the run.

### Out of Scope
- "Idea Futura" local-model PR title/description (`HISTORIAS.md:985`, confirmed out).
- HU-016 re-promote (`SourceRunID`), HU-017 cleanup (`Cleanup` state), HU-015 quick deploy.
- CLI push/PR subcommand (TUI-only this slice).

## Capabilities

### New
- `push-pr-preparation`: post-success push + PR-prep TUI flow via `StatePushPreparation` (confirm→push→base/compare/title→gh-create-or-compare-fallback) plus a PURE compare-URL normalizer that derives host/org/repo from origin.

### Modified
- `prereq-check`: add informative, non-blocking `gh` availability/auth doctor check.
- `run-persistence`: `Record.PRUrl` additive (`omitempty`, no schema bump) + `Writer.MarkPRCreated(runID, prURL)` (symmetry with `MarkCanceled`).

## Approach

New `internal/github` sibling module over `exec.Runner` (like `internal/delta`/`internal/salesforce`): `gh auth status` detection, `CreatePR`, and a PURE compare-URL normalizer. Push stays in `internal/git`: new `Push(ctx,dir,branch)` (`git push -u origin`) + `RemoteURL(ctx,dir,name)`. `internal/app` NEVER execs directly (boundary test holds).

- **Compare-URL** derives `https://<host>/<org>/<repo>/compare/<target>...<deploy>` from origin (SSH `git@<host>:org/repo(.git)`, HTTPS, `ssh://git@<host>/...`, trailing `.git`). Unrecognized form → fail gracefully (raw origin + manual data), never a malformed link. Host comes from origin (the user's gh is on `github.ibm.com` Enterprise).
- **gh detection** via Runner non-zero-exit-as-data: Runner error = binary missing; nil error + `ExitCode!=0` = unauthed; `ExitCode==0` = authed.
- **CreatePR** = explicit-confirm-only (never automatic), fully non-interactive flags, Raw preserved on success AND failure.

**Build order:** (1) `git.Push`/`RemoteURL` + `internal/github` module → (2) `StatePushPreparation` wiring (pull `Succeeded` out of the quit-only handler, mirroring HU-012's `c` sub-flow) → (3) `PRUrl` persistence → (4) `gh` doctor check.

**Testing (strict TDD):** pure units — compare-URL normalization incl. an Enterprise-host (`github.ibm.com`) case, title generation, gh-state branching; real `git.Push`/`RemoteURL` integration on a temp repo + bare local remote (`-short`-skippable); FakeRunner-canned `gh` for `CreatePR`; `Model.Update` transitions via injected Deps. **Real `gh pr create` is NEVER run in tests** (would create a real PR on github.ibm.com).

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `internal/github/` | New | gh detect, `CreatePR`, pure compare-URL normalizer |
| `internal/git` | Modified | + `Push`, `RemoteURL` |
| `internal/app` (`app.go`/`keys.go`/`view.go`/`update.go`) | Modified | `StatePushPreparation` + its own handler |
| `internal/runs` | Modified | `Record.PRUrl` + `MarkPRCreated` |
| prereq/doctor | Modified | informative `gh` check |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| Compare-URL edge cases / Enterprise hosts | Med | Derive host from origin; table-test incl. Enterprise case; unrecognized→graceful |
| `gh pr create` prompts interactively | Med | Fully explicit non-interactive flags, no TTY assumptions |
| State-machine blast radius | Med | Well-precedented (HU-012 sub-flow); boundary test holds |
| Push mutates shared Git state | Low | `-u origin <branch>` only; rollback below |
| HU-014 before HU-016 (docs order) | Low | Additive/harmless; noted |

## Rollback Plan

New module + additive field — revert the branch to drop all code. Push is the only external side effect: to undo, delete the pushed remote branch (`git push origin --delete <branch>`). No PR is created without explicit confirmation, so rollback has no PR to close. `PRUrl` is `omitempty`, so older `run.json` files are unaffected.

## Dependencies

- `gh` is an OPTIONAL dependency (informative doctor check only; never blocking).
- E2E uses a bare local remote + FakeRunner-canned `gh`; no real org, CI-safe.

## Success Criteria

- [ ] Push offered only after a successful validation; not primary after failure.
- [ ] Confirm runs `git push -u origin <branch>`; then base/compare/title shown.
- [ ] gh authed + confirm creates the PR and records `PRUrl`; absent/unauth shows the origin-derived compare URL and the flow continues.
- [ ] PR failure shows error + manual data without aborting.
- [ ] Informative `gh` doctor check present; `internal/app` boundary test holds.
