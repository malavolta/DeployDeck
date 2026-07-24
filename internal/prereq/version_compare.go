package prereq

import (
	"strconv"
	"strings"
)

// compareVersions compares two dotted version strings numerically, segment
// by segment (e.g. "2.9.0" < "2.10.0", unlike a plain string compare). It
// returns -1, 0, or 1. A missing trailing segment is treated as 0 ("2.43"
// == "2.43.0"). Non-numeric segments fall back to a lexical comparison of
// that segment, so oddly formatted versions degrade gracefully instead of
// panicking.
func compareVersions(a, b string) int {
	as := splitVersion(a)
	bs := splitVersion(b)

	n := len(as)
	if len(bs) > n {
		n = len(bs)
	}

	for i := 0; i < n; i++ {
		av, bv := "0", "0"
		if i < len(as) {
			av = as[i]
		}
		if i < len(bs) {
			bv = bs[i]
		}

		ai, aErr := strconv.Atoi(av)
		bi, bErr := strconv.Atoi(bv)
		if aErr == nil && bErr == nil {
			if ai != bi {
				if ai < bi {
					return -1
				}
				return 1
			}
			continue
		}

		if av != bv {
			if av < bv {
				return -1
			}
			return 1
		}
	}

	return 0
}

func splitVersion(v string) []string {
	v = strings.TrimPrefix(strings.TrimSpace(v), "v")
	if v == "" {
		return nil
	}
	return strings.Split(v, ".")
}
