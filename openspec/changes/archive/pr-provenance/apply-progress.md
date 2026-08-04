# Apply Progress: PR Provenance (verifiable DeployDeck-created marker)

**Mode**: Strict TDD
**Status**: 27/27 tasks complete. All 6 phases done in a single batch (no prior apply-progress existed).

## TDD Cycle Evidence

| Task | RED (test written first) | GREEN (implementation passes) | REFACTOR |
|---|---|---|---|
| 1.1-1.4 | `internal/provenance/provenance_test.go` + `boundary_test.go` written against a non-existent package; `go test ./internal/provenance/...` failed to compile (`undefined: secret`, `undefined: Sign`, ...) | `internal/provenance/provenance.go` created (`Sign`/`RenderFooter`/`RenderMarker`/`ParseMarker`/`Verify`/`Compose`/`Result` enum); `go test ./internal/provenance/... -race -v` — all 9 top-level tests + subtests PASS | None needed — implementation matched the design contract on first GREEN |
| 2.1 | Added `TestOwnerRepo`/`TestParsePRURL` to `compare_url_test.go`; `go test ./internal/github/... -run "OwnerRepo\|ParsePRURL"` failed to compile (`undefined: github.OwnerRepo`, `undefined: github.ParsePRURL`) | Added `OwnerRepo`/`ParsePRURL`/`prURLPattern` to `compare_url.go`; same run — all cases PASS | None needed |
| 2.3 | `internal/github/pr_details_test.go` created (4 cases: success, gh missing, non-zero exit, malformed JSON); failed to compile (`c.PRDetails undefined`) | Added `PRDetails` to `Client` interface + `client` impl + `prDetails` struct in `client.go`; `go test ./internal/github/... -run PRDetails` — all 4 PASS | None needed |
| 3.1-3.2 | `internal/app/create_pr_provenance_test.go` created (4 tests); compiled fine (symbols already existed from Phase 1-2) but FAILED on assertion — unmodified `createPRCmd` still sent the bare `effectiveDescription()`/empty body (captured in the RED run's failure output below) | Modified `createPRCmd` in `commands.go` to derive `ownerRepo` via `github.OwnerRepo(m.originURL)` and call `provenance.Compose(...)` before `gh.CreatePR`; all 4 new tests + the full `internal/app` suite (incl. 3 pre-existing files whose body-literal assertions needed updating to the new spec-mandated body shape) PASS | None needed |
| 4.1 | `cmd/deploydeck/pr_verify_test.go` created (14 test functions/subtests); failed to compile (`undefined: verifyFn`, `undefined: runPRVerify`) | Created `cmd/deploydeck/pr.go` (`newPRCmd`/`newPRVerifyCmd`/`runPRVerify`/`exitError`/`verifyFn` seam); registered in `main.go` + `exitCodeFor` dispatch; `go test ./cmd/deploydeck/... -race -v` — all PASS | None needed |

### RED confirmation transcripts (representative)

```
$ go test ./internal/provenance/... -race -v
internal/provenance/provenance_test.go:17:13: undefined: secret
...
FAIL	github.com/malavolta/DeployDeck/internal/provenance [build failed]

$ go test ./internal/github/... -race -run "OwnerRepo|ParsePRURL" -v
internal/github/compare_url_test.go:123:22: undefined: github.OwnerRepo
internal/github/compare_url_test.go:157:22: undefined: github.ParsePRURL
FAIL	github.com/malavolta/DeployDeck/internal/github [build failed]

$ go test ./internal/app/... -race -run "CreatePR|Provenance" -v
--- FAIL: TestCreatePRCmd_Provenance_AIAcceptedPath
    create_pr_provenance_test.go:52: createPRCmd should compose body via
    provenance.Compose(effectiveDescription(), ownerRepo, head, runID), calls:
    [{gh [pr create --base UAT --head deploy/PROJ-1-to-UAT --title
    PROJ-1 - Promote changes to UAT --body AI drafted description] ...}]
(4 assertion failures against the unmodified createPRCmd)

$ go test ./cmd/deploydeck/... -race -run "PRVerify" -v
cmd/deploydeck/pr_verify_test.go:50:13: undefined: verifyFn
cmd/deploydeck/pr_verify_test.go:64:10: undefined: runPRVerify
FAIL	github.com/malavolta/DeployDeck/cmd/deploydeck [build failed]
```

## Work Unit Evidence

| Unit | Focused test command | Exact result | Runtime harness | Rollback boundary |
|---|---|---|---|---|
| 1: `internal/provenance` + `internal/github` primitives | `go test ./internal/provenance/... ./internal/github/... -race -v` | PASS — 9 provenance test funcs (+ subtests) + `TestOwnerRepo`/`TestParsePRURL`/`TestClient_PRDetails_*` (4) all green | N/A — pure logic + `FakeRunner`-backed github tests, no real `gh`/network | Revert `internal/provenance/` (new dir) + the `OwnerRepo`/`ParsePRURL`/`PRDetails` additions to `internal/github/{compare_url,client}.go` — zero callers existed before Phase 3 |
| 2: `internal/app` `createPRCmd` composition | `go test ./internal/app/... -race -run "CreatePR\|Provenance"` then full `go test ./internal/app/... -race` | Focused: PASS (7 test funcs). Full package: PASS (`TestApp_NeverImportsExecSeam` included) | N/A — `FakeRunner`-backed `github.Client`, matches `push_preparation_test.go`'s existing pattern | Revert `createPRCmd`'s `ownerRepo`/`provenance.Compose` call (~10 lines) in `commands.go`, plus the body-literal updates in the 3 pre-existing test files — `gh.CreatePR` and the exec boundary stay untouched |
| 3: `cmd/deploydeck pr verify` + build/CI wiring | `go test ./cmd/deploydeck/... -race -v` | PASS — 14 new test funcs/subtests + all pre-existing cmd/deploydeck tests green | Cobra `Execute()` end-to-end test (`TestNewPRVerifyCmd_Execute_PropagatesExitCodeViaExitError`) drives the REAL `RunE` (real `github.New(exec.NewOSRunner())` composed, never invoked — `ParsePRURL` fails first, so no real `gh`/network touched); `goreleaser check` unavailable locally (not installed) — `.goreleaser.yaml`/`release.yml` validated as well-formed YAML (`ruby -ryaml`) instead, `ci.yml`'s existing goreleaser job covers the rest on the next CI run | Delete `cmd/deploydeck/pr.go` + `pr_verify_test.go`, revert `main.go`'s `newPRCmd()` registration + `exitCodeFor`/dispatch change, revert the 2 build/CI/doc files — independent of Units 1-2 |

## Deviations from Design

1. **Task 3.1's "sig bound...returns Verified" sub-case is NOT literally reachable inside `internal/app`'s test binary.** `provenance.secret` is package-private and injected only via release ldflags; every `go test` run (no ldflags) has `secret == ""`, and `provenance.Verify` checks `secret == ""` (→ `DevVerifier`) BEFORE ever reaching a byte comparison — so `Verified`/`Mismatch` are architecturally unreachable from outside the `provenance` package without a real secret. This is intentional design behavior (a dev-built verifier must never assert authenticity), not a gap to work around with test hacks. Resolution: the cryptographic binding itself (owner/repo lowering, branch/runID case-exactness, wrong-repo/branch/runID → Mismatch) is exhaustively covered with a REAL test secret in Phase 1's `internal/provenance/provenance_test.go` (`TestSign_PayloadCanonicalization`, `TestVerify`). `internal/app`'s test instead proves WIRING correctness: an exact-match assertion against an independently-computed `provenance.Compose(...)` call (which is sensitive to a wrong `runID`, since `runID` is the one component still visible in the marker text even under the dev sentinel), plus a dedicated `TestCreatePRCmd_Provenance_RunIDWiring` varying `m.runID` across two models.
2. **Same root cause reappears in task 4.1** (`cmd/deploydeck`'s `runPRVerify`): with `secret == ""` in every `go test` run, the real `provenance.Verify` can only ever return `DevMarker` (sig literally `"dev"`) or `DevVerifier` (any other sig, since `secret == ""`) — `Verified` and `Mismatch` are both unreachable through the genuine crypto path. Resolution: added a package-private `verifyFn = provenance.Verify` seam in `cmd/deploydeck/pr.go`, overridable only from `pr_verify_test.go` (same package), used ONLY for the `Verified`/`Mismatch`/cross-repo-mismatch test cases. The `NoMarker`/`DevSignedMarker`/`DevBuiltVerifier`/`Degraded` cases all exercise the REAL `provenance` + `github` functions with no seam involved — those ARE naturally reachable given `secret == ""`. `runPRVerify`'s public signature/behavior is unchanged from design.md; `verifyFn` is a pure internal test seam.
3. **Updated 3 pre-existing test files outside the literal task list scope** (`internal/app/push_preparation_test.go`, `push_preparation_ai_test.go`, `push_pr_e2e_test.go`): none of these set `m.originURL`, so under the new `createPRCmd` they all degrade to a footer-only (or description+footer) body instead of the historical bare/empty body. This is a REQUIRED consequence of the push-pr-preparation spec's own MODIFIED requirement ("the submitted body SHALL always end with the visible footer... on BOTH the AI-accepted and non-AI paths... the non-AI path... default"), not a scope violation — leaving them unfixed would have left the full-suite gate (task 6.4) permanently red.
4. **`goreleaser` is not installed in this environment** — task 5.4's primary verification (`goreleaser check`) could not run locally. Substituted a YAML well-formedness check (`ruby -ryaml`, since the local `python3` lacks a `yaml` module) on both modified files, consistent with the task's own documented fallback ("otherwise note that `ci.yml`'s existing `goreleaser check`/`--snapshot` job validates the new ldflag on the next PR").

None of these deviations change any public signature, exit-code mapping, spec scenario, or payload/marker format from design.md — they are test-strategy adaptations forced by `secret` being intentionally unexported and ldflags-only.

## Final Gate Results

```
$ PATH=/usr/local/go/bin:$PATH go build ./...
(clean, exit 0)

$ PATH=/usr/local/go/bin:$PATH go vet ./...
(clean, exit 0)

$ gofmt -l internal cmd
(empty output)

$ PATH=/usr/local/go/bin:$PATH go test ./... -race -count=1
ok  	github.com/malavolta/DeployDeck/cmd/deploydeck	6.102s
ok  	github.com/malavolta/DeployDeck/internal/ai	2.190s
ok  	github.com/malavolta/DeployDeck/internal/app	31.869s
ok  	github.com/malavolta/DeployDeck/internal/config	3.125s
ok  	github.com/malavolta/DeployDeck/internal/delta	10.315s
ok  	github.com/malavolta/DeployDeck/internal/exec	2.692s
ok  	github.com/malavolta/DeployDeck/internal/git	64.288s
ok  	github.com/malavolta/DeployDeck/internal/github	1.600s
ok  	github.com/malavolta/DeployDeck/internal/prereq	5.428s
ok  	github.com/malavolta/DeployDeck/internal/provenance	3.617s
ok  	github.com/malavolta/DeployDeck/internal/runs	2.367s
ok  	github.com/malavolta/DeployDeck/internal/salesforce	1.973s
ok  	github.com/malavolta/DeployDeck/internal/update	2.042s
ok  	github.com/malavolta/DeployDeck/internal/version	1.580s

$ go test ./internal/app/... ./internal/provenance/... -race -run "NeverImports" -v
--- PASS: TestApp_NeverImportsExecSeam
--- PASS: TestProvenance_NeverImportsExecNetGithub
```

## Files Touched

| File | Action | Phase |
|---|---|---|
| `internal/provenance/provenance.go` | Created | 1 |
| `internal/provenance/provenance_test.go` | Created | 1 |
| `internal/provenance/boundary_test.go` | Created | 1 |
| `internal/github/compare_url.go` | Modified (`OwnerRepo`, `ParsePRURL`, `prURLPattern`) | 2 |
| `internal/github/compare_url_test.go` | Modified (`TestOwnerRepo`, `TestParsePRURL`) | 2 |
| `internal/github/client.go` | Modified (`PRDetails` on `Client` + `client`, `prDetails` struct) | 2 |
| `internal/github/pr_details_test.go` | Created | 2 |
| `internal/app/commands.go` | Modified (`createPRCmd` composes `provenance.Compose`) | 3 |
| `internal/app/create_pr_provenance_test.go` | Created | 3 |
| `internal/app/push_preparation_test.go` | Modified (body-literal assertions updated) | 3 |
| `internal/app/push_preparation_ai_test.go` | Modified (body-literal assertions updated) | 3 |
| `internal/app/push_pr_e2e_test.go` | Modified (body-literal assertions updated) | 3 |
| `cmd/deploydeck/pr.go` | Created (`newPRCmd`, `newPRVerifyCmd`, `runPRVerify`, `exitError`, `verifyFn`) | 4 |
| `cmd/deploydeck/pr_verify_test.go` | Created | 4 |
| `cmd/deploydeck/main.go` | Modified (`newPRCmd()` registered, `exitCodeFor` + exit-code-aware `main()`) | 4 |
| `.goreleaser.yaml` | Modified (`provenance.secret` ldflag) | 5 |
| `.github/workflows/release.yml` | Modified (`PROVENANCE_SECRET` env) | 5 |
| `docs/ACTIVATION-CHECKLIST.md` | Modified (documented new secret prerequisite) | 5 |

## Status

27/27 tasks complete. Ready for verify.
