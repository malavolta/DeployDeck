package app

import (
	"os"
	"strings"
	"testing"
)

// TestFooterSource_NoLiteralFooters: every footer is derived from bindings.
func TestFooterSource_NoLiteralFooters(t *testing.T) {
	src, err := os.ReadFile("view.go")
	if err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(string(src), `footer("`); n != 0 {
		t.Errorf(`view.go still has %d literal footer(" call sites`, n)
	}
}
