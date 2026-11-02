package quint

import (
	"fmt"
	"sort"

	"github.com/glycerine/insdict"
)

// runtimeReferenceOrder returns Immutable.Map constructor key traversal. Small
// array maps retain insertion order; the ninth distinct insertion creates a HAMT
// with the ninth key inserted before the original eight. Trie traversal compares
// successive low-to-high five-bit hash fragments and preserves insertion order
// within a full-hash collision node. Repeated keys replace their value without
// changing their insertion position.
//
// This is construction-only compatibility machinery. Reconstructing from the
// final values cannot reproduce deletion/update histories of persistent maps.
func runtimeReferenceOrder(values []RuntimeValue) ([]RuntimeValue, error) {
	positions := insdict.NewDict[string, int]()
	owned := make([]RuntimeValue, 0, len(values))
	for _, value := range values {
		if !value.Valid() || value.closure != nil {
			return nil, fmt.Errorf("reference traversal requires concrete runtime values")
		}
		if index, found := positions.Get2(value.identity); found {
			owned[index] = value
		} else {
			positions.Set(value.identity, len(owned))
			owned = append(owned, value)
		}
	}
	if len(owned) <= 8 {
		return owned, nil
	}
	type entry struct {
		value RuntimeValue
		hash  uint32
	}
	entries := make([]entry, 0, len(owned))
	appendValue := func(value RuntimeValue) error {
		hash, err := runtimeReferenceHash(value)
		if err != nil {
			return err
		}
		entries = append(entries, entry{value, uint32(hash)})
		return nil
	}
	if err := appendValue(owned[8]); err != nil {
		return nil, err
	}
	for i, value := range owned {
		if i != 8 {
			if err := appendValue(value); err != nil {
				return nil, err
			}
		}
	}
	sort.SliceStable(entries, func(i, j int) bool {
		left, right := entries[i].hash, entries[j].hash
		for shift := uint(0); shift < 32; shift += 5 {
			a, b := (left>>shift)&31, (right>>shift)&31
			if a != b {
				return a < b
			}
		}
		return false
	})
	for i, entry := range entries {
		owned[i] = entry.value
	}
	return owned, nil
}
