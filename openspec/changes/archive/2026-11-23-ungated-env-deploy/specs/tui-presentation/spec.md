# Delta for TUI Presentation

## MODIFIED Requirements

### Requirement: Esc And Q Rule Per Screen Category

(Existing table and scenarios unchanged, with this variant added.) The typed-confirmation row for `DESPLEGAR` SHALL go back to its origin: `StateSucceeded` when opened from the result screen, `StateRunHistory` when opened from run history. "Back to the parent" for `StateQuickDeploy` therefore means the state that opened it.

(Previously: `DESPLEGAR` always went back to run history.)

#### Scenario: DESPLEGAR cancel follows its origin
- GIVEN `StateQuickDeploy` opened from `StateSucceeded`
- WHEN the user presses `esc`, and separately `q` on an empty buffer
- THEN each yields `StateSucceeded` with no quit command

## ADDED Requirements

### Requirement: Succeeded Screen Shows The Deploy Offer And Hint Consistently

When the quick-deploy offer is available on `StateSucceeded`, the footer SHALL list `x desplegar` and the keymap SHALL bind `x` in that state; when it is not available, neither the hint nor the binding SHALL appear. The result screen SHALL show one offer line: the offer when available, or the unavailable reason (existing Spanish reason text, or the execution-disabled reason) when the target is eligible-by-target but not deployable. No line SHALL be shown for gated or production targets. Footers and handlers SHALL stay in parity, and the footer grammar of other keys stays byte-identical.

#### Scenario: Footer and keymap agree when available
- GIVEN `StateSucceeded` with an available offer
- WHEN the footer renders and `x` is dispatched
- THEN the footer lists `x desplegar` and the handler acts on `x`

#### Scenario: No hint when absent
- GIVEN `StateSucceeded` for a gated or production target
- WHEN the footer renders
- THEN no `x desplegar` hint is listed and `x` is not bound

#### Scenario: Unavailable offer shows a reason but no hint
- GIVEN an ineligible run or `AllowExecution` false on an ungated non-production target
- WHEN the result screen renders
- THEN the offer line shows the unavailable reason and the footer lists no executable `x desplegar` hint
