# Apply Progress: ungated-env-deploy

Status: all tasks done (every task in tasks.md marked `[x]`).

## TDD evidence (RED -> GREEN)

| Task | RED | GREEN |
|---|---|---|
| 1.1-1.3 | `result_offer_test.go` table test: build failed, `undefined: deployOffer/offerAvailable/...` | `result_offer.go` `resultDeployOffer`; `TestResultDeployOffer` ok |
| 2.1-2.3 | upsert/offer/onReportDone tests: `m.upsertResultRun`/`m.resultOffer` undefined; after the helpers, `TestOnReportDone_UpsertsOnlyOnSuccess` failed (loaded=false) because the `update.go` hook was not applied yet | `upsertResultRun`, `resultOffer`, `resultRunIndex`; hook after `m.state = terminalState(...)` in `onReportDone`; all ok |
| 2.4 | regression guard (history reload replaces `m.runs`, cursor reset) | passed with no code change, as predicted |
| 3.1-3.3 | `result_offer_keys_test.go`: `quickFromResult` undefined | `quickFromResult` field, `x` case in `keySucceeded`, `quickDeployBack()` for esc/q, flag reset in history `x`; ok |
| 3.4 | matrix row "DESPLEGAR from result" added | passes (behavior from 3.3) |
| 3.5 | e2e `TestResultDeploy_E2E`: first run failed on my own assertion order (offer is hidden while in `StateQuickDeploy`, so it is checked after Esc) | fixed in the test; ok |
| 4.1-4.2 | parity fixtures, pinned `x desplegar`, view line tests: `TestFooterParity_PinnedFragments`, `TestFooterParity_Structural`, `TestViewValidationResult_OfferLine` FAIL | `keymap.go` hint (disabled unless available), `view.go` `deployOfferLine`; all ok |
| 4.4 | boundary tests | no new imports in `internal/app`; ok |
| 5.1-5.3 | docs | README section, HU-020 plus table rows plus HU-015 criterion, mockup with unavailable variant |

## Deviations

- `quickDeployBack()` helper in `keys.go` (shared by the esc and q arms) instead of two inline blocks.
- Missing `JobID` or `Alias` hides the offer inside `m.resultOffer()`, not in the pure predicate (missing `TestLevel` is already "unavailable" via `QuickDeployEligible`).
- The unavailable hint is a disabled binding, which `footerOf` does not render, so the footer shows nothing for it. The reason line carries the message, and `x` is a strict no-op.
- `keymap_states_test.go` was not edited; the pinned fragment in `footer_parity_test.go` covers `x desplegar`.
- HU-020 is listed as "17b" in the implementation-order list to avoid renumbering.
- `gofmt -w` on `app.go` (field alignment after adding `quickFromResult`).

## Final verification

- `go vet ./...` clean; `gofmt -l .` empty; `go test ./...` all packages ok (app 113s, git 315s).
- `stateCount` unchanged; the footer parity and esc/q matrix tests pass.

## git diff --stat (tracked) plus new files

10 files changed, 159 insertions(+), 13 deletions(-); untracked: `internal/app/result_offer.go` (87), `result_offer_test.go` (190), `result_offer_keys_test.go` (118). Total ~570 changed lines, within the ~650 budget.

## Fix round

- W1: `TestResultDeploy_E2E` is now table-driven (no gate entry, `Gates{"UAT": {Enabled:false}}`). It asserts the offer is available, `gateCheckingRunID == ""` after Enter and exactly one `sf` call (no gh). It passed on the first run, as a safety net on existing behavior.
- W2/S2: README section now states that `testLevel` must be `RunLocalTests` or `RunAllTestsInOrg` (whitelist in `internal/runs/quick_deploy.go`), and that a branch must sit under any `branches` key except `production`, with `main` always production (`git.IsProductionTarget`).
- W3: task 4.1 text now says coverage lives in `footer_parity_test.go` fixtures.
- S1: gated-target negative case added to `TestViewValidationResult_OfferLine` and the footer pinned negatives. Both passed first run.
- S3 skipped by request.
- go vet clean, `gofmt -l .` empty, `go test ./...` green.
