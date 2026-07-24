// Command deploydeck is a TUI + CLI that guides Salesforce commit promotion
// through Git cherry-picks: prerequisite check, commit discovery, selection,
// target/sandbox selection, promotion branch creation and controlled
// cherry-pick with post-pick verification.
package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"deploydeck/internal/config"
	"deploydeck/internal/exec"
	"deploydeck/internal/git"
	"deploydeck/internal/prereq"
	"deploydeck/internal/salesforce"
)

// Deps carries the constructed application dependencies injected into the
// Cobra command tree.
type Deps struct {
	// NewChecker builds a prereq.Checker rooted at dir. main() wires real
	// OS-backed dependencies (NewOSRunner, config.Load, a real
	// ProcessProber); tests inject fakes without changing this signature.
	NewChecker func(dir string) (*prereq.Checker, error)
}

// newRootCmd builds the deploydeck Cobra root command and registers its
// subcommands.
func newRootCmd(deps Deps) *cobra.Command {
	root := &cobra.Command{
		Use:   "deploydeck",
		Short: "Guides Salesforce commit promotion through controlled Git cherry-picks",
	}

	root.AddCommand(newDoctorCmd(deps))

	return root
}

// errDoctorBlocked is returned by the doctor RunE when at least one
// PrereqCheck is blocking, giving `deploydeck doctor` a non-zero exit code
// distinct from the success case (HU-001: "deploydeck doctor CLI
// Subcommand").
var errDoctorBlocked = errors.New("doctor: one or more blocking prerequisites failed")

// newDoctorCmd builds the doctor subcommand, running every HU-001
// local-prerequisite check via deps.NewChecker and reporting the result.
func newDoctorCmd(deps Deps) *cobra.Command {
	return &cobra.Command{
		Use:          "doctor",
		Short:        "Checks local prerequisites for running deploydeck",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			dir, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("doctor: resolving working directory: %w", err)
			}

			checker, err := deps.NewChecker(dir)
			if err != nil {
				return fmt.Errorf("doctor: %w", err)
			}

			checks, err := checker.Check(cmd.Context())
			if err != nil {
				return fmt.Errorf("doctor: %w", err)
			}

			if renderPrereqChecks(cmd.OutOrStdout(), checks) {
				return errDoctorBlocked
			}
			return nil
		},
	}
}

// renderPrereqChecks writes one line per check to w and reports whether any
// check is blocking.
func renderPrereqChecks(w io.Writer, checks []prereq.PrereqCheck) (blocking bool) {
	for _, c := range checks {
		fmt.Fprintf(w, "[%s] %s: %s\n", c.Status, c.Name, c.Detail)
		if c.FixCommand != "" {
			fmt.Fprintf(w, "    fix: %s\n", c.FixCommand)
		}
		if c.Status == prereq.StatusBlocking {
			blocking = true
		}
	}
	return blocking
}

// defaultChecker builds a prereq.Checker backed by real OS processes: a
// shared OSRunner for git/sf, deploydeck.yaml loaded from dir, and a
// single-instance lock at dir/.deploydeck/lock guarded by a real
// OSProcessProber.
func defaultChecker(dir string) (*prereq.Checker, error) {
	cfg, err := config.Load(dir)
	if err != nil {
		return nil, fmt.Errorf("loading %s: %w", config.FileName, err)
	}

	runner := exec.NewOSRunner()

	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown-host"
	}
	self := prereq.LockInfo{PID: os.Getpid(), PName: "deploydeck", Host: hostname}
	lock := prereq.NewLock(filepath.Join(dir, ".deploydeck", "lock"), self, prereq.NewOSProcessProber(runner))

	return &prereq.Checker{
		Dir:    dir,
		Git:    git.New(runner),
		SF:     salesforce.New(runner),
		Config: cfg,
		Lock:   lock,
	}, nil
}

func main() {
	deps := Deps{NewChecker: defaultChecker}

	if err := newRootCmd(deps).Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
