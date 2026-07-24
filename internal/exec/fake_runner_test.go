package exec_test

import (
	"context"
	"testing"

	"deploydeck/internal/exec"
)

func TestFakeRunner_ReturnsCannedResultForMatchingNameAndArgs(t *testing.T) {
	fr := exec.NewFakeRunner()
	want := exec.CommandResult{
		ExitCode: 0,
		Stdout:   []byte("2.43.0"),
	}
	fr.When("git", []string{"--version"}, want)

	got, err := fr.Run(context.Background(), exec.CommandRequest{
		Name: "git",
		Args: []string{"--version"},
	})
	if err != nil {
		t.Fatalf("expected no error for a canned request, got: %v", err)
	}
	if string(got.Stdout) != "2.43.0" {
		t.Fatalf("expected canned stdout %q, got %q", "2.43.0", got.Stdout)
	}
	if got.ExitCode != 0 {
		t.Fatalf("expected canned exit code 0, got %d", got.ExitCode)
	}
}

func TestFakeRunner_UnmatchedRequestErrorsExplicitly(t *testing.T) {
	fr := exec.NewFakeRunner()
	fr.When("git", []string{"--version"}, exec.CommandResult{})

	_, err := fr.Run(context.Background(), exec.CommandRequest{
		Name: "git",
		Args: []string{"status", "--porcelain"},
	})
	if err == nil {
		t.Fatal("expected an explicit error for a request with no canned response, got nil")
	}
}
