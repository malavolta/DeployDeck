package app

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/charmbracelet/x/ansi"

	"github.com/malavolta/DeployDeck/internal/salesforce"
)

// failureKind is the single cause a report is classified under.
type failureKind int

const (
	// causeNone: not a failure (succeeded, partial, or still running).
	causeNone failureKind = iota
	causeCanceled
	causeMetadata
	causeTests
	causeCoverage
	causeOrgError
	causeGeneric
)

// verdict is the classifier output: the cause, a one-line headline (already
// prefixed with a mark() token) and an optional explanatory hint.
type verdict struct {
	kind     failureKind
	headline string
	hint     string
}

// isFailure reports whether the verdict carries a failure headline.
func (v verdict) isFailure() bool { return v.kind != causeNone }

// zeroCoverageListMax caps the 0%-coverage class names named in the hint.
const zeroCoverageListMax = 5

var coverageWarningPattern = regexp.MustCompile(`(\d+)%.*?(\d+)%`)

// classify maps a parsed report to one cause. Priority: canceled > metadata >
// tests > coverage > org error > generic. Only a Failed or Canceled status
// yields a failure verdict; Succeeded, SucceededPartial and non-terminal
// statuses get an informational headline and kind causeNone. Pure: org text is
// sanitized, no I/O.
func classify(r salesforce.DeployReport) verdict {
	switch r.Status {
	case "Canceled":
		h := mark("XX") + " Cancelado"
		if r.CanceledByName != "" {
			h += " por " + sanitizeTermLine(r.CanceledByName)
		}
		return verdict{kind: causeCanceled, headline: h}
	case "Failed":
		return classifyFailed(r)
	case "Succeeded":
		return verdict{headline: mark("OK") + " Validación exitosa"}
	case "SucceededPartial":
		return verdict{headline: mark("!!") + " Resultado parcial"}
	default:
		return verdict{headline: mark("i") + " En curso"}
	}
}

func classifyFailed(r salesforce.DeployReport) verdict {
	xx := mark("XX")
	if hasMetadataError(r) {
		n := r.NumberComponentErrors
		if n == 0 {
			n = countErrorComponents(r)
		}
		return verdict{kind: causeMetadata, headline: fmt.Sprintf("%s Falló por errores de metadata: %d componente(s)", xx, n)}
	}
	if len(r.TestFailures) > 0 || r.NumberTestErrors > 0 {
		n := r.NumberTestErrors
		if n == 0 {
			n = len(r.TestFailures)
		}
		return verdict{kind: causeTests, headline: fmt.Sprintf("%s Falló por tests: %d fallido(s)", xx, n)}
	}
	if len(r.CodeCoverageWarnings) > 0 {
		return verdict{kind: causeCoverage, headline: coverageHeadline(r), hint: coverageHint(r)}
	}
	if r.ErrorMessage != "" {
		return verdict{kind: causeOrgError, headline: xx + " Falló: " + sanitizeTermLine(r.ErrorMessage)}
	}
	return verdict{kind: causeGeneric, headline: xx + " La validación falló sin detalle estructurado"}
}

func countErrorComponents(r salesforce.DeployReport) int {
	n := 0
	for _, f := range r.ComponentFailures {
		if f.ProblemType != "Warning" {
			n++
		}
	}
	return n
}

func hasMetadataError(r salesforce.DeployReport) bool {
	return r.NumberComponentErrors > 0 || countErrorComponents(r) > 0
}

func coverageHeadline(r salesforce.DeployReport) string {
	for _, w := range r.CodeCoverageWarnings {
		if w.Name != "" {
			continue
		}
		if m := coverageWarningPattern.FindStringSubmatch(w.Message); m != nil {
			return fmt.Sprintf("%s Falló por cobertura: %s%% global < %s%%", mark("XX"), m[1], m[2])
		}
	}
	return mark("XX") + " Falló por cobertura de código"
}

func coverageHint(r salesforce.DeployReport) string {
	hint := "El promedio global incluye todas las clases Apex de la org, no solo las de este run."
	var zero []string
	for _, c := range r.CodeCoverage {
		if c.NumLocations > 0 && c.Percent() == 0 {
			zero = append(zero, sanitizeTermLine(c.Name))
		}
	}
	if len(zero) == 0 {
		return hint
	}
	shown, extra := zero, 0
	if len(shown) > zeroCoverageListMax {
		extra = len(shown) - zeroCoverageListMax
		shown = shown[:zeroCoverageListMax]
	}
	hint += " Clases al 0%: " + strings.Join(shown, ", ")
	if extra > 0 {
		hint += fmt.Sprintf(" +%d más", extra)
	}
	return hint + "."
}

// zeroListMarker starts the 0%-coverage class list inside a hint; failureBlock
// puts it on its own line(s) so a long class list is never glued to the
// explanatory sentence.
const zeroListMarker = "Clases al 0%:"

// wrapIndented word-wraps text to width cells, prefixing every line with the
// 2-space indent (continuations included). width <= 0 leaves it on one line.
// A single word longer than the room is kept whole (never split or dropped).
func wrapIndented(text string, width int) []string {
	const indent = "  "
	words := strings.Fields(text)
	if width <= 0 || len(words) == 0 {
		return []string{indent + strings.Join(words, " ")}
	}
	var out []string
	cur := ""
	for _, w := range words {
		switch {
		case cur == "":
			cur = w
		case ansi.StringWidth(indent+cur+" "+w) <= width:
			cur += " " + w
		default:
			out = append(out, indent+cur)
			cur = w
		}
	}
	return append(out, indent+cur)
}

// wrapLine word-wraps one 2-space-indented body line to width with a 4-space
// hanging indent on continuations. width <= 0 or a line that already fits is
// returned untouched (byte-identical to the unwrapped output).
func wrapLine(line string, width int) string {
	if width <= 0 || ansi.StringWidth(line) <= width {
		return line
	}
	ls := wrapIndented(strings.TrimPrefix(line, "  "), width-2)
	for i := 1; i < len(ls); i++ {
		ls[i] = "  " + ls[i]
	}
	return strings.Join(ls, "\n")
}

// hintLines splits a hint into its sentence and 0%-class list, wrapped to
// width, one muted string per rendered line.
func hintLines(hint string, width int) []string {
	paras := []string{hint}
	if i := strings.Index(hint, zeroListMarker); i > 0 {
		paras = []string{strings.TrimSpace(hint[:i]), hint[i:]}
	}
	var out []string
	for _, p := range paras {
		for _, l := range wrapIndented(p, width) {
			out = append(out, "  "+styleDim.Render(strings.TrimPrefix(l, "  ")))
		}
	}
	return out
}

// failureBlock renders a failure verdict's headline (and hint, muted, wrapped
// to width) as an indented block; empty for a non-failure verdict so callers
// can prepend it unconditionally. width 0 (unsized) leaves the hint unwrapped.
func failureBlock(v verdict, width int) string {
	if !v.isFailure() {
		return ""
	}
	out := "\n  " + v.headline + "\n"
	if v.hint != "" {
		out += strings.Join(hintLines(v.hint, width), "\n") + "\n"
	}
	return out
}
