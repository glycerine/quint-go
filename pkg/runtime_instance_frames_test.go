package quint

import (
	"context"
	"fmt"
	"testing"
)

// TestInstanceCallFrames pins the execution-frame tree for calls made through
// instance overrides, including a nested instance (module Q imports P with an
// override that reads Q's constant). TypeScript agrees on every value below.
func TestInstanceCallFrames(t *testing.T) {
	src := `module P {const n:int pure def apply(f,x)=f(x)+n} module Q {const m:int import P(n = m*2) as X pure def h(f,x)=X::apply(f,x)} module M {import Q(m=5) as A import Q(m=1) as B def g(x)=x*3 val r=A::h(g,1) val s=B::h((y)=>y+1,2) val t=A::h((y)=>y+1,2)}`
	ids := NewIDGenerator()
	modules, err := parseModulesWithIDs(src, ids)
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name   string
		frames []string // app names, outermost first
		result []string
	}{
		{"r", []string{"Rec", "A::h", "X::apply", "f"}, []string{"true", "13", "13", "3"}},
		{"s", []string{"Rec", "B::h", "X::apply", "f"}, []string{"true", "5", "5", "3"}},
		{"t", []string{"Rec", "A::h", "X::apply", "f"}, []string{"true", "13", "13", "3"}},
	}
	for _, tc := range cases {
		ev, err := NewProjectEvaluator(context.Background(), modules, ids, "M", nil, false)
		if err != nil {
			t.Fatal(err)
		}
		rec := NewTraceRecorder(3, nil)
		ctx := WithExecutionListener(context.Background(), rec)
		if _, err := ev.Evaluate(ctx, tc.name, nil); err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		var names, results []string
		var walk func(f ExecutionFrame)
		walk = func(f ExecutionFrame) {
			names = append(names, f.App.Opcode)
			results = append(results, fmt.Sprint(f.Result))
			if len(f.Subframes) > 0 {
				walk(f.Subframes[0])
			}
		}
		walk(rec.CurrentFrame())
		if len(names) != len(tc.frames) {
			t.Fatalf("%s: frame chain %v, want %v", tc.name, names, tc.frames)
		}
		for i := range names {
			if names[i] != tc.frames[i] || results[i] != tc.result[i] {
				t.Fatalf("%s: frame %d = %s=%s, want %s=%s", tc.name, i, names[i], results[i], tc.frames[i], tc.result[i])
			}
		}
	}
}
