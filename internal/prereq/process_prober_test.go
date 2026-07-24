package prereq_test

import (
	"testing"
	"time"

	"deploydeck/internal/exec"
	"deploydeck/internal/prereq"
)

// psLayout mirrors the fixed-width layout `ps -o lstart=` prints under the C
// locale, so tests can build matching/mismatching start-times the same way
// the prober parses them.
const psLayout = "Mon Jan  2 15:04:05 2006"

func mustParsePS(t *testing.T, raw string) time.Time {
	t.Helper()
	tm, err := time.ParseInLocation(psLayout, raw, time.Local)
	if err != nil {
		t.Fatalf("test setup: could not parse %q: %v", raw, err)
	}
	return tm
}

func TestOSProcessProber_Alive_TableDriven(t *testing.T) {
	const pid = 4242
	psArgs := []string{"-p", "4242", "-o", "lstart="}

	wellFormed := "Wed Jul 24 10:15:03 2026"
	matching := mustParsePS(t, wellFormed)

	tests := []struct {
		name      string
		psResult  exec.CommandResult
		startedAt time.Time
		want      bool
		reason    string
	}{
		{
			name:      "localized non-empty line is fail-safe alive",
			psResult:  exec.CommandResult{ExitCode: 0, Stdout: []byte("lun. 22 jun. 14:13:00 2026\n")},
			startedAt: matching,
			want:      true,
			reason:    "a present-but-unparseable (e.g. Spanish-locale) line must NOT be read as a dead owner — refuse takeover",
		},
		{
			name:      "garbage non-empty line is fail-safe alive",
			psResult:  exec.CommandResult{ExitCode: 0, Stdout: []byte("this is not a timestamp\n")},
			startedAt: matching,
			want:      true,
			reason:    "any non-empty unparseable line means we cannot verify liveness — assume alive",
		},
		{
			name:      "empty output means process not found (dead)",
			psResult:  exec.CommandResult{ExitCode: 0, Stdout: []byte("")},
			startedAt: matching,
			want:      false,
			reason:    "an empty ps result unambiguously means no such process — takeover allowed",
		},
		{
			name:      "non-zero exit means no such process (dead)",
			psResult:  exec.CommandResult{ExitCode: 1, Stdout: []byte("")},
			startedAt: matching,
			want:      false,
			reason:    "ps -p exits non-zero when the pid does not exist — dead",
		},
		{
			name:      "well-formed line matching stored start is alive",
			psResult:  exec.CommandResult{ExitCode: 0, Stdout: []byte(wellFormed + "\n")},
			startedAt: matching,
			want:      true,
			reason:    "a live pid whose start-time matches the stored start-time is the real owner",
		},
		{
			name:      "well-formed line with mismatched start-time is dead (pid reuse)",
			psResult:  exec.CommandResult{ExitCode: 0, Stdout: []byte(wellFormed + "\n")},
			startedAt: matching.Add(10 * time.Second),
			want:      false,
			reason:    "a live pid whose start-time differs beyond tolerance was reused — original owner is gone",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fr := exec.NewFakeRunner()
			fr.When("ps", psArgs, tt.psResult)

			prober := prereq.NewOSProcessProber(fr)
			got := prober.Alive(pid, tt.startedAt)
			if got != tt.want {
				t.Fatalf("Alive() = %v, want %v (%s)", got, tt.want, tt.reason)
			}
		})
	}
}

func TestOSProcessProber_Alive_PinsCLocale(t *testing.T) {
	fr := exec.NewFakeRunner()
	fr.When("ps", []string{"-p", "4242", "-o", "lstart="}, exec.CommandResult{ExitCode: 0, Stdout: []byte("Wed Jul 24 10:15:03 2026\n")})

	prober := prereq.NewOSProcessProber(fr)
	prober.Alive(4242, time.Now())

	if len(fr.Calls) == 0 {
		t.Fatal("expected the prober to invoke ps through the runner")
	}
	env := fr.Calls[0].Env
	hasLCAll, hasLang := false, false
	for _, e := range env {
		if e == "LC_ALL=C" {
			hasLCAll = true
		}
		if e == "LANG=C" {
			hasLang = true
		}
	}
	if !hasLCAll || !hasLang {
		t.Fatalf("expected ps to run with a pinned C locale (LC_ALL=C, LANG=C) so lstart output is stable English, got Env=%v", env)
	}
}
