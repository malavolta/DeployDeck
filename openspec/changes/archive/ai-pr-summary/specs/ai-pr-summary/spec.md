# AI PR Summary Specification

## Purpose

At `pushReady` (`StatePushPreparation`), DeployDeck MAY draft a
conventional-commit PR title and description using an OPTIONAL local,
HTTP-reachable model, from the ticket's commit subjects
(`m.plan.SelectedCommits`) and the `PackageSummary` metadata-delta counts
already on `Model`. This closes HU-014's deferred "Idea Futura"
(`docs/HISTORIAS.md:966-985`): the suggestion is additive, opt-in,
on-demand, and never auto-submitted, alongside the existing
`github.SuggestedTitle` formula.

## Requirements

### Requirement: AI Configuration Is Optional And Zero-Value-Safe

The system SHALL treat an absent `ai` configuration block as `Enabled=false`
(feature off), and, WHEN `enabled: true` is set, SHALL require non-empty
`endpoint` and `model`, failing config validation otherwise.

#### Scenario: Absent ai block is safe and off
- GIVEN no `ai` block is present in `deploydeck.yaml`
- WHEN configuration loads
- THEN the AI-suggestion feature is disabled and no defaulting error occurs

#### Scenario: enabled=true without endpoint or model fails validation
- GIVEN `ai: { enabled: true }` with an empty `endpoint` or `model`
- WHEN configuration is validated
- THEN validation fails

### Requirement: No AI Config Leaves The Push/PR Flow Unchanged

The system MUST NOT offer the AI-suggestion affordance, and the push/PR
preparation flow MUST behave identically to the flow without this feature,
when no `ai` configuration is present or `Enabled` is `false`.

#### Scenario: Disabled AI leaves pushReady unchanged
- GIVEN `ai` is absent or `enabled: false`
- WHEN the user reaches `pushReady` and completes push/PR preparation
- THEN the screen and flow match pre-existing HU-014 behavior exactly, with
  no AI-suggestion affordance shown

### Requirement: On-Demand Suggestion Generation

The system SHALL generate a PR title/description suggestion ONLY in
response to an explicit user keypress at `pushReady` (never automatically
after push), composed from the ticket's commit subjects and the
`PackageSummary` counts already on `Model`.

#### Scenario: Explicit keypress triggers generation
- GIVEN AI is configured and reachable
- WHEN the user explicitly requests a suggestion at `pushReady`
- THEN a title and description are generated from the commit subjects and
  metadata-delta counts

#### Scenario: Suggestion never auto-fires after push
- GIVEN a push just completed successfully
- WHEN `pushReady` is reached
- THEN no AI generation request is made until the user explicitly requests
  one

### Requirement: Explicit Accept Overrides Only The PR-Creation Title Source

The system SHALL require a distinct explicit accept action, separate from
the request action, before an AI-suggested title is used, and SHALL apply
an accepted title ONLY to the `gh pr create --title` argument's source.

#### Scenario: Accepting overrides the PR-creation title source
- GIVEN a suggestion has been generated
- WHEN the user explicitly accepts it
- THEN the accepted title becomes the source for `gh pr create --title`

#### Scenario: Ignoring the suggestion keeps the formula title
- GIVEN a suggestion has been generated but not accepted
- WHEN the user proceeds without accepting
- THEN `github.SuggestedTitle` remains the title source

### Requirement: Suggestion Is Never Auto-Submitted

The system MUST NOT create or submit a PR using an AI-suggested title
without both the explicit accept action and the pre-existing explicit
PR-creation confirmation gate.

#### Scenario: Accepted suggestion still requires PR-creation confirmation
- GIVEN the user has explicitly accepted an AI-suggested title
- WHEN PR creation has not yet been explicitly confirmed
- THEN no PR is created

### Requirement: Silent Graceful Degradation

The system SHALL degrade to "no suggestion offered" without surfacing an
error when the endpoint is unreachable, the request times out, or the
response is malformed/unparseable, and the push/PR flow SHALL continue
unaffected.

#### Scenario: Unreachable endpoint degrades silently
- GIVEN AI is configured but the endpoint is unreachable
- WHEN the user requests a suggestion
- THEN no suggestion is shown, no error is surfaced, and the flow continues

#### Scenario: Timeout degrades silently
- GIVEN the request exceeds its bounded timeout
- WHEN the user requests a suggestion
- THEN no suggestion is shown, no error is surfaced, and the flow continues

#### Scenario: Malformed response degrades silently
- GIVEN the model returns a response the parser cannot extract a
  title/description from
- WHEN the user requests a suggestion
- THEN no suggestion is shown, no error is surfaced, and the flow continues
