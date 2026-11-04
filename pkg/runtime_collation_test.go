package quint

import (
	"bytes"
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"sync"
	"testing"

	"golang.org/x/text/collate"
	"golang.org/x/text/language"
)

func TestRuntimeCollatorLeaseAndRetention(t *testing.T) {
	t.Parallel()
	pool := runtimeCollatorPool{idle: make(chan *runtimeCollator, 1)}
	first, second := pool.acquire(), pool.acquire()
	if first == second {
		t.Fatal("simultaneous operations share a mutable collator")
	}
	first.CompareString("retained left", "retained right")
	pool.release(first)
	// The pinned collation engine keeps its last input strings in two private
	// iterators. Verify actual retained references, without GC timing or unsafe.
	iterators := reflect.ValueOf(first.value).Elem().FieldByName("_iter")
	if !iterators.IsValid() || iterators.Len() != 2 {
		t.Fatal("review the pool's input retention contract for this collation engine")
	}
	for i := 0; i < 2; i++ {
		iterator := iterators.Index(i).FieldByName("Iter")
		if iterator.FieldByName("str").Len() != 0 || iterator.FieldByName("bytes").Len() != 0 {
			t.Fatal("released collator retains caller input")
		}
	}
	pool.release(second) // A full idle pool drops this independent lease.
	if len(pool.idle) != 1 || pool.acquire() != first {
		t.Fatal("idle capacity or completed lease reuse changed")
	}
	pool.release(nil)
	var disabled runtimeCollatorPool
	disabled.release(disabled.acquire()) // A nil queue also never blocks.
}

func TestRuntimeCollatorOversizedInputs(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, left, right string
		keep              bool
	}{
		{"boundary", strings.Repeat("x", runtimeCollatorMaxInput), "z", true},
		{"left_oversized", strings.Repeat("x", runtimeCollatorMaxInput+1), "z", false},
		{"right_oversized", "z", strings.Repeat("x", runtimeCollatorMaxInput+1), false},
		{"unicode_boundary", strings.Repeat("é", runtimeCollatorMaxInput/2), "z", true},
		{"combining_expansion", strings.Repeat("a\u0301", runtimeCollatorMaxInput), strings.Repeat("á", runtimeCollatorMaxInput), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			pool := runtimeCollatorPool{idle: make(chan *runtimeCollator, 1)}
			comparison := pool.acquire()
			want := collate.New(language.AmericanEnglish).CompareString(tc.left, tc.right)
			if got := comparison.CompareString(tc.left, tc.right); got != want {
				t.Fatal(got, want)
			}
			comparison.CompareString("a", "b") // A small last input cannot hide an earlier large one.
			pool.release(comparison)
			if (len(pool.idle) == 1) != tc.keep {
				t.Fatal("oversized scratch retention changed", len(pool.idle), tc.keep)
			}
		})
	}
}

func TestRuntimeCollatorUnicodeReuseAndConcurrency(t *testing.T) {
	t.Parallel()
	pool := runtimeCollatorPool{idle: make(chan *runtimeCollator, 32)}
	words := []string{"", "a", "A", "Z", "é", "e\u0301", "e\u0300", "ß", "ss", "İ", "ı", "😀", "\ue000", "\uffff", "-10", "-2", "10", "2", "a-b", "a b", "\"a\"", "\x00", "\xff"}
	// Hold more simultaneous leases than the production idle capacity. Nested
	// or concurrent callers must progress independently when the queue is empty.
	leases := make(chan *runtimeCollator, 40)
	start := make(chan struct{})
	var workers sync.WaitGroup
	for range 40 {
		workers.Go(func() {
			comparison := pool.acquire()
			leases <- comparison
			<-start
			fresh := collate.New(language.AmericanEnglish)
			for i, left := range words {
				for _, right := range words[i:] {
					if got, want := comparison.CompareString(left, right), fresh.CompareString(left, right); got != want {
						t.Errorf("%q vs %q: %d != %d", left, right, got, want)
					}
				}
			}
			pool.release(comparison)
		})
	}
	seen := map[*runtimeCollator]bool{}
	for range 40 {
		lease := <-leases
		if seen[lease] {
			t.Error("concurrent leases share comparison state")
		}
		seen[lease] = true
	}
	close(start)
	workers.Wait()
	if len(pool.idle) != cap(pool.idle) {
		t.Fatal("idle pool exceeded or lost its capacity", len(pool.idle))
	}
	// Borrow again after a prior operation has changed both iteration buffers.
	reused := pool.acquire()
	if reused.CompareString("é", "e\u0301") != 0 || reused.CompareString("a", "Z") >= 0 {
		t.Fatal("previous inputs changed a later operation")
	}
	pool.release(reused)
}

