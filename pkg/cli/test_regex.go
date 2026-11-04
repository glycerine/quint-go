package cli

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/dlclark/regexp2"
)

// Quint definition names are ASCII. This adapter implements flagless JavaScript
// name matching with regexp2's native Go ECMAScript engine. Its parser also
// accepts .NET constructs, so the translation validates the JavaScript group
// grammar, preserves lexical capture numbering, and translates identity escapes.
// Balancing captures implement per-iteration resets; progress guards discard
// optional empty iterations while preserving required empty iterations.
type testJavaScriptRegex struct{ compiled *regexp2.Regexp }

const testRegexMatchTimeout = 25 * time.Millisecond

func testCompileJavaScriptRegex(pattern string) (*testJavaScriptRegex, error) {
	translated, err := testTranslateJavaScriptRegex(pattern)
	if err != nil {
		return nil, err
	}
	compiled, err := regexp2.Compile(translated, regexp2.ECMAScript)
	if err != nil {
		return nil, fmt.Errorf("invalid or unsupported --match expression %q: %w", pattern, err)
	}
	compiled.MatchTimeout = testRegexMatchTimeout
	return &testJavaScriptRegex{compiled}, nil
}

func (r *testJavaScriptRegex) MatchString(ctx context.Context, name string) (matched bool, err error) {
	// Some valid ECMAScript expressions trigger an upstream engine backtracking
	// panic. Treat these as unsupported expressions and abort test discovery.
	defer func() {
		if recover() != nil {
			matched = false
			if err = ctx.Err(); err == nil {
				err = fmt.Errorf("unsupported --match expression: regular-expression engine could not evaluate this form")
			}
		}
	}()
	if err := ctx.Err(); err != nil {
		return false, err
	}
	for _, c := range name {
		if c > 127 {
			return false, fmt.Errorf("--match supports ASCII Quint definition names")
		}
	}
	matched, err = r.compiled.MatchString(name)
	if canceled := ctx.Err(); canceled != nil {
		return false, canceled
	}
	if err != nil {
		return false, fmt.Errorf("--match exceeded its regular-expression execution budget: %w", err)
	}
	return matched, nil
}

type testRegexGroup struct {
	firstCapture, ownCapture int
	lookbehind               bool
	body                     int
	outputStart, outputBody  int
	rightToLeft, assertion   bool
	repeatRightToLeft        bool
}

type testRegexReference struct {
	start, end int
	name       string
	open       []int
}

