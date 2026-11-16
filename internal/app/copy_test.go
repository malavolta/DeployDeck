package app

import (
	"strings"
	"testing"
	"time"

	"github.com/malavolta/DeployDeck/internal/delta"
	"github.com/malavolta/DeployDeck/internal/git"
	"github.com/malavolta/DeployDeck/internal/runs"
)

func TestCopy_EnglishLabelsReplaced(t *testing.T) {
	pkg := New(Deps{Dir: "/repo", Config: testConfig()})
	pkg.state = StatePackageReview
	pkg.summary = delta.PackageSummary{
		HasDestructive:   true,
		Types:            []delta.MetadataTypeSummary{{Name: "ApexClass", Count: 1}},
		DestructiveTypes: []delta.MetadataTypeSummary{{Name: "ApexClass", Count: 1}},
	}
	cl := cleanupListModel()
	cl.cleanupBranches = []cleanupRow{
		{DeployBranch: git.DeployBranch{Name: "deploy/DD-1", Pushed: true, LastCommit: time.Now()}},
		{DeployBranch: git.DeployBranch{Name: "deploy/DD-2", LastCommit: time.Now()}},
	}
	hist := historyModel([]runs.Record{{RunID: "r", JobID: "0Af", CreatedAt: time.Now()}})
	push := New(Deps{Dir: "/repo", Config: testConfig()})
	push.state, push.pushPhase = StatePushPreparation, pushReady
	push.plan.PromotionBranch = "deploy/DD-1"

	cases := []struct {
		name, view string
		banned     []string
		want       string
	}{
		{"destructive", pkg.View(), []string{"Destructive changes"}, "Cambios destructivos"},
		{"cleanup pushed", cl.View(), []string{"pushed ", " local "}, "pusheada"},
		{"cleanup local", cl.View(), []string{" local "}, "sin push"},
		{"validating", hist.View(), []string{"validating"}, "validando"},
		{"pr title", push.View(), []string{"title:"}, "título:"},
	}
	for _, c := range cases {
		for _, b := range c.banned {
			if strings.Contains(c.view, b) {
				t.Errorf("%s: English fragment %q still shown:\n%s", c.name, b, c.view)
			}
		}
		if !strings.Contains(c.view, c.want) {
			t.Errorf("%s: missing Spanish %q:\n%s", c.name, c.want, c.view)
		}
	}
}

func TestCopy_ResultTitleAccentedAndQuestionsInverted(t *testing.T) {
	m := New(Deps{Dir: "/repo"})
	m.state = StateFailed
	if !strings.Contains(m.View(), "Resultado De Validación") {
		t.Error("result title must be accented")
	}
	for name, v := range map[string]string{
		"delete": func() string {
			c := cleanupListModel()
			c.cleanupPhase, c.cleanupDeleteTarget = cleanupConfirm, "deploy/DD-1"
			return c.View()
		}(),
		"prune": func() string { c := cleanupListModel(); c.cleanupPhase = cleanupPruneConfirm; return c.View() }(),
	} {
		if !strings.Contains(v, "¿") || !strings.Contains(v, "?") {
			t.Errorf("%s prompt must open with ¿ and close with ?:\n%s", name, v)
		}
	}
}
