# Design: Local-Model PR Title/Description Suggestion (`ai-pr-summary`)

## Technical Approach

New `internal/ai` package is the sole `net/http` boundary (mirrors `internal/update/checker.go`).
`internal/app` reaches it ONLY through a plain-typed scalar `Deps.GenerateSummary` (like
`Deps.CheckUpdate`); `internal/prereq.Checker.AI` holds the typed `ai.Client` directly (like `GH`).
At `pushReady`, an explicit context-sensitive `a` keypress requests, then a second `a` accepts, an
OpenAI-compatible chat completion drafted from data already on `Model` (no new git/HTTP in app).
Everything degrades to "no suggestion; flow unchanged" (mirrors `CheckGH`/`checkUpdateCmd`).

## Architecture Decisions

### ADR-1: Scalar `Deps.GenerateSummary` signature (resolves Q1)
**Choice**: `func(ctx context.Context, ticket string, commitSubjects []string, componentSummary string) (title, description string, err error)` — confirm the proposed shape unchanged.
**Alternatives**: passing `delta.PackageSummary` or an `ai.SummaryRequest`.
**Rationale**: only std/plain types → leak-free, mirrors `CheckUpdate`'s scalar seam and passes
`boundary_test`. `componentSummary` is pre-rendered inside `internal/app` from `m.summary`
(`delta.PackageSummary`, already imported there), so `internal/ai` needs no `internal/delta`
dependency either. `ai.SummaryRequest`/`ai.SummaryResult` stay behind the closure in `main.go`.

### ADR-2: Labeled-line response contract + lenient parser (resolves Q2)
**Choice**: instruct the model to emit two labels — `TITLE:` and `DESCRIPTION:` — NOT JSON.
**Alternatives**: JSON envelope.
**Rationale**: small models routinely emit invalid JSON (code fences, unescaped quotes, trailing
prose). `parse.go` `ParseSummary(raw) (title, description string, ok bool)`:
1. strip markdown fences / decoration, normalize newlines;
2. case-insensitively find a line whose trimmed form starts with `TITLE:` → title = remainder,
   **first line only**, control chars stripped, length-capped (≤120);
3. find `DESCRIPTION:` → description = text to end/next label, trimmed, capped (chars + lines);
4. **title empty/absent → `ok=false` → empty result → no suggestion**. Description may be empty
   (title-only suggestion allowed). Never surface partial/garbled text. Any transport/timeout/
   malformed/oversized error → `ok=false` equivalent (empty title, `err` swallowed in app).

### ADR-3: Accepted title is one source of truth everywhere (resolves Q3)
**Choice**: on accept, the AI title replaces `github.SuggestedTitle` in ALL slots — the `title:`
display line, the `gh pr create` preview string, the compare-URL manual-copy block, AND the actual
`--title` argument. A `Model.effectiveTitle()` helper returns `aiTitle` when `aiAccepted`, else
`github.SuggestedTitle(...)`; `viewPRData` and `createPRCmd` both call it.
**Alternatives**: override only the executed `--title`.
**Rationale**: prevents the displayed command from drifting from the executed one. The AI
**description is display-only** this slice — `CreatePR` forces `--body ""`; body injection is a
separate change (out of scope). Pre-accept, every slot is byte-for-byte unchanged.

### ADR-4: `internal/ai` shape mirrors `internal/update` discipline
`Client` interface: `GenerateSummary(ctx, SummaryRequest) (SummaryResult, error)` and
`Doctor(ctx) DoctorState` (total, enum `Unreachable|ModelMissing|Ready`, CheckGH-shaped).
Concrete `client{Endpoint, Model string; HTTPClient *http.Client}`; `New(endpoint, model string, hc *http.Client) Client`.
Both methods: nil-`HTTPClient` guard, `http.NewRequestWithContext`, caller-set timeout,
`io.LimitReader(body, 1<<20)`. `GenerateSummary` → `POST {endpoint}/v1/chat/completions`
`{model, messages:[system,user], stream:false}`, decode `choices[0].message.content` → `ParseSummary`.
`Doctor` → `GET {endpoint}/v1/models`, decode `data[].id`, membership of configured `Model`.
`prompt.go` `Build(ticket, commitSubjects, componentSummary) (system, user string)` with truncation
caps (max N subjects, per-subject length, total char budget) — pure, table-tested.

### ADR-5: `AIConfig` zero-value-safe (like `QuickDeployConfig`)
`AIConfig{Endpoint, Model string; Enabled bool}` on `Config.AI yaml:"ai"`. No defaulting entry
(omitted → `Enabled=false` → off). `Validate`: `Enabled=true` requires non-empty `Endpoint`+`Model`.

### ADR-6: `CheckAI` informative, `StatusWarning`-only (copies `CheckGH`)
Nil `Checker.AI` → `StatusOK` "skipped". Else switch `Doctor(ctx)`: `Ready`→OK; `ModelMissing`→
Warning; `Unreachable`→Warning. Never `StatusBlocking`.

### ADR-7: nil-dep disables the affordance; on-demand context-sensitive `a`
`main` wires `Deps.GenerateSummary` and `Checker.AI` ONLY when `cfg.AI.Enabled` — else both nil
(nil-degrades convention). `a` at `pushReady`: no suggestion & dep present → request; suggestion
present & unaccepted → accept; `aiPending` or already accepted → inert. Two explicit presses gate
the title into `gh pr create`.

## Data Flow / Sequence (push-prep AI)

