package quint

import "strings"

// PostprocessTLA repairs the upstream printer's single-angle action syntax.
// Comments, strings, and already-correct double-angle actions are preserved.
func PostprocessTLA(tla string) string {
	return FixAppliedLetOperators(FixAngleActions(tla))
}

// FixAppliedLetOperators rewrites `(LET D IN f)(args)`, which SANY and TLC reject,
// as `(LET D IN f(args))`. The rewrite applies only when f is an operator with
// parameters defined by D and args is non-empty, so meaning is unchanged.
func FixAppliedLetOperators(tla string) string {
	for {
		rewritten, changed := rewriteFirstAppliedLet(tla)
		if !changed {
			return tla
		}
		tla = rewritten
	}
}

// rewriteFirstAppliedLet performs at most one rewrite, scanning code outside comments and strings.
func rewriteFirstAppliedLet(tla string) (string, bool) {
	code := tlaCodeMask(tla)
	for start := 0; start < len(tla); start++ {
		if tla[start] != '(' || !code[start] || !strings.HasPrefix(tla[start+1:], "LET") {
			continue
		}
		if start+4 >= len(tla) || !isTLASpace(tla[start+4]) {
			continue
		}
		end := matchingTLAClose(tla, code, start, '(', ')')
		if end < 0 || end+1 >= len(tla) || tla[end+1] != '(' || !code[end+1] {
			continue
		}
		callEnd := matchingTLAClose(tla, code, end+1, '(', ')')
		if callEnd < 0 {
			continue
		}
		args := strings.TrimSpace(tla[end+2 : callEnd])
		body := tla[start+1 : end]
		trimmed := strings.TrimRight(body, " \t\r\n")
		name := trailingTLAIdentifier(trimmed)
		if name == "" || args == "" {
			continue
		}
		head := strings.TrimRight(trimmed[:len(trimmed)-len(name)], " \t\r\n")
		if !strings.HasSuffix(head, "IN") || (len(head) > 2 && isTLAIdentifierByte(head[len(head)-3])) {
			continue
		}
		if !strings.Contains(body, name+"(") {
			continue
		}
		rewritten := tla[:start] + "(" + trimmed + "(" + args + ")" + body[len(trimmed):] + ")" + tla[callEnd+1:]
		return rewritten, true
	}
	return tla, false
}

// matchingTLAClose returns the index of the parenthesis closing the one at open, or -1.
func matchingTLAClose(tla string, code []bool, open int, opener, closer byte) int {
	depth := 0
	for i := open; i < len(tla); i++ {
		if !code[i] {
			continue
		}
		switch tla[i] {
		case opener:
			depth++
		case closer:
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

func trailingTLAIdentifier(text string) string {
	i := len(text)
	for i > 0 && isTLAIdentifierByte(text[i-1]) {
		i--
	}
	if i == len(text) || (text[i] >= '0' && text[i] <= '9') {
		return ""
	}
	return text[i:]
}

func isTLAIdentifierByte(c byte) bool {
	return (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '_'
}

func isTLASpace(c byte) bool { return c == ' ' || c == '\t' || c == '\n' || c == '\r' }

// FixAngleActions turns <A>_v into <<A>>_v outside comments and strings.
func FixAngleActions(tla string) string {
	code := tlaCodeMask(tla)
	positions := map[int]bool{}
	for from := 0; from < len(tla); {
		offset := strings.Index(tla[from:], ">_")
		if offset < 0 {
			break
		}
		close := from + offset
		from = close + 1
		if !code[close] || (close > 0 && tla[close-1] == '>') {
			continue
		}
		i := close - 1
		for i >= 0 {
			c := tla[i]
			if (c == ')' || c == ']') && code[i] {
				open := byte('(')
				if c == ']' {
					open = '['
				}
				depth, matching := 0, -1
				for j := i; j >= 0; j-- {
					if !code[j] {
						continue
					}
					if tla[j] == c {
						depth++
					} else if tla[j] == open {
						depth--
						if depth == 0 {
							matching = j
							break
						}
					}
				}
				if matching < 0 {
					i = close - 1
					break
				}
				i = matching - 1
			} else if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '_' || c == '!' || c == '\'' {
				i--
			} else {
				break
			}
		}
		start, open := i+1, i
		if start < close && open >= 0 && tla[open] == '<' && code[open] && (open == 0 || tla[open-1] != '<') {
			positions[open], positions[close] = true, true
		}
	}
	var out strings.Builder
	for i := 0; i < len(tla); i++ {
		out.WriteByte(tla[i])
		if positions[i] {
			out.WriteByte(tla[i])
		}
	}
	return out.String()
}

func tlaCodeMask(tla string) []bool {
	code := make([]bool, len(tla))
	for i := range code {
		code[i] = true
	}
	depth := 0
	for i := 0; i < len(tla); {
		if depth > 0 {
			switch {
			case strings.HasPrefix(tla[i:], "(*"):
				depth++
				code[i], code[i+1] = false, false
				i += 2
			case strings.HasPrefix(tla[i:], "*)"):
				depth--
				code[i], code[i+1] = false, false
				i += 2
			default:
				code[i] = false
				i++
			}
		} else if strings.HasPrefix(tla[i:], "(*") {
			depth = 1
			code[i], code[i+1] = false, false
			i += 2
		} else if strings.HasPrefix(tla[i:], `\*`) {
			for i < len(tla) && tla[i] != '\n' {
				code[i] = false
				i++
			}
		} else if tla[i] == '"' {
			code[i] = false
			i++
			for i < len(tla) && tla[i] != '"' {
				if tla[i] == '\\' {
					code[i] = false
					i++
				}
				if i < len(tla) {
					code[i] = false
					i++
				}
			}
			if i < len(tla) {
				code[i] = false
				i++
			}
		} else {
			i++
		}
	}
	return code
}
