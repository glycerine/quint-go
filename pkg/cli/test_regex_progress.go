package cli

import (
	"fmt"
	"strconv"
	"strings"
)

// Progress guards duplicate a nullable atom once when separating mandatory and
// optional iterations. Bound expansion independently of the execution timeout.
const testRegexTranslationLimit = 64 << 10

type testRegexQuantification struct {
	present, unbounded, lazy bool
	minimum, maximum, length int
}

func (q testRegexQuantification) repeated() bool { return q.present && (q.unbounded || q.maximum > 1) }
func (q testRegexQuantification) optional() bool {
	return q.present && (q.unbounded || q.maximum > q.minimum)
}

func testRegexReadQuantifier(s string) (q testRegexQuantification) {
	if s == "" {
		return q
	}
	switch s[0] {
	case '*':
		q = testRegexQuantification{present: true, unbounded: true, length: 1}
	case '+':
		q = testRegexQuantification{present: true, unbounded: true, minimum: 1, length: 1}
	case '?':
		q = testRegexQuantification{present: true, maximum: 1, length: 1}
	case '{':
		end := strings.IndexByte(s, '}')
		if end < 0 {
			return q
		}
		lo, hi, comma := strings.Cut(s[1:end], ",")
		decimal := func(n string) bool {
			if n == "" {
				return false
			}
			for i := range n {
				if n[i] < '0' || n[i] > '9' {
					return false
				}
			}
			return true
		}
		if !decimal(lo) || comma && hi != "" && !decimal(hi) {
			return q
		}
		minimum, err := strconv.Atoi(lo)
		if err != nil {
			return q
		}
		q = testRegexQuantification{present: true, minimum: minimum, maximum: minimum, length: end + 1}
		if comma {
			if hi == "" {
				q.unbounded = true
			} else {
				q.maximum, err = strconv.Atoi(hi)
				if err != nil || q.maximum < minimum {
					return testRegexQuantification{}
				}
			}
		}
	default:
		return q
	}
	if q.length < len(s) && s[q.length] == '?' {
		q.lazy = true
		q.length++
	}
	return q
}

// Legacy decimal escapes refer to a capture only when their entire decimal
// number is in range. Otherwise flagless ECMAScript uses an octal prefix (at most
// 3 digits below 0400) or the identity escapes \8 and \9.
func testRegexLegacyDecimal(s string) string {
	limit := 3
	if s[0] >= '4' {
		limit = 2
	}
	n, used := 0, 0
	for used < len(s) && used < limit && s[used] >= '0' && s[used] <= '7' {
		n = n*8 + int(s[used]-'0')
		used++
	}
	if used == 0 {
		n, used = int(s[0]), 1
	}
	return fmt.Sprintf(`\x%02x%s`, n, s[used:])
}

// At entry position p, save the complete remaining suffix. At q >= p that
// suffix can match through absolute end only if q == p, so the final negative
// lookahead rejects exactly a zero-progress iteration. Lookbehind uses the dual
// saved prefix. Guards are outside the atom so assertion bodies retain their own
// execution direction. Required iterations execute first (textually last in RTL).
func testRegexGuardIterations(atom string, q testRegexQuantification, rtl bool, index int) string {
	name := fmt.Sprintf("__quintTail%d", index)
	reset := fmt.Sprintf("(?(%s)(?<-%s>))", name, name)
	entry := fmt.Sprintf(`(?=(?<%s>[\s\S]*\z))`, name)
	exit := fmt.Sprintf(`(?!\k<%s>\z)`, name)
	if rtl {
		entry = fmt.Sprintf(`(?<=(?<%s>\A[\s\S]*))`, name)
		exit = fmt.Sprintf(`(?<!\A\k<%s>)`, name)
	}
	guarded := "(?:" + reset + entry + atom + exit + ")"
	if rtl {
		guarded = "(?:" + exit + atom + entry + reset + ")"
	}
	count := "*"
	if !q.unbounded {
		count = fmt.Sprintf("{0,%d}", q.maximum-q.minimum)
	}
	if q.lazy {
		count += "?"
	}
	optional := guarded + count
	required := ""
	if q.minimum > 0 {
		required = fmt.Sprintf("(?:%s){%d}", atom, q.minimum)
	}
	if rtl {
		return "(?:" + optional + required + ")"
	}
	return "(?:" + required + optional + ")"
}
