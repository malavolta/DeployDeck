# Exploration — pr-provenance

Mark DeployDeck-created PRs with a verifiable provenance signature so anyone can tell a PR was
created by the tool (user goal: a minimal restriction that is hard to replicate manually, accepted
that a motivated attacker can extract the secret from the binary). Architecture agreed with the
user before exploration: PR body gains a visible footer + an invisible HTML comment
`<!-- deploydeck: v1 run:<RunID> sig:<sig> -->` with
`sig = HMAC-SHA256(secret, "owner/repo|headBranch|runID")[:16 hex]`; new pure package
`internal/provenance` (Sign/RenderMarker/ParseMarker/Verify, `var secret` injected via release
`-ldflags -X`); new `github.Client.PRDetails`; new `deploydeck pr verify <url>` subcommand.
Payload deliberately excludes headSHA so incremental-promotion appends never invalidate the
signature. Verified against the code.

## Current state

1. **PR body composition site** — `internal/app/commands.go:1462-1476` (`createPRCmd`). The
   closure does `body := m.effectiveDescription()`; title via `effectiveTitle()`
   (`internal/app/app.go:672-677`), body via `effectiveDescription()` (`app.go:688-693`, returns
   `m.aiDescription` if `m.aiAccepted`, else `""`). This is the SINGLE site where body reaches
   `gh.CreatePR` — AI and non-AI paths converge here. The closure captures `gh, base, head,
   title, body, ctx`; it does NOT yet capture `m.runID` or `m.originURL` (both already exist on
   `Model`).
2. **owner/repo availability** — `internal/github/compare_url.go:38-56`. `CompareURL` formats a
   full URL; the parser `parseOrigin` (line 49) is UNEXPORTED and returns `(host, org, repo, ok)`.
   Nothing today yields a bare `"owner/repo"` string. `m.originURL` (`app.go:452`) is populated
   pre-`pushReady` by `preparePRCmd` (`commands.go:1319-1324`, via `git.Service.RemoteURL`), so
   the raw origin URL is in scope; a new exported helper over `parseOrigin` is needed.
3. **RunID availability** — `m.runID` is set well before push/PR: `internal/app/update.go:593`
   (new run) and `:671` (incremental collision path, SAME RunID — asserted by
   `incremental_promotion_e2e_test.go:118` and `branch_collision_test.go:307`).
   `MarkPRCreated(m.runID, msg.url)` at `update.go:1222-1223` proves `m.runID` is live through
   `createPRCmd`.
4. **`github.Client` interface + fakes** — `internal/github/client.go:26-55`: `AuthStatus`,
   `CreatePR`, `PRForBranch`. No dedicated fake struct exists — all tests build a real
   `github.New(exec.NewFakeRunner())` and can `gh` responses on the FakeRunner (e.g.
   `internal/app/push_preparation_test.go:147,236,...`). Adding `PRDetails` = interface method +
   impl + canned `gh pr view` responses; no fake-struct surface.
5. **cmd subcommand pattern** — `cmd/deploydeck/main.go:55-78` (`newRootCmd`) wires
   `newDoctorCmd(deps)` and `newRunsCmd()`. `newRunsCmd` (83-91) is the template: needs nothing
   from `Deps`, builds its own dependencies inline (mirroring `github.New(exec.NewOSRunner())` at
   `main.go:219`/`:281`). NOTE: `delta`/`validate` "standalone modes" are TUI states, not cobra
   subcommands. Cobra-level tests live in `cmd/deploydeck/root_test.go` (`cmd.SetArgs`/`Execute`).
6. **Version + ldflags precedent** — `internal/version/version.go:17,21,24` (`Version`, `Commit`,
   `Date`). `.goreleaser.yaml:36-40` carries the `-X .../internal/version.*` ldflags block — the
   new `-X .../internal/provenance.secret={{ .Env.* }}` line goes there. Secret env injection:
   `.github/workflows/release.yml:46-60` (goreleaser-action `env:` block, currently
   `GITHUB_TOKEN`/`HOMEBREW_TAP_TOKEN`/`SCOOP_TOKEN`). CI snapshot job (`ci.yml:58-63`) sets no
   env — snapshot/dev builds naturally get the package zero-value secret (the distinguishable dev
   value).
7. **Marker collision risk** — none. `CreatePR` passes `body` verbatim as a discrete exec arg.
   The AI cap `maxDescriptionChars=4000` (`internal/ai/parse.go:15,142`) applies BEFORE the
   description lands in `m.aiDescription` — appending the marker afterward is never clipped.
   `maxUserPromptChars` bounds only the outbound AI prompt.
8. **Existing spec surface** — `push-pr-preparation`: Requirement "PR Creation Requires Explicit
   Confirmation And Records The URL" is MODIFIED (body gains footer+marker). `release-pipeline`:
   provenance-secret injection needs a NEW requirement (closest existing: "Injected Version
   Matches Build ldflags", version-only). `openspec/specs/pr-provenance/` does not exist — brand
   new capability.

## Key decisions for propose/design

- Export the `owner/repo` derivation from `compare_url.go`'s `parseOrigin` instead of duplicating
  the SSH/HTTPS regex set in `internal/provenance` — parsing stays single-sourced in
  `internal/github` (pure string logic; no exec involved).
- `createPRCmd` captures `runID`/`originURL` and calls pure `internal/provenance` to build the
  final body — the exec boundary is untouched.
- `pr verify <url>` derives owner/repo from the URL ARGUMENT itself, not from
  `gh pr view --json headRepository*` (those report the fork's repo on cross-fork PRs, not the
  base repo the marker was signed against).
- `PRDetails` requests only `headRefName,body` — minimal, stable field set.
- `pr verify` wires its own `github.New(exec.NewOSRunner())` in `cmd/deploydeck` (runs-cmd
  pattern); no `app.Deps` involvement.

## Risks

- No existing helper yields bare `owner/repo`; adding one touches stable, tested
  `compare_url.go`/`compare_url_test.go`.
- The incremental "reuse & append" path SKIPS `createPRCmd` when an open PR exists
  (`PRForBranch` short-circuit, `commands.go:1312-1317`) — the marker is written ONCE at original
  creation and never refreshed. RunID is stable across that lifecycle so `pr verify` still
  validates; design must not assume per-push body regeneration.
- No gh minimum-version check exists; `gh pr view --json` field mismatch must degrade like
  `PRForBranch`'s exit-code-as-data tolerance (`client.go:119-123`), not crash.
- `effectiveDescription()` returns `""` on the non-AI path — the footer+marker becomes the FIRST
  non-empty default PR body this flow ever produced; confirm footer-only body is intended UX.
- Dev/snapshot builds get a zero-value secret — design must define what Sign/Verify do with an
  empty secret (distinguishable dev marker vs signing with empty key).
- `internal/update/checker.go:19` has an unrelated hardcoded `ownerRepo` placeholder for the
  self-update checker — separate concern, do not conflate.

Artifact store: OpenSpec (Engram MCP unavailable).
