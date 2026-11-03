package tlc

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	quint "github.com/glycerine/quint-go/pkg"
	goapalache "github.com/glycerine/quint-go/pkg/backend/apalache"
)

// This corpus compares the two production checker paths. Apalache consumes the
// assignment-based Quint IR; native TLC consumes generated predicate-based TLA+.
// The selected violating properties identify a unique initial state or successor,
// so the witness assertions check semantics without relying on enumeration order.
func TestGeneratedTLAInitializationSemanticsCorpus(t *testing.T) {
	if os.Getenv("QUINT_CORPUS_TESTS") == "" {
		t.Skip("actual Apalache/TLC initializer corpus; set QUINT_CORPUS_TESTS=1")
	}
	for _, tc := range []struct {
		name, source, safe, violated string
		initial, distinct            int64
		steps                        int
		witness                      []map[string]string
		run                          bool
	}{
		{
			name: "nondet helper lambda",
			source: `module M {
  var x:int action write(n)=x'=n
  action init=nondet n=Set(1,2).oneOf(); write(n)
  action step=x'=x val safe=@SAFE@
}`,
			safe: "x==1 or x==2", violated: "x!=2", initial: 2, distinct: 2,
			witness: []map[string]string{{"x": "2"}},
		},
		{
			name: "dependent nested nondet",
			source: `module M {
  var x:int var y:int action write(a,b)=all { x'=a, y'=b }
  action init=nondet a=Set(1,2).oneOf(); nondet b=Set(a,a+1).oneOf(); write(a,b)
  action step=all { x'=x, y'=y } val safe=@SAFE@
}`,
			safe: "(x==1 or x==2) and (y==x or y==x+1)", violated: "x!=2 or y!=3", initial: 4, distinct: 4,
			witness: []map[string]string{{"x": "2", "y": "3"}},
		},
		{
			name: "nested all any if",
			source: `module M {
  var x:int var y:int action pair(a,b)=all { x'=a, y'=b }
  action init=any { pair(0,1), all { true, if(1<2) pair(2,3) else pair(4,5) } }
  action step=all { x'=x, y'=y } val safe=@SAFE@
}`,
			safe: "(x==0 and y==1) or (x==2 and y==3)", violated: "x!=2 or y!=3", initial: 2, distinct: 2,
			witness: []map[string]string{{"x": "2", "y": "3"}},
		},
		{
			name: "deep helper preserves successor assignment",
			source: `module M {
  var x:int action leaf(n)=x'=n
  action a(n)=leaf(n) action b(n)=a(n) action c(n)=b(n)
  action d(n)=c(n) action e(n)=d(n) action f(n)=e(n)
  action init=f(7) action step=x'=8 val safe=@SAFE@
}`,
			safe: "x==7 or x==8", violated: "x==7", initial: 1, distinct: 2, steps: 1,
			witness: []map[string]string{{"x": "7"}, {"x": "8"}},
		},
		{
			name: "instance constant override and pure helper",
			source: `module A {
  const bound:int var x:int pure val offset=2 pure def plus(n)=n+offset
  action write(n)=x'=plus(n) action init=write(bound) action step=x'=x
}
module M {
  import A(bound=3) as I
  action init=I::init action step=I::step val safe=@SAFE@
}`,
			safe: "I::x==5", violated: "I::x!=5", initial: 1, distinct: 1,
			witness: []map[string]string{{"M::I::x": "5"}},
		},
		{
			name: "run sharing init removed before conversion",
			source: `module M {
  var x:int action write=x'=1 action init=write action step=x'=x
  run sample=init.then(step) val safe=@SAFE@
}`,
			safe: "x==1", violated: "x!=1", initial: 1, distinct: 1, run: true,
			witness: []map[string]string{{"x": "1"}},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, violation := range []bool{false, true} {
				name, property := "safe", tc.safe
				if violation {
					name, property = "violation", tc.violated
				}
				t.Run(name, func(t *testing.T) {
					ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
					defer cancel()
					compiled := compileSource(t, strings.ReplaceAll(tc.source, "@SAFE@", property))
					before, err := json.Marshal(compiled.MainModule)
					if err != nil {
						t.Fatal(err)
					}
					if tc.run {
						found := false
						for _, d := range compiled.MainModule.Declarations {
							found = found || d.Kind == "def" && d.Qualifier == quint.Run && d.Name == "sample"
						}
						if !found {
							t.Fatal("run definition was absent before checker preparation")
						}
						tla, err := goapalache.New().TLA(ctx, compiled)
						if err != nil || strings.Contains(tla, "sample") {
							t.Fatalf("run was not removed before TLA generation: %s\n%v", tla, err)
						}
					}
					bound := 1
					if violation {
						bound = tc.steps
					}
					symbolic, err := goapalache.New().Check(ctx, compiled, nil, goapalache.CheckOptions{MaxSteps: bound})
					if err != nil {
						t.Fatal("assignment IR checker:", err)
					}
					concrete, err := Check(ctx, compiled, Options{Workers: 1, ITF: true})
					if err != nil || !concrete.Complete || concrete.ITFError != "" {
						t.Fatal("generated TLA checker:", concrete, err)
					}
					after, err := json.Marshal(compiled.MainModule)
					if err != nil || !bytes.Equal(before, after) {
						t.Fatal("checker preparation mutated compiled module", err)
					}
					if !violation {
						if symbolic.Outcome != goapalache.CheckSafe || concrete.Outcome != Safe || concrete.InitialStates != tc.initial || int64(concrete.DistinctStates) != tc.distinct {
							t.Fatalf("symbolic=%+v concrete=%+v; expected %d initial/%d distinct states", symbolic, concrete, tc.initial, tc.distinct)
						}
						if len(symbolic.Counterexamples) != 0 || len(concrete.ITF) != 0 || len(concrete.Trace) != 0 {
							t.Fatal("safe model returned a counterexample", symbolic, concrete)
						}
						return
					}
					if symbolic.Outcome != goapalache.CheckViolation || len(symbolic.Counterexamples) != 1 || concrete.Outcome != Violation {
						t.Fatal("expected independent property violations", symbolic, concrete)
					}
					for _, witness := range []json.RawMessage{symbolic.Counterexamples[0], concrete.ITF} {
						assertGenerationIntegerWitness(t, witness, tc.witness)
					}
				})
			}
		})
	}
}

func assertGenerationIntegerWitness(t *testing.T, encoded json.RawMessage, expected []map[string]string) {
	t.Helper()
	trace, err := quint.DecodeITFTrace(encoded)
	if err != nil || trace.Loop != nil || len(trace.States) != len(expected) {
		t.Fatalf("unexpected finite witness: %s; %v", encoded, err)
	}
	for i, state := range trace.States {
		if state.Kind != "app" || state.Opcode != "Rec" || len(state.Args) != 2*len(expected[i]) {
			t.Fatalf("unexpected state %d: %s", i, encoded)
		}
		values := make(map[string]string, len(expected[i]))
		for j := 0; j < len(state.Args); j += 2 {
			if state.Args[j].Kind != "str" || state.Args[j+1].Kind != "int" {
				t.Fatalf("expected integer assignment at state %d: %s", i, encoded)
			}
			values[state.Args[j].Str] = state.Args[j+1].Int.String()
		}
		if !reflect.DeepEqual(values, expected[i]) {
			t.Fatalf("state %d = %v, expected %v; %s", i, values, expected[i], encoded)
		}
	}
}