func TestRuntimeCollationNormalizationAndSerialization(t *testing.T) {
	t.Parallel()
	app := normalizePreallocApp
	text := func(value string) Expr { return Expr{Kind: "str", Str: value} }
	values := []Expr{
		app("Set", text("é"), text("e\u0301"), text("Z"), text("a"), text("é")),
		app("Set", text("e\u0301"), text("é"), text("A"), text("z")),
		app("Map", app("Tup", text("é"), app("Set", text("ß"), text("ss"))), app("Tup", text("e\u0301"), text("last")), app("Tup", text("é"), text("replacement"))),
		app("List", app("Set", text("z"), text("a")), app("Rec", text("field"), app("Set", text("😀"), text("\uffff"), text("\ue000")))),
		app("Set", text(strings.Repeat("a\u0301", 3000)), text(strings.Repeat("á", 3000))),
	}
	for _, input := range values {
		before, _ := json.Marshal(input)
		want, oldErr := normalizeValueAppendOracle(input)
		got, newErr := NormalizeValue(input)
		oldJSON, _ := json.Marshal(want)
		newJSON, _ := json.Marshal(got)
		if oldErr != nil || newErr != nil || !bytes.Equal(newJSON, oldJSON) || normalizedValueKey(got) != normalizedValueKey(want) {
			t.Fatal("normalization order, stable ties, or identity changed", string(newJSON), string(oldJSON), newErr, oldErr)
		}
		value, err := NewRuntimeValue(input)
		if err != nil {
			t.Fatal(err)
		}
		for _, checked := range []bool{false, true} {
			actualState, expectedState := &RuntimeObjectHashState{}, &RuntimeObjectHashState{}
			fresh := runtimeSerialization{objects: expectedState, comparison: &runtimeCollator{value: collate.New(language.AmericanEnglish), reusable: true}}
			var actual Expr
			if checked {
				fresh.ctx = context.Background()
				actual, err = SerializeRuntimeValueContext(fresh.ctx, value, actualState)
			} else {
				actual, err = SerializeRuntimeValue(value, actualState)
			}
			expected, expectedErr := fresh.expression(value)
			a, _ := json.Marshal(actual)
			b, _ := json.Marshal(expected)
			if err != nil || expectedErr != nil || !bytes.Equal(a, b) || actualState.Counter() != expectedState.Counter() {
				t.Fatal("serialization or object-hash history changed", string(a), string(b), err, expectedErr)
			}
			// Returned trees remain owned after later pooled operations.
			if len(actual.Args) > 0 {
				actual.Args[0] = Expr{Kind: "str", Str: "caller changed"}
			}
			again, repeatErr := SerializeRuntimeValue(value, nil)
			c, _ := json.Marshal(again)
			if repeatErr != nil || !bytes.Equal(c, b) {
				t.Fatal("display scratch escaped into a retained value", repeatErr)
			}
		}
		after, _ := json.Marshal(input)
		if !bytes.Equal(before, after) {
			t.Fatal("normalization or serialization mutated caller input")
		}
	}
}
