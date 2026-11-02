package quint

import "strings"

// PostprocessTLA repairs the upstream printer's single-angle action syntax.
// Comments, strings, and already-correct double-angle actions are preserved.
func PostprocessTLA(tla string) string { return FixAngleActions(tla) }

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