func testTranslateJavaScriptRegex(pattern string) (string, error) {
	invalid := func(reason string) error { return fmt.Errorf("invalid --match expression %q: %s", pattern, reason) }
	unsupported := func(reason string) error { return fmt.Errorf("unsupported --match expression %q: %s", pattern, reason) }
	var out strings.Builder
	groups := []testRegexGroup{}
	named := map[string]int{}
	clear := map[int]bool{}
	closedGroups := []testRegexClosedGroup{}
	edits := []testRegexEdit{}
	references := []testRegexReference{}
	captures := 0
	hasNamedCapture := testRegexHasNamedCapture(pattern)
	addReference := func(start, end int, name string) {
		var open []int
		for _, group := range groups {
			if group.ownCapture != 0 {
				open = append(open, group.ownCapture)
			}
		}
		references = append(references, testRegexReference{start, end, name, open})
	}
	for i := 0; i < len(pattern); {
		c := pattern[i]
		if c == '\\' {
			start := out.Len()
			if i+1 == len(pattern) {
				return "", invalid("trailing escape")
			}
			escape := pattern[i+1]
			i += 2
			switch {
			case escape >= '1' && escape <= '9':
				begin := i - 1
				for i < len(pattern) && pattern[i] >= '0' && pattern[i] <= '9' {
					i++
				}
				out.WriteByte('\\')
				out.WriteString(pattern[begin:i])
				addReference(start, out.Len(), pattern[begin:i])
			case escape == 'k':
				if !hasNamedCapture {
					out.WriteString(`\x6b`)
					break
				}
				if i < len(pattern) && pattern[i] == '<' {
					end := strings.IndexByte(pattern[i+1:], '>')
					if end >= 0 {
						end += i + 1
						out.WriteString(pattern[i-2 : end+1])
						addReference(start, out.Len(), "k:"+pattern[i+1:end])
						i = end + 1
						break
					}
				}
				out.WriteString(`\x6b`)
				addReference(start, out.Len(), "k!")
			case escape == 'c':
				if i < len(pattern) && testRegexASCIIAlpha(pattern[i]) {
					fmt.Fprintf(&out, `\x%02x`, pattern[i]%32)
					i++
				} else {
					out.WriteString(`\\c`)
				}
			case strings.ContainsRune("bBdDsSwWfnrtv0", rune(escape)) || escape >= '1' && escape <= '9':
				out.WriteByte('\\')
				out.WriteByte(escape)
			case escape == 'u' || escape == 'x':
				count := 2
				if escape == 'u' {
					count = 4
				}
				if i+count <= len(pattern) && testRegexHex(pattern[i:i+count]) {
					out.WriteByte('\\')
					out.WriteByte(escape)
					out.WriteString(pattern[i : i+count])
					i += count
				} else {
					fmt.Fprintf(&out, `\x%02x`, escape)
				}
			default:
				// Flagless JavaScript treats other escapes as their literal
				// character, including .NET anchors and Unicode property escapes.
				if escape < utf8.RuneSelf {
					fmt.Fprintf(&out, `\x%02x`, escape)
				} else {
					r, size := utf8.DecodeRuneInString(pattern[i-1:])
					testRegexWriteRune(&out, r)
					i += size - 1
				}
			}
			continue
		}
		if c >= utf8.RuneSelf {
			r, size := utf8.DecodeRuneInString(pattern[i:])
			testRegexWriteRune(&out, r)
			i += size
			continue
		}
		switch c {
		case '[':
			class, end, err := testRegexTranslateClass(pattern, i)
			if err != nil {
				return "", invalid(err.Error())
			}
			out.WriteString(class)
			i = end
		case '(':
			group := testRegexGroup{firstCapture: captures + 1, outputStart: out.Len()}
			if len(groups) > 0 {
				group.rightToLeft = groups[len(groups)-1].rightToLeft
			}
			group.repeatRightToLeft = group.rightToLeft
			i++
			if i >= len(pattern) || pattern[i] != '?' {
				captures++
				group.ownCapture = captures
				fmt.Fprintf(&out, "(?<%d>", captures)
			} else {
				i++
				if i >= len(pattern) {
					return "", invalid("incomplete group")
				}
				switch pattern[i] {
				case ':', '=', '!':
					if pattern[i] != ':' {
						group.assertion, group.rightToLeft = true, false
					}
					out.WriteString("(?")
					out.WriteByte(pattern[i])
					i++
				case '<':
					i++
					if i < len(pattern) && (pattern[i] == '=' || pattern[i] == '!') {
						group.lookbehind = true
						group.assertion, group.rightToLeft = true, true
						out.WriteString("(?<")
						out.WriteByte(pattern[i])
						i++
					} else {
						end := strings.IndexByte(pattern[i:], '>')
						if end < 0 {
							return "", invalid("unterminated capture name")
						}
						end += i
						name, valid := testRegexCaptureName(pattern[i:end])
						if !valid {
							return "", invalid("invalid capture name")
						}
						if _, exists := named[name]; exists {
							return "", invalid("duplicate capture name")
						}
						captures++
						group.ownCapture = captures
						named[name] = captures
						fmt.Fprintf(&out, "(?<%d>", captures)
						i = end + 1
					}
				default:
					return "", invalid("group syntax is not supported by JavaScript")
				}
			}
			group.body = i
			group.outputBody = out.Len()
			groups = append(groups, group)
		case ')':
			if len(groups) == 0 {
				return "", invalid("unmatched closing group")
			}
			group := groups[len(groups)-1]
			groups = groups[:len(groups)-1]
			quantifier := testRegexReadQuantifier(pattern[i+1:])
			if group.lookbehind && quantifier.present {
				return "", invalid("lookbehind cannot be quantified")
			}
			consuming := !group.assertion && testRegexMustConsume(pattern[group.body:i], hasNamedCapture)
			closedGroups = append(closedGroups, testRegexClosedGroup{group, out.Len(), captures, quantifier, !consuming})
			out.WriteByte(c)
			i++
		default:
			out.WriteByte(c)
			i++
		}
	}
	if len(groups) != 0 {
		return "", invalid("unterminated character class or group")
	}
	translated := out.String()
	for i := len(references) - 1; i >= 0; i-- {
		reference := references[i]
		n := 0
		replacement := translated[reference.start:reference.end]
		if reference.name == "k!" {
			if len(named) > 0 {
				return "", invalid("malformed named capture reference")
			}
		} else if name, namedReference := strings.CutPrefix(reference.name, "k:"); namedReference {
			name, valid := testRegexCaptureName(name)
			if !valid {
				return "", invalid("invalid capture reference name")
			}
			var exists bool
			n, exists = named[name]
			if !exists {
				return "", invalid("reference to unknown capture name")
			}
			replacement = fmt.Sprintf(`(?:\%d)`, n)
		} else {
			var err error
			n, err = strconv.Atoi(reference.name)
			if err != nil || n > captures {
				n = 0
				// Freeze Annex B octal/identity escapes before internal guard
				// captures increase the native engine's capture count.
				replacement = testRegexLegacyDecimal(reference.name)
			}
		}
		// A reference to its still-open enclosing capture is always empty in
		// JavaScript, including a later iteration of a repeated group. .NET can
		// retain the preceding iteration's capture, so make the empty atom explicit.
		for _, open := range reference.open {
			if n == open {
				replacement, n = "(?:)", 0
				break
			}
		}
		edits = append(edits, testRegexEdit{reference.start, reference.end, 2, replacement})
	}
	for n := 1; n <= captures; n++ {
		clear[n] = true
	}
	result, err := testRegexApplyResets(translated, closedGroups, clear, edits)
	if err != nil {
		return "", unsupported(err.Error())
	}
	return result, nil
}

func testRegexASCIIAlpha(c byte) bool { return c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' }

// JavaScript without a Unicode flag matches UTF-16 code units. Escaping each
// surrogate separately also makes a following quantifier apply to the low unit.
func testRegexWriteRune(out *strings.Builder, r rune) {
	if r > 0xffff {
		hi, lo := utf16.EncodeRune(r)
		fmt.Fprintf(out, `\u%04x\u%04x`, hi, lo)
	} else {
		out.WriteRune(r)
	}
}

func testRegexHasNamedCapture(pattern string) bool {
	inClass := false
	for i := 0; i < len(pattern); i++ {
		switch pattern[i] {
		case '\\':
			i++
		case '[':
			inClass = true
		case ']':
			inClass = false
		case '(':
			if !inClass && i+3 < len(pattern) && pattern[i+1:i+3] == "?<" && pattern[i+3] != '=' && pattern[i+3] != '!' {
				return true
			}
		}
	}
	return false
}

func testRegexHex(s string) bool {
	for _, c := range s {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F') {
			return false
		}
	}
	return true
}
func testRegexQuantifier(s string) (present, repeated, optional bool) {
	q := testRegexReadQuantifier(s)
	return q.present, q.repeated(), q.optional()
}
