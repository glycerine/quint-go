package quint

import (
	"bytes"
	"testing"
)

// FuzzITFTraceDecoding checks the whole-trace ITF decoder. Accepted traces must
// retain their original bytes and keep state metadata aligned with states, and
// encoding must reach a fixed point after one decode/encode cycle. Rejected
// input must return an error rather than panic.
func FuzzITFTraceDecoding(f *testing.F) {
	for _, seed := range []string{
		`{"vars":["x"],"states":[{"x":{"#bigint":"0"}},{"x":{"#bigint":"1"}}]}`,
		`{"#meta":{"format":"ITF"},"vars":["x","y"],"states":[{"#meta":{"index":0},"x":true,"y":"s"}],"loop":0}`,
		`{"vars":["s"],"states":[{"s":{"#set":[{"#bigint":"2"},{"#bigint":"1"}]}}]}`,
		`{"vars":["m"],"states":[{"m":{"#map":[[{"#bigint":"1"},"a"]]}}]}`,
		`{"vars":["t"],"states":[{"t":{"#tup":[{"#bigint":"1"},false]}}]}`,
		`{"vars":["r"],"states":[{"r":{"a":{"#bigint":"-7"},"__hidden":1}}]}`,
		`{"vars":["x"],"states":[]}`,
		`{"vars":["x"],"states":[{"x":"😀"}]}`,
		`{"vars":["x"],"states":[{"x":"\ud83d"}]}`,
		`{"vars":["x"],"states":[{"x":{"#bigint":"1.5"}}]}`,
		`{"vars":["x"]}`,
	} {
		f.Add([]byte(seed))
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 8<<10 {
			t.Skip()
		}
		trace, err := DecodeITFTrace(data)
		if err != nil {
			return
		}
		if !bytes.Equal(trace.Raw, data) {
			t.Fatal("decoded trace did not retain its original bytes")
		}
		if len(trace.StateMetadata) != len(trace.States) {
			t.Fatalf("state metadata misaligned: %d metadata for %d states", len(trace.StateMetadata), len(trace.States))
		}
		first, err := EncodeITFTrace(trace.Vars, trace.States, false)
		if err != nil {
			t.Fatal("accepted trace cannot be encoded", err)
		}
		again, err := DecodeITFTrace(first)
		if err != nil {
			t.Fatalf("encoded trace cannot be decoded: %v\n%s", err, first)
		}
		second, err := EncodeITFTrace(again.Vars, again.States, false)
		if err != nil || !bytes.Equal(first, second) {
			t.Fatalf("ITF trace encoding is not a fixed point: %v\nfirst %s\n next %s", err, first, second)
		}
	})
}
