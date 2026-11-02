package apalache

import (
	"context"
	"encoding/json"
	"fmt"
	cmd "github.com/glycerine/apalache-go/shai/cmdexecutor"
	"strings"
	"testing"
)

func TestRejectInvalidConfigAndCancellation(t *testing.T) {
	e := New()
	if _, err := e.Run(context.Background(), cmd.Cmd_CHECK, json.RawMessage(`{`)); err == nil {
		t.Fatal("accepted malformed JSON")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := e.Run(ctx, cmd.Cmd_CHECK, json.RawMessage(`{}`)); err != context.Canceled {
		t.Fatalf("got %v", err)
	}
}

// These typed declarations exercise the exact qnt JSON transport used by Quint,
// including verification's synthesized q:: names.
func TestTypedQuintCommands(t *testing.T) {
	for _, invariant := range []bool{true, false} {
		declarations := []any{}
		types := map[string]any{}
		for i, name := range []string{"q::init", "q::step", "q::inv"} {
			id := 2*i + 2
			value := true
			qualifier := "action"
			if i == 2 {
				value = invariant
				qualifier = "val"
			}
			declarations = append(declarations, map[string]any{"id": id, "kind": "def", "name": name, "qualifier": qualifier, "expr": map[string]any{"id": id + 1, "kind": "bool", "value": value}})
			for _, n := range []int{id, id + 1} {
				types[fmt.Sprint(n)] = map[string]any{"type": map[string]any{"kind": "bool"}}
			}
		}
		ir, err := json.Marshal(map[string]any{"stage": "compiling", "modules": []any{map[string]any{"id": 1, "name": "Probe", "declarations": declarations}}, "types": types, "table": map[string]any{}})
		if err != nil {
			t.Fatal(err)
		}
		config, err := json.Marshal(map[string]any{"source": map[string]any{"kind": "string", "format": "qnt", "content": string(ir)}, "checker": map[string]any{"init": "q::init", "next": "q::step", "inv": []string{"q::inv"}, "length": 0}})
		if err != nil {
			t.Fatal(err)
		}
		response, err := New().Run(context.Background(), cmd.Cmd_CHECK, config)
		if err != nil {
			t.Fatal(err)
		}
		if invariant {
			if response.GetSuccess() == "" {
				t.Fatalf("safe check: %v", response)
			}
		} else {
			failure := response.GetFailure()
			if failure == nil || !strings.Contains(failure.GetData(), `"checking_result":"Error"`) {
				t.Fatalf("violating check: %v", response)
			}
		}
		response, err = New().Run(context.Background(), cmd.Cmd_TLA, config)
		if err != nil {
			t.Fatal(err)
		}
		var tla string
		if err := json.Unmarshal([]byte(response.GetSuccess()), &tla); err != nil || !strings.Contains(tla, "MODULE Probe") {
			t.Fatalf("TLA: %v %v", response, err)
		}
	}
}
