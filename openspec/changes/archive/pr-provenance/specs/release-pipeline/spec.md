# Delta for Release Pipeline

## ADDED Requirements

### Requirement: Release Builds Inject The Provenance Secret

Release builds (triggered by a `v*` tag) MUST inject
`internal/provenance.secret` via goreleaser `-ldflags -X`, sourced from a
CI secret. Snapshot/dev/CI builds MUST carry the zero-value (empty) secret
and therefore produce only dev-signed markers, never a verifiable HMAC
signature.

(Sibling of "Injected Version Matches Build ldflags"; verification: CI
release job)

#### Scenario: Release build signs verifiably
- GIVEN a release build injects a non-empty provenance secret via ldflags
  from the CI secret
- WHEN a PR is created and later checked with `deploydeck pr verify <url>`
- THEN the marker's signature was computed with the injected secret and
  verification succeeds

#### Scenario: Snapshot build emits a dev marker
- GIVEN a snapshot/CI build with no provenance secret injected
- WHEN a PR body is signed
- THEN the marker carries `sig:dev`, and `deploydeck pr verify <url>`
  reports it as dev-signed/unverifiable
