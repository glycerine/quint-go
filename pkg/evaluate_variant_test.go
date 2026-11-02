package quint

import (
	"context"
	"errors"
	"testing"
)

func TestVariantMatchValidationAndOrder(t *testing.T) {
	for _, source := range []string{
		`matchVariant()`,
		`matchVariant(variant("A",1),"A")`,
		`matchVariant(1,"A",x=>x)`,
		`matchVariant(variant("A",1),1,x=>x)`,
		`matchVariant(variant("A",1),"A",1)`,
		`matchVariant(variant("A",1),"A",(x,y)=>x)`,
	} {
		e, err := ParseExpression(source)
		if err != nil {
			t.Fatal(source, err)
		}
		if _, err := EvaluatePureExpression(context.Background(), e, nil); err == nil {
			t.Fatal("invalid variant match accepted", source)
		}
	}
	// Labels are visited in source order; errors in earlier labels propagate.
	source := `matchVariant(variant("A",1),1/0,x=>0,"A",x=>x)`
	e, err := ParseExpression(source)
	if err != nil {
		t.Fatal(err)
	}
	_, err = EvaluatePureExpression(context.Background(), e, nil)
	var d Diagnostic
	if !errors.As(err, &d) || d.Code != "QNT503" {
		t.Fatal(err)
	}
	// Matching preserves the source variant and the payload's owned collection.
	e, err = ParseExpression(`{ val v=variant("A",List(1,2)); List(match v { A(x)=>x.append(3) },match v { A(x)=>x }) }`)
	if err != nil {
		t.Fatal(err)
	}
	got, err := EvaluatePureExpression(context.Background(), e, nil)
	if err != nil || got.String() != "List(List(1, 2, 3), List(1, 2))" {
		t.Fatal(got.String(), err)
	}
}
