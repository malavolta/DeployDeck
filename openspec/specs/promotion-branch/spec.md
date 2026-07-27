# Capability: Promotion Branch

## Overview

The promotion branch capability creates a temporary working branch from the target branch's remote HEAD, applies selected commits to it, and provides conflict resolution during cherry-pick. It ensures the branch is always based on fresh remote state and offers collision handling for existing branches.

## Requirements

### Requirement: Fetch-Before-Branch-Creation Ordering

The system SHALL run `git fetch origin` before creating the temporary promotion branch.

#### Scenario: Fetch runs before branch creation
- GIVEN a valid target
- WHEN the promotion branch is created
- THEN `git fetch origin` executes before the branch is created

### Requirement: Branch Base From Fetched Remote, Not Stale Local Ref

The system SHALL base the new branch on the just-fetched `origin/<target>`, not on a stale local tracking ref.

#### Scenario: Branch follows remote advance discovered by fetch
- GIVEN `origin/<target>` advances to a new HEAD after the local clone, and only a fetch retrieves it
- WHEN the promotion branch is created
- THEN it starts exactly from the post-fetch `origin/<target>` HEAD, not the outdated local ref

### Requirement: Configurable Branch Name Templating

The system SHALL generate the branch name from the configured format (default `deploy/{{ticket}}-to-{{target}}`), and SHALL offer the user to edit it before creation.

#### Scenario: Branch name follows the configured template
- GIVEN a ticket and a target
- WHEN the branch name is generated
- THEN it follows the configured `branchFormat`

### Requirement: Existing Temp Branch Collision Handling

The system SHALL prompt the user for an action when the temp branch name already exists locally or remotely.

#### Scenario: Existing branch prompts for action
- GIVEN a branch with the target name already exists locally or in `origin`
- WHEN creation is attempted
- THEN the user is prompted to choose an action

### Requirement: Protected Branch Guard

The system SHALL validate the user is not directly modifying a protected branch when starting branch creation.

#### Scenario: User on protected branch is not modified directly
- GIVEN the user is currently on a protected branch
- WHEN the promotion branch flow starts
- THEN the protected branch itself is not modified directly

### Requirement: Fetch Failure Halts Flow Without Branch Change

The system SHALL stop the flow without changing the current branch when `git fetch` fails.

#### Scenario: Fetch failure stops without side effects
- GIVEN `git fetch origin` fails
- WHEN branch creation is attempted
- THEN the flow stops and the current branch is unchanged

### Requirement: Created Branch Registration In Deployment Plan

The system SHALL record the final created branch name in the `DeploymentPlan`.

#### Scenario: Successful creation registers the branch name
- GIVEN the promotion branch is created successfully
- WHEN creation completes
- THEN the `DeploymentPlan` records the final branch name

## Design Notes

- Branch name templating uses `strings.NewReplacer` for simple literal-token substitution (`{{ticket}}`, `{{target}}`), not Go text/template parsing.
- The branch is created via `git checkout -b <name> origin/<target>` after fetching, ensuring it tracks the freshly fetched remote HEAD.
- Protected branches are configured in the YAML configuration file and are skipped from direct modification.
