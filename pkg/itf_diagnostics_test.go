package quint

import (
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestUpstreamITFDiagnostics(t *testing.T) {
	var fixtures []struct {
		Name    string
		Input   json.RawMessage
		States  [][]DebugMessage
		Pending []DebugMessage
	}
	data, err := os.ReadFile("testdata/parser/itf_diagnostics.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	for _, f := range fixtures {
		t.Run(f.Name, func(t *testing.T) {
			trace, err := DecodeITFTrace(f.Input)
			if err != nil {
				t.Fatal(err)
			}
			before := string(trace.Raw)
			states, err := ITFDiagnostics(trace)
			if err != nil {
				t.Fatal(err)
			}
			pending, err := PendingITFDiagnostics(trace)
			if err != nil {
				t.Fatal(err)
			}
			got, _ := json.Marshal(struct {
				States  [][]DebugMessage
				Pending []DebugMessage
			}{states, pending})
			want, _ := json.Marshal(struct {
				States  [][]DebugMessage
				Pending []DebugMessage
			}{f.States, f.Pending})
			var a, b any
			json.Unmarshal(got, &a)
			json.Unmarshal(want, &b)
			if !reflect.DeepEqual(a, b) {
				t.Fatalf("Go %s\nTS %s", got, want)
			}
			if string(trace.Raw) != before {
				t.Fatal("trace changed")
			}
		})
	}
}

func TestITFDiagnosticsMalformedMetadata(t *testing.T) {
	for _, value := range []string{`null`, `[]`, `42`, `""`, `"null"`, `"{}"`, `"[null]"`, `"[{\"value\":1}]"`, `"[{\"label\":1,\"value\":1}]"`, `"[{\"label\":\"x\"}]"`, `"[{\"label\":\"x\",\"value\":null}]"`} {
		metadata := json.RawMessage(`{"diagnostics":` + value + `}`)
		if _, err := ITFDiagnostics(ITFTrace{StateMetadata: []json.RawMessage{nil, metadata}}); err == nil || !strings.Contains(err.Error(), "state 1") {
			t.Fatal(value, err)
		}
	}
	for _, metadata := range []string{`1`, `[]`, `{`} {
		if _, err := ITFDiagnostics(ITFTrace{StateMetadata: []json.RawMessage{json.RawMessage(metadata)}}); err == nil {
			t.Fatal("invalid metadata accepted", metadata)
		}
		if _, err := PendingITFDiagnostics(ITFTrace{Metadata: json.RawMessage(metadata)}); err == nil {
			t.Fatal("invalid pending metadata accepted", metadata)
		}
	}
	for _, value := range []string{`null`, `false`, `0`, `-0`, `0.0`, `0e3`, `""`} {
		result, err := PendingITFDiagnostics(ITFTrace{Metadata: json.RawMessage(`{"pending_diagnostics":` + value + `}`)})
		if err != nil || result == nil || len(result) != 0 {
			t.Fatal(value, result, err)
		}
	}
	for _, value := range []string{`true`, `[]`, `"null"`, `"bad"`} {
		if _, err := PendingITFDiagnostics(ITFTrace{Metadata: json.RawMessage(`{"pending_diagnostics":` + value + `}`)}); err == nil || !strings.Contains(err.Error(), "pending diagnostics") {
			t.Fatal(value, err)
		}
	}
}
