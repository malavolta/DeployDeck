package app

import (
	"strings"
	"testing"
)

const wrongWordNotice = "escribe BORRAR exactamente para confirmar el borrado"

// borrarEntryPoints builds the strong BORRAR confirmation from both entry
// points: the current run's branch deletion (Succeeded) and the cleanup list.
func borrarEntryPoints(t *testing.T) map[string]func(t *testing.T) Model {
	return map[string]func(t *testing.T) Model{
		"current run (Succeeded)": func(t *testing.T) Model {
			m, _, _ := strongConfirmModel(t)
			return m
		},
		"current run (Aborted)": func(t *testing.T) Model {
			m, _, _ := strongConfirmModel(t)
			m.state = StateAborted
			return m
		},
		"cleanup list": func(t *testing.T) Model {
			m, _, _ := branchCleanupStrongConfirmModel(t)
			return m
		},
	}
}

func TestBorrar_WrongWordRendersNotice(t *testing.T) {
	for name, build := range borrarEntryPoints(t) {
		for _, typed := range []string{"borrar", "BORR"} {
			t.Run(name+"/"+typed, func(t *testing.T) {
				m := typeString(build(t), typed)
				before := m.state
				next, cmd := m.Update(keyPress("enter"))
				nm := next.(Model)
				if !strings.Contains(nm.View(), wrongWordNotice) {
					t.Fatalf("view must render the wrong-word notice, got:\n%s", nm.View())
				}
				if cmd != nil {
					t.Error("a wrong word must not fire a command (no delete)")
				}
				if nm.state != before || nm.cleanupPhase != cleanupStrongConfirm || nm.deleteConfirm != typed {
					t.Errorf("state/phase/buffer must be kept, got %v %v %q", nm.state, nm.cleanupPhase, nm.deleteConfirm)
				}
			})
		}
	}
}

func TestBorrar_ExactWordDeletesWithoutNotice(t *testing.T) {
	m, fr, branch := strongConfirmModel(t)
	m = typeString(m, "BORRAR")
	next, cmd := m.Update(keyPress("enter"))
	if cmd == nil {
		t.Fatal("the exact word must fire the delete command")
	}
	if strings.Contains(next.(Model).View(), wrongWordNotice) {
		t.Error("no wrong-word notice with the exact word")
	}
	cmd()
	deleted := false
	for _, c := range fr.Calls {
		if strings.Join(c.Args, " ") == "branch -D -- "+branch {
			deleted = true
		}
	}
	if !deleted {
		t.Errorf("branch must be deleted, calls: %v", fr.Calls)
	}
}

func TestBorrar_NoticeClears(t *testing.T) {
	for name, build := range borrarEntryPoints(t) {
		t.Run(name+"/rune", func(t *testing.T) {
			m := typeString(build(t), "bor")
			next, _ := m.Update(keyPress("enter"))
			next, _ = next.(Model).Update(keyPress("x"))
			if strings.Contains(next.(Model).View(), wrongWordNotice) {
				t.Error("typing a rune must clear the notice")
			}
		})
		t.Run(name+"/esc", func(t *testing.T) {
			m := typeString(build(t), "bor")
			next, _ := m.Update(keyPress("enter"))
			next, _ = next.(Model).Update(keyPress("esc"))
			if strings.Contains(next.(Model).View(), wrongWordNotice) {
				t.Error("esc must clear the notice")
			}
		})
	}
}
