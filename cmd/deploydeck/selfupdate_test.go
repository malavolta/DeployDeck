package main

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"reflect"
	"strings"
	"testing"

	"github.com/malavolta/DeployDeck/internal/app"
	"github.com/malavolta/DeployDeck/internal/update"
)

type fakeRun struct {
	calls [][]string
	err   error
}

func (f *fakeRun) run(argv []string) error {
	f.calls = append(f.calls, argv)
	return f.err
}

func TestRunUpgrade(t *testing.T) {
	brew := []string{"brew", "upgrade", "--cask", "deploydeck"}
	tests := []struct {
		name      string
		argv      []string
		goos      string
		runErr    error
		wantCalls int
		wantFail  bool     // something printed to stderr
		wantOut   []string // each must appear in stdout+stderr
	}{
		{"success", brew, "darwin", nil, 1, false, []string{"brew upgrade --cask deploydeck", "Listo."}},
		{"non-zero exit", brew, "darwin", errors.New("exit status 1"), 1, true,
			[]string{"No se pudo actualizar DeployDeck", "brew upgrade --cask deploydeck", update.ReleasesURL}},
		{"cannot start", brew, "darwin", &exec.Error{Name: "brew", Err: exec.ErrNotFound}, 1, true,
			[]string{"brew upgrade --cask deploydeck", update.ReleasesURL}},
		{"nil argv", nil, "darwin", nil, 0, true, []string{update.ReleasesURL}},
		{"windows prints only", []string{"scoop", "update", "deploydeck"}, "windows", nil, 0, false,
			[]string{"Para completar la actualización ejecuta: scoop update deploydeck"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := &fakeRun{err: tt.runErr}
			var out, errb bytes.Buffer
			runUpgrade(tt.argv, tt.goos, &out, &errb, f.run)
			if len(f.calls) != tt.wantCalls || (tt.wantCalls == 1 && !reflect.DeepEqual(f.calls[0], tt.argv)) {
				t.Fatalf("run calls = %v, want %d call(s) with %v", f.calls, tt.wantCalls, tt.argv)
			}
			if (errb.Len() > 0) != tt.wantFail {
				t.Errorf("stderr = %q, wantFail %v", errb.String(), tt.wantFail)
			}
			for _, w := range tt.wantOut {
				if all := out.String() + errb.String(); !strings.Contains(all, w) {
					t.Errorf("output %q missing %q", all, w)
				}
			}
		})
	}
}

func TestComposeUpdate(t *testing.T) {
	found := func(string) (string, error) { return "/usr/bin/x", nil }
	missing := func(string) (string, error) { return "", errors.New("not found") }
	const brewExe = "/opt/homebrew/Caskroom/deploydeck/1.0.0/deploydeck"
	tests := []struct {
		name         string
		exe, optOut  string
		lookPath     func(string) (string, error)
		wantCheck    bool
		wantUpgrade  bool
		unusedServer bool
	}{
		{"opt-out disables the check", brewExe, "1", found, false, true, true},
		{"empty value does not opt out", brewExe, "", found, true, true, false},
		{"brew tool missing", brewExe, "", missing, true, false, false},
		{"manual", "/usr/local/bin/deploydeck", "", found, true, false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url := "http://unused"
			if tt.unusedServer {
				srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
					t.Error("update server must not be called")
				}))
				defer srv.Close()
				url = srv.URL
			}
			getenv := func(k string) string {
				if k == "DEPLOYDECK_NO_UPDATE_CHECK" {
					return tt.optOut
				}
				return ""
			}
			w := composeUpdate(url, t.TempDir(), "v1.0.0", tt.exe, "darwin", getenv, tt.lookPath)
			if (w.CheckUpdate != nil) != tt.wantCheck || w.CanUpgrade != tt.wantUpgrade {
				t.Fatalf("CheckUpdate wired=%v CanUpgrade=%v, want %v/%v", w.CheckUpdate != nil, w.CanUpgrade, tt.wantCheck, tt.wantUpgrade)
			}
		})
	}
}

func TestPostRun_NoRequestNeverRuns(t *testing.T) {
	f := &fakeRun{}
	var out, errb bytes.Buffer
	postRun(app.New(app.Deps{}), update.MethodBrew, "darwin", &out, &errb, f.run)
	if len(f.calls) != 0 || out.Len() != 0 || errb.Len() != 0 {
		t.Fatalf("calls=%d out=%q err=%q, want none", len(f.calls), out.String(), errb.String())
	}
}

func TestNewCheckUpdate_FailedCheckNotCached(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	dir := t.TempDir()
	has, latest, err := newCheckUpdate("http://127.0.0.1:1", dir, "v1.0.0")(ctx)
	if err == nil || has || latest != "" {
		t.Fatalf("got %v %q %v, want false, empty, error", has, latest, err)
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 0 {
		t.Fatalf("cache dir has %d entries, want 0", len(entries))
	}
}
