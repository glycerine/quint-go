package quint

import (
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestUpstreamTraceOutputTemplates(t *testing.T) {
	var fixtures []struct {
		Name, Template, Test string
		Index                int
		AutoAppend           bool
		Output, Named        string
	}
	data, err := os.ReadFile("testdata/parser/trace_output.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	for _, f := range fixtures {
		t.Run(f.Name, func(t *testing.T) {
			if got := ExpandOutputTemplate(f.Template, f.Index, f.AutoAppend); got != f.Output {
				t.Fatalf("got %q want %q", got, f.Output)
			}
			if got := ExpandNamedOutputTemplate(f.Template, f.Test, f.Index, f.AutoAppend); got != f.Named {
				t.Fatalf("got %q want %q", got, f.Named)
			}
		})
	}
}

func TestUpstreamITFHeaders(t *testing.T) {
	var fixtures []struct {
		Name          string
		Input, Output json.RawMessage
		Header        ITFHeader
	}
	data, err := os.ReadFile("testdata/parser/itf_header.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	for _, f := range fixtures {
		t.Run(f.Name, func(t *testing.T) {
			before := string(f.Input)
			output, err := AddITFHeader(f.Input, f.Header)
			if err != nil {
				t.Fatal(err)
			}
			var got, want any
			json.Unmarshal(output, &got)
			json.Unmarshal(f.Output, &want)
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("Go %s\nTS %s", output, f.Output)
			}
			if string(f.Input) != before {
				t.Fatal("input changed")
			}
			again, err := AddITFHeader(output, ITFHeader{Source: "other"})
			if err != nil {
				t.Fatal(err)
			}
			var repeated any
			json.Unmarshal(again, &repeated)
			if !reflect.DeepEqual(got, repeated) {
				t.Fatal("existing metadata was replaced")
			}
		})
	}
}

func TestITFHeaderValidationAndPrecision(t *testing.T) {
	for _, value := range []string{"", `null`, `[]`, `{`, `{} {}`} {
		if _, err := AddITFHeader([]byte(value), ITFHeader{}); err == nil {
			t.Fatal("invalid trace accepted", value)
		}
	}
	input := []byte(`{"vars":["x"],"states":[{"x":9007199254740993}],"loop":0,"params":["N"],"custom":"keep"}`)
	encoded, err := AddITFHeader(input, ITFHeader{Source: "S", Status: "ok", CreatedAt: "fixed date", Timestamp: 123})
	if err != nil {
		t.Fatal(err)
	}
	for _, part := range []string{`9007199254740993`, `"loop":0`, `"params":["N"]`, `"custom":"keep"`, `"description":"Created by Quint on fixed date"`, `"timestamp":123`} {
		if !strings.Contains(string(encoded), part) {
			t.Fatal("field lost", part, string(encoded))
		}
	}
	decoded, err := DecodeITFTrace(encoded)
	if err != nil || decoded.Loop == nil || *decoded.Loop != 0 || decoded.Params[0] != "N" {
		t.Fatal(decoded, err)
	}
	input[0] = 'x'
	if encoded[0] != '{' {
		t.Fatal("output aliased input")
	}
}
