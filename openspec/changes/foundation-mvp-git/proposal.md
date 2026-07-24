# Proposal: Foundation + MVP Git (HU-001..HU-006)

**Delivery phase:** Fase 1 — MVP Git (`EPICA.md` Plan De Entrega).

## Intent

Promoting commits between Salesforce environments today is a manual, error-prone Git cherry-pick flow: engineers hand-check prerequisites, grep the log for a ticket, eyeball commit order, pick commits one-by-one, and reason about conflicts and partial promotions by hand. This change bootstraps the DeployDeck Go project (the first code in the repo) and ships a TUI + CLI that guides that flow safely from prereq check to a verified cherry-pick. Success = a user goes doctor → ticket → commit selection → target sandbox → temp branch → controlled cherry-pick + post-pick verification, all covered by CI.

## Scope

### In Scope
- Bootstrap: `go.mod` (module `deploydeck`), `cmd/deploydeck` (Cobra), `.gitignore` (incl. `.deploydeck/`).
- Modules: `internal/{exec,config,git,app,prereq}` + minimal read-only `internal/salesforce` shim.
- HU-001 prereq/doctor · HU-002 find commits by ticket · HU-003 manual selection · HU-004 target/sandbox · HU-005 temp promotion branch · HU-006 controlled cherry-pick + post-pick verification.

### Out of Scope
- HU-007/008 delta + `sfdx-git-delta` spike (Fase 2 — zero sgd touchpoints here) · HU-010/011 validate (Fase 3) · HU-009/012 queue (Fase 4) · HU-013–019 productization/push/PR (Fase 5+).
- Real-org e2e (`DEPLOYDECK_E2E_ORG`) — local-only, not exercised in this slice.

### Resolved (do not reopen)
- Module path `deploydeck` (placeholder, renameable via `go mod edit`). `internal/salesforce` = read-only shim only (`sf --version`, `plugins --json`, `org list --json`); full deploy/validate/queue service deferred to Fase 3 — later phases **extend**, not rewrite. `internal/prereq` = separate package (clean exec boundary, reused by TUI + `doctor`). Scope edge = HU-006 → `PickVerification`; `DeltaGeneration`+ is OUT. DEC-001 is team-process context only; HU-006 still ships the empty-pick `--skip` squash safety-net regardless.

## Capabilities

### New Capabilities
- `prereq-check`: HU-001 local prerequisites + `deploydeck doctor` CLI.
- `commit-discovery`: HU-002 ticket search, topo-order, equivalence.
- `commit-selection`: HU-003 manual selection + per-file dependency warning.
- `target-selection`: HU-004 destination env / sandbox alias.
- `promotion-branch`: HU-005 temporary promotion branch creation.
- `cherry-pick`: HU-006 controlled cherry-pick + post-pick verification.

### Modified Capabilities
- None (first code change; no existing specs).

## Approach

Single `internal/exec` boundary (`CommandRequest`/`CommandResult`), non-interactive Git (`GIT_EDITOR=true`, `GIT_TERMINAL_PROMPT=0`, `GIT_PAGER=cat`); `internal/app` never execs directly. Build order (exploration §4): exec → config → git core → HU-001→002→003→004→005→006. HU-006 verifies via content-equivalence (`git cherry`/patch-id) plus the empty-pick `--skip` safety-net. Strict TDD: pure-unit tests (parsing, config, branch templating, selection logic) + a shared `newTempRepo` integration harness for HU-002/003/005/006; all six HUs' tests run in CI (`go test ./...`).

## Affected Areas
| Area | Impact | Description |
|---|---|---|
| `go.mod`, `.gitignore` | New | Bootstrap; ignore `.deploydeck/`. |
| `cmd/deploydeck` | New | Cobra entry + `doctor` subcommand. |
| `internal/exec` | New | Sole external-command boundary. |
| `internal/config` | New | YAML load/defaults/validate. |
| `internal/git` | New | status/branches/commits/cherry-pick. |
| `internal/app` | New | TUI state machine (PrereqCheck→PickVerification). |
| `internal/prereq` | New | HU-001 checks, reused by doctor. |
| `internal/salesforce` | New | Minimal read-only shim. |

## Risks
| Risk | Likelihood | Mitigation |
|---|---|---|
| HU-006 stateful cherry-pick (12 AC, talla L) | High | Incremental RED→GREEN per AC; reconcile `CHERRY_PICK_HEAD`/sequencer; `--continue/--skip/--abort`. |
| Squash merges hide original commits | Med | Empty-pick `--skip` safety-net + equivalence check. |
| Shared-file partial promotion | Med | Dependency warning + post-pick verify vs source branch. |

## Rollback Plan

All work happens on local temp branches — no push/PR/sandbox writes. Runtime rollback: `git cherry-pick --abort` + delete the temp promotion branch (source branches untouched). Change rollback: revert bootstrap commits / remove `cmd`+`internal` tree; repo returns to pre-code planning-only state.

## Dependencies
- Local `git`; `sf` CLI present only for HU-001/004 read-only checks (faked in tests).

## Open items for sdd-design
- `.deploydeck/lock` liveness/PID mechanism (undefined in docs).
- `internal/salesforce` shim interface shape (keep Fase 3-extensible).

## Success Criteria
- [ ] `deploydeck doctor` reports prereqs with non-zero exit on blockers.
- [ ] Full TUI flow PrereqCheck→PickVerification works on a temp repo.
- [ ] All six HUs' unit + integration tests green in CI (`go test ./...`); no real e2e org required.
