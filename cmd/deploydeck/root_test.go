package main

import "testing"

func TestNewRootCmd_RegistersDoctorSubcommand(t *testing.T) {
	cmd := newRootCmd(Deps{})

	doctorCmd, _, err := cmd.Find([]string{"doctor"})
	if err != nil {
		t.Fatalf("expected root command to register a doctor subcommand, got error: %v", err)
	}
	if doctorCmd.Name() != "doctor" {
		t.Fatalf("expected doctor subcommand name %q, got %q", "doctor", doctorCmd.Name())
	}
}

func TestNewRootCmd_DoctorStubExitsZero(t *testing.T) {
	cmd := newRootCmd(Deps{})
	cmd.SetArgs([]string{"doctor"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("expected doctor stub to exit 0 (nil error), got: %v", err)
	}
}
