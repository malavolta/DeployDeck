# Design: PR Provenance (verifiable DeployDeck-created marker)

## Technical Approach

A new PURE leaf `internal/provenance` owns all crypto + string logic (HMAC, footer, marker
grammar, verify classification) behind five primitives plus a thin `Compose`. `createPRCmd`
(`internal/app/commands.go:1462`) captures the already-live `m.runID`/`m.originURL`, derives
`owner/repo` through a new exported `github.OwnerRepo` wrapper over `parseOrigin`, and calls
`provenance.Compose` to append footer+marker to `effectiveDescription()` — the exec boundary
(`gh.CreatePR`) is untouched and `TestApp_NeverImportsExecSeam` still holds (app imports the pure
`provenance`, never exec/version). Verification lives entirely in `cmd/deploydeck`: a new
`deploydeck pr verify <url>` derives `owner/repo` from the URL argument, fetches `headRefName`+`body`
via a new `github.Client.PRDetails`, parses the marker, and re-runs `provenance.Verify`. Maps to
proposal D1–D9.

## Architecture Decisions

| Decision | Choice | Rejected | Rationale |
|---|---|---|---|
| Provenance home | Pure `internal/provenance` (`crypto/hmac`, `crypto/sha256`, `encoding/hex`, `regexp`, `internal/version`); NO exec/net/github imports, guarded by a package boundary test | crypto inline in app; provenance imports github for parsing | keeps app exec-free (D3); a github import would pull `exec` transitively and break purity |
| Payload canonicalization | `sig = HMAC-SHA256(secret, lower(host/owner/repo)+"\n"+headBranch+"\n"+runID)`, `hex[:16]` | raw-case owner/repo; include headSHA; bare (non-host-qualified) owner/repo; `\|`-joined fields | owner/repo is GitHub-case-insensitive so lowercasing both sides kills URL-vs-remote casing false-negatives; branch/runID stay case-exact; headSHA excluded so incremental appends stay valid (D1). **Remediation:** the payload MUST include the host — a bare `owner/repo` lets a marker signed for github.com verify unchanged on an Enterprise host sharing the same org/repo/branch/runID. Fields are joined with `"\n"`, not `"\|"` — `\|` is legal in a git branch name (and the marker's own `\S+`-matched runID), so a `\|`-joined payload is not injective across a field boundary (`Sign(o,"a\|b","c") == Sign(o,"a","b\|c")`); `\n` cannot occur in a git ref name, so the newline-joined payload stays injective |
| Dev sentinel | `secret==""` ⇒ `Sign` returns literal `"dev"` (never a real HMAC); marker renders `sig:dev` | sign with empty key; omit marker on dev | an empty-key HMAC is publicly forgeable and indistinguishable from release — dev builds must be explicitly unverifiable (D4) |
| Marker grammar | `<!-- deploydeck: v1 run:(\S+) sig:([0-9a-f]{16}\|dev) -->`; `ParseMarkers` returns EVERY match, in document order (`[]Marker{RunID,Sig}`) | free-form; JSON-in-comment; first-match-only `ParseMarker` | `\S+` runID (git branches carry no spaces), strict sig alternation, `v1` scheme tag reserves evolution (D2). **Remediation:** first-match parsing let a stale/prepended marker mask the genuine one; `ParseMarkers` returns all matches so the caller classifies every one (any-of) instead of trusting whichever comes first |
| Footer version | `RenderFooter()` sources `internal/version.Version` internally (`vX.Y.Z`; `dev`→"(dev build)") | pass version through app | app must not import `internal/version` (boundary test); provenance is the pure owner (D2) |
| owner/repo helpers | `github.OwnerRepo(originURL)` (thin wrapper over `parseOrigin`) for creation; SEPARATE `github.ParsePRURL(prURL)` for `pr verify`; BOTH return HOST-QUALIFIED `"host/owner/repo"` | one helper for both; bare `owner/repo` return | a PR URL `https://host/owner/repo/pull/N` has a different anchored shape than an origin remote — `parseOrigin`'s `$`-anchored regexes reject it (D7/D8). **Remediation:** both helpers were dropping the already-captured host group; a marker signed via `OwnerRepo`'s bare `owner/repo` and re-verified via `ParsePRURL`'s bare `owner/repo` would collapse two different hosts to the same string, defeating the host-binding the payload now requires |
| Compose sanitize | `Compose` strips every pre-existing deploydeck marker from the incoming body (`markerPattern.ReplaceAllString` + blank-line collapse) BEFORE appending its own footer+marker | trust the incoming body as marker-free | an AI-generated description can echo a prior marker verbatim; without stripping, Compose would ship TWO markers with the stale one first, and first-match verification would report a genuine PR as forged. "Exactly one marker" is now unconditional, not merely the common case |
| PRDetails degrade | `gh pr view <url> --json headRefName,body`; Runner error OR exit≠0 OR bad JSON ⇒ `err` (never panic) | exit≠0⇒nil like `PRForBranch` | for verify, an unfetchable PR is a genuine degrade, not a create-fallthrough; caller maps any err to the degraded exit code. Conscious refinement of proposal D6's "like `PRForBranch`" wording: D6's intent — non-crashing degrade — holds; `PRForBranch`'s specific exit≠0-as-data mapping exists to fall through to create, which has no analog in verify |
| Verify outcomes | enum `Verified\|Mismatch\|DevMarker\|DevVerifier`; `pr verify` maps to distinct exit codes; ANY-OF classification across every `ParseMarkers` result (Verified > Dev > Mismatch); an unrecognized/future Result guards to exit 4, never exit 1 | 0-vs-1 only; first-match-only classification; unrecognized Result silently treated as Mismatch | distinguishing forgery from environmental (dev/degraded) lets CI fail on forgery yet skip on gh-missing — worth a ~4-line exit-code-aware branch in `main()`. **Remediation:** first-match classification let a prepended forged/stale marker discredit or mask a genuine one; any-of classification (a genuine "verified" always outranks noise elsewhere in the body) fixes both directions. The `default:` case in the exit-code switch is now an explicit guard for an out-of-range `Result`, distinct from the explicit `case Mismatch:` |
| Secret injection | goreleaser ldflag `-X …provenance.secret={{ envOrDefault "PROVENANCE_SECRET" "" }}` | `{{ .Env.PROVENANCE_SECRET }}` (hard fail) | `.Env` errors when unset — that breaks the CI snapshot job (no secret) which must compile to the empty/dev secret; `envOrDefault` defaults snapshot→dev, real release injects the secret |

## Data Flow

    Creation (internal/app):
      createPRCmd: capture m.runID, m.originURL, head=m.plan.PromotionBranch, body=effectiveDescription()
        github.OwnerRepo(originURL) ─→ ownerRepo (HOST-QUALIFIED "host/owner/repo")
        provenance.Compose(body, ownerRepo, head, runID)
          stripMarkers(body)                (sanitize: remove any pre-existing marker first)
          RenderFooter()                    (visible)
          Sign(ownerRepo, head, runID) → RenderMarker(runID, sig)   (invisible; skipped if ownerRepo=="")
        → gh.CreatePR(base, head, title, bodyWithFooterMarker)   [exec boundary, unchanged]

    Verify (cmd/deploydeck pr verify <url>):
      ParsePRURL(url) ─→ ownerRepo (HOST-QUALIFIED "host/owner/repo")
      github.New(exec.NewOSRunner()).PRDetails(ctx, url) ─→ headBranch, body   (err → exit 4)
      provenance.ParseMarkers(body) ─→ []Marker{RunID,Sig}   (empty → exit 2)
      bestVerifyResult: provenance.Verify(ownerRepo, headBranch, m.RunID, m.Sig) for EVERY marker, any-of
        Verified→0   DevMarker/DevVerifier→3   Mismatch→1   unrecognized Result→4

## File Changes

| File | Action | Description |
|---|---|---|
| `internal/provenance/provenance.go` | Create | `var secret` + `Sign`/`RenderFooter`/`RenderMarker`/`ParseMarkers`/`Verify`/`Compose`; `Result` enum; `Marker` struct |
| `internal/provenance/provenance_test.go` | Create | RED unit coverage (below) |
| `internal/provenance/boundary_test.go` | Create | RED: `build.ImportDir` forbids `os/exec`, `internal/exec`, `net/http`, `internal/github` |
| `internal/github/compare_url.go` | Modify | add exported `OwnerRepo(originURL)` over `parseOrigin`; add `ParsePRURL(prURL)` + PR-URL regex; BOTH host-qualified |
| `internal/github/compare_url_test.go` | Modify | RED `OwnerRepo` + `ParsePRURL` cases (host-qualified) |
| `internal/github/client.go` | Modify | add `PRDetails` to `Client` interface + `client` impl + `prDetails` JSON struct |
| `internal/github/pr_details_test.go` | Create | RED `PRDetails` via `FakeRunner` |
| `internal/app/commands.go` | Modify | `createPRCmd` captures `runID`/`originURL`; derive `ownerRepo`; `provenance.Compose` body |
| `internal/app/create_pr_provenance_test.go` | Create | RED body-append (AI / non-AI empty / marker-exactly-once / payload binding) |
| `cmd/deploydeck/pr.go` | Create | `newPRCmd`+`newPRVerifyCmd`+testable `runPRVerify(w, gh, url) int`+`bestVerifyResult`/`verifyResultRank` (any-of classification)+`exitError` |
| `cmd/deploydeck/main.go` | Modify | register `newPRCmd()`; exit-code-aware branch in `main()` |
| `cmd/deploydeck/pr_verify_test.go` | Create | RED cobra `Execute` + `runPRVerify` via `FakeRunner`-backed client; any-of + exhaustive Result-mapping cases |
| `.goreleaser.yaml` | Modify | provenance.secret ldflag (`envOrDefault`) beside the version ldflags |
| `.github/workflows/release.yml` | Modify | `PROVENANCE_SECRET: ${{ secrets.PROVENANCE_SECRET }}` in the goreleaser env |
| `docs/ACTIVATION-CHECKLIST.md` | Modify | list `PROVENANCE_SECRET` as a release prerequisite (fail-soft: dev markers if unset) |

## Interfaces / Contracts

```go
// internal/provenance  (PURE — no exec, no net)
var secret string // ldflags-injected at release only

func Sign(ownerRepo, headBranch, runID string) string        // 16-hex HMAC, or "dev" when secret=="" — ownerRepo MUST be host-qualified
func RenderFooter() string                                   // "Created with DeployDeck vX.Y.Z" (version pkg)
func RenderMarker(runID, sig string) string                  // <!-- deploydeck: v1 run:<runID> sig:<sig> -->
func ParseMarkers(body string) []Marker                      // EVERY v1 marker, in order; nil when absent/foreign
func Verify(ownerRepo, headBranch, runID, sig string) Result
func Compose(body, ownerRepo, headBranch, runID string) string // sanitizes pre-existing markers, then body+footer+marker; ownerRepo=="" ⇒ footer only

type Marker struct { RunID, Sig string }

type Result int
const ( Verified Result = iota; Mismatch; DevMarker; DevVerifier ) // sig=="dev"→DevMarker; secret==""→DevVerifier

// internal/github
func OwnerRepo(originURL string) (ownerRepo string, ok bool)  // HOST-QUALIFIED "host/owner/repo" via parseOrigin
func ParsePRURL(prURL string) (ownerRepo string, ok bool)     // HOST-QUALIFIED "host/owner/repo" from https://host/owner/repo/pull/N
PRDetails(ctx context.Context, url string) (headBranch, body string, err error) // gh pr view <url> --json headRefName,body

// cmd/deploydeck
func runPRVerify(w io.Writer, gh github.Client, url string) int // 0 verified,1 mismatch,2 missing,3 dev,4 degraded/unrecognized
func bestVerifyResult(ownerRepo, headBranch string, markers []provenance.Marker) (result provenance.Result, runID string) // any-of: Verified > Dev > Mismatch
```

`pr verify` exit codes: `0` verified (ANY parsed marker verifies) · `1` mismatch (forged / copied to another
repo·branch·host, and no marker verifies) · `2` no marker parses at all · `3` dev (a marker carries `sig:dev`,
OR the verifying binary is a dev build — and no marker verifies) · `4` degraded (gh absent/unauth, PRDetails
error, unparseable URL) OR an unrecognized/future Result value (never routed through the Mismatch/forgery
message). CI reads: `0`=pass, `1`/`2`=genuine fail, `3`/`4`=environmental/skip. Classification is ANY-OF
across every `ParseMarkers` result, not first-match: a genuine "verified" always outranks noise (a stale
echoed marker, or a bad-faith prepended forgery) elsewhere in the body.

## Testing Strategy (strict TDD — RED first)

| Layer | What | Test file |
|---|---|---|
| Unit | `Sign` determinism · `hex[:16]` truncation+lowercase · payload canon (host/owner/repo lowered, branch/runID exact, `\n` seps injective even when a field contains `\|`) · dev sentinel | `internal/provenance/provenance_test.go` |
| Unit | `RenderMarker`↔`ParseMarkers` roundtrip (single + multiple, in order) · absent marker→nil slice · foreign/`v2`/garbage→nil slice | same |
| Unit | `RenderFooter`: injected semver → `Created with DeployDeck vX.Y.Z` · default `Version=="dev"` → dev-build variant | same |
| Unit | `Verify`: Verified · Mismatch (tampered sig / wrong repo / wrong host, same org·repo·branch·runID / wrong branch / wrong runID) · DevMarker · DevVerifier | same |
| Unit | `Compose`: body+footer+marker · sanitizes a pre-existing marker so exactly one genuine marker ships and verifies · empty body→footer+marker · ownerRepo==""→footer only · marker count==1 | same |
| Arch | provenance imports none of exec/net/github | `internal/provenance/boundary_test.go` |
| Unit | `OwnerRepo`/`ParsePRURL` return HOST-QUALIFIED `host/owner/repo` (ssh/https/ssh-url, enterprise host, arbitrary host, casing preserved; junk→ok=false) | `internal/github/compare_url_test.go` |
| Integration | `PRDetails` via `FakeRunner`: success · gh missing (Runner err) · exit≠0 · malformed JSON — all non-panic | `internal/github/pr_details_test.go` |
| Unit | `createPRCmd`: AI path (body+footer+marker) · non-AI empty path (footer+marker only) · marker exactly once · sig bound to host/owner/repo·head·runID | `internal/app/create_pr_provenance_test.go` |
| Arch | `TestApp_NeverImportsExecSeam` still green after provenance import | `internal/app/boundary_test.go` (existing) |
| Cmd | `pr verify` via cobra `Execute` + `runPRVerify` core (FakeRunner gh): verified→0, forged→1, no marker→2, dev→3, gh degraded→4, unrecognized Result→4 (exhaustive table) | `cmd/deploydeck/pr_verify_test.go` |
| Cmd | any-of classification: prepended bogus/dev marker before a genuine one ⇒ genuine still wins (exit 0); only-bogus ⇒ 1; only-dev ⇒ 3 | `cmd/deploydeck/pr_verify_test.go` |
| Cross-repo/host | sign repoA/branchX/hostA, verify against repoB or branchY or a DIFFERENT host (same org/repo/branch/runID) ⇒ Mismatch (exit 1) | provenance + pr_verify tests |

## Threat Matrix

| Boundary | Applicability | Design response | Planned RED test |
|---|---|---|---|
| Git repository selection | N/A: provenance is pure; verify never touches a git repo (URL + `gh` only) | — | — |
| PR commands | Applicable: new `gh pr view <url> --json headRefName,body` | discrete-slice args, no shell, no env-prefix; exec confined to `internal/github`; `CreatePR` unchanged | `pr_details_test.go` |
| Secret handling | Applicable: HMAC key via ldflags at release only | `var secret` zero-value in source/dev/snapshot; `Sign` never emits a real HMAC with an empty key; secret never logged/echoed | dev-sentinel + goreleaser `envOrDefault` |
| Marker spoofing | Applicable: hand-copied / hand-forged / hand-prepended marker | payload binds HOST+owner/repo+branch+runID (newline-joined, injective) ⇒ copy elsewhere — including a same-org/repo/branch/runID copy to a DIFFERENT host — fails; 16-hex HMAC deters hand-forgery; `Verify`→Mismatch; `ParseMarkers`+any-of classification means a prepended forged/stale marker can never mask or discredit a genuine one elsewhere in the body | Verify Mismatch + cross-repo/branch/host tests + any-of classification tests |
| Degrade paths | Applicable: gh missing/unauth, bad JSON, unparseable URL, unrecognized future `Result` | `PRDetails` returns err (never panic); `pr verify` maps to exit 4 with a clear message; an out-of-range `Result` guards to exit 4, never the Mismatch/forgery message | `pr_details_test.go` + `pr_verify_test.go` degrade cases + exhaustive Result-mapping table |

## Migration / Rollout

No migration, no schema/`SchemaVersion` change. Additive at the single composition site and a new
subcommand. `.goreleaser.yaml`/`release.yml` gain the secret ldflag; the release workflow stays
activation-gated (`RELEASE_ACTIVATED`) and snapshot/CI builds compile to the empty/dev secret via
`envOrDefault`. Revert = drop the `Compose` call + `pr verify`; existing markers become harmless
unverifiable text. `PROVENANCE_SECRET` is documented as an activation prerequisite — if unset at a
real release the failure mode is fail-soft (release binaries emit dev markers), never a build crash.

## Open Questions

- [x] Persist the computed `sig` on the run record for later `runs` cross-check? **No (YAGNI).** The
  sig is recomputable from owner/repo+branch+runID+secret and `pr verify` reads it from the live PR;
  no consumer justifies a schema field. A future `runs show` can recompute or read the PR.
- [x] Does `pr verify` need `--json`/`--quiet` for CI? **No.** Distinct exit codes ARE the machine
  contract; stdout carries the human message. `--json` can be added later without breaking codes.
- [x] Confirm the pinned goreleaser v2 exposes `envOrDefault`. **Confirmed by the orchestrator:**
  `release.yml` pins `goreleaser-action@v7` with `version: "~> v2"`, and `envOrDefault` has existed
  since goreleaser v1.14 — the ldflag template is valid and `ci.yml` needs no changes.
