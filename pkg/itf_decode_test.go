package quint

import (
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestUpstreamITFDecoding(t *testing.T) {
	var fixtures []struct {
		Name   string
		Input  json.RawMessage
		Output []Expr
	}
	data, err := os.ReadFile("testdata/parser/itf_decode.json")
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
			got, _ := json.Marshal(trace.States)
			want, _ := json.Marshal(f.Output)
			var a, b any
			json.Unmarshal(got, &a)
			json.Unmarshal(want, &b)
			if !reflect.DeepEqual(a, b) {
				t.Fatalf("Go %s\nTS %s", got, want)
			}
			if string(trace.Raw) != string(f.Input) {
				t.Fatal("raw trace changed")
			}
			if f.Name == "metadata" && (trace.Loop == nil || *trace.Loop != 0 || len(trace.Metadata) == 0 || len(trace.StateMetadata) != 1 || len(trace.StateMetadata[0]) == 0) {
				t.Fatal("metadata lost", trace)
			}
		})
	}
}

func TestITFNumbersAndObjectOrder(t *testing.T) {
	for _, tc := range []struct{ input, want string }{{"1.0", "1"}, {"1e3", "1000"}, {"-2E2", "-200"}, {"9007199254740993", "9007199254740993"}, {`{"#bigint":"-9007199254740993123456789"}`, "-9007199254740993123456789"}} {
		value, err := DecodeITFValue([]byte(tc.input), nil)
		if err != nil || value.Kind != "int" || value.Int.String() != tc.want {
			t.Fatal(value, err)
		}
	}
	value, err := DecodeITFValue([]byte(`{"b":0,"10":1,"2":2,"a":3,"01":4,"4294967295":5,"0":6}`), nil)
	if err != nil {
		t.Fatal(err)
	}
	names := []string{}
	for i := 0; i < len(value.Args); i += 2 {
		names = append(names, value.Args[i].Str)
	}
	if !reflect.DeepEqual(names, []string{"0", "2", "10", "b", "a", "01", "4294967295"}) {
		t.Fatal(names)
	}
	ids := NewIDGenerator()
	first, err := DecodeITFValue([]byte(`true`), ids)
	if err != nil {
		t.Fatal(err)
	}
	next, err := DecodeITFValue([]byte(`false`), ids)
	if err != nil || first.ID.String() != "1" || next.ID.String() != "2" {
		t.Fatal(first, next, err)
	}
}

func TestITFRejectsMalformedValues(t *testing.T) {
	for _, input := range []string{`null`, `{`, `true false`, `1.5`, `{"#bigint":1}`, `{"#bigint":"bad"}`, `{"#set":null}`, `{"#tup":{}}`, `{"#map":null}`, `{"#map":[[1]]}`, `{"#map":[[1,2,3]]}`, `{"#unserializable":2}`, `{"#unserializable":null}`} {
		if _, err := DecodeITFValue([]byte(input), nil); err == nil {
			t.Fatal("malformed value accepted", input)
		}
	}
	for _, input := range []string{`null`, `{}`, `{"states":null}`, `{"states":[null]}`, `{"states":[]} {}`} {
		if _, err := DecodeITFTrace([]byte(input)); err == nil {
			t.Fatal("malformed trace accepted", input)
		}
	}
	if _, err := DecodeITFTrace([]byte(`{"states":[null]}`)); err == nil || !strings.Contains(err.Error(), "state 0") {
		t.Fatal(err)
	}
}

func TestITFTraceSourceIsolation(t *testing.T) {
	input := []byte(`{"vars":["x"],"params":["N"],"loop":0,"#meta":{"extra":42},"states":[{"#meta":{"index":0},"x":{"#bigint":"2"}}]}`)
	trace, err := DecodeITFTrace(input)
	if err != nil {
		t.Fatal(err)
	}
	input[0] = 'x'
	if trace.Raw[0] != '{' || trace.Metadata[0] != '{' || trace.StateMetadata[0][0] != '{' || trace.Params[0] != "N" || trace.Vars[0] != "x" {
		t.Fatal("trace metadata changed")
	}
	trace.States[0].Args[1].Int, _ = ParseInteger("3")
	if !strings.Contains(string(trace.Raw), `"#bigint":"2"`) {
		t.Fatal("raw trace mutated")
	}
}
