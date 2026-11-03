package cli

import (
	"strings"
	"unicode/utf8"
)

// Report whether every successful path consumes text. Backreferences are
// conservatively empty: their value can be unset or an empty capture. Assertions
// consume nothing, even when their nested expression consumes text.
func testRegexMustConsume(source string, hasNamedCapture bool) bool {
	r := testRegexConsumptionReader{source: source, hasNamedCapture: hasNamedCapture}
	return r.disjunction()
}

type testRegexConsumptionReader struct {
	source          string
	index           int
	hasNamedCapture bool
}

func (r *testRegexConsumptionReader) disjunction() bool {
	allBranches, branch := true, false
	for r.index < len(r.source) && r.source[r.index] != ')' {
		if r.source[r.index] == '|' {
			allBranches = allBranches && branch
			branch = false
			r.index++
			continue
		}
		consumes := r.atom()
		if r.index < len(r.source) {
			switch r.source[r.index] {
			case '*', '?':
				consumes = false
				r.index++
			case '+':
				r.index++
			case '{':
				if end, zero := r.countedQuantifier(); end > r.index {
					consumes = consumes && !zero
					r.index = end
				}
			}
			if r.index < len(r.source) && r.source[r.index] == '?' {
				r.index++ // Lazy quantification has the same minimum width.
			}
		}
		branch = branch || consumes
	}
	return allBranches && branch
}

func (r *testRegexConsumptionReader) atom() bool {
	c := r.source[r.index]
	r.index++
	switch c {
	case '^', '$':
		return false
	case '[':
		_, end, err := testRegexTranslateClass(r.source, r.index-1)
		if err != nil {
			r.index = len(r.source)
			return false
		}
		r.index = end
		return true
	case '(':
		assertion := false
		if r.index < len(r.source) && r.source[r.index] == '?' {
			r.index++
			if r.index < len(r.source) {
				switch r.source[r.index] {
				case '=', '!':
					assertion = true
					r.index++
				case ':':
					r.index++
				case '<':
					r.index++
					if r.index < len(r.source) && (r.source[r.index] == '=' || r.source[r.index] == '!') {
						assertion = true
						r.index++
					} else if end := strings.IndexByte(r.source[r.index:], '>'); end >= 0 {
						r.index += end + 1
					}
				}
			}
		}
		consumes := r.disjunction()
		if r.index < len(r.source) && r.source[r.index] == ')' {
			r.index++
		}
		return consumes && !assertion
	case '\\':
		if r.index == len(r.source) {
			return false
		}
		escape := r.source[r.index]
		r.index++
		switch {
		case escape == 'b' || escape == 'B':
			return false
		case escape >= '0' && escape <= '9':
			for r.index < len(r.source) && r.source[r.index] >= '0' && r.source[r.index] <= '9' {
				r.index++
			}
			return false
		case escape == 'k':
			if r.hasNamedCapture && r.index < len(r.source) && r.source[r.index] == '<' {
				if end := strings.IndexByte(r.source[r.index:], '>'); end >= 0 {
					r.index += end + 1
				}
			}
			return false
		case escape == 'c':
			if r.index < len(r.source) && testRegexASCIIAlpha(r.source[r.index]) {
				r.index++
			}
		case escape == 'u' || escape == 'x':
			count := 2
			if escape == 'u' {
				count = 4
			}
			if r.index+count <= len(r.source) && testRegexHex(r.source[r.index:r.index+count]) {
				r.index += count
			}
		case escape >= utf8.RuneSelf:
			_, size := utf8.DecodeRuneInString(r.source[r.index-1:])
			r.index += size - 1
		}
		return true
	default:
		if c >= utf8.RuneSelf {
			_, size := utf8.DecodeRuneInString(r.source[r.index-1:])
			r.index += size - 1
		}
		return true
	}
}

// The source is already validated by the main translator. Recognize only
// decimal counted quantifiers; other brace forms are JavaScript literals.
func (r *testRegexConsumptionReader) countedQuantifier() (end int, zero bool) {
	i, begin := r.index+1, r.index+1
	zero = true
	for i < len(r.source) && r.source[i] >= '0' && r.source[i] <= '9' {
		zero = zero && r.source[i] == '0'
		i++
	}
	if i == begin {
		return r.index, false
	}
	if i < len(r.source) && r.source[i] == ',' {
		i++
		for i < len(r.source) && r.source[i] >= '0' && r.source[i] <= '9' {
			i++
		}
	}
	if i < len(r.source) && r.source[i] == '}' {
		return i + 1, zero
	}
	return r.index, false
}
