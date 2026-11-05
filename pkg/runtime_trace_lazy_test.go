package quint

import "testing"

// A pending state read from the trace must equal the record Record builds from
// the same storage, and Len must not need the record to be built.
func TestRuntimeTraceLazyStateMatchesRecord(t *testing.T) {
	storage := NewVariableStorageWithMetadata(true)
	_, next := storage.Register("x", "x")
	value, err := NewRuntimeValue(Expr{Kind: "str", Str: "a"})
	if err != nil {
		t.Fatal(err)
	}
	next.Value = value
	var trace RuntimeTrace
	if err := trace.Shift(storage); err != nil {
		t.Fatal(err)
	}
	if trace.Len() != 1 || trace.states[0].pending == nil {
		t.Fatalf("expected one pending state, got len %d", trace.Len())
	}
	want, err := storage.Record()
	if err != nil {
		t.Fatal(err)
	}
	states, err := trace.States()
	if err != nil {
		t.Fatal(err)
	}
	if len(states) != 1 || !states[0].Equal(want) {
		t.Fatalf("lazy state differs from Record: %v", states)
	}
	if trace.states[0].pending != nil {
		t.Fatal("States should leave the built state in place")
	}
}
