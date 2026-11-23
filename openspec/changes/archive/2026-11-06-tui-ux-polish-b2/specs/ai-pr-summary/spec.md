# AI PR Summary Delta (tui-ux-polish-b2)

Tag: `[U1]`. Scenarios run through `Model.Update`/`View` under the Ascii profile.

## ADDED Requirements

### Requirement: [U1] The Suggestion Key Is Inert Once The PR Exists

At `pushReady`, the `a` binding (AI suggestion) MUST be enabled only while AI is configured AND no PR exists yet (`prURL` is empty). Once the PR URL is set, `a` MUST be disabled: the footer MUST NOT list it (the footer shows only `q salir`, plus `? ayuda` per `tui-presentation`) and pressing `a` MUST NOT change the state, start generation or return a command.

#### Scenario: `a` is offered before the PR exists
- GIVEN push-ready with AI configured and no PR
- WHEN the view renders
- THEN the footer contains an `a` entry and pressing `a` starts generation

#### Scenario: `a` is inert after the PR exists
- GIVEN push-ready with AI configured and a PR URL set
- WHEN the view renders
- THEN the footer has no `a` entry
- AND WHEN the user presses `a`
- THEN the state is unchanged and no command is returned

#### Scenario: Without AI `a` stays inert regardless of the PR
- GIVEN push-ready without AI configured
- WHEN the user presses `a`
- THEN nothing happens, as before

## MODIFIED Requirements

None.

## REMOVED Requirements

None.
