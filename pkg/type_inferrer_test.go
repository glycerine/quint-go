package quint

import "testing"

func TestIncrementalTypeInferrer(t *testing.T) {
	ms, err := ParseModules("module M { def identity(x)=x val a=identity(1) val b=identity(true) }")
	if err != nil {
		t.Fatal(err)
	}
	linked, _ := ResolveLinkedModules(ms)
	inferrer := NewTypeInferrer(linked.Table, nil)
	first, err := inferrer.InferTypes(ms[0].Declarations[:1])
	if err != nil {
		t.Fatal(err)
	}
	second, err := inferrer.InferTypes(ms[0].Declarations[1:])
	if err != nil {
		t.Fatal(err)
	}
	if first.Types != second.Types {
		t.Fatal("type table replaced")
	}
	for _, e := range second.Errors.All() {
		t.Fatal(e)
	}
	if second.Types.Get(ms[0].Declarations[1].ID.String()).Type.Kind != "int" || second.Types.Get(ms[0].Declarations[2].ID.String()).Type.Kind != "bool" {
		t.Fatal("incremental types")
	}
	resumed := NewTypeInferrer(linked.Table, second.Types)
	if resumed.Context.Types != second.Types {
		t.Fatal("seed table not reused")
	}
	if _, err = resumed.InferTypes(nil); err != nil {
		t.Fatal(err)
	}
}
func TestTypeInferrerAssumptionAndSimplification(t *testing.T) {
	ms, err := ParseModules("module M { val r={x:1,y:true}.with(\"x\",2) assume axiom=1 }")
	if err != nil {
		t.Fatal(err)
	}
	linked, _ := ResolveLinkedModules(ms)
	inferrer := NewTypeInferrer(linked.Table, nil)
	result, err := inferrer.InferTypes(ms[0].Declarations)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range result.Errors.All() {
		t.Fatal(e)
	}
	typ := result.Types.Get(ms[0].Declarations[0].ID.String()).Type
	if typ.Fields.Other.Kind != "empty" || len(typ.Fields.Fields) != 2 {
		t.Fatal("row not simplified")
	}
	if len(inferrer.Constraints) != 1 {
		t.Fatal("pending constraint lost")
	}
	if _, err = inferrer.Solve(); err != nil {
		t.Fatal(err)
	}
	count := 0
	for range result.Errors.All() {
		count++
	}
	if count != 1 {
		t.Fatal("live diagnostics")
	}
}
