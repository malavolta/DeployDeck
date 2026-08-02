package app

import (
	"strings"
	"testing"

	"github.com/malavolta/DeployDeck/internal/config"
)

// demoTicketConfig mirrors testConfig but scopes TicketPatterns to the
// DEMO-* shape used by the ticket-from-branch fixtures below.
func demoTicketConfig() config.Config {
	cfg := testConfig()
	cfg.TicketPatterns = []string{"DEMO-[0-9]+"}
	return cfg
}

// TestKeyMainMenu_SeedsTicketFromCurrentBranch is the ticket-auto-suggest
// seed point: entering StateTicketInput via the "Promocionar ticket" entry
// pre-fills m.ticket from m.originalBranch when it matches a configured
// ticket pattern, and flags it via m.ticketFromBranch.
func TestKeyMainMenu_SeedsTicketFromCurrentBranch(t *testing.T) {
	t.Run("branch matches a configured pattern: seeds ticket + flag", func(t *testing.T) {
		m := mainMenuModel(0)
		m.deps.Config = demoTicketConfig()
		m.originalBranch = "DEMO-2-mi-cambio"

		next := advance(t, m, keyPress("enter"))

		if next.State() != StateTicketInput {
			t.Fatalf("Promote entry should reach StateTicketInput, got %v", next.State())
		}
		if next.ticket != "DEMO-2" {
			t.Errorf("ticket = %q, want DEMO-2 (seeded from originalBranch)", next.ticket)
		}
		if !next.ticketFromBranch {
			t.Error("ticketFromBranch should be true after seeding from the branch")
		}
	})

	t.Run("branch matches nothing: no seed", func(t *testing.T) {
		m := mainMenuModel(0)
		m.deps.Config = demoTicketConfig()
		m.originalBranch = "main"

		next := advance(t, m, keyPress("enter"))

		if next.ticket != "" {
			t.Errorf("ticket should stay empty, got %q", next.ticket)
		}
		if next.ticketFromBranch {
			t.Error("ticketFromBranch should stay false when nothing matched")
		}
	})

	t.Run("originalBranch empty (timing): no seed, no panic", func(t *testing.T) {
		m := mainMenuModel(0)
		m.deps.Config = demoTicketConfig()
		m.originalBranch = ""

		next := advance(t, m, keyPress("enter"))

		if next.ticket != "" {
			t.Errorf("ticket should stay empty, got %q", next.ticket)
		}
		if next.ticketFromBranch {
			t.Error("ticketFromBranch should stay false with no originalBranch")
		}
	})

	t.Run("existing ticket is never overwritten", func(t *testing.T) {
		m := mainMenuModel(0)
		m.deps.Config = demoTicketConfig()
		m.originalBranch = "DEMO-2-mi-cambio"
		m.ticket = "MANUAL-9"

		next := advance(t, m, keyPress("enter"))

		if next.ticket != "MANUAL-9" {
			t.Errorf("a pre-existing ticket must not be overwritten, got %q", next.ticket)
		}
		if next.ticketFromBranch {
			t.Error("ticketFromBranch must stay false when the ticket was not seeded")
		}
	})
}

// TestViewTicket_ShowsSuggestionHint proves the Spanish hint renders when the
// ticket buffer was auto-seeded from the branch.
func TestViewTicket_ShowsSuggestionHint(t *testing.T) {
	m := New(Deps{Dir: "/repo", Config: testConfig()})
	m.state = StateTicketInput
	m.ticket = "DEMO-2"
	m.ticketFromBranch = true

	v := m.View()
	want := "(sugerido desde la rama actual)"
	if !strings.Contains(v, want) {
		t.Errorf("viewTicket missing suggestion hint %q:\n%s", want, v)
	}
}

// TestViewTicket_NoHintWithoutFlagOrTicket proves the hint is absent both
// when the flag is false and (defensively) when the ticket buffer is empty.
func TestViewTicket_NoHintWithoutFlagOrTicket(t *testing.T) {
	const hint = "(sugerido desde la rama actual)"

	t.Run("flag false", func(t *testing.T) {
		m := New(Deps{Dir: "/repo", Config: testConfig()})
		m.state = StateTicketInput
		m.ticket = "DEMO-2"
		v := m.View()
		if strings.Contains(v, hint) {
			t.Errorf("hint should not render when ticketFromBranch is false:\n%s", v)
		}
	})

	t.Run("flag true but ticket empty", func(t *testing.T) {
		m := New(Deps{Dir: "/repo", Config: testConfig()})
		m.state = StateTicketInput
		m.ticketFromBranch = true
		v := m.View()
		if strings.Contains(v, hint) {
			t.Errorf("hint should not render with an empty ticket buffer:\n%s", v)
		}
	})
}

// TestKeyTicket_ClearsSeedFlagOnEdit is the "no longer pristine" rule: once
// the user edits the seeded buffer (typing or backspacing), the suggestion
// flag clears and the hint stops rendering.
func TestKeyTicket_ClearsSeedFlagOnEdit(t *testing.T) {
	t.Run("typing a rune clears the flag", func(t *testing.T) {
		m := New(Deps{Dir: "/repo", Config: testConfig()})
		m.state = StateTicketInput
		m.ticket = "DEMO-2"
		m.ticketFromBranch = true

		next, _ := m.Update(keyPress("x"))
		nm := next.(Model)

		if nm.ticketFromBranch {
			t.Error("typing a rune should clear ticketFromBranch")
		}
		if nm.ticket != "DEMO-2x" {
			t.Errorf("ticket = %q, want DEMO-2x", nm.ticket)
		}
	})

	t.Run("backspacing clears the flag", func(t *testing.T) {
		m := New(Deps{Dir: "/repo", Config: testConfig()})
		m.state = StateTicketInput
		m.ticket = "DEMO-2"
		m.ticketFromBranch = true

		next, _ := m.Update(keyPress("backspace"))
		nm := next.(Model)

		if nm.ticketFromBranch {
			t.Error("backspacing should clear ticketFromBranch")
		}
		if nm.ticket != "DEMO-" {
			t.Errorf("ticket = %q, want DEMO-", nm.ticket)
		}
	})
}
