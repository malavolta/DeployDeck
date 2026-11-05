package app

import (
	"testing"

	"github.com/charmbracelet/bubbles/key"
)

func TestHint_CopiesWithoutMutatingTheVocabulary(t *testing.T) {
	before := keys.Retry.Help()
	h := hint(keys.Retry, "r", "reintentar", false)
	if h.Enabled() || h.Help().Key != "r" || h.Help().Desc != "reintentar" {
		t.Fatalf("hint must carry help and enablement: %+v enabled=%v", h.Help(), h.Enabled())
	}
	if !keys.Retry.Enabled() || keys.Retry.Help() != before {
		t.Fatal("hint must not mutate the shared binding")
	}
}

func TestFooterOf(t *testing.T) {
	on := func(k, d string) key.Binding { return hint(keys.Retry, k, d, true) }
	off := func(k, d string) key.Binding { return hint(keys.Retry, k, d, false) }
	helpless := key.NewBinding(key.WithKeys("z"))
	cases := []struct {
		name string
		bs   []key.Binding
		want string
	}{
		{"none", nil, ""},
		{"all disabled", []key.Binding{off("a", "x"), off("b", "y")}, ""},
		{"help-less only", []key.Binding{helpless}, ""},
		{"one", []key.Binding{on("a", "uno")}, "\n[ a uno ]\n"},
		{"three space separator", []key.Binding{on("a", "uno"), on("b", "dos")}, "\n[ a uno   b dos ]\n"},
		{"leading disabled leaves no separator", []key.Binding{off("a", "x"), on("b", "dos")}, "\n[ b dos ]\n"},
		{"middle disabled leaves one separator", []key.Binding{on("a", "uno"), off("b", "x"), helpless, on("c", "tres")}, "\n[ a uno   c tres ]\n"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := footerOf(c.bs...); got != c.want {
				t.Errorf("footerOf = %q, want %q", got, c.want)
			}
		})
	}
}
