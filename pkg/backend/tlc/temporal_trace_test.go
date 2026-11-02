package tlc

import (
	"testing"

	native "github.com/glycerine/tlago/tlc"
)

func TestTemporalLoop(t *testing.T) {
	for _, tc := range []struct {
		name     string
		length   int
		messages []native.Message
		want     int
		present  bool
	}{
		{"no counterexample", 0, nil, 0, false},
		{"safety counterexample", 2, []native.Message{{Code: native.ECTLCStatePrint2}}, 0, false},
		{"stuttering", 3, []native.Message{{Code: native.ECTLCStatePrint3, StateNumber: 4, Params: []string{"4"}}}, 2, true},
		{"single-state stuttering", 1, []native.Message{{Code: native.ECTLCStatePrint3}}, 0, true},
		{"back to first", 3, []native.Message{{Code: native.ECTLCBackToState, Params: []string{"1", "action"}}}, 0, true},
		{"back to final", 3, []native.Message{{Code: native.ECTLCBackToState, Params: []string{"3"}}}, 2, true},
		{"state ordinal", 3, []native.Message{{Code: native.ECTLCBackToState, StateInfo: &native.TLCStateInfo{}, StateNumber: 2}}, 1, true},
		{"ordinal without state info", 3, []native.Message{{Code: native.ECTLCBackToState, StateNumber: 2}}, 1, true},
		{"matching ordinal fields", 3, []native.Message{{Code: native.ECTLCBackToState, StateNumber: 2, Params: []string{"2", "action"}}}, 1, true},
		{"repeated same marker", 3, []native.Message{{Code: native.ECTLCBackToState, Params: []string{"2"}}, {Code: native.ECTLCBackToState, StateNumber: 2}}, 1, true},
		{"formatting is not a marker", 2, []native.Message{{Code: native.ECTLCBackToState, FormattingOnly: true}}, 0, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			loop, err := temporalLoop(tc.messages, tc.length)
			if err != nil || (loop != nil) != tc.present || loop != nil && *loop != tc.want {
				t.Fatalf("loop=%v err=%v; want present=%v target=%d", loop, err, tc.present, tc.want)
			}
		})
	}
}

func TestTemporalLoopRejectsInvalidMarkers(t *testing.T) {
	for _, tc := range []struct {
		name     string
		length   int
		messages []native.Message
	}{
		{"negative length", -1, nil},
		{"stuttering empty trace", 0, []native.Message{{Code: native.ECTLCStatePrint3}}},
		{"back edge empty trace", 0, []native.Message{{Code: native.ECTLCBackToState, Params: []string{"1"}}}},
		{"missing ordinal", 2, []native.Message{{Code: native.ECTLCBackToState}}},
		{"zero ordinal", 2, []native.Message{{Code: native.ECTLCBackToState, Params: []string{"0"}}}},
		{"negative ordinal", 2, []native.Message{{Code: native.ECTLCBackToState, StateNumber: -1}}},
		{"ordinal beyond trace", 2, []native.Message{{Code: native.ECTLCBackToState, Params: []string{"3"}}}},
		{"nonnumeric ordinal", 2, []native.Message{{Code: native.ECTLCBackToState, Params: []string{"first"}}}},
		{"overflowing ordinal", 2, []native.Message{{Code: native.ECTLCBackToState, Params: []string{"99999999999999999999999"}}}},
		{"fractional ordinal", 2, []native.Message{{Code: native.ECTLCBackToState, Params: []string{"1.5"}}}},
		{"whitespace ordinal", 2, []native.Message{{Code: native.ECTLCBackToState, Params: []string{" 1"}}}},
		{"invalid state ordinal", 2, []native.Message{{Code: native.ECTLCBackToState, StateInfo: &native.TLCStateInfo{}, Params: []string{"1"}}}},
		{"disagreeing ordinal fields", 2, []native.Message{{Code: native.ECTLCBackToState, StateNumber: 1, Params: []string{"2"}}}},
		{"conflicting targets", 2, []native.Message{{Code: native.ECTLCBackToState, StateNumber: 1}, {Code: native.ECTLCBackToState, StateNumber: 2}}},
		{"conflicting marker kinds", 2, []native.Message{{Code: native.ECTLCStatePrint3}, {Code: native.ECTLCBackToState, StateNumber: 2}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if loop, err := temporalLoop(tc.messages, tc.length); err == nil || loop != nil {
				t.Fatalf("accepted closing marker: loop=%v err=%v", loop, err)
			}
		})
	}
}
