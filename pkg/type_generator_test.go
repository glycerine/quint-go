package quint

import "testing"

func TestExpressionConstraintGeneration(t *testing.T) {
	for _, tc := range []struct{ source, want string }{
		{"1+2", "int"}, {"List(1,2)", "List[int]"}, {"{x:1,y:true}.x", "int"},
		{"(1,true)._2", "bool"}, {"{ val x=1; x+2 }", "int"},
		{"{ def identity(x)=x; (identity(1),identity(true)) }", "(int, bool)"},
	} {
		t.Run(tc.source, func(t *testing.T) {
			e, err := ParseExpression(tc.source)
			if err != nil {
				t.Fatal(err)
			}
			names := NewExpressionNameResolver(NewNameScope("M"))
			names.Resolve(&e, 0)
			if len(names.Errors) != 0 {
				t.Fatal(names.Errors)
			}
			generator := NewTypeConstraintGenerator(TypeUnifier{Table: names.Table})
			if err = generator.Generate(&e); err != nil {
				t.Fatal(err)
			}
			if _, err = generator.Solve(); err != nil {
				t.Fatal(err)
			}
			for _, e := range generator.Context.Errors.All() {
				t.Fatal(e)
			}
			got := TypeString(SimplifyType(generator.Context.Types.Get(e.ID.String()).Type))
			if got != tc.want {
				t.Fatalf("%s want %s", got, tc.want)
			}
		})
	}
}
func TestExpressionInferenceErrorsAndLambda(t *testing.T) {
	for _, source := range []string{"1+true", "List(1,true)", "{ val x:bool=1; x }"} {
		e, err := ParseExpression(source)
		if err != nil {
			t.Fatal(err)
		}
		names := NewExpressionNameResolver(NewNameScope("M"))
		names.Resolve(&e, 0)
		generator := NewTypeConstraintGenerator(TypeUnifier{Table: names.Table})
		if err = generator.Generate(&e); err != nil {
			t.Fatal(err)
		}
		if _, err = generator.Solve(); err != nil {
			t.Fatal(err)
		}
		if !generator.failed() {
			t.Fatal("accepted " + source)
		}
	}
	e, err := ParseExpression("(x)=>x+1")
	if err != nil {
		t.Fatal(err)
	}
	names := NewExpressionNameResolver(NewNameScope("M"))
	names.Resolve(&e, 0)
	generator := NewTypeConstraintGenerator(TypeUnifier{Table: names.Table})
	if err = generator.Generate(&e); err != nil {
		t.Fatal(err)
	}
	if _, err = generator.Solve(); err != nil {
		t.Fatal(err)
	}
	if generator.failed() {
		t.Fatal("lambda rejected")
	}
	got := TypeString(generator.Context.Types.Get(e.ID.String()).Type)
	if got != "(int) => int" {
		t.Fatal(got)
	}
}
