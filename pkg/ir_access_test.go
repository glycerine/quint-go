package quint

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
	"testing"
)

type irAccessEvent struct {
	Event string `json:"event"`
	Kind  string `json:"kind"`
	ID    *ID    `json:"id,omitempty"`
	Depth int    `json:"depth"`
}

type irAccessFixture struct {
	Name    string          `json:"name"`
	Modules []Module        `json:"modules"`
	Events  []irAccessEvent `json:"events"`
	Finds   []irAccessQuery `json:"finds"`
}

type irAccessQuery struct {
	Kind string          `json:"kind"`
	ID   ID              `json:"id"`
	Node json.RawMessage `json:"node"`
}

func irAccessFixtures(t *testing.T) []irAccessFixture {
	t.Helper()
	data, err := os.ReadFile("testdata/parser/api_ir_access.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixtures []irAccessFixture
	if err := json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	return fixtures
}

func irAccessTestVisitor(visit func(irAccessEvent) error) *IRVisitor {
	v := &IRVisitor{DefinitionDepth: -1}
	event := func(name, kind string, id *ID) error {
		return visit(irAccessEvent{name, kind, id, v.DefinitionDepth})
	}
	v.EnterModule = func(m *Module) error { return event("enterModule", "module", &m.ID) }
	v.ExitModule = func(m *Module) error { return event("exitModule", "module", &m.ID) }
	v.EnterDeclaration = func(d *Declaration) error { return event("enterDecl", d.Kind, &d.ID) }
	v.ExitDeclaration = func(d *Declaration) error { return event("exitDecl", d.Kind, &d.ID) }
	v.EnterDefinition = func(d IRDefinition) error { id := d.ID(); return event("enterDef", d.Kind(), &id) }
	v.ExitDefinition = func(d IRDefinition) error { id := d.ID(); return event("exitDef", d.Kind(), &id) }
	v.EnterExpression = func(e *Expr) error { return event("enterExpr", e.Kind, &e.ID) }
	v.ExitExpression = func(e *Expr) error { return event("exitExpr", e.Kind, &e.ID) }
	v.EnterType = func(t *Type) error { return event("enterType", t.Kind, t.ID) }
	v.ExitType = func(t *Type) error { return event("exitType", t.Kind, t.ID) }
	v.EnterRow = func(r *Row) error { return event("enterRow", r.Kind, nil) }
	v.ExitRow = func(r *Row) error { return event("exitRow", r.Kind, nil) }
	return v
}

func TestUpstreamIRAccess(t *testing.T) {
	t.Parallel()
	for _, fixture := range irAccessFixtures(t) {
		t.Run(fixture.Name, func(t *testing.T) {
			t.Parallel()
			before, err := json.Marshal(fixture.Modules)
			if err != nil {
				t.Fatal(err)
			}
			events := []irAccessEvent{}
			visitor := irAccessTestVisitor(func(e irAccessEvent) error { events = append(events, e); return nil })
			if err := irAccessModules(fixture.Modules, visitor); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(events, fixture.Events) {
				got, _ := json.Marshal(events)
				want, _ := json.Marshal(fixture.Events)
				t.Fatalf("Go %s\nTS %s", got, want)
			}
			for _, query := range fixture.Finds {
				var node any
				switch query.Kind {
				case "expression":
					node, err = FindExpression(fixture.Modules, query.ID)
				case "type":
					node, err = FindType(fixture.Modules, query.ID)
				case "definition":
					var definition IRDefinition
					definition, err = FindDefinition(fixture.Modules, query.ID)
					if definition.Declaration != nil {
						node = definition.Declaration
					} else if definition.Operator != nil {
						node = definition.Operator
					}
				case "parameter":
					node, err = FindParameter(fixture.Modules, query.ID)
				default:
					t.Fatalf("unknown oracle query kind %s", query.Kind)
				}
				if err != nil {
					t.Fatalf("%s %s: %v", query.Kind, query.ID, err)
				}
				got, marshalErr := json.Marshal(node)
				if marshalErr != nil {
					t.Fatal(marshalErr)
				}
				var actual, expected any
				if err := json.Unmarshal(got, &actual); err != nil {
					t.Fatal(err)
				}
				if err := json.Unmarshal(query.Node, &expected); err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(actual, expected) {
					t.Errorf("%s %s: Go %s; TS %s", query.Kind, query.ID, got, query.Node)
				}
			}
			after, err := json.Marshal(fixture.Modules)
			if err != nil || string(before) != string(after) {
				t.Fatalf("traversal changed caller IR: %v", err)
			}
		})
	}
}

func TestIRVisitorCallbackErrors(t *testing.T) {
	t.Parallel()
	fixture := irAccessFixtures(t)[0]
	stop := errors.New("stop visiting")
	for index := range fixture.Events {
		count := 0
		visitor := irAccessTestVisitor(func(e irAccessEvent) error {
			count++
			if count == index+1 {
				return stop
			}
			return nil
		})
		visitor.DefinitionDepth = 37
		entryDepth := visitor.DefinitionDepth
		var err error
		for i := range fixture.Modules {
			entryDepth = visitor.DefinitionDepth
			if err = visitor.WalkModule(&fixture.Modules[i]); err != nil {
				break
			}
		}
		if !errors.Is(err, stop) {
			t.Fatalf("callback %d: %v", index, err)
		}
		if count != index+1 || visitor.DefinitionDepth != entryDepth {
			t.Fatalf("callback %d: visited %d, retained depth %d", index, count, visitor.DefinitionDepth)
		}
	}
}

func TestIRAccessBorrowedPointersAndParameters(t *testing.T) {
	t.Parallel()
	id := func(n string) ID {
		value, err := ParseID(n)
		if err != nil {
			t.Fatal(err)
		}
		return value
	}
	typeID := id("4")
	typ := Type{Kind: "int", ID: &typeID}
	local := &OperatorDef{ID: id("6"), Kind: "def", Name: "local", Expr: &Expr{ID: id("7"), Kind: "bool"}}
	lambda := &Expr{ID: id("2"), Kind: "lambda", Params: []Parameter{{ID: id("3"), Name: "p", TypeAnnotation: &typ}}, Body: &Expr{ID: id("5"), Kind: "let", Opdef: local, Body: &Expr{ID: id("8"), Kind: "name", Name: "p"}}}
	modules := []Module{{Declarations: []Declaration{{ID: id("1"), Kind: "def", Expr: lambda}}}}
	if got, err := FindExpression(modules, id("8")); err != nil || got != lambda.Body.Body {
		t.Fatalf("expression pointer %p, %v", got, err)
	}
	if got, err := FindType(modules, typeID); err != nil || got != &typ {
		t.Fatalf("type pointer %p, %v", got, err)
	}
	if got, err := FindDefinition(modules, id("1")); err != nil || got.Declaration != &modules[0].Declarations[0] {
		t.Fatalf("declaration pointer %v, %v", got, err)
	}
	if got, err := FindDefinition(modules, id("6")); err != nil || got.Operator != local {
		t.Fatalf("local definition pointer %v, %v", got, err)
	}
	if got, err := FindParameter(modules, id("3")); err != nil || got != &lambda.Params[0] {
		t.Fatalf("parameter pointer %p, %v", got, err)
	}
	var events []string
	v := IRVisitor{
		EnterParameter: func(p *Parameter) error { events = append(events, "enter:"+p.Name); return nil },
		EnterType:      func(typ *Type) error { events = append(events, "type:"+typ.Kind); return nil },
		ExitParameter:  func(p *Parameter) error { events = append(events, "exit:"+p.Name); return nil },
	}
	if err := v.WalkModule(&modules[0]); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(events, []string{"enter:p", "type:int", "exit:p"}) {
		t.Fatal(events)
	}
	stop := errors.New("parameter callback stopped")
	for _, enter := range []bool{true, false} {
		visitor := IRVisitor{DefinitionDepth: 17}
		callback := func(*Parameter) error { return stop }
		if enter {
			visitor.EnterParameter = callback
		} else {
			visitor.ExitParameter = callback
		}
		if err := visitor.WalkExpression(lambda); !errors.Is(err, stop) || visitor.DefinitionDepth != 17 {
			t.Fatalf("parameter callback: %v, depth %d", err, visitor.DefinitionDepth)
		}
	}
}

func TestIRVisitorMalformedTrees(t *testing.T) {
	t.Parallel()
	leaf := &Expr{Kind: "bool"}
	typeLeaf := &Type{Kind: "bool"}
	cases := []struct {
		name string
		walk func(*IRVisitor) error
	}{
		{"declaration", func(v *IRVisitor) error { return v.WalkDeclaration(&Declaration{Kind: "bad"}) }},
		{"definition_nil", func(v *IRVisitor) error { return v.WalkDefinition(IRDefinition{}) }},
		{"definition_both", func(v *IRVisitor) error {
			return v.WalkDefinition(IRDefinition{Declaration: &Declaration{}, Operator: &OperatorDef{}})
		}},
		{"definition_kind", func(v *IRVisitor) error {
			return v.WalkDefinition(IRDefinition{Declaration: &Declaration{Kind: "import"}})
		}},
		{"definition_expr", func(v *IRVisitor) error {
			return v.WalkDefinition(IRDefinition{Declaration: &Declaration{Kind: "def"}})
		}},
		{"assumption_expr", func(v *IRVisitor) error {
			return v.WalkDefinition(IRDefinition{Declaration: &Declaration{Kind: "assume"}})
		}},
		{"expression", func(v *IRVisitor) error { return v.WalkExpression(&Expr{Kind: "bad"}) }},
		{"lambda_body", func(v *IRVisitor) error { return v.WalkExpression(&Expr{Kind: "lambda"}) }},
		{"let_definition", func(v *IRVisitor) error { return v.WalkExpression(&Expr{Kind: "let", Body: leaf}) }},
		{"type", func(v *IRVisitor) error { return v.WalkType(&Type{Kind: "bad"}) }},
		{"element_type", func(v *IRVisitor) error { return v.WalkType(&Type{Kind: "set"}) }},
		{"function_result", func(v *IRVisitor) error { return v.WalkType(&Type{Kind: "fun", Arg: typeLeaf}) }},
		{"operator_result", func(v *IRVisitor) error { return v.WalkType(&Type{Kind: "oper"}) }},
		{"application_constructor", func(v *IRVisitor) error { return v.WalkType(&Type{Kind: "app"}) }},
		{"row_type", func(v *IRVisitor) error { return v.WalkType(&Type{Kind: "rec"}) }},
		{"row", func(v *IRVisitor) error { return v.WalkRow(&Row{Kind: "bad"}) }},
		{"row_tail", func(v *IRVisitor) error { return v.WalkRow(&Row{Kind: "row"}) }},
	}
	for _, item := range cases {
		t.Run(item.name, func(t *testing.T) {
			v := IRVisitor{DefinitionDepth: 9}
			if err := item.walk(&v); err == nil || v.DefinitionDepth != 9 {
				t.Fatalf("error %v, depth %d", err, v.DefinitionDepth)
			}
		})
	}
	modules := []Module{{Declarations: []Declaration{{Kind: "def", Expr: &Expr{Kind: "bad"}}}}}
	if got, err := FindExpression(modules, ID{}); err == nil || got != nil {
		t.Fatalf("expression: %v, %v", got, err)
	}
	if got, err := FindType(modules, ID{}); err == nil || got != nil {
		t.Fatalf("type: %v, %v", got, err)
	}
	if got, err := FindDefinition(modules, ID{}); err == nil || got.Valid() {
		t.Fatalf("definition: %v, %v", got, err)
	}
	if got, err := FindParameter(modules, ID{}); err == nil || got != nil {
		t.Fatalf("parameter: %v, %v", got, err)
	}
}

func TestIRVisitorNilAndZeroValues(t *testing.T) {
	t.Parallel()
	var visitor *IRVisitor
	for _, err := range []error{visitor.WalkModule(nil), visitor.WalkDeclaration(nil), visitor.WalkExpression(nil), visitor.WalkType(nil), visitor.WalkRow(nil), visitor.WalkModule(&Module{})} {
		if err != nil {
			t.Fatal(err)
		}
	}
	v := IRVisitor{}
	if err := v.WalkExpression(&Expr{Kind: "bool"}); err != nil {
		t.Fatal(err)
	}
	if got, err := FindExpression(nil, ID{}); got != nil || err != nil {
		t.Fatalf("empty expression: %v, %v", got, err)
	}
	if got, err := FindType(nil, ID{}); got != nil || err != nil {
		t.Fatalf("empty type: %v, %v", got, err)
	}
	if got, err := FindDefinition(nil, ID{}); got.Valid() || err != nil {
		t.Fatalf("empty definition: %v, %v", got, err)
	}
	if got, err := FindParameter(nil, ID{}); got != nil || err != nil {
		t.Fatalf("empty parameter: %v, %v", got, err)
	}
	d := IRDefinition{}
	if d.Valid() || d.Kind() != "" || d.ID() != (ID{}) {
		t.Fatal(fmt.Sprint(d))
	}
}
