package ai

// capRunes truncates s to at most max runes (never bytes, so a truncation
// never splits a multi-byte character), returning s unchanged when it is
// already short enough.
func capRunes(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max])
}
