package cli

import (
	"fmt"
	"sort"
	"strings"
)

type testRegexClosedGroup struct {
	group            testRegexGroup
	end, lastCapture int
	quantifier       testRegexQuantification
	nullable         bool
}

type testRegexEdit struct {
	start, end int
	kind       int // Reference replacement, suffix, then prefix at equal offsets.
	text       string
}

// A balancing capture removes the preceding value and records its undo on the
// engine's ordinary backtracking stack. Clearing at capture/iteration entry keeps
// one live value and implements ECMAScript's per-iteration capture reset.
func testRegexApplyResets(source string, groups []testRegexClosedGroup, clear map[int]bool, edits []testRegexEdit) (string, error) {
	for index, item := range groups {
		g := item.group
		var reset strings.Builder
		if clear[g.ownCapture] {
			fmt.Fprintf(&reset, "(?(%d)(?<-%d>))", g.ownCapture, g.ownCapture)
		}
		if item.quantifier.repeated() {
			for n := g.firstCapture; n <= item.lastCapture; n++ {
				if n != g.ownCapture && clear[n] {
					fmt.Fprintf(&reset, "(?(%d)(?<-%d>))", n, n)
				}
			}
		}
		if reset.Len() > 0 {
			prefix, suffix := reset.String()+"(?:", ")"
			if g.rightToLeft {
				prefix, suffix = "(?:", ")"+reset.String()
			}
			edits = append(edits,
				testRegexEdit{g.outputBody, g.outputBody, 0, prefix},
				testRegexEdit{item.end, item.end, 1, suffix})
		}
		if !item.nullable || !item.quantifier.optional() {
			continue
		}
		start, end := g.outputStart, item.end+1
		atom := testRegexRenderRange(source, start, end, edits)
		// The atom appears at most twice in this split. Limit expansion before
		// constructing the replacement, including recursively split children.
		if len(atom) > testRegexTranslationLimit/2 {
			return "", fmt.Errorf("nullable-repeat translation exceeds %d bytes", testRegexTranslationLimit)
		}
		replacement := testRegexGuardIterations(atom, item.quantifier, g.repeatRightToLeft, index)
		if len(replacement) > testRegexTranslationLimit {
			return "", fmt.Errorf("nullable-repeat translation exceeds %d bytes", testRegexTranslationLimit)
		}
		end += item.quantifier.length
		remaining := edits[:0]
		for _, edit := range edits {
			if edit.start < start || edit.end > end {
				remaining = append(remaining, edit)
			}
		}
		edits = append(remaining, testRegexEdit{start, end, 2, replacement})
	}
	result := testRegexRenderRange(source, 0, len(source), edits)
	if len(result) > testRegexTranslationLimit && len(source) <= testRegexTranslationLimit {
		return "", fmt.Errorf("capture-reset translation exceeds %d bytes", testRegexTranslationLimit)
	}
	return result, nil
}

func testRegexRenderRange(source string, start, end int, edits []testRegexEdit) string {
	selected := make([]testRegexEdit, 0, len(edits))
	for _, edit := range edits {
		if edit.start >= start && edit.end <= end {
			selected = append(selected, edit)
		}
	}
	// At a common original offset, replace a reference before surrounding it.
	sort.SliceStable(selected, func(i, j int) bool {
		if selected[i].start != selected[j].start {
			return selected[i].start > selected[j].start
		}
		return selected[i].kind > selected[j].kind
	})
	result := source[start:end]
	for _, edit := range selected {
		result = result[:edit.start-start] + edit.text + result[edit.end-start:]
	}
	return result
}
