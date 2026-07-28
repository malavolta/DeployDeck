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
	osexec "os/exec"
	"path/filepath"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"deploydeck/internal/app"
	"deploydeck/internal/config"
	"deploydeck/internal/delta"
	"deploydeck/internal/exec"
	"deploydeck/internal/git"
	"deploydeck/internal/github"
	"deploydeck/internal/prereq"
	"deploydeck/internal/runs"
	"deploydeck/internal/salesforce"
	"deploydeck/internal/version"
)

// Deps carries the constructed application dependencies injected into the
// Cobra command tree.
type Deps struct {
	// NewChecker builds a prereq.Checker rooted at dir. main() wires real
	// OS-backed dependencies (NewOSRunner, config.Load, a real
	// ProcessProber); tests inject fakes without changing this signature.
	NewChecker func(dir string) (*prereq.Checker, error)
	// RunTUI launches the Bubble Tea promotion flow rooted at dir. main()
	// wires the real program; tests inject a fake to assert routing without
	// launching a terminal program.
	RunTUI func(dir string) error
}

// newRootCmd builds the deploydeck Cobra root command and registers its
// subcommands. With no subcommand, the root launches the Bubble Tea TUI; the
// `doctor` subcommand remains for the CLI prerequisite check.
func newRootCmd(deps Deps) *cobra.Command {
	root := &cobra.Command{
		Use:          "deploydeck",
		Short:        "Guides Salesforce commit promotion through controlled Git cherry-picks",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			dir, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("resolving working directory: %w", err)
			}
			if deps.RunTUI == nil {
				return errors.New("no TUI runner configured")
			}
			return deps.RunTUI(dir)
		},
	}

	root.AddCommand(newDoctorCmd(deps))
	root.AddCommand(newRunsCmd())

	root.Version = version.String()

	return root
}

// newRunsCmd builds the `runs` command group managing the local run history
// under .deploydeck/runs/. It needs nothing from Deps — its single subcommand
// composes config.Load + runs.NewWriter directly from the resolved cwd.
func newRunsCmd() *cobra.Command {
	runsCmd := &cobra.Command{
		Use:          "runs",
		Short:        "Manage the local run history under .deploydeck/runs/",
		SilenceUsage: true,
	}
	runsCmd.AddCommand(newRunsPruneCmd())
	return runsCmd
}

// newRunsPruneCmd builds the `runs prune` subcommand (HU-013 run-retention:
// "aplicar politica de retencion configurable ... con comando deploydeck runs
// prune"). It resolves the working directory, loads the configured
// keepLast/keepDays bounds, and prunes only the runs outside BOTH conditions,
// printing what was removed. A load/prune failure returns a non-zero exit,
// mirroring newDoctorCmd's error path.
func newRunsPruneCmd() *cobra.Command {
	return &cobra.Command{
		Use:          "prune",
		Short:        "Prune local runs outside the configured retention window",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			dir, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("runs prune: resolving working directory: %w", err)
			}
			return runPrune(cmd.OutOrStdout(), dir)
		},
	}
}

// runPrune loads dir's retention config and prunes the local run history under
// dir/.deploydeck/runs/, writing a summary of what was removed to w. It is the
// testable core of `deploydeck runs prune`: RunE only supplies the resolved
// working directory. Prune touches ONLY per-run directories enumerated by the
// writer — never arbitrary paths (runs package's threat-matrix guarantee).
func runPrune(w io.Writer, dir string) error {
	cfg, err := config.Load(dir)
	if err != nil {
		return fmt.Errorf("runs prune: loading %s: %w", config.FileName, err)
	}

	removed, err := runs.NewWriter(dir).Prune(cfg.Runs.KeepLast, cfg.Runs.KeepDays, time.Now())
	if err != nil {
		return fmt.Errorf("runs prune: %w", err)
	}

	if len(removed) == 0 {
		fmt.Fprintln(w, "runs prune: nothing to prune (all runs are within the retention window)")
		return nil
	}
	fmt.Fprintf(w, "runs prune: removed %d run(s):\n", len(removed))
	for _, id := range removed {
		fmt.Fprintf(w, "  %s\n", id)
	}
	return nil
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
		// GH backs the informative, non-blocking gh doctor check (HU-014).
		GH: github.New(runner),
	}, nil
}

// defaultRunTUI composes the real NewOSRunner-backed services and launches the
// Bubble Tea promotion flow. The interactive editor handoff (tea.ExecProcess)
// is built HERE — main may import os/exec, so internal/app never has to (it
// stays behind the service seam, enforced by internal/app/boundary_test.go).
func defaultRunTUI(dir string) error {
	cfg, err := config.Load(dir)
	if err != nil {
		return fmt.Errorf("loading %s: %w", config.FileName, err)
	}

	runner := exec.NewOSRunner()
	deps := app.Deps{
		Git:        git.New(runner),
		SF:         salesforce.New(runner),
		Delta:      delta.New(runner),
		GH:         github.New(runner),
		Runs:       runs.NewWriter(dir),
		Config:     cfg,
		Dir:        dir,
		NewChecker: defaultChecker,
		Edit:       editHandoff,
		// Now is left nil: production uses the real time.Now clock.
	}

	program := tea.NewProgram(app.New(deps))
	_, err = program.Run()
	return err
}

// editHandoff returns a tea.Cmd that suspends the TUI and opens $EDITOR on
// path for manual conflict resolution (the mockup's `e` affordance). It is the
// ONLY sanctioned use of tea.ExecProcess — an interactive editor handoff,
// never a git/sf command — and lives in main so internal/app stays exec-free.
func editHandoff(path string) tea.Cmd {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vi"
	}
	return tea.ExecProcess(osexec.Command(editor, path), func(error) tea.Msg { return nil })
}

func main() {
	deps := Deps{
		NewChecker: defaultChecker,
		RunTUI:     defaultRunTUI,
	}

	if err := newRootCmd(deps).Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
