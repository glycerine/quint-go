package cli

import (
	"strconv"
	"strings"
	"unicode"
	"unicode/utf16"
	"unicode/utf8"
)

// Capture names use RegExpIdentifierName even when the expression has no
// Unicode flag. Resolve identifier escapes before duplicate/reference lookup;
// the engine receives numeric groups and never interprets these names itself.
func testRegexCaptureName(source string) (string, bool) {
	if source == "" {
		return "", false
	}
	var name strings.Builder
	first := true
	for i := 0; i < len(source); {
		var r rune
		if source[i] != '\\' {
			var size int
			r, size = utf8.DecodeRuneInString(source[i:])
			if r == utf8.RuneError && size == 1 {
				return "", false
			}
			i += size
		} else {
			if i+2 > len(source) || source[i+1] != 'u' {
				return "", false
			}
			i += 2
			if i < len(source) && source[i] == '{' {
				end := strings.IndexByte(source[i+1:], '}')
				if end < 0 {
					return "", false
				}
				end += i + 1
				digits := source[i+1 : end]
				code, err := strconv.ParseUint(digits, 16, 32)
				if digits == "" || !testRegexHex(digits) || err != nil || code > utf8.MaxRune {
					return "", false
				}
				r = rune(code)
				i = end + 1
			} else {
				if i+4 > len(source) || !testRegexHex(source[i:i+4]) {
					return "", false
				}
				code, _ := strconv.ParseUint(source[i:i+4], 16, 16)
				r = rune(code)
				i += 4
				if r >= 0xd800 && r <= 0xdbff {
					// Only adjacent fixed-width escapes combine. A braced
					// surrogate escape is an invalid identifier code point.
					if i+6 > len(source) || source[i:i+2] != `\u` || !testRegexHex(source[i+2:i+6]) {
						return "", false
					}
					low, _ := strconv.ParseUint(source[i+2:i+6], 16, 16)
					if low < 0xdc00 || low > 0xdfff {
						return "", false
					}
					r = utf16.DecodeRune(r, rune(low))
					i += 6
				}
			}
		}
		// ID_Start and ID_Continue remove syntax and whitespace characters
		// from the general categories (notably the letter U+2E2F).
		identifier := !unicode.Is(unicode.Pattern_Syntax, r) && !unicode.Is(unicode.Pattern_White_Space, r)
		start := r == '$' || r == '_' || identifier && (unicode.IsLetter(r) || unicode.Is(unicode.Nl, r) || unicode.Is(unicode.Other_ID_Start, r))
		continuation := start || identifier && (unicode.Is(unicode.Mn, r) || unicode.Is(unicode.Mc, r) || unicode.Is(unicode.Nd, r) || unicode.Is(unicode.Pc, r) || unicode.Is(unicode.Other_ID_Continue, r)) || r == '\u200c' || r == '\u200d'
		if first && !start || !first && !continuation {
			return "", false
		}
		name.WriteRune(r)
		first = false
	}
	return name.String(), true
}
