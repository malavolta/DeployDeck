package prereq

import (
	"context"

	"github.com/malavolta/DeployDeck/internal/config"
)

// nameConfigFile is the PrereqCheck.Name CheckConfig uses.
const nameConfigFile = "config file"

// CheckConfig runs config.Config.Validate() as a blocking prerequisite
// check (config-validation-wiring ADR-1, prereq-check spec: "Config
// Validity Check (Blocking)"). c.Config MUST already have been through
// applyDefaults (i.e. produced by config.Load) — see the field's own doc
// comment (ADR-6) — since Validate rejects the zero pollIntervalSeconds/
// pollTimeoutSeconds that every real, minimally-authored deploydeck.yaml
// carries before defaults are applied.
//
// ctx is accepted and unused, and the error return is always nil: this
// keeps the same signature shape as the eight sibling single-check methods
// (CheckGitignore also ignores its ctx), so Check()'s body stays one
// uniform shape.
func (c *Checker) CheckConfig(ctx context.Context) (PrereqCheck, error) {
	if err := c.Config.Validate(); err != nil {
		return PrereqCheck{
			Name:       nameConfigFile,
			Status:     StatusBlocking,
			Detail:     err.Error(), // verbatim; Validate()'s error already names the rule and the offending key (ADR-4)
			FixCommand: "edit " + c.configFilePath(),
		}, nil
	}
	return PrereqCheck{
		Name:   nameConfigFile,
		Status: StatusOK,
		Detail: c.configFilePath() + " is valid", // discloses WHICH file was located
	}, nil
}

// configFilePath degrades a zero-value ConfigPath to the bare file name
// rather than emitting "edit " with an empty operand.
func (c *Checker) configFilePath() string {
	if c.ConfigPath == "" {
		return config.FileName // "deploydeck.yaml"
	}
	return c.ConfigPath
}
