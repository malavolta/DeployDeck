# Proposal: Source Resolution — Dedupe Candidates + Current-Branch Confirm

## Intent

Promoting a pushed feature branch (present both locally and as `origin/<name>`) to the
first pipeline environment dead-ends: `CandidateBranches` counts the same logical branch
twice, so `SelectSingleSource` raises `ErrMultipleSourceBranches` even though
`git log origin/INT..origin/<branch>` is non-empty. The ambiguous-source path renders
"No se encontraron commits" with no escape hatch (`NoResultsAlternatives` only fires when
candidates are empty). Found via dogfooding. Fix the bug, and make the current branch the
obvious source when discovery is otherwise ambiguous.

**Delivery phase**: Fase 5 — Productización. **User story**: HU-002 / HU-004 (source-branch selection).

## Decisions

**D1 — Dedupe local vs remote-tracking inside `CandidateBranches` only.** Collapse local `X`
and `origin/X` into one candidate (keep bare/local name) via a new unexported helper called
at the END of `CandidateBranches`. A single candidate auto-resolves in `SelectSingleSource`
case 1 (ignores `selected`), so this alone fixes the bug. Minimal, surgical; distinct
branches untouched.
- **Hard constraint**: MUST NOT dedupe in shared `branchesByPattern`/`ListBranches`. The
  standalone-delta picker (`standaloneBranchesCmd`) requires local+remote as separate rows
  and is tested for it (`standalone_delta_test.go:33-53`, `standalone_modes_e2e_test.go:175-192`).

**D2 — Prefer the current branch, gated by an EXPLICIT confirm prompt.** When the user is
checked out on a candidate and no pipeline suggestion applies (first-env target), DeployDeck
PROMPTS ("¿Usar la rama actual '<X>' como origen? [s/N]") before using it — a NEW TUI confirm
state, not silent selection nor a passive label. Chosen over the exploration's silent-inference
recommendation to avoid surprise from ambient checkout.
- Reuse `m.originalBranch` (captured pre-discovery via `originalBranchCmd`); NO new git/exec
  call — `internal/app` stays off the exec seam. Guard detached HEAD (`!= "" && != "HEAD"`)
  like `quitCmd`.
- Pipeline suggestion (`SuggestDefaultSource`, re-promotion) keeps priority.
- Single-source invariant preserved: flow still funnels through unmodified `SelectSingleSource`.

## Scope

**In scope**: dedupe helper + `CandidateBranches`; `resolveSource` gains a `currentBranch`
input; new confirm-prompt TUI state + key/view; `discoverCmd` threads `m.originalBranch`;
full test suite (below).

**Out of scope (follow-up change)**: a full source-branch PICKER for genuinely-distinct
multiple candidates not resolvable by current-branch inference. **Known residual gap**:
genuinely ambiguous with no current-branch match still dead-ends silently.

## Capabilities

### New Capabilities
None.

### Modified Capabilities
- `commit-discovery`: "Candidate Branch Search By Name" gains local/remote-tracking dedupe;
  NEW requirement — current branch preferred as source via an explicit confirm prompt when no
  pipeline suggestion applies; pipeline suggestion and single-source enforcement unchanged.

## Testing (strict TDD, `go test ./...`)
- Git unit: local+remote same branch → 1 candidate; over-aggression guard (distinct names → 2);
  `ListBranches`-not-deduped guard.
- NEW `internal/app/flow_test.go`: `resolveSource` current-branch preference; HEAD / absent /
  not-a-candidate fallbacks; pipeline-priority regression.
- Git e2e: repro (non-empty `OrderedCommits`) + distinct-candidates scenario.
- App e2e: full flow reaches `StateCommitSelection` with `len(m.items) > 0`.
- Confirm-prompt TUI state exercised via `Model.Update`.

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| Dedupe leaks into `ListBranches` | Med | Scope to `CandidateBranches`; standalone tests guard |
| Confirm state complicates the flow | Med | Small isolated state; resolved in sdd-design |
| Unpushed-only local candidate still hard-fails | Low (pre-existing) | Out of scope; not worsened |

## Rollback Plan

Purely additive/behavioral in source resolution: no shared Git state mutation, no Salesforce,
no schema. **Rollback = revert the commit.** The new TUI state is reachable only in the
previously-dead-end ambiguous path.

## Success Criteria
- [ ] Repro promotion (local+origin same branch, first-env target) reaches commit selection
      with a non-empty range.
- [ ] User on a candidate branch is prompted and can confirm/decline using it as source.
- [ ] Pipeline suggestion and standalone-delta picker behavior unchanged (green tests).

## Residual design questions (for sdd-design)
- Exact placement of the confirm-prompt state in the discovery → selection flow (before or
  after `discoverCmd` resolves; how "[s/N]" routes back into `resolveSource`).
- Where the current-branch-among-candidates match is computed (inside `resolveSource` vs a
  pre-check that gates the prompt), given the prompt must fire BEFORE final selection.
