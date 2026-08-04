package git_test

import (
	"testing"

	"github.com/malavolta/DeployDeck/internal/git"
)

// TestParseCherryPickTrailers_SingleTrailer is task 1.1 (RED):
// ParseCherryPickTrailers extracts the source SHA from a single
// "(cherry picked from commit <sha>)" trailer written by `-x`
// (incremental-promotion spec: "Commit reapplied under a different SHA is
// detected via the trailer").
func TestParseCherryPickTrailers_SingleTrailer(t *testing.T) {
	log := []byte(`commit aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa
Author: ana <ana@example.com>
Date:   Mon Jan 1 00:00:00 2024 +0000

    PROJ-1 fix bug

    (cherry picked from commit 1111111111111111111111111111111111111111)
`)
	got := git.ParseCherryPickTrailers(log)
	if !got["1111111111111111111111111111111111111111"] {
		t.Fatalf("expected the trailer's source SHA to be marked present, got %v", got)
	}
	if len(got) != 1 {
		t.Fatalf("expected exactly 1 entry, got %d: %v", len(got), got)
	}
}

// TestParseCherryPickTrailers_NoTrailer is task 1.1 (RED): a log with no
// cherry-pick provenance trailer at all yields an empty (non-nil-required)
// map, never a panic.
func TestParseCherryPickTrailers_NoTrailer(t *testing.T) {
	log := []byte(`commit aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa
Author: ana <ana@example.com>
Date:   Mon Jan 1 00:00:00 2024 +0000

    PROJ-1 fix bug, no cherry-pick provenance
`)
	got := git.ParseCherryPickTrailers(log)
	if len(got) != 0 {
		t.Fatalf("expected no trailers, got %v", got)
	}
}

// TestParseCherryPickTrailers_MultilineLogWithMixedTrailers is task 1.1
// (RED): a multi-commit log where only SOME commits carry the trailer
// yields exactly the trailer-bearing SHAs, ignoring commits with none.
func TestParseCherryPickTrailers_MultilineLogWithMixedTrailers(t *testing.T) {
	log := []byte(`commit 3333333333333333333333333333333333333333
Author: ana <ana@example.com>
Date:   Mon Jan 1 00:00:00 2024 +0000

    PROJ-1 add A

    (cherry picked from commit 1111111111111111111111111111111111111111)

commit 4444444444444444444444444444444444444444
Author: ana <ana@example.com>
Date:   Mon Jan 2 00:00:00 2024 +0000

    unrelated commit, no trailer

commit 5555555555555555555555555555555555555555
Author: ana <ana@example.com>
Date:   Mon Jan 3 00:00:00 2024 +0000

    PROJ-1 add B

    (cherry picked from commit 2222222222222222222222222222222222222222)
`)
	got := git.ParseCherryPickTrailers(log)
	for _, want := range []string{
		"1111111111111111111111111111111111111111",
		"2222222222222222222222222222222222222222",
	} {
		if !got[want] {
			t.Errorf("expected %q to be marked present, got %v", want, got)
		}
	}
	if len(got) != 2 {
		t.Fatalf("expected exactly 2 entries, got %d: %v", len(got), got)
	}
}

// TestParseCherryPickTrailers_MalformedTrailerIgnoredNoPanic is task 1.1
// (RED): a trailer-shaped line with no valid hex SHA (or an empty SHA) is
// silently ignored rather than matched or panicking.
func TestParseCherryPickTrailers_MalformedTrailerIgnoredNoPanic(t *testing.T) {
	log := []byte(`commit aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa
Author: ana <ana@example.com>
Date:   Mon Jan 1 00:00:00 2024 +0000

    PROJ-1 fix bug

    (cherry picked from commit not-a-real-sha)
    (cherry picked from commit )
`)
	var got map[string]bool
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("ParseCherryPickTrailers panicked on malformed input: %v", r)
			}
		}()
		got = git.ParseCherryPickTrailers(log)
	}()
	if len(got) != 0 {
		t.Fatalf("expected malformed trailer lines to be ignored, got %v", got)
	}
}
