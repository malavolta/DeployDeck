# Tasks: HU-019 — DeployDeck Release Pipeline

Capabilities: `release-versioning`, `update-notification`, `release-pipeline`.
Design validated SOUND (fresh-context). Strict TDD: every Go behavior = RED
(failing test) immediately followed by GREEN. 5 ordered groups, each
`go build ./... && go vet ./...` + its own `go test` slice, independently
green. Real org NOT needed. Real network NOT needed (httptest only).
`goreleaser` NOT installed locally — Group E is CI-validated, no local test.
`internal/app` stays free of `net/http`/`internal/version`/`internal/update`
(scalar `CheckUpdate`).

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | ~750-900 (mostly additions: 2 new packages + tests, `internal/app` wiring + tests, CI/release YAML, README, checklist) |
| 400-line budget risk | High |
| Chained PRs recommended | Yes |
| Suggested split | PR 1 (Group A) → PR 2 (Group B) → PR 3 (Group C) → PR 4 (Group D) → PR 5 (Group E) |
| Delivery strategy | ask-on-risk (session default; not overridden in preflight) |
| Chain strategy | pending — suggest `stacked-to-main` (each group is additive/degrade-safe until Group D wires them; ask maintainer to confirm) |

Decision needed before apply: Yes
Chained PRs recommended: Yes
Chain strategy: pending
400-line budget risk: High

### Suggested Work Units

| Unit | Goal | Likely PR | Focused test command | Runtime harness | Rollback boundary |
|------|------|-----------|----------------------|-----------------|-------------------|
| 1 | `internal/version` + `--version` wiring | PR 1 | `go test ./internal/version/... ./cmd/deploydeck/... -run Version` | N/A — pure unit test, no real org/network | delete `internal/version/`; revert the `root.Version=` line in `newRootCmd` |
| 2 | `internal/update` (`Checker`, `HasNewer`) | PR 2 | `go test ./internal/update/...` | N/A — `httptest.Server` stands in for GitHub, no real network | delete `internal/update/` (unreferenced until PR 4) |
| 3 | `internal/app` update-check wiring + banner | PR 3 | `go test ./internal/app/... -run "CheckUpdate|Init|View"` | N/A — `Model.Update()`/fakes, no TUI harness needed | revert `Deps.CheckUpdate`/`Init`/`Update`/`View` edits; nil `CheckUpdate` already degrades |
| 4 | `main.go` composition (`defaultCheckUpdate` + wiring) | PR 4 | `go test ./cmd/deploydeck/... -run DefaultCheckUpdate` | N/A — canceled-context test proves the seam without a live GitHub call | revert `defaultCheckUpdate` + its one-line `Deps{}` wiring |
| 5 | CI/release/docs infra | PR 5 | N/A — CI-validated only | GitHub Actions `ci.yml` goreleaser job (`goreleaser release --snapshot --clean` asserting `dist/`) | delete `.goreleaser.yaml`, both workflow files, `README.md`, `docs/ACTIVATION-CHECKLIST.md` — no code dependency |

## Phase 1: `internal/version` + `--version` wiring (Group A)

