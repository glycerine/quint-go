package quint

import (
	"fmt"
	"unicode/utf16"
)

// runtimeReferenceHash is Immutable.js hash(value), including its signed-small-
// integer (smi) coercion. It is private compatibility machinery for collection
// traversal, not the public structural RuntimeValue.Hash contract.
//
// Runtime constructors normalize set elements and map keys; concrete normalForm
// calls also normalize nested values but do not change their hashes. Records are
// sorted by UTF-16 field names on construction, lists/tuples retain their order,
// and sets/maps have unordered collection hashes. Lambdas inherit hashCode zero;
// their mock display trees therefore suffice for hashing but never equality.
// The canonical runtime display IR
// therefore retains all information needed for these hashes, though it cannot
// reproduce collection traversal or mutation history.
func runtimeReferenceHash(value RuntimeValue) (int32, error) {
	if !value.Valid() {
		return 0, fmt.Errorf("reference hashing requires a concrete runtime value")
	}
	code, err := runtimeReferenceHashCode(value.expression)
	return runtimeReferenceSMI(uint32(code)), err
}

// runtimeReferenceHashCode returns the runtime class's hashCode before the
// Immutable.hash smi conversion. Input must be canonical concrete IR. Integer
// hashCodes are unsigned low 32 bits; variants can exceed signed 32 bits.
func runtimeReferenceHashCode(e Expr) (int64, error) {
	switch e.Kind {
	case "lambda":
		// RuntimeValueLambda inherits the base class's zero hashCode. Its mock
		// display expression does not identify the executable closure object.
		return 0, nil
	case "bool":
		if e.Bool {
			return 1, nil
		}
		return 0, nil
	case "int":
		n := e.Int.Big()
		low := uint32(n.Uint64())
		if n.Sign() < 0 {
			low = 0 - low
		}
		return int64(low), nil
	case "str":
		return int64(runtimeReferenceStringHash(e.Str)), nil
	case "app":
	default:
		return 0, fmt.Errorf("reference hashing does not support %s", e.Kind)
	}
	if e.Opcode == "variant" {
		if len(e.Args) != 2 || e.Args[0].Kind != "str" {
			return 0, fmt.Errorf("invalid variant for reference hashing")
		}
		payload, err := runtimeReferenceHashCode(e.Args[1])
		if err != nil {
			return 0, err
		}
		// The reference deliberately hashes only the payload, ignoring its label.
		return int64(runtimeReferenceSMI(uint32(payload))) + payload, nil
	}
	ordered := false
	keyed := false
	size := len(e.Args)
	switch e.Opcode {
	case "List", "Tup":
		ordered = true
	case "Rec":
		ordered = true
		keyed = true
		size /= 2
	case "Set":
	case "Map":
		keyed = true
	default:
		return 0, fmt.Errorf("reference hashing does not support %s", e.Opcode)
	}
	var h uint32
	if ordered {
		h = 1
	}
	hash := func(e Expr) (int32, error) {
		code, err := runtimeReferenceHashCode(e)
		return runtimeReferenceSMI(uint32(code)), err
	}
	for i := 0; i < len(e.Args); i++ {
		var child int32
		var err error
		if keyed {
			var key, value Expr
			if e.Opcode == "Rec" {
				key = e.Args[i]
				i++
				if i >= len(e.Args) {
					return 0, fmt.Errorf("invalid record for reference hashing")
				}
				value = e.Args[i]
			} else {
				pair := e.Args[i]
				if pair.Kind != "app" || pair.Opcode != "Tup" || len(pair.Args) != 2 {
					return 0, fmt.Errorf("invalid map entry for reference hashing")
				}
				key, value = pair.Args[0], pair.Args[1]
			}
			keyHash, e1 := hash(key)
			valueHash, e2 := hash(value)
			if e1 != nil {
				return 0, e1
			}
			if e2 != nil {
				return 0, e2
			}
			child = runtimeReferenceHashMerge(valueHash, keyHash)
		} else {
			child, err = hash(e.Args[i])
			if err != nil {
				return 0, err
			}
		}
		if ordered {
			h = 31*h + uint32(child)
		} else {
			h += uint32(child)
		}
	}
	return int64(runtimeReferenceMurmur(uint32(size), h)), nil
}

func runtimeReferenceSMI(h uint32) int32 { return int32(((h >> 1) & 0x40000000) | (h & 0xbfffffff)) }
func runtimeReferenceStringHash(s string) int32 {
	var h uint32
	for _, unit := range utf16.Encode([]rune(s)) {
		h = 31*h + uint32(unit)
	}
	return runtimeReferenceSMI(h)
}
func runtimeReferenceHashMerge(a, b int32) int32 {
	return int32(uint32(a) ^ (uint32(b) + 0x9e3779b9 + (uint32(a) << 6) + uint32(a>>2)))
}
func runtimeReferenceMurmur(size, h uint32) int32 {
	h *= 0xcc9e2d51
	h = ((h << 15) | (h >> 17)) * 0x1b873593
	h = ((h << 13) | (h >> 19)) * 5
	h = (h + 0xe6546b64) ^ size
	h = (h ^ (h >> 16)) * 0x85ebca6b
	h = (h ^ (h >> 13)) * 0xc2b2ae35
	return runtimeReferenceSMI(h ^ (h >> 16))
}
