package git_test

import (
	"context"
	"testing"

	"deploydeck/internal/exec"
	"deploydeck/internal/git"
)

func TestService_EveryGitRequestCarriesNonInteractiveEnv(t *testing.T) {
	fr := exec.NewFakeRunner()
	fr.When("git", []string{"rev-parse", "--show-toplevel"}, exec.CommandResult{
		ExitCode: 0,
		Stdout:   []byte("/repo\n"),
	})

	svc := git.New(fr)

	if _, err := svc.RepoRoot(context.Background(), "/repo/sub"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(fr.Calls) != 1 {
		t.Fatalf("expected exactly 1 captured call, got %d", len(fr.Calls))
	}

	wantEnv := map[string]string{
		"GIT_EDITOR":          "true",
		"GIT_TERMINAL_PROMPT": "0",
		"GIT_PAGER":           "cat",
	}
	gotEnv := map[string]string{}
	for _, kv := range fr.Calls[0].Env {
		for k := range wantEnv {
			prefix := k + "="
			if len(kv) > len(prefix) && kv[:len(prefix)] == prefix {
				gotEnv[k] = kv[len(prefix):]
			}
		}
	}

	for k, want := range wantEnv {
		got, ok := gotEnv[k]
		if !ok {
			t.Fatalf("expected captured Env to contain %s, got Env=%v", k, fr.Calls[0].Env)
		}
		if got != want {
			t.Fatalf("expected %s=%s, got %s=%s", k, want, k, got)
		}
	}
}
