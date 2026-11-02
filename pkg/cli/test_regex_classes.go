package cli

import (
	"fmt"
	"strconv"
	"strings"
	"unicode/utf16"
	"unicode/utf8"
)

type testRegexClassAtom struct {
	unit rune
	set  byte // d, D, s, S, w or W; zero denotes a single UTF-16 unit.
}

type testRegexClassReader struct {
	source  string
	index   int
	pending *rune
}

func (r *testRegexClassReader) atom() (testRegexClassAtom, error) {
	if r.pending != nil {
		unit := *r.pending
		r.pending = nil
		return testRegexClassAtom{unit: unit}, nil
	}
	if r.index >= len(r.source) {
		return testRegexClassAtom{}, fmt.Errorf("unterminated character class")
	}
	unit, size := utf8.DecodeRuneInString(r.source[r.index:])
	r.index += size
	if unit == '\\' {
		if r.index >= len(r.source) {
			return testRegexClassAtom{}, fmt.Errorf("trailing class escape")
		}
		unit, size = utf8.DecodeRuneInString(r.source[r.index:])
		r.index += size
		if strings.ContainsRune("dDsSwW", unit) {
			return testRegexClassAtom{set: byte(unit)}, nil
		}
		switch unit {
		case 'b':
			unit = '\b'
		case 'f':
			unit = '\f'
		case 'n':
			unit = '\n'
		case 'r':
			unit = '\r'
		case 't':
			unit = '\t'
		case 'v':
			unit = '\v'
		case 'c':
			if r.index < len(r.source) && (testRegexASCIIAlpha(r.source[r.index]) || r.source[r.index] == '_' || r.source[r.index] >= '0' && r.source[r.index] <= '9') {
				unit = rune(r.source[r.index] % 32)
				r.index++
			} else {
				// Legacy class escapes preserve both characters in \c.
				pending := unit
				unit, r.pending = '\\', &pending
			}
		case 'u', 'x':
			count := 2
			if unit == 'u' {
				count = 4
			}
			if r.index+count <= len(r.source) && testRegexHex(r.source[r.index:r.index+count]) {
				value, _ := strconv.ParseUint(r.source[r.index:r.index+count], 16, 16)
				unit = rune(value)
				r.index += count
			}
		default:
			if unit >= '0' && unit <= '7' {
				unit -= '0'
				for n := 1; n < 3 && r.index < len(r.source); n++ {
					digit := rune(r.source[r.index]) - '0'
					if digit < 0 || digit > 7 || unit*8+digit > 255 {
						break
					}
					unit = unit*8 + digit
					r.index++
				}
			}
		}
	}
	if unit > 0xffff {
		hi, lo := utf16.EncodeRune(unit)
		unit, r.pending = hi, &lo
	}
	return testRegexClassAtom{unit: unit}, nil
}

func testRegexWriteClassAtom(out *strings.Builder, atom testRegexClassAtom) {
	if atom.set != 0 {
		out.WriteByte('\\')
		out.WriteByte(atom.set)
	} else {
		fmt.Fprintf(out, `\u%04x`, atom.unit)
	}
}

// JavaScript without flags has a flat class grammar: '[' inside a class is
// literal. Annex B permits a class escape at either end of a range by treating
// the hyphen literally. Rendering explicit atoms prevents .NET subtraction,
// POSIX classes, and accidental ranges after a class escape.
func testRegexTranslateClass(source string, start int) (string, int, error) {
	r := testRegexClassReader{source: source, index: start + 1}
	var out strings.Builder
	out.WriteByte('[')
	if r.index < len(source) && source[r.index] == '^' {
		out.WriteByte('^')
		r.index++
	}
	for {
		if r.pending == nil && r.index < len(source) && source[r.index] == ']' {
			out.WriteByte(']')
			return out.String(), r.index + 1, nil
		}
		left, err := r.atom()
		if err != nil {
			return "", r.index, err
		}
		if r.pending == nil && r.index+1 < len(source) && source[r.index] == '-' && source[r.index+1] != ']' {
			r.index++
			right, err := r.atom()
			if err != nil {
				return "", r.index, err
			}
			if left.set == 0 && right.set == 0 {
				if left.unit > right.unit {
					return "", r.index, fmt.Errorf("character class range is out of order")
				}
				testRegexWriteClassAtom(&out, left)
				out.WriteByte('-')
				testRegexWriteClassAtom(&out, right)
			} else {
				testRegexWriteClassAtom(&out, left)
				out.WriteString(`\u002d`)
				testRegexWriteClassAtom(&out, right)
			}
		} else {
			testRegexWriteClassAtom(&out, left)
		}
	}
}
