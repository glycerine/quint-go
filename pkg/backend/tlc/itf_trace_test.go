package tlc

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/glycerine/insdict"
	quint "github.com/glycerine/quint-go/pkg"
	native "github.com/glycerine/tlago/tlc"
)

func TestNativeITFTraceMetadataAndNames(t *testing.T) {
	integer := quint.Type{Kind: "int"}
	compiled := quint.CompilationResult{MainModule: quint.Module{Declarations: []quint.Declaration{
		{Kind: "var", Name: "I::x", TypeAnnotation: &integer},
		{Kind: "var", Name: "dollar$name", TypeAnnotation: &integer},
	}}}
	variables, err := nativeITFVariables(compiled)
	if err != nil || len(variables) != 2 || variables[0].tlaName != "I_x" || variables[1].tlaName != "dollar_si_name" {
		t.Fatal(variables, err)
	}
	for _, target := range []int{-1, 0, 1, 2} {
		var loop *int
		if target >= 0 {
			loop = &target
		}
		states := [][]native.Value{{native.NewIntValue(-2147483648), native.NewIntValue(0)}, {native.NewIntValue(2147483647), native.NewIntValue(1)}, {native.NewIntValue(0), native.NewIntValue(2)}}
		encoded, err := nativeITFTrace(context.Background(), variables, states, loop)
		if err != nil {
			t.Fatal(err)
		}
		decoded, err := quint.DecodeITFTrace(encoded)
		if err != nil || !reflect.DeepEqual(decoded.Vars, []string{"I::x", "dollar$name"}) || len(decoded.States) != 3 || !reflect.DeepEqual(decoded.Loop, loop) {
			t.Fatal(string(encoded), err)
		}
		if !strings.Contains(string(encoded), `"#bigint":"2147483647"`) || !strings.Contains(string(encoded), `"#bigint":"-2147483648"`) || strings.Contains(string(encoded), "dollar_si_name") {
			t.Fatal(string(encoded))
		}
		for i, meta := range decoded.StateMetadata {
			var value struct {
				Index int `json:"index"`
			}
			if err := json.Unmarshal(meta, &value); err != nil || value.Index != i {
				t.Fatal(string(meta), err)
			}
		}
	}
}

func TestNativeITFVariablesRejectCollisionsAndMissingTypes(t *testing.T) {
	integer := quint.Type{Kind: "int"}
	for _, names := range [][]string{{"A::x", "A_x"}, {"a!b", "a_i_b"}, {"x", "x"}, {""}, {"#meta"}} {
		compiled := quint.CompilationResult{}
		for _, name := range names {
			compiled.MainModule.Declarations = append(compiled.MainModule.Declarations, quint.Declaration{Kind: "var", Name: name, TypeAnnotation: &integer})
		}
		if _, err := nativeITFVariables(compiled); err == nil {
			t.Fatal("accepted variable names", names)
		}
	}
	compiled := quint.CompilationResult{MainModule: quint.Module{Declarations: []quint.Declaration{{Kind: "var", Name: "x"}}}}
	if _, err := nativeITFVariables(compiled); err == nil {
		t.Fatal("accepted missing variable type")
	}
	// The exact checked type used by the printer overrides a declaration's
	// original annotation after inference/alias expansion.
	compiled.Analysis.Types = insdict.NewDict[string, quint.TypeScheme]()
	compiled.Analysis.Types.Set(compiled.MainModule.Declarations[0].ID.String(), quint.TypeScheme{Type: integer})
	variables, err := nativeITFVariables(compiled)
	if err != nil || len(variables) != 1 || variables[0].typ.Kind != "int" {
		t.Fatal(variables, err)
	}
}

