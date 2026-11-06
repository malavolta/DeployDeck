package app

import (
	"testing"
	"time"

	"github.com/malavolta/DeployDeck/internal/git"
	"github.com/malavolta/DeployDeck/internal/github"
	"github.com/malavolta/DeployDeck/internal/runs"
)

// TestFooterPins_BytesAreUnchanged pins every footer string produced before
// the keymap refactor (the byte-identical contract of tui-presentation).
func TestFooterPins_BytesAreUnchanged(t *testing.T) {
	branch := func(s State) func(*Model) {
		return func(m *Model) { m.state, m.plan.PromotionBranch = s, "deploy/DD-1" }
	}
	push := func(p pushPhase, auth github.AuthState, ai bool, more func(*Model)) func(*Model) {
		return func(m *Model) {
			m.state, m.pushPhase, m.authState = StatePushPreparation, p, auth
			if ai {
				m.deps.GenerateSummary = stubAI
			}
			if more != nil {
				more(m)
			}
		}
	}
	cleanup := func(p cleanupPhase) func(*Model) {
		return func(m *Model) { *m = cleanupListModel(); m.cleanupPhase = p }
	}
	hist := func(rs ...runs.Record) func(*Model) {
		return func(m *Model) { m.state, m.runs = StateRunHistory, rs }
	}
	authed := github.AuthAuthenticated
	cases := []struct {
		name string
		set  func(*Model)
		want string
	}{
		{"help open", func(m *Model) { m.state, m.helpOpen = StateMainMenu, true }, "?/Esc cerrar"},
		{"in-flight", st(StateCherryPicking), "ctrl+c interrumpe"},
		{"discovery", st(StateCommitDiscovery), "ctrl+c interrumpe"},
		{"error", st(StateError), "Enter/q/Esc salir   ? ayuda"},
		{"failed", st(StateFailed), "Enter/q/Esc salir   ? ayuda"},
		{"succeeded no branch", st(StateSucceeded), "Enter/q/Esc salir   ? ayuda"},
		{"succeeded", branch(StateSucceeded), "p preparar push   d borrar rama   Enter/q/Esc salir   ? ayuda"},
		{"aborted", branch(StateAborted), "d borrar rama   Enter/q/Esc salir   ? ayuda"},
		{"prereq", st(StatePrereqCheck), "r reintentar   c continuar con warnings   q/Esc salir   ? ayuda"},
		{"main menu", st(StateMainMenu), "Enter seleccionar   ↑/↓ navegar   q/Esc salir   ? ayuda"},
		{"main menu update", func(m *Model) {
			m.state, m.updateAvailable, m.deps.CanUpgrade = StateMainMenu, true, true
		}, "Enter seleccionar   ↑/↓ navegar   u actualizar   q/Esc salir   ? ayuda"},
		{"delta source", st(StateDeltaSourceSelect), "Enter seleccionar   ↑/↓ navegar   Esc volver   q salir   ? ayuda"},
		{"package path", st(StatePackageSelect), "Enter continuar   q/Esc volver"},
		{"sandbox", st(StateSandboxSelect), "Enter seleccionar   ↑/↓ navegar   Esc volver   q salir   ? ayuda"},
		{"ticket", st(StateTicketInput), "Enter buscar   Esc volver"},
		{"source confirm", st(StateSourceConfirm), "s confirmar   n/N/Enter declinar   Esc volver   q salir   ? ayuda"},
		{"selection empty", st(StateCommitSelection), "Esc volver   q salir   ? ayuda"},
		{"selection", func(m *Model) { m.state, m.items = StateCommitSelection, []git.CommitSelectionItem{{}} },
			"↑/↓ navegar   Space/x marcar   Enter continuar   Esc volver   q salir   ? ayuda"},
		{"target", st(StateTargetSelection), "Enter seleccionar   ↑/↓ navegar   Esc volver   q salir   ? ayuda"},
		{"plan preview", st(StatePlanPreview), "Enter ejecutar   Esc volver   q salir   ? ayuda"},
		{"conflict continue on", func(m *Model) { m.state, m.continueEnabled = StateCherryPickConflict, true },
			"↑/↓ elegir   e editor   c continuar   s saltar   a abortar   q salir (el run queda reanudable)   ? ayuda"},
		{"verification", st(StatePickVerification), "Enter continuar igualmente   e editar selección   q salir   ? ayuda"},
		{"delta failed", func(m *Model) { m.state, m.deltaErr = StateDeltaGeneration, errStub }, "r reintentar   e editar selección   q salir   ? ayuda"},
		{"delta running", st(StateDeltaGeneration), "q salir   ? ayuda"},
		{"package review", st(StatePackageReview), "Enter validar   e editar selección   q salir   ? ayuda"},
		{"package review empty", func(m *Model) { m.state, m.summary.Empty = StatePackageReview, true },
			"o override   Enter validar   e editar selección   q salir   ? ayuda"},
		{"package review delta", func(m *Model) { m.state, m.standaloneMode = StatePackageReview, "delta" }, "q salir   ? ayuda"},
		{"queue", st(StateQueueReview), "r refrescar   Enter continuar validación   Esc volver   q salir   ? ayuda"},
		{"queue failed", func(m *Model) { m.state, m.queueErr = StateQueueReview, errStub }, "r reintentar   Enter continuar validación   Esc volver   q salir   ? ayuda"},
		{"validation start failed", func(m *Model) { m.state, m.validateErr = StateValidationStart, errStub }, "r reintentar   q salir   ? ayuda"},
		{"validation start", st(StateValidationStart), "q salir   ? ayuda"},
		{"polling", st(StateValidationPolling), "r refrescar   c cancelar mi job   q salir (el job sigue activo)   ? ayuda"},
		{"cancel confirm", st(StateCancelConfirm), "Enter confirmar   Esc volver"},
		{"history empty", hist(), "Esc volver   q salir   ? ayuda"},
		{"history", hist(runs.Record{RunID: "r", Status: "InProgress", JobID: "J", CreatedAt: time.Now()}),
			"Enter reanudar   d detalle   ↑/↓ navegar   Esc volver   q salir   ? ayuda"},
		{"history terminal row", hist(runs.Record{RunID: "r", Status: "Failed", JobID: "J", CreatedAt: time.Now()}),
			"d detalle   ↑/↓ navegar   Esc volver   q salir   ? ayuda"},
		{"history re-promote", hist(runs.Record{RunID: "r", Status: "Succeeded", Commits: []string{"a"}, CreatedAt: time.Now()}),
			"d detalle   r re-promocionar   ↑/↓ navegar   Esc volver   q salir   ? ayuda"},
		{"quick deploy", func(m *Model) { m.state, m.runs = StateQuickDeploy, []runs.Record{{RunID: "r"}} }, "Enter confirmar   q/Esc volver"},
		{"quick deploy none", st(StateQuickDeploy), "q/Esc volver"},
		{"gate blocked", st(StateDeployGateBlocked), "Esc volver al historial   q salir   ? ayuda"},
		{"collision", st(StateBranchCollision), "r reusar   d borrar y recrear   c/Esc cancelar   q salir   ? ayuda"},
		{"push confirm", push(pushConfirm, authed, false, nil), "p ejecutar push   Esc volver   q salir   ? ayuda"},
		{"pushing", push(pushPushing, authed, false, nil), "q salir   ? ayuda"},
		{"ready authed", push(pushReady, authed, false, nil), "g crear PR con gh   q salir   ? ayuda"},
		{"ready authed ai", push(pushReady, authed, true, nil), "g crear PR con gh   a sugerir título   q salir   ? ayuda"},
		{"ready authed accept", push(pushReady, authed, true, func(m *Model) { m.aiTitle = "t" }), "g crear PR con gh   a aceptar sugerencia   q salir   ? ayuda"},
		{"ready compare ai", push(pushReady, github.AuthUnauthenticated, true, nil), "a sugerir título   q salir   ? ayuda"},
		{"ready compare", push(pushReady, github.AuthUnauthenticated, false, nil), "q salir   ? ayuda"},
		{"pr confirm", push(pushPRConfirm, authed, false, nil), "y confirmar crear PR   n/Esc/q cancelar   ? ayuda"},
		{"pr confirm ai ready", push(pushPRConfirm, authed, true, func(m *Model) { m.aiTitle = "t" }), "y con IA   d título default   n/Esc/q cancelar   ? ayuda"},
		{"pr confirm ai pending", push(pushPRConfirm, authed, true, func(m *Model) { m.aiPending = true }), "n/Esc/q volver   ? ayuda"},
		{"pr creating", push(pushPRCreating, authed, false, nil), "q salir   ? ayuda"},
		{"pr created", push(pushReady, authed, false, func(m *Model) { m.prURL = "u" }), "q salir   ? ayuda"},
		{"cleanup loading", func(m *Model) { *m = cleanupListModel(); m.cleanupPhase = cleanupLoading }, "ctrl+c interrumpe"},
		{"cleanup empty", func(m *Model) { *m = cleanupListModel(); m.cleanupBranches = nil }, "p retención de runs   Esc volver   q salir   ? ayuda"},
		{"cleanup list", cleanup(cleanupBrowsing), "d borrar   p retención de runs   ↑/↓ navegar   Esc volver   q salir   ? ayuda"},
		{"cleanup counting", cleanup(cleanupCounting), "Esc cancelar   ? ayuda"},
		{"cleanup confirm", cleanup(cleanupConfirm), "y confirmar   n/Esc/q cancelar   ? ayuda"},
		{"cleanup strong", cleanup(cleanupStrongConfirm), "Enter confirmar   Esc cancelar"},
		{"cleanup prune", cleanup(cleanupPruneConfirm), "y confirmar   n/Esc/q cancelar   ? ayuda"},
		{"delete confirm", func(m *Model) { branch(StateSucceeded)(m); m.cleanupPhase = cleanupConfirm }, "y confirmar   n/Esc/q cancelar   ? ayuda"},
		{"delete strong", func(m *Model) { branch(StateAborted)(m); m.cleanupPhase = cleanupStrongConfirm }, "Enter confirmar   Esc cancelar"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m := New(Deps{Dir: "/repo", Config: testConfig()})
			c.set(&m)
			if _, text, ok := footerKeys(m.View()); !ok || text != c.want {
				t.Errorf("footer = %q (ok=%v), want %q", text, ok, c.want)
			}
		})
	}
}
