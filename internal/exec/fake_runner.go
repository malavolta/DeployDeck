package exec

import (
	"context"
	"fmt"
	"strings"
)

// FakeRunner is a Runner test double: it returns a canned CommandResult for
// requests registered via When, matched by Name+Args, and records every
// request it receives in Calls so tests can assert on what was sent
// (e.g. captured Env).
type FakeRunner struct {
	responses map[string]CommandResult
	Calls     []CommandRequest
}

// NewFakeRunner returns an empty FakeRunner ready for When registrations.
func NewFakeRunner() *FakeRunner {
	return &FakeRunner{responses: make(map[string]CommandResult)}
}

// When registers the CommandResult to return for a request matching the
// given Name and Args exactly.
func (f *FakeRunner) When(name string, args []string, result CommandResult) {
	f.responses[requestKey(name, args)] = result
}

// Run returns the canned result registered via When for req.Name+req.Args.
// An unmatched request returns an explicit error rather than a zero-value
// result, so tests fail loudly instead of silently passing on empty data.
func (f *FakeRunner) Run(_ context.Context, req CommandRequest) (CommandResult, error) {
	f.Calls = append(f.Calls, req)

	result, ok := f.responses[requestKey(req.Name, req.Args)]
	if !ok {
		return CommandResult{}, fmt.Errorf("exec: FakeRunner has no canned response for %s %v", req.Name, req.Args)
	}
	return result, nil
}

func requestKey(name string, args []string) string {
	return name + "\x00" + strings.Join(args, "\x00")
}
