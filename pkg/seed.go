package quint

import (
	"fmt"
	"math/big"
	"strings"
)

// ParseSeed implements the CLI's JavaScript BigInt string conversion. It accepts
// signed decimal and unsigned 0x/0o/0b forms, including surrounding ECMAScript
// whitespace. Empty/whitespace-only input means zero. Seeds remain unbounded;
// constructor/reset normalization is owned by RNG.
func ParseSeed(text string) (Integer, error) {
	invalid := func() (Integer, error) { return Integer{}, fmt.Errorf("--seed must be a big integer, found: %s", text) }
	s := strings.TrimFunc(text, seedWhitespace)
	if s == "" {
		return Integer{}, nil
	}
	digits, base := s, 10
	if len(s) >= 2 && s[0] == '0' {
		switch s[1] {
		case 'x', 'X':
			base = 16
			digits = s[2:]
		case 'o', 'O':
			base = 8
			digits = s[2:]
		case 'b', 'B':
			base = 2
			digits = s[2:]
		}
	}
	if base == 10 && (digits[0] == '+' || digits[0] == '-') {
		digits = digits[1:]
	}
	if digits == "" {
		return invalid()
	}
	for _, c := range digits {
		value := -1
		switch {
		case c >= '0' && c <= '9':
			value = int(c - '0')
		case c >= 'a' && c <= 'f':
			value = int(c-'a') + 10
		case c >= 'A' && c <= 'F':
			value = int(c-'A') + 10
		}
		if value < 0 || value >= base {
			return invalid()
		}
	}
	n, ok := new(big.Int).SetString(digits, base)
	if !ok {
		return invalid()
	}
	if base == 10 && s[0] == '-' {
		n.Neg(n)
	}
	return IntegerFromBig(n), nil
}

func seedWhitespace(r rune) bool {
	switch r {
	case '\t', '\n', '\v', '\f', '\r', ' ', 0x00a0, 0x1680, 0x2028, 0x2029, 0x202f, 0x205f, 0x3000, 0xfeff:
		return true
	}
	return r >= 0x2000 && r <= 0x200a
}
