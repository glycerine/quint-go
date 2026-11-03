package cli

import (
	"fmt"
	"sort"
	"strings"
)

type testRegexClosedGroup struct {
	group            testRegexGroup
	end, lastCapture int
	repeated         bool
}

type testRegexEdit struct {
	start, end int
	kind       int // Reference replacement, suffix, then prefix at equal offsets.
	text       string
}

// A balancing capture removes the preceding value and records its undo on the
// engine's ordinary backtracking stack. Clearing at every capture entry keeps
// at most one live value; clearing descendants at a consuming iteration's entry
// then implements JavaScript's per-iteration reset. Nullable loops are excluded
// because JavaScript discards optional iterations which make no progress.
func testRegexApplyResets(source string, groups []testRegexClosedGroup, clear map[int]bool, edits []testRegexEdit) string {
	for _, item := range groups {
		g := item.group
		var reset strings.Builder
		if clear[g.ownCapture] {
			fmt.Fprintf(&reset, "(?(%d)(?<-%d>))", g.ownCapture, g.ownCapture)
		}
		if item.repeated {
			for n := g.firstCapture; n <= item.lastCapture; n++ {
				if n != g.ownCapture && clear[n] {
					fmt.Fprintf(&reset, "(?(%d)(?<-%d>))", n, n)
				}
			}
		}
		if reset.Len() == 0 {
			continue
		}
		prefix, suffix := reset.String()+"(?:", ")"
		if g.rightToLeft {
			prefix, suffix = "(?:", ")"+reset.String()
		}
		edits = append(edits,
			testRegexEdit{g.outputBody, g.outputBody, 0, prefix},
			testRegexEdit{item.end, item.end, 1, suffix})
	}
	// All positions refer to the original translation. At a common offset a
	// reference changes first, then the suffix and prefix surround that result.
	sort.SliceStable(edits, func(i, j int) bool {
		if edits[i].start != edits[j].start {
			return edits[i].start > edits[j].start
		}
		return edits[i].kind > edits[j].kind
	})
	for _, edit := range edits {
		source = source[:edit.start] + edit.text + source[edit.end:]
	}
	return source
}
