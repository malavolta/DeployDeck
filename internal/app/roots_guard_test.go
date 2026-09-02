package app

// roots_guard_test.go is design.md ADR-6's compensating control: ADR-3
// deliberately keeps Deps.Dir as a real, non-deleted field (so the 241
// existing Deps{Dir: ...} test literals stay compiling unchanged) rather
// than deleting it to force compiler enumeration of every read site. That
// trade gives up compile-time enforcement that deps.Dir is read ONLY inside
// normalizeRoots — this test buys the invariant back as an EXECUTABLE
// check instead of trusting a one-off grep during apply.
//
// Adding this new _test.go file is safe against boundary_test.go's
// forbidden-import allow-list: build.ImportDir(".", 0) reports a package's
// own (non-test) file imports in pkg.Imports, and a _test.go file's
// imports land in pkg.TestImports/XTestImports instead — never pkg.Imports
// — so this file's own imports (go/build, os, regexp, strings) can never
// trip TestApp_NeverImportsExecSeam's forbidden list, whatever they are.

import (
	"go/build"
	"os"
	"regexp"
	"strings"
	"testing"
)

// depsDirPattern matches any read of the Deps.Dir compatibility base field,
// however it is spelled at the call site ("m.deps.Dir", a bare "deps.Dir"
// inside a helper taking *Deps, etc.) — the common suffix ".deps.Dir" (or
// its receiver-less "deps.Dir" form) is what every such read shares.
var depsDirPattern = regexp.MustCompile(`\bdeps\.Dir\b`)

// TestNoDirReadsOutsideNormalizeRoots scans every non-test .go file this
// package owns and fails if deps.Dir is read anywhere outside
// normalizeRoots's own body — the ONLY place design.md ADR-3 permits it.
// Every other site must read one of the three named roots (GitRoot/
// ProjectDir/ArtifactsRoot) instead.
func TestNoDirReadsOutsideNormalizeRoots(t *testing.T) {
	pkg, err := build.ImportDir(".", 0)
	if err != nil {
		t.Fatalf("importing internal/app package: %v", err)
	}
	if len(pkg.GoFiles) == 0 {
		t.Fatal("expected at least one non-test .go file in internal/app — guard would be vacuous")
	}

	for _, name := range pkg.GoFiles {
		data, err := os.ReadFile(name)
		if err != nil {
			t.Fatalf("reading %s: %v", name, err)
		}
		src := string(data)
		if name == "app.go" {
			src = stripNormalizeRootsBody(t, src)
		}

		for i, line := range strings.Split(src, "\n") {
			if depsDirPattern.MatchString(line) {
				t.Errorf(
					"%s:~%d reads deps.Dir outside normalizeRoots (ADR-6 violation): %q — route through GitRoot/ProjectDir/ArtifactsRoot instead",
					name, i+1, strings.TrimSpace(line),
				)
			}
		}
	}
}

// stripNormalizeRootsBody removes normalizeRoots's own function body from
// src so its two INTENTIONAL deps.Dir reads (the fallback assignments
// themselves — the sanctioned exception) never trip the guard above.
func stripNormalizeRootsBody(t *testing.T, src string) string {
	t.Helper()
	const marker = "func normalizeRoots(deps *Deps) {"

	start := strings.Index(src, marker)
	if start < 0 {
		t.Fatal("normalizeRoots not found in app.go — guard cannot verify the ADR-6 boundary")
	}

	openBrace := start + len(marker) - 1 // marker itself ends in "{"
	depth := 0
	end := -1
	for i := openBrace; i < len(src); i++ {
		switch src[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				end = i + 1
			}
		}
		if end != -1 {
			break
		}
	}
	if end == -1 {
		t.Fatal("could not find the end of normalizeRoots's body — guard cannot verify the ADR-6 boundary")
	}

	return src[:start] + src[end:]
}
