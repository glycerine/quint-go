package quint

import (
	"strings"
	"unicode/utf8"
)

// decodeSourceUTF8 matches Node Buffer.toString('utf8'): each maximal prefix of
// a valid UTF-8 sequence is replaced once when truncated/interrupted; invalid
// leading bytes and out-of-range second bytes are replaced separately.
func decodeSourceUTF8(data []byte) string {
	if utf8.Valid(data) {
		return string(data)
	}
	var out strings.Builder
	out.Grow(len(data))
	for i := 0; i < len(data); {
		r, n := utf8.DecodeRune(data[i:])
		if r != utf8.RuneError || n > 1 {
			out.Write(data[i : i+n])
			i += n
			continue
		}
		lead := data[i]
		width := 0
		switch {
		case lead >= 0xc2 && lead <= 0xdf:
			width = 2
		case lead >= 0xe0 && lead <= 0xef:
			width = 3
		case lead >= 0xf0 && lead <= 0xf4:
			width = 4
		}
		consumed := 1
		for consumed < width && i+consumed < len(data) {
			b := data[i+consumed]
			if b < 0x80 || b > 0xbf {
				break
			}
			if consumed == 1 && ((lead == 0xe0 && b < 0xa0) || (lead == 0xed && b > 0x9f) || (lead == 0xf0 && b < 0x90) || (lead == 0xf4 && b > 0x8f)) {
				break
			}
			consumed++
		}
		out.WriteRune(utf8.RuneError)
		i += consumed
	}
	return out.String()
}
