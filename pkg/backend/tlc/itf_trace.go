package tlc

import (
	"context"
	"encoding/json"
	"fmt"

	checker "github.com/glycerine/apalache-go"
	"github.com/glycerine/insdict"
	quint "github.com/glycerine/quint-go/pkg"
	native "github.com/glycerine/tlago/tlc"
)

type itfVariable struct {
	name, tlaName string
	typ           quint.Type
}

func nativeITFVariables(compiled quint.CompilationResult) ([]itfVariable, error) {
	variables := []itfVariable{}
	names := insdict.NewDict[string, string]()
	for _, decl := range compiled.MainModule.Declarations {
		if decl.Kind != "var" {
			continue
		}
		if decl.Name == "" || decl.Name == "#meta" {
			return nil, fmt.Errorf("TLC ITF invalid variable name %q", decl.Name)
		}
		name := checker.ParseableTLAName(decl.Name)
		if old, ok := names.Get2(name); ok {
			return nil, fmt.Errorf("TLC ITF variable names %q and %q collide as %q", old, decl.Name, name)
		}
		names.Set(name, decl.Name)
		var typ quint.Type
		if compiled.Analysis.Types != nil {
			if scheme, ok := compiled.Analysis.Types.Get2(decl.ID.String()); ok {
				typ = scheme.Type
			}
		}
		if typ.Kind == "" && decl.TypeAnnotation != nil {
			typ = *decl.TypeAnnotation
		}
		if typ.Kind == "" {
			return nil, fmt.Errorf("TLC ITF variable %s has no checked type", decl.Name)
		}
		if compiled.Table != nil {
			var err error
			typ, err = quint.InlineTypeAliasesInType(compiled.Table, typ)
			if err != nil {
				return nil, fmt.Errorf("TLC ITF variable %s type: %w", decl.Name, err)
			}
		}
		variables = append(variables, itfVariable{name: decl.Name, tlaName: name, typ: typ})
	}
	return variables, nil
}

// nativeITFTrace eagerly snapshots values; no native object escapes ownership.
// The common Quint encoder preserves value tags, variable/field order, and
// exact decimal integers. TLC's closing transition uses ITF's zero-based loop.
func nativeITFTrace(ctx context.Context, variables []itfVariable, states [][]native.Value, loop *int) (json.RawMessage, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if len(states) == 0 {
		return nil, fmt.Errorf("TLC ITF requires a concrete counterexample")
	}
	if loop != nil && (*loop < 0 || *loop >= len(states)) {
		return nil, fmt.Errorf("TLC ITF loop %d is outside %d states", *loop, len(states))
	}
	names := make([]string, len(variables))
	for i, variable := range variables {
		names[i] = variable.name
	}
	expressions := make([]quint.Expr, 0, len(states))
	for index, state := range states {
		if len(state) != len(variables) {
			return nil, fmt.Errorf("TLC ITF state %d has %d values for %d variables", index, len(state), len(variables))
		}
		args := make([]quint.Expr, 0, 2*len(variables))
		for i, value := range state {
			expr, err := nativeITFExpr(ctx, value, variables[i].typ)
			if err != nil {
				return nil, fmt.Errorf("TLC ITF state %d variable %s: %w", index, variables[i].name, err)
			}
			args = append(args, quint.Expr{Kind: "str", Str: variables[i].name}, expr)
		}
		expressions = append(expressions, quint.Expr{Kind: "app", Opcode: "Rec", Args: args})
	}
	encoded, err := quint.EncodeITFTrace(names, expressions, false)
	if err != nil {
		return nil, err
	}
	var trace struct {
		Vars   []string          `json:"vars"`
		States []json.RawMessage `json:"states"`
		Loop   *int              `json:"loop,omitempty"`
	}
	if err := json.Unmarshal(encoded, &trace); err != nil {
		return nil, err
	}
	trace.Loop = loop
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return json.Marshal(trace)
}

func nativeITFStates(messages []native.Message) []*native.TLCStateMut {
	states := []*native.TLCStateMut{}
	for _, message := range messages {
		if message.Code != native.ECTLCStatePrint1 && message.Code != native.ECTLCStatePrint2 {
			continue
		}
		state := message.State
		if message.StateInfo != nil {
			state = message.StateInfo.GetOriginalState()
		}
		if state != nil {
			states = append(states, state)
		}
	}
	return states
}
