package quint

import (
	"fmt"
	"sort"
)

// RuntimeObjectHashState makes the TypeScript runtime's process-wide Immutable
// object UID counter explicit. Share one state between operations when their
// traversal must reproduce the same reference process. The zero value starts
// before UID 1; a state is not safe for concurrent use.
type RuntimeObjectHashState struct{ counter uint32 }

// Counter returns the counter after the most recent object was first hashed.
func (s *RuntimeObjectHashState) Counter() uint32 { return s.counter }

// Restore resumes an observed reference counter. Immutable resets the counter
// to zero immediately after allocating UID 0x40000000.
func (s *RuntimeObjectHashState) Restore(counter uint32) error {
	if counter >= 0x40000000 {
		return fmt.Errorf("object hash counter must be below 0x40000000")
	}
	s.counter = counter
	return nil
}

// NextHash allocates the reference's next raw object hash. Unlike runtime
// hashCode values, this UID must not pass through the signed-small-int coercion.
func (s *RuntimeObjectHashState) NextHash() uint32 {
	s.counter++
	value := s.counter
	if s.counter&0x40000000 != 0 {
		s.counter = 0
	}
	return value
}

type runtimeObjectIdentity struct {
	hash   uint32
	hashed bool
}

func (i *runtimeObjectIdentity) hashWith(state *RuntimeObjectHashState) uint32 {
	if !i.hashed {
		i.hash, i.hashed = state.NextHash(), true
	}
	return i.hash
}

type runtimeObjectEntry[T any] struct {
	Key   *runtimeObjectIdentity
	Value T
}

// runtimeObjectSet models insertion-only Immutable.Set<object> traversal. The
// small array map compares object identity without hashing; its ninth distinct
// object is hashed first, followed by the original eight. Once converted, trie
// traversal compares successive low-to-high five-bit hash fragments and keeps
// insertion order for full collisions. No node deletion or value replacement is
// required at these object-set construction boundaries.
type runtimeObjectSet[T any] struct {
	entries []runtimeObjectEntry[T]
	trie    bool
	// dirty marks entries appended in trie mode but not yet sorted. Sorting is
	// deferred to Entries so each insert is O(1) instead of shifting the tail.
	dirty bool
	// Lookup only; traversal always comes from entries.
	seen map[*runtimeObjectIdentity]bool
}

func (s *runtimeObjectSet[T]) Add(state *RuntimeObjectHashState, key *runtimeObjectIdentity, value T) {
	if s.trie {
		if s.seen[key] {
			return
		}
	} else {
		for _, entry := range s.entries {
			if entry.Key == key {
				return
			}
		}
	}
	entry := runtimeObjectEntry[T]{Key: key, Value: value}
	if !s.trie && len(s.entries) < 8 {
		s.entries = append(s.entries, entry)
		return
	}
	key.hashWith(state)
	if !s.trie {
		for _, old := range s.entries {
			old.Key.hashWith(state)
		}
		s.entries = append([]runtimeObjectEntry[T]{entry}, s.entries...)
		s.trie = true
		s.seen = make(map[*runtimeObjectIdentity]bool, len(s.entries))
		for _, entry := range s.entries {
			s.seen[entry.Key] = true
		}
		sort.SliceStable(s.entries, func(i, j int) bool {
			return runtimeObjectHashLess(s.entries[i].Key.hash, s.entries[j].Key.hash)
		})
	} else {
		// An incremental insert placed each key after every existing entry whose
		// hash is not greater, so equal hashes keep insertion order. A stable
		// sort of the insertion-ordered slice gives the same order, so the sort
		// is deferred to Entries.
		s.seen[key] = true
		s.entries = append(s.entries, entry)
		s.dirty = true
	}
}

func runtimeObjectHashLess(left, right uint32) bool {
	for shift := uint(0); shift < 32; shift += 5 {
		a, b := (left>>shift)&31, (right>>shift)&31
		if a != b {
			return a < b
		}
	}
	return false
}

func (s *runtimeObjectSet[T]) Entries() []runtimeObjectEntry[T] {
	if s.dirty {
		sort.SliceStable(s.entries, func(i, j int) bool {
			return runtimeObjectHashLess(s.entries[i].Key.hash, s.entries[j].Key.hash)
		})
		s.dirty = false
	}
	return append([]runtimeObjectEntry[T]{}, s.entries...)
}
