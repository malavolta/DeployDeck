# Apply Report: gh-optional-doctor

Status: done (all 7 phases, tasks.md fully marked [x]).

Gate: `go build ./... && go vet ./... && go test ./...` green (incl. internal/app boundary and footer parity). gofmt clean.

Changed lines (excluding openspec/): 497 insertions, 28 deletions = 525 total (budget ~550).
Code about 190, tests about 335.

Deviations:
- Existing-test fixture updates: `ghRunner("authed"/"unauthed")` also registers the `--hostname=github.com` probe; `gateCheckModel` and the no-PR gate test use an authed gh runner; gate call-count assertions went 4->5 and 1->2 (auth probe); `TestInit_Batches_PrereqAndCheckUpdate` expects 3 cmds (spinner seed); `richModel` resets `spinning=false` (New now arms it with NewChecker).
- PR timeout note copy is Spanish ("No se pudo verificar gh (tiempo agotado)...") per UI-copy rule.
- Phase 3 and phase 1 RED were observed as compile failures (undefined symbols), not assertion failures.
- Retry (`r`) shows no loading line (accepted mismatch).

Risks: `Init` now returns a 3-cmd batch with NewChecker; Doctor probe depends on `Checker.GitRoot` being set (it is in production wiring).
