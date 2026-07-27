# Delta for Cherry-Pick

Closes the HU-006 reopen/resume deferral noted in the main spec's Design Notes ("Run persistence (resume after closing and reopening) is intentionally deferred to HU-013 and out of scope for this change", `cherry-pick/spec.md:142`). At archive time, replace that line with a pointer to `run-resume`, which decides WHEN to route here; this capability defines what the conflict screen accepts once entered. No existing requirement's behavior changes.

## ADDED Requirements

### Requirement: Resumed Entry Accepts Rehydrated Conflict Context

The cherry-pick conflict screen SHALL accept entry either from a freshly started sequence or from a resumed run, and SHALL display the resumed run's ticket, pick index/total, and current commit identically to a fresh sequence. Once entered, the existing live re-polling and continue-gating requirements apply unchanged regardless of entry path.

#### Scenario: Resumed run shows ticket and pick N of M
- GIVEN `run-resume` routes into `StateCherryPickConflict` for a run with a persisted ticket, `PickIndex`, and `PickTotal`
- WHEN the conflict screen renders
- THEN it shows the ticket and "pick N of M" matching the resumed record (HU-013 AC `docs/HISTORIAS.md:846`)

#### Scenario: Resumed conflict screen behaves like a fresh one
- GIVEN a resumed conflict screen with unresolved paths
- WHEN the user attempts to continue
- THEN continue is disabled per the existing unresolved-state gating, same as a freshly started sequence
