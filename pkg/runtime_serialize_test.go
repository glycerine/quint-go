package quint

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"testing"
)

func TestUpstreamRuntimeSerialization(t *testing.T) {
	t.Parallel()
	var fixtures []struct {
		Source string
		Input  Expr
		Runs   []struct {
			InitialUID uint32
			FinalUID   uint32
			Output     Expr
		}
	}
	data, err := os.ReadFile("testdata/parser/runtime_serialize.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	for _, fixture := range fixtures {
		t.Run(fixture.Source, func(t *testing.T) {
			value, err := EvaluatePureExpression(context.Background(), fixture.Input, nil)
			if err != nil {
				t.Fatal(err)
			}
			for index, run := range fixture.Runs {
				var state RuntimeObjectHashState
				if err = state.Restore(run.InitialUID); err != nil {
					t.Fatal(err)
				}
				got, err := SerializeRuntimeValue(value, &state)
				if err != nil {
					t.Fatal(index, err)
				}
				actual, err := json.Marshal(got)
				if err != nil {
					t.Fatal(err)
				}
				want, err := json.Marshal(run.Output)
				if err != nil {
					t.Fatal(err)
				}
				if !bytes.Equal(actual, want) || state.Counter() != run.FinalUID {
					t.Fatalf("run %d counter=%d want %d\ngot  %s\nwant %s", index, state.Counter(), run.FinalUID, actual, want)
				}
			}
		})
	}
}

func TestRuntimeSerializationHistoryAndOwnership(t *testing.T) {
	input, err := ParseExpression("Map((0,1),(1,1),(2,1),(3,1),(4,1),(5,1),(6,1),(\"é\",1),(\"é\",1))")
	if err != nil {
		t.Fatal(err)
	}
	value, err := NewRuntimeValue(input)
	if err != nil {
		t.Fatal(err)
	}
	first, err := runtimeSerialize(value)
	if err != nil {
		t.Fatal(err)
	}
	firstJSON, _ := json.Marshal(first)
	var state RuntimeObjectHashState
	seen := map[string]bool{}
	for i := 0; i < 8; i++ {
		output, err := SerializeRuntimeValue(value, &state)
		if err != nil {
			t.Fatal(err)
		}
		encoded, _ := json.Marshal(output)
		seen[string(encoded)] = true
	}
	if len(seen) != 2 || state.Counter() != 8*18 {
		t.Fatal("serialization lost observable UID history", len(seen), state.Counter())
	}
	// Every serialized node is caller-owned; a nested mutation cannot change
	// retained children or a later fresh-state serialization.
	first.Args[0].Args[0] = Expr{Kind: "str", Str: "mutated"}
	again, err := runtimeSerialize(value)
	againJSON, _ := json.Marshal(again)
	if err != nil || !bytes.Equal(againJSON, firstJSON) {
		t.Fatal("serialization mutated retained map", err)
	}
	closure := newRuntimeClosure(2, func(context.Context, []RuntimeValue) (RuntimeValue, error) { return RuntimeValue{}, nil })
	list, err := newRuntimeComposite("List", []RuntimeValue{closure})
	if err != nil {
		t.Fatal(err)
	}
	output, err := runtimeSerialize(list)
	if err != nil {
		t.Fatal(err)
	}
	output.Args[0].Params[0].Name = "mutated"
	output.Args[0].Body.Str = "mutated"
	fresh, err := runtimeSerialize(list)
	if err != nil || fresh.Args[0].Params[0].Name != "_a0" || fresh.Args[0].Body.Str != "lambda_2_params" {
		t.Fatal("serialization shares closure expression", err)
	}
}

func TestRuntimeSerializationValidation(t *testing.T) {
	for _, value := range []RuntimeValue{
		{},
		{identity: "unsupported", expression: Expr{Kind: "name", Name: "unsupported"}},
		{identity: "unsupported", expression: Expr{Kind: "app", Opcode: "unsupported"}},
		{identity: "missing", expression: Expr{Kind: "app", Opcode: "List"}},
		{identity: "missing", expression: Expr{Kind: "app", Opcode: "Set"}},
		{identity: "missing", expression: Expr{Kind: "app", Opcode: "Map"}},
	} {
		if _, err := runtimeSerialize(value); err == nil {
			t.Fatal("invalid serialization accepted", value.String())
		}
	}
}