```
pushReady ──'a'(1st)──▶ aiPending=true ──▶ aiSuggestCmd
  (guard: deps.GenerateSummary!=nil, !aiPending, !aiAccepted)      │ ctx=WithTimeout
                                                                   │ ticket, subjects(m.plan.SelectedCommits[].Commit.Subject),
                                                                   │ componentSummary(render m.summary)
                                                                   ▼
main closure ─▶ ai.Client.GenerateSummary ─POST /v1/chat/completions─▶ local model
  (only place net/http runs)                                       │
                                                                   ▼ choices[0].message.content
                                                              ParseSummary
                                                                   ▼
onAISuggestDone{title,description,err} ─▶ aiPending=false
   err|title=="" ──▶ aiErr set, NO suggestion (silent, flow unchanged)
   title!=""     ──▶ aiTitle/aiDescription set (proposed, NOT accepted)
                                                                   ▼
pushReady ──'a'(2nd)──▶ aiAccepted=true      effectiveTitle() ⇒ aiTitle
                                                                   ▼
'g' ─▶ pushPRConfirm ─'y'─▶ createPRCmd(--title effectiveTitle())  ← accepted title only here
```

## File Changes

| File | Action | Description |
|------|--------|-------------|
| `internal/ai/{client,prompt,parse}.go` | Create | Sole `net/http` boundary; interface + HTTP impl, prompt build+truncation, lenient parser |
| `internal/config/config.go` | Modify | `AIConfig` block + `Config.AI` field |
| `internal/config/validate.go` | Modify | `Enabled=true` ⇒ non-empty `Endpoint`+`Model` |
| `internal/prereq/checker.go` | Modify | `AI ai.Client` field |
| `internal/prereq/checker_ai.go` | Create | `CheckAI` (nil-skip 3-state, warning-only) |
| `internal/prereq/checker_check.go` | Modify | register `CheckAI` |
| `internal/app/app.go` | Modify | `Deps.GenerateSummary`; Model `aiTitle/aiDescription/aiAccepted/aiPending/aiErr`; `effectiveTitle()` |
| `internal/app/commands.go` | Modify | `aiSuggestCmd`; `createPRCmd` title ⇒ `effectiveTitle()` |
| `internal/app/update.go` | Modify | `onAISuggestDone` reducer |
| `internal/app/keys.go` | Modify | context-sensitive `a` at `pushReady` |
| `internal/app/view.go` | Modify | render AI block; `viewPRData` title ⇒ `effectiveTitle()` |
| `internal/app/boundary_test.go` | Modify | add `internal/ai` to `forbidden` |
| `cmd/deploydeck/main.go` | Modify | compose `ai.New` once; wire typed `Checker.AI` + scalar closure when `Enabled` |

## Testing Strategy

| Layer | What | Approach |
|-------|------|----------|
| Unit | `prompt.Build` + truncation caps | table-driven, pure |
| Unit | `ParseSummary`: labeled/fenced/partial/newline-title/`--`-prefixed/garbled | raw-text fixtures |
| Unit | `AIConfig` validation (omitted valid/off; `Enabled` requires endpoint+model) | mirrors `validate_test.go` |
| Unit | Degradation: nil `Deps.GenerateSummary`, nil `Checker.AI` | mirrors `checker_gh_test.go`/`checkUpdateCmd` |
| E2E (hermetic, CI, `-race -short`) | generate→parse→suggest + 3 `CheckAI` states | `httptest.Server` canned OpenAI JSON: success / unreachable / timeout(short ctx) / malformed / oversized-body cap; doctor: reachable+listed / reachable+missing / unreachable |
| Integration (TUI) | `keyPushPreparation` request→accept via `Model.Update` with fake dep; title reaches `--title` ONLY after 2nd `a` | mirrors `push_preparation_test.go` |
| Real-model e2e (opt-in, self-skips CI) | real local server, non-empty conventional-commit title | env `DEPLOYDECK_E2E_AI_ENDPOINT`/`DEPLOYDECK_E2E_AI_MODEL` (single gate, like `real_org_e2e_test.go`); example model `qwen2.5-coder:3b` |

## Threat Matrix

| Boundary | Cases | Applicability | Design response | RED tests |
|---|---|---|---|---|
| Documentation-like paths | — | N/A: no file classification | — | — |
| Git repository selection | — | N/A: reuses `m.plan`, no new git call | — | — |
| Commit state | — | N/A: no commit op | — | — |
| Push state | — | N/A: no push op | — | — |
| PR commands | `--title` value; `--`-prefixed / newline / oversized title | **Applicable**: AI title → `gh pr create --title` | Title is untrusted model output: parser forces single line, strips control chars, caps length; passed as a discrete `Args` slice element after `--title` (never a shell, no flag re-parse); reaches gh ONLY after explicit 2nd `a` accept | Parser test: newline/`--`/oversized title truncated-or-rejected; TUI test: `--title` == accepted title, single arg, only post-accept |

Process-integration note: the endpoint is user-owned config (trust boundary), so SSRF is out of
scope; response safety (`io.LimitReader` cap, context timeout, malformed→degrade) is covered by the
hermetic e2e.

## Migration / Rollout

No migration. Purely additive, default-off (`ai` block absent → `Enabled=false`). Rollback = revert;
the `AIConfig` block and the `boundary_test` forbidden entry are inert when the feature is unused.

## Open Questions

- [ ] None blocking. (Body injection into `gh pr create --body` deferred to a future slice.)
