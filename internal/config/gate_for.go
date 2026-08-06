package config

import "path"

// GateFor resolves the GateConfig for target, trying an exact Gates key
// match first, then a glob match (e.g. "Release/*") via path.Match —
// mirroring SandboxFor's own exact-then-glob resolution order (design.md
// "GateFor exact→path.Match mirroring internal/config/sandbox_for.go"). The
// returned bool is true ONLY when a matched entry is also Enabled — an
// unmatched target and a matched-but-disabled entry both mean "ungated,
// deploy as today" (deploy-gate spec: "An environment with no matching gate
// entry is ungated").
func (c Config) GateFor(target string) (GateConfig, bool) {
	if gate, ok := c.Gates[target]; ok {
		return gate, gate.Enabled
	}

	for pattern, gate := range c.Gates {
		matched, err := path.Match(pattern, target)
		if err != nil {
			continue
		}
		if matched {
			return gate, gate.Enabled
		}
	}

	return GateConfig{}, false
}