- [x] 1.1 RED — `internal/version/version_test.go`: table test `TestString` — default (`Version="dev"`) → `String()=="dev"`; injected (`Version="1.2.3",Commit="abc",Date="2026-01-01"`) → `String()` contains `"1.2.3"`, `"abc"`, `"2026-01-01"`. Fails: package doesn't exist. → `release-versioning`: Version Variables Package (both scenarios).
- [x] 1.2 GREEN — `internal/version/version.go`: `var (Version = "dev"; Commit = ""; Date = "")` **string vars** (ldflags-settable, not const) + `func String() string` (`"dev"` when `Version=="dev"`, else formatted with Commit/Date). `go test ./internal/version/...` green.
- [x] 1.3 RED — `cmd/deploydeck/root_test.go`: add `TestNewRootCmd_Version_Default` — `cmd := newRootCmd(Deps{})`, `cmd.SetOut(&buf)`, `cmd.SetArgs([]string{"--version"})`, `Execute()` → buf contains `"dev"`. Add `TestNewRootCmd_Version_Injected` (no `t.Parallel`) — save+`defer` restore `version.Version`, set `version.Version="9.9.9"` **before** `newRootCmd(...)`, same SetOut/SetArgs/Execute → buf contains `"9.9.9"`. Fails: `root.Version` unset today (Cobra's `--version` is inert). → `release-versioning`: CLI Version Exposure.
- [x] 1.4 GREEN — `cmd/deploydeck/main.go`: in `newRootCmd`, add `root.Version = version.String()` (import `deploydeck/internal/version`). `go test ./cmd/deploydeck/... -run Version` green.
- [x] 1.5 Group A verification — `go build ./... && go vet ./... && go test ./internal/version/... ./cmd/deploydeck/...`

## Phase 2: `internal/update` (Group B)

- [x] 2.1 RED — `internal/update/hasnewer_test.go`: table test `TestHasNewer` — cases: newer(`1.0.0`,`1.1.0`)→true; equal→false; older→false; `v`-prefix(`v1.0.0`,`v1.1.0`)→true; current=`"dev"`→false; malformed latest/current (e.g. `"not-a-version"`)→false. Fails: `update.HasNewer` doesn't exist. → `update-notification`: Semver-Based Newer-Version Detection + Silent Skip discipline (malformed/`"dev"`→false).
- [x] 2.2 GREEN — `internal/update/hasnewer.go`: `func HasNewer(current, latest string) bool` — strip leading `v`, split `MAJOR.MINOR.PATCH`, int-compare; any parse error or `current=="dev"` → `false` (never panics). `go test ./internal/update/... -run HasNewer` green.
- [x] 2.3 RED — `internal/update/checker_test.go`: `TestChecker_Latest_Success` — `httptest.NewServer` returns `200 {"tag_name":"v1.2.3"}`; `Checker{BaseURL:srv.URL,HTTPClient:srv.Client()}.Latest(ctx)` → `("v1.2.3", nil)`. `TestChecker_Latest_NonOK` — server returns `401` → non-nil err. `TestChecker_Latest_Timeout` — handler sleeps past a short `context.WithTimeout` → non-nil err (proves the timeout path, no hang). Fails: `Checker`/`Latest` don't exist. → `update-notification`: Silent Skip on Check Failure (timeout/non-2xx/401) + Non-Blocking Startup Check (bounded).
- [x] 2.4 GREEN — `internal/update/checker.go`: `type Checker struct{BaseURL string; HTTPClient *http.Client}`, `func (c Checker) Latest(ctx) (string, error)` — GET `BaseURL+"/repos/"+ownerRepo+"/releases/latest"`, decode `{tag_name}`, non-2xx → error. Add `const ownerRepo = "OWNER/REPO" // PLACEHOLDER — set at activation checklist step (a)`. `go test ./internal/update/...` green.
- [x] 2.5 Group B verification — `go build ./... && go vet ./... && go test ./internal/update/...`

## Phase 3: `internal/app` non-blocking wiring (Group C)

- [x] 3.1 RED — `internal/app/update_check_test.go` (`package app`): `TestCheckUpdateCmd_NilDeps_ReturnsNil` — `New(Deps{}).checkUpdateCmd()` → `nil`. Fails to compile: no `Deps.CheckUpdate` field, no `checkUpdateCmd` method, no `updateCheckTimeout` const. → `update-notification`: Non-Blocking Startup Check (nil-degrades, design ADR-2).
- [x] 3.2 GREEN — `internal/app/app.go`: add `CheckUpdate func(ctx context.Context) (bool, string, error)` to `Deps`. `internal/app/commands.go`: add `const updateCheckTimeout = 3 * time.Second`, `type updateCheckDoneMsg struct{hasUpdate bool; latest string; err error}`, stub `func (m Model) checkUpdateCmd() tea.Cmd` returning `nil` when `deps.CheckUpdate==nil`. `go test ./internal/app/... -run CheckUpdateCmd_NilDeps` green.
- [x] 3.3 RED — same file: `TestCheckUpdateCmd_FedFake_YieldsDoneMsg` — `m:=New(Deps{CheckUpdate: func(ctx)(bool,string,error){return true,"9.9.9",nil}})`; `cmd:=m.checkUpdateCmd()`; require non-nil; `cmd()` → `updateCheckDoneMsg{hasUpdate:true, latest:"9.9.9", err:nil}`. Fails: stub still returns nil unconditionally. → `update-notification`: Non-Blocking Startup Check (3s `context.WithTimeout`).
- [x] 3.4 GREEN — `checkUpdateCmd` body: wraps `context.WithTimeout(context.Background(), updateCheckTimeout)` inside the returned closure, calls `m.deps.CheckUpdate(ctx)`, returns `updateCheckDoneMsg{...}`. `go test ./internal/app/... -run CheckUpdateCmd` green.
- [x] 3.5 RED — `TestInit_Batches_PrereqAndCheckUpdate` — fake `NewChecker`+fake `CheckUpdate` both set → `m.Init()` non-nil, invoking it yields a `tea.BatchMsg` carrying both commands. Nil-`CheckUpdate` case (only `NewChecker` set) → `Init()` still non-nil (existing prereq flow unaffected). Fails: `Init()` still returns only `m.runPrereqCmd()`. → `update-notification`: Non-Blocking Startup Check (concurrent with startup).
- [x] 3.6 GREEN — `internal/app/app.go`: `func (m Model) Init() tea.Cmd { return tea.Batch(m.runPrereqCmd(), m.checkUpdateCmd()) }`. `go test ./internal/app/... -run Init` green.
- [x] 3.7 RED — same file: `TestOnUpdateCheckDone_HasUpdate_SetsFields` — feed `updateCheckDoneMsg{hasUpdate:true,latest:"9.9.9"}` into `m.Update(...)` → returned `Model.updateAvailable==true`, `updateLatest=="9.9.9"`. `TestOnUpdateCheckDone_ErrOrNoUpdate_NoOp` — subtests `{err:someErr}` and `{hasUpdate:false}` → `updateAvailable` stays `false`, `m.state` unchanged (startup not gated). Fails: no `updateCheckDoneMsg` case in `Update()`, no `updateAvailable`/`updateLatest` fields. → `update-notification`: Silent Skip on Check Failure (identical no-branch treatment) + Semver-Based Newer-Version Detection.
- [x] 3.8 GREEN — `internal/app/app.go`: add `updateAvailable bool`, `updateLatest string` to `Model`. `internal/app/update.go`: add `case updateCheckDoneMsg: return m.onUpdateCheckDone(msg)`; `onUpdateCheckDone`: `if msg.err!=nil || !msg.hasUpdate { return m, nil }`, else set both fields. `go test ./internal/app/... -run OnUpdateCheckDone` green.
- [x] 3.9 RED — same file: `TestView_UpdateBanner_ShownWhenAvailable` — construct `Model{state: StateTicketInput, updateAvailable:true, updateLatest:"9.9.9"}` (white-box, `package app`), `m.View()` contains `"9.9.9"` AND the underlying ticket-input content. `TestView_NoBanner_WhenUnavailable` — same with `updateAvailable:false` → output does NOT contain `"9.9.9"`. Fails: `View()` never prepends a banner today. → `update-notification`: Semver-Based Newer-Version Detection (notice shown/not shown).
- [x] 3.10 GREEN — `internal/app/view.go`: rename the existing switch body to `func (m Model) viewBody() string {...}` (byte-identical), add `func (m Model) View() string { return m.updateBanner() + m.viewBody() }` and `func (m Model) updateBanner() string { if !m.updateAvailable { return "" }; return fmt.Sprintf("A newer DeployDeck (%s) is available\n", m.updateLatest) }`. `go test ./internal/app/...` full green (32 existing `View()` callers keep passing since `viewBody` is verbatim + banner defaults empty).
- [x] 3.11 RED — `internal/app/boundary_test.go` (`package app_test`): extend `TestApp_NeverImportsExecSeam`'s `forbidden` slice with `"net/http"`, `"deploydeck/internal/version"`, `"deploydeck/internal/update"`. Run immediately after 3.1-3.10: if it fails, Group C's `CheckUpdate` wiring leaked a forbidden import and must be fixed before GREEN. → design ADR-2 import-cycle discipline (mandatory note 2). Mechanism proven via a throwaway `_ "net/http"` blank-import file: test failed for the right reason, file removed.
- [x] 3.12 GREEN — confirm `go test ./internal/app/... -run TestApp_NeverImportsExecSeam` passes, proving the scalar `Deps.CheckUpdate func(ctx)(bool,string,error)` signature (3.1-3.10) never pulled `net/http`/`internal/version`/`internal/update` into `internal/app`.
- [x] 3.13 Group C verification — `go build ./... && go vet ./... && go test ./internal/app/...`

## Phase 4: `main.go` composition (Group D)

- [x] 4.1 RED — `cmd/deploydeck/main_test.go` (new): `TestDefaultCheckUpdate_DegradesOnError` — `ctx, cancel := context.WithCancel(context.Background()); cancel()` (pre-canceled, no real network hit), `defaultCheckUpdate(ctx)` → `(false, "", err!=nil)`. Fails: `defaultCheckUpdate` doesn't exist. → `update-notification`: Silent Skip on Check Failure, composition seam only (real network not called in tests).
- [x] 4.2 GREEN — `cmd/deploydeck/main.go`: add `func defaultCheckUpdate(ctx context.Context) (bool, string, error)` — builds `update.Checker{BaseURL:"https://api.github.com", HTTPClient:&http.Client{}}`, calls `Latest(ctx)`, on error returns `false,"",err`, else `update.HasNewer(version.Version, latest), latest, nil`; add `context`/`net/http`/`deploydeck/internal/update`/`deploydeck/internal/version` imports. `go test ./cmd/deploydeck/... -run DefaultCheckUpdate` green. (Implementation extracts the pure comparison into an unexported `decideUpdate(latest string, fetchErr error) (bool, string, error)` helper so the branch/comparison logic is triangulated with `TestDecideUpdate` — err / newer / equal / older / `"dev"` cases — without an independent seam beyond what the design's `defaultCheckUpdate` contract specifies.)
- [x] 4.3 GREEN (mechanical, no new test — compile-verified transitively by 4.1/4.2) — wire `CheckUpdate: defaultCheckUpdate` into the `app.Deps{}` literal in `defaultRunTUI` (`cmd/deploydeck/main.go:216-239`); `root.Version` already set in Group A. → `release-versioning`/`update-notification`: real composition wiring (no independent seam beyond 4.1/4.2 — `defaultRunTUI` launches a real `tea.Program`, matching the existing untested pattern for its other `Deps` fields).
- [x] 4.4 Group D verification — `go build ./... && go vet ./... && go test ./cmd/deploydeck/... ./internal/update/... ./internal/version/... ./internal/app/...`

## Phase 5: CI/release/docs (Group E — CI-validated, NO local test)

No local test signal exists for this group (`goreleaser` not installed; no `actionlint`-equivalent). Every task below is verified ONLY by its CI job — do not invent a local goreleaser/actionlint test.

- [x] 5.1 CI-validated, no local test — Create `.goreleaser.yaml`: builds macOS amd64+arm64/Linux amd64+arm64/Windows amd64; `ldflags: -s -w -X deploydeck/internal/version.Version={{.Version}} -X deploydeck/internal/version.Commit={{.Commit}} -X deploydeck/internal/version.Date={{.Date}}`; archives tar.gz (+zip on Windows); checksums; `brews:`/`scoops:` stanzas with a clearly-commented **PLACEHOLDER** owner/repo, activation-gated. Verification: `goreleaser check` (CI job). → `release-pipeline`: goreleaser Configuration Validity + Injected Version Matches Build ldflags; AC2's "formula/manifest actually updated" marked documented-not-exercised.
- [x] 5.2 CI-validated, no local test — Create `.github/workflows/ci.yml`: job 1 (`pull_request`+`push`) `setup-go@1.26`, `go build ./...`, `go vet ./...`, `go test ./... -race`; job 2 runs `goreleaser check` + `goreleaser release --snapshot --clean`, asserting `dist/` contains the macOS/Linux/Windows binaries + Homebrew formula + Scoop manifest, no publish. → `release-pipeline`: CI Build/Vet/Test on Every PR + Snapshot Build Produces Versioned Multi-Platform Artifacts.
- [x] 5.3 CI-validated, no local test — Create `.github/workflows/release.yml`: on `push: tags: v*`, checkout + `setup-go` + goreleaser-action, write-token secret **PLACEHOLDER** (activation-gated — will only truly publish once `origin` + aux repos + token exist). → `release-pipeline`: Real Publication Activation Prerequisites (documented, not exercised); AC3 marked documented-not-exercised.
- [x] 5.4 CI-validated, no local test — Create root `README.md`: install (`go install .../deploydeck@vX` placeholder path), update-check/banner behavior, `deploydeck --version` usage. → `release-versioning`/`update-notification` doc coverage.
- [x] 5.5 CI-validated, no local test — Create `docs/ACTIVATION-CHECKLIST.md`: (a) real module path once `origin` exists; (b) create `homebrew-tap`/`scoop-bucket` repos; (c) provision + wire the write-token secret; (d) public/private repo decision + install-docs impact; (e) cut the first real semver tag. Explicitly flag AC2's "actually updated" clause and all of AC3 as **infra-gated, documented-not-exercised** — never silently marked done. → `release-pipeline`: Real Publication Activation Prerequisites.
- [x] 5.6 CI-validated, no local test — Modify `openspec/config.yaml` `testing.ci.scope` addendum reflecting the new `ci.yml` (build/vet/test + goreleaser check/snapshot on every PR; real e2e stays local-only/opt-in, unchanged).
- [x] 5.7 Group E verification — CI job only: `ci.yml` build/vet/test + `goreleaser check` + `goreleaser release --snapshot --clean` (asserts `dist/`). No local `go build`/`go vet`/`go test` gate applies beyond what Groups A-D already cover.

## Review Remediations (adversarial)

Strict TDD (RED→GREEN, confirmed right-reason failure) for F1-F2; YAML-only
hardening for F3-F4 (no local Go test — validated by parsing + manual read).
Full suite (`go build ./... && go vet ./... && go test ./... -race -count=1`)
stayed green throughout; `internal/app` untouched,
`TestApp_NeverImportsExecSeam` still passes.

- [x] F1 [LOW code] `internal/update/checker.go` — `Checker.HTTPClient` nil
  panicked the calling goroutine instead of degrading to an error.
  RED: `TestChecker_Latest_NilHTTPClient` (`internal/update/checker_test.go`)
  reproduced `panic: runtime error: invalid memory address or nil pointer
  dereference` at `checker.go:48` (`c.HTTPClient.Do(req)`), confirmed
  right-reason before the fix. GREEN: added a nil-`HTTPClient` guard at the
  top of `Latest` returning `fmt.Errorf("update: no HTTP client
  configured")`.
- [x] F2 [LOW code] `internal/update/checker.go` — response body decode was
  unbounded, allowing a hostile/huge body to exhaust memory.
  RED: `TestChecker_Latest_LargeBody` streamed a JSON body whose padding
  field alone is 2 MiB; before the fix `Latest` decoded it fully with no
  error (confirmed the vulnerability, not just a assertion typo). GREEN:
  wrapped the decoder in `io.LimitReader(resp.Body, maxResponseBodyBytes)`
  with `maxResponseBodyBytes = 1 << 20` (1 MiB); the truncated body now fails
  to decode and `Latest` returns an error. Existing
  `TestChecker_Latest_Success`/`_NonOK`/`_Timeout` stayed green (small bodies
  decode fine under the cap).
- [x] F3 [MED config] `.github/workflows/ci.yml` (2 occurrences) and
  `.github/workflows/release.yml` (1 occurrence) — `goreleaser-action`
  `version: latest` was non-reproducible/future-fragile. Changed to
  `version: "~> v2"` in both files (tracks the v2 line, not bleeding edge).
  No local test; YAML re-parsed successfully after the edit
  (`ruby -ryaml -e "YAML.load_file(...)"`). CI-validated only.
- [x] F4 [LOW config] `.github/workflows/release.yml` — an accidental `v*`
  tag push would publish real GitHub-release binaries (with the placeholder
  `<OWNER>` in `.goreleaser.yaml`) before failing on the missing
  homebrew-tap/scoop-bucket repos, contradicting the prior header comment's
  implied fail-closed framing. (a) Rewrote the header comment to state
  accurately that goreleaser builds/uploads real release artifacts first and
  only fails later on the brews/scoops publish steps. (b) Added
  `if: vars.RELEASE_ACTIVATED == 'true'` on the `release` job, commented and
  pointing at `docs/ACTIVATION-CHECKLIST.md`, so the workflow stays inert
  until that repository variable is explicitly flipped at activation. YAML
  re-parsed successfully after the edit. The `vars.*` gating semantics are
  GitHub-Actions-runtime only and were NOT exercised by any real workflow
  run — this is a documented, not locally-verifiable, control.
