// Command deploydeck is a TUI + CLI that guides Salesforce commit promotion
// through Git cherry-picks: prerequisite check, commit discovery, selection,
// target/sandbox selection, promotion branch creation and controlled
// cherry-pick with post-pick verification.
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// Deps carries the constructed application dependencies injected into the
// Cobra command tree. It is intentionally minimal during bootstrap; later
// phases (HU-001 prereq/doctor) extend it with the checker/services
// composition without changing the newRootCmd(deps) signature.
type Deps struct{}

// newRootCmd builds the deploydeck Cobra root command and registers its
// subcommands. The doctor subcommand is a stub in this bootstrap phase
// (exit 0 placeholder); its real prerequisite-check behavior is wired in
// Phase 5 (HU-001).
func newRootCmd(deps Deps) *cobra.Command {
	root := &cobra.Command{
		Use:   "deploydeck",
		Short: "Guides Salesforce commit promotion through controlled Git cherry-picks",
	}

	root.AddCommand(newDoctorCmd(deps))

	return root
}

// newDoctorCmd builds the doctor subcommand. It is a stub in this bootstrap
// phase: it does nothing and exits 0. Phase 5 wires it to internal/prereq.Checker
// with a distinct non-zero exit on blockers.
func newDoctorCmd(_ Deps) *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Checks local prerequisites for running deploydeck (stub)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return nil
		},
	}
}

func main() {
	if err := newRootCmd(Deps{}).Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
