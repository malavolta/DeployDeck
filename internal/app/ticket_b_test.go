package app

import "testing"

func TestTicketB(t *testing.T) {
	const pin = "Enter buscar   Esc volver"
	m := New(Deps{Dir: "/repo", Config: testConfig()})
	m.state = StateTicketInput
	if _, text, _ := footerKeys(m.View()); text != pin {
		t.Errorf("empty-buffer footer = %q", text)
	}
	nm, _ := press(m, "b")
	if nm.state != StateBranchCleanup || nm.ticket != "" {
		t.Errorf("b on an empty buffer must open cleanup: state=%v ticket=%q", nm.state, nm.ticket)
	}
	m.ticket = "WE"
	if _, text, _ := footerKeys(m.View()); text != pin {
		t.Errorf("non-empty footer = %q", text)
	}
	nm, cmd := press(m, "b")
	if nm.state != StateTicketInput || nm.ticket != "WEb" || cmd != nil {
		t.Errorf("b on a non-empty buffer must be typed: state=%v ticket=%q", nm.state, nm.ticket)
	}
}
