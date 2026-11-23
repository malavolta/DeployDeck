package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	osexec "os/exec"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/malavolta/DeployDeck/internal/app"
	"github.com/malavolta/DeployDeck/internal/update"
)

// runFunc is the injectable exec seam; tests never run a package manager.
type runFunc func(argv []string) error

// runInherited runs argv with the process stdio, in the restored terminal.
func runInherited(argv []string) error {
	cmd := osexec.Command(argv[0], argv[1:]...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	return cmd.Run()
}

// runUpgrade runs the post-TUI upgrade once; failures only print manual
// instructions (exit status unchanged). Windows prints the command instead,
// because the running .exe is locked (ADR-5).
func runUpgrade(argv []string, goos string, stdout, stderr io.Writer, run runFunc) {
	if len(argv) == 0 {
		fmt.Fprintf(stderr, "No se pudo determinar el comando de actualización. Descarga la versión desde %s\n", update.ReleasesURL)
		return
	}
	cmdline := strings.Join(argv, " ")
	if goos == "windows" {
		fmt.Fprintf(stdout, "Para completar la actualización ejecuta: %s\n", cmdline)
		return
	}
	fmt.Fprintf(stdout, "Actualizando DeployDeck: %s\n", cmdline)
	if err := run(argv); err != nil {
		fmt.Fprintf(stderr, "No se pudo actualizar DeployDeck (%v). Actualiza manualmente con:\n  %s\no descarga la versión desde %s\n", err, cmdline, update.ReleasesURL)
		return
	}
	fmt.Fprintln(stdout, "Listo. Ejecuta deploydeck --version para confirmar la versión instalada.")
}

type updateWiring struct {
	CheckUpdate func(ctx context.Context) (bool, string, error)
	CanUpgrade  bool
	Method      update.Method
}

// composeUpdate wires the cached check (nil when DEPLOYDECK_NO_UPDATE_CHECK is
// non-empty) and CanUpgrade (known method whose tool is on PATH).
func composeUpdate(baseURL, cacheDir, current, exe, goos string, getenv func(string) string, lookPath func(string) (string, error)) updateWiring {
	w := updateWiring{Method: update.DetectMethod(exe, goos, getenv)}
	if t := w.Method.Tool(); t != "" {
		_, err := lookPath(t)
		w.CanUpgrade = err == nil
	}
	if getenv("DEPLOYDECK_NO_UPDATE_CHECK") == "" {
		w.CheckUpdate = newCheckUpdate(baseURL, cacheDir, current)
	}
	return w
}

// newCheckUpdate builds app.Deps.CheckUpdate over the 24h cache.
func newCheckUpdate(baseURL, cacheDir, current string) func(context.Context) (bool, string, error) {
	checker := update.Checker{BaseURL: baseURL, HTTPClient: &http.Client{}}
	cache := update.Cache{Dir: cacheDir, Now: time.Now}
	return func(ctx context.Context) (bool, string, error) {
		latest, err := cache.Latest(ctx, checker.Latest)
		return decideUpdate(current, latest, err)
	}
}

// resolveExePath returns the symlink-resolved executable, "" on error (Manual).
func resolveExePath() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		return resolved
	}
	return exe
}

// postRun runs the requested upgrade, if any, once the TUI has quit.
func postRun(final tea.Model, m update.Method, goos string, stdout, stderr io.Writer, run runFunc) {
	model, ok := final.(app.Model)
	if !ok || !model.UpgradeRequested() {
		return
	}
	runUpgrade(update.UpgradeArgs(m, model.UpdateLatest()), goos, stdout, stderr, run)
}