func TestNativeITFTraceFailures(t *testing.T) {
	variables := []itfVariable{{name: "x", tlaName: "x", typ: quint.Type{Kind: "bool"}}}
	for _, tc := range []struct {
		states [][]native.Value
		loop   *int
	}{
		{nil, nil},
		{[][]native.Value{{}}, nil},
		{[][]native.Value{{nil}}, nil},
		{[][]native.Value{{native.NewIntValue(1)}}, nil},
		{[][]native.Value{{native.BoolFalse}}, new(-1)},
		{[][]native.Value{{native.BoolFalse}}, new(1)},
	} {
		if encoded, err := nativeITFTrace(context.Background(), variables, tc.states, tc.loop); err == nil || len(encoded) > 0 {
			t.Fatal(string(encoded), err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := nativeITFTrace(ctx, variables, [][]native.Value{{native.BoolFalse}}, nil); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if result, err := CheckTLA(context.Background(), "M.tla", booleanTLA, Options{ITF: true}); err == nil || result.Complete || result.Outcome != "" {
		t.Fatal(result, err)
	}
	if states := nativeITFStates([]native.Message{{Code: native.ECTLCStatePrint1}, {Code: native.ECTLCStatePrint3}, {Code: native.ECTLCStatePrint2}}); len(states) != 0 {
		t.Fatal(states)
	}
}

func TestNativeITFExportFailurePreservesVerdict(t *testing.T) {
	variables := []itfVariable{{name: "x", tlaName: "x", typ: quint.Type{Kind: "int"}}}
	result, err := checkTLA(context.Background(), "M.tla", booleanTLA, Options{Workers: 1, Invariants: []string{"Bad"}, ITF: true}, variables)
	if err != nil || !result.Complete || result.Outcome != Violation || result.ITFError == "" || len(result.ITF) != 0 || len(result.Trace) != 2 {
		t.Fatal(result, err)
	}
}

func TestNativeITFSourceCorpus(t *testing.T) {
	if os.Getenv("QUINT_CORPUS_TESTS") == "" {
		t.Skip("native TLC typed ITF source corpus; set QUINT_CORPUS_TESTS=1")
	}
	for _, tc := range []struct {
		name, source, temporal string
		outcome                Outcome
		loop                   bool
	}{
		{"safe", `module M { var x:bool action init=x'=false action step=x'=not(x) val safe=true }`, "", Safe, false},
		{"safety next state", `module M { var x:bool action init=x'=false action step=x'=not(x) val safe=not(x) }`, "", Violation, false},
		{"safety initial", `module M { var x:bool action init=x'=false action step=x'=not(x) val safe=x }`, "", Violation, false},
		{"stuttering temporal", `module M { var x:bool action init=x'=false action step=x'=not(x) val safe=true temporal live=eventually(x) }`, "live", Violation, true},
		{"fair lasso", `module M { var x:int action init=x'=0 action step=x'=1-x val safe=true temporal live=weakFair(step,x) implies eventually(x==2) }`, "live", Violation, true},
		{"original namespace", `module A { var x:bool action init=x'=false action step=x'=not(x) val safe=not(x) } module M { import A as I action init=I::init action step=I::step val safe=I::safe }`, "", Violation, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			compiled := compileSource(t, tc.source, tc.temporal)
			opts := Options{Workers: 2, ITF: true}
			if tc.temporal != "" {
				opts.Temporal = []string{"q_temporalProps"}
			}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			result, err := Check(ctx, compiled, opts)
			if err != nil || !result.Complete || result.Outcome != tc.outcome {
				t.Fatal(result, err)
			}
			if tc.outcome == Safe {
				if len(result.ITF) > 0 {
					t.Fatal(string(result.ITF))
				}
				return
			}
			trace, err := quint.DecodeITFTrace(result.ITF)
			if err != nil || len(trace.States) != len(result.Trace) || len(trace.Vars) != 1 || (trace.Loop != nil) != tc.loop || !reflect.DeepEqual(trace.Loop, result.Loop) {
				t.Fatal(string(result.ITF), result, err)
			}
			for _, decl := range compiled.MainModule.Declarations {
				if decl.Kind == "var" && trace.Vars[0] != decl.Name {
					t.Fatal(trace.Vars, decl.Name)
				}
			}
			for i, state := range trace.States {
				if len(state.Args) != 2 || state.Args[0].Str != trace.Vars[0] {
					t.Fatal(string(result.ITF))
				}
				value := state.Args[1]
				text := result.Trace[i].Values[0].Value
				if value.Kind == "bool" {
					if value.Bool != (text == "TRUE") || text != "TRUE" && text != "FALSE" {
						t.Fatal(string(result.ITF), result.Trace)
					}
				} else if value.Kind != "int" || value.Int.String() != text {
					t.Fatal(string(result.ITF), result.Trace)
				}
			}
		})
	}
}

func TestNativeITFStructuredSourceCorpus(t *testing.T) {
	if os.Getenv("QUINT_CORPUS_TESTS") == "" {
		t.Skip("native TLC structured typed ITF source corpus; set QUINT_CORPUS_TESTS=1")
	}
	source := `module M {
type Maybe = Some(int) | None
var n:int
var xs:List[int]
var pair:(int,bool)
var bag:Set[int]
var mapping:str -> int
var record:{a:int,b:bool}
var some:Maybe
var none:Maybe
var unit:()
action init=all { n'=2147483647, xs'=[1,2], pair'=(3,true), bag'=Set(2,1), mapping'=Map(("a",1),("b",2)), record'={a:4,b:true}, some'=Some(5), none'=None, unit'=() }
action step=all { n'=n, xs'=xs, pair'=pair, bag'=bag, mapping'=mapping, record'=record, some'=some, none'=none, unit'=unit }
val safe=some != Some(5)
}`
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	result, err := Check(ctx, compileSource(t, source), Options{Workers: 2, ITF: true})
	if err != nil || !result.Complete || result.Outcome != Violation {
		t.Fatal(result, err)
	}
	decoded, err := quint.DecodeITFTrace(result.ITF)
	if err != nil || len(decoded.Vars) != 9 || len(decoded.States) != 1 || decoded.Loop != nil {
		t.Fatal(string(result.ITF), err)
	}
	for _, fragment := range []string{
		`"n":{"#bigint":"2147483647"}`,
		`"xs":[{"#bigint":"1"},{"#bigint":"2"}]`,
		`"pair":{"#tup":[{"#bigint":"3"},true]}`,
		`"bag":{"#set":[{"#bigint":"1"},{"#bigint":"2"}]}`,
		`"mapping":{"#map":[["a",{"#bigint":"1"}],["b",{"#bigint":"2"}]]}`,
		`"record":{"a":{"#bigint":"4"},"b":true}`,
		`"some":{"tag":"Some","value":{"#bigint":"5"}}`,
		`"none":{"tag":"None","value":{"#tup":[]}}`,
		`"unit":{"#tup":[]}`,
	} {
		if !strings.Contains(string(result.ITF), fragment) {
			t.Fatalf("missing %s in %s", fragment, result.ITF)
		}
	}
}
