package quint

import (
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"
)

type irTransformEvent struct {
	Event string `json:"event"`
	Kind  string `json:"kind"`
	ID    *ID    `json:"id,omitempty"`
}

type irTransformFixture struct {
	Name, Mode string
	Input      struct {
		Kind string
		Node json.RawMessage
	}
	Output json.RawMessage
	Error  string
	Events []irTransformEvent
}

func irTransformFixtures(t *testing.T) []irTransformFixture {
	t.Helper()
	data, err := os.ReadFile("testdata/parser/api_ir_transform.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixtures []irTransformFixture
	if err := json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	return fixtures
}

func irTransformTestID(text string) ID { value, _ := ParseID(text); return value }

func irTransformTestTransformer(mode string, visit func(irTransformEvent) error) *IRTransformer {
	fortyTwo, _ := ParseInteger("42")
	seven, _ := ParseInteger("7")
	added := func() Expr { return Expr{ID: irTransformTestID("8002"), Kind: "int", Int: seven} }
	event := func(name, kind string, id *ID) error { return visit(irTransformEvent{name, kind, id}) }
	module := func(phase string) func(Module) (Module, error) {
		return func(m Module) (Module, error) {
			if err := event(phase+"Module", "module", &m.ID); err != nil {
				return Module{}, err
			}
			if mode == phase+"_module_replace" {
				name := "Entered"
				if phase == "exit" {
					name = "Exited"
				}
				m = Module{ID: m.ID, Name: name, Declarations: []Declaration{{ID: irTransformTestID("8000"), Kind: "def", Name: "added", Qualifier: PureVal, Expr: &Expr{ID: irTransformTestID("8001"), Kind: "app", Opcode: "Tup", Args: []Expr{added()}}}}}
			}
			return m, nil
		}
	}
	declaration := func(phase string) func(Declaration) (Declaration, error) {
		return func(d Declaration) (Declaration, error) {
			if err := event(phase+"Decl", d.Kind, &d.ID); err != nil {
				return Declaration{}, err
			}
			if mode == phase+"_decl_rename" {
				if d.Kind == "import" || d.Kind == "export" || d.Kind == "instance" {
					d.ProtoName = "NewModuleName"
				} else {
					d.Name = "NewName"
				}
			}
			if mode == "enter_decl_kind_change" && phase == "enter" {
				d = Declaration{ID: d.ID, Kind: "def", Name: "converted", Qualifier: PureVal, Expr: &Expr{ID: irTransformTestID("8401"), Kind: "bool", Bool: true}}
			}
			return d, nil
		}
	}
	definition := func(phase string) func(IRDefinition) (IRDefinition, error) {
		return func(d IRDefinition) (IRDefinition, error) {
			id := d.ID()
			if err := event(phase+"Def", d.Kind(), &id); err != nil {
				return IRDefinition{}, err
			}
			if mode == phase+"_def_rename" {
				if d.Declaration != nil {
					d.Declaration.Name = "NewName"
				} else {
					d.Operator.Name = "NewName"
				}
			}
			if mode == "let_definition_invalid" && phase == "enter" {
				typeID := irTransformTestID("8402")
				d = IRDefinition{Declaration: &Declaration{ID: id, Kind: "const", Name: "notOperator", TypeAnnotation: &Type{ID: &typeID, Kind: "int"}}}
			}
			return d, nil
		}
	}
	expression := func(phase string) func(Expr) (Expr, error) {
		return func(e Expr) (Expr, error) {
			if err := event(phase+"Expr", e.Kind, &e.ID); err != nil {
				return Expr{}, err
			}
			if mode == phase+"_expr_42" {
				return Expr{ID: e.ID, Kind: "int", Int: fortyTwo}, nil
			}
			if mode == "enter_expr_kind_change" && phase == "enter" && e.Kind == "name" || mode == "exit_expr_kind_change" && phase == "exit" && e.Kind == "bool" {
				return Expr{ID: e.ID, Kind: "app", Opcode: "Tup", Args: []Expr{added()}}, nil
			}
			return e, nil
		}
	}
	typ := func(phase string) func(Type) (Type, error) {
		return func(typ Type) (Type, error) {
			if err := event(phase+"Type", typ.Kind, typ.ID); err != nil {
				return Type{}, err
			}
			if mode == "enter_type_bool" && phase == "enter" {
				return Type{ID: typ.ID, Kind: "bool"}, nil
			}
			if mode == "exit_type_var" && phase == "exit" {
				return Type{Kind: "var", Name: "trans"}, nil
			}
			if mode == "exit_type_rename_consts" && phase == "exit" && typ.Kind == "const" {
				typ.Name = "Renamed"
			}
			if mode == "enter_type_kind_change" && phase == "enter" && typ.Kind == "var" {
				id := irTransformTestID("8400")
				return Type{ID: typ.ID, Kind: "set", Elem: &Type{ID: &id, Kind: "int"}}, nil
			}
			return typ, nil
		}
	}
	row := func(phase string) func(Row) (Row, error) {
		return func(row Row) (Row, error) {
			if err := event(phase+"Row", row.Kind, nil); err != nil {
				return Row{}, err
			}
			if mode == "enter_row_new_fields" && phase == "enter" && row.Kind == "row" {
				row.Fields = []RowField{{Name: "newField", Type: Type{Kind: "int"}}}
			}
			if mode == "exit_row_var" && phase == "exit" {
				return Row{Kind: "var", Name: "exited"}, nil
			}
			if mode == "sum_row_invalid" && phase == "enter" && row.Kind == "row" {
				return Row{Kind: "var", Name: "notRow"}, nil
			}
			return row, nil
		}
	}
	return &IRTransformer{EnterModule: module("enter"), ExitModule: module("exit"), EnterDeclaration: declaration("enter"), ExitDeclaration: declaration("exit"), EnterDefinition: definition("enter"), ExitDefinition: definition("exit"), EnterExpression: expression("enter"), ExitExpression: expression("exit"), EnterType: typ("enter"), ExitType: typ("exit"), EnterRow: row("enter"), ExitRow: row("exit")}
}

func irTransformTestRun(fixture irTransformFixture, transformer *IRTransformer) (any, error) {
	switch fixture.Input.Kind {
	case "module":
		var node Module
		if err := json.Unmarshal(fixture.Input.Node, &node); err != nil {
			return nil, err
		}
		return transformer.TransformModule(node)
	case "declaration", "definition":
		var node Declaration
		if err := json.Unmarshal(fixture.Input.Node, &node); err != nil {
			return nil, err
		}
		if fixture.Input.Kind == "declaration" {
			return transformer.TransformDeclaration(node)
		}
		definition, err := transformer.TransformDefinition(IRDefinition{Declaration: &node})
		if definition.Declaration != nil {
			return definition.Declaration, err
		}
		return definition.Operator, err
	case "expression":
		var node Expr
		if err := json.Unmarshal(fixture.Input.Node, &node); err != nil {
			return nil, err
		}
		return transformer.TransformExpression(node)
	case "type":
		var node Type
		if err := json.Unmarshal(fixture.Input.Node, &node); err != nil {
			return nil, err
		}
		return transformer.TransformType(node)
	case "row":
		var node Row
		if err := json.Unmarshal(fixture.Input.Node, &node); err != nil {
			return nil, err
		}
		return transformer.TransformRow(node)
	}
	panic("unexpected fixture input")
}

func TestUpstreamIRTransformation(t *testing.T) {
	t.Parallel()
	for _, fixture := range irTransformFixtures(t) {
		t.Run(fixture.Name, func(t *testing.T) {
			t.Parallel()
			events := []irTransformEvent{}
			transformer := irTransformTestTransformer(fixture.Mode, func(e irTransformEvent) error { events = append(events, e); return nil })
			output, err := irTransformTestRun(fixture, transformer)
			if fixture.Error != "" {
				if err == nil || !strings.Contains(strings.ToLower(fixture.Error), err.Error()) {
					t.Fatalf("Go error %v, TS %s", err, fixture.Error)
				}
			} else {
				if err != nil {
					t.Fatal(err)
				}
				data, err := json.Marshal(output)
				if err != nil {
					t.Fatal(err)
				}
				var actual, expected any
				if err := json.Unmarshal(data, &actual); err != nil {
					t.Fatal(err)
				}
				if err := json.Unmarshal(fixture.Output, &expected); err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(actual, expected) {
					t.Fatalf("Go %s\nTS %s", data, fixture.Output)
				}
			}
			if !reflect.DeepEqual(events, fixture.Events) {
				got, _ := json.Marshal(events)
				want, _ := json.Marshal(fixture.Events)
				t.Fatalf("Go events %s\nTS events %s", got, want)
			}
		})
	}
}

func TestIRTransformerCallbackErrors(t *testing.T) {
	t.Parallel()
	fixture := irTransformFixtures(t)[0]
	var input Module
	if err := json.Unmarshal(fixture.Input.Node, &input); err != nil {
		t.Fatal(err)
	}
	stop := errors.New("transformation stopped")
	for index := range fixture.Events {
		count := 0
		transformer := irTransformTestTransformer("identity", func(irTransformEvent) error {
			count++
			if count == index+1 {
				return stop
			}
			return nil
		})
		if _, err := transformer.TransformModule(input); !errors.Is(err, stop) || count != index+1 {
			t.Fatalf("event %d: %d callbacks, error %v", index, count, err)
		}
	}
}

func irTransformMutablePointers(value reflect.Value, pointers map[uintptr]bool) {
	if !value.IsValid() {
		return
	}
	switch value.Kind() {
	case reflect.Pointer:
		if !value.IsNil() {
			pointers[value.Pointer()] = true
			irTransformMutablePointers(value.Elem(), pointers)
		}
	case reflect.Slice:
		if value.Len() > 0 {
			pointers[value.Pointer()] = true
		}
		for i := 0; i < value.Len(); i++ {
			irTransformMutablePointers(value.Index(i), pointers)
		}
	case reflect.Struct:
		for i := 0; i < value.NumField(); i++ {
			if value.Type().Field(i).PkgPath == "" {
				irTransformMutablePointers(value.Field(i), pointers)
			}
		}
	}
}

func TestIRTransformerFreshOwnership(t *testing.T) {
	t.Parallel()
	input := irAccessFixtures(t)[0].Modules[0]
	doc, depth, hidden, alias := "documentation", 7, true, "alias"
	input.Doc = &doc
	for i := range input.Declarations {
		d := &input.Declarations[i]
		d.Depth, d.Hidden, d.Doc = &depth, &hidden, &doc
		d.DefName, d.QualifiedName, d.FromSource = &alias, &alias, &alias
		d.Namespaces, d.Shadowing, d.ImportedFrom = []string{"ns"}, json.RawMessage(`true`), json.RawMessage(`{"source":"main"}`)
	}
	lambda := input.Declarations[8].Expr
	lambda.Params[0].Depth, lambda.Params[0].Shadowing = &depth, json.RawMessage(`false`)
	local := lambda.Body.Opdef
	local.Depth, local.Hidden, local.Doc, local.Namespaces = &depth, &hidden, &doc, []string{"local"}
	local.Shadowing, local.ImportedFrom = json.RawMessage(`true`), json.RawMessage(`{"source":"local"}`)
	before, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	var captured Module
	transformer := IRTransformer{EnterModule: func(m Module) (Module, error) { captured = m; return m, nil }}
	output, err := transformer.TransformModule(input)
	if err != nil {
		t.Fatal(err)
	}
	inputPointers, outputPointers, capturedPointers := map[uintptr]bool{}, map[uintptr]bool{}, map[uintptr]bool{}
	irTransformMutablePointers(reflect.ValueOf(input), inputPointers)
	irTransformMutablePointers(reflect.ValueOf(output), outputPointers)
	irTransformMutablePointers(reflect.ValueOf(captured), capturedPointers)
	for pointer := range outputPointers {
		if inputPointers[pointer] || capturedPointers[pointer] {
			t.Fatalf("output shares mutable storage at %x", pointer)
		}
	}
	*output.Doc = "changed"
	output.Declarations[8].Expr.Params[0].TypeAnnotation.Kind = "bool"
	output.Declarations[8].Expr.Body.Opdef.Namespaces[0] = "changed"
	after, err := json.Marshal(input)
	if err != nil || string(before) != string(after) {
		t.Fatalf("changed input: %v", err)
	}
	var nilTransformer *IRTransformer
	copy, err := nilTransformer.TransformModule(input)
	if err != nil {
		t.Fatal(err)
	}
	copyPointers := map[uintptr]bool{}
	irTransformMutablePointers(reflect.ValueOf(copy), copyPointers)
	for pointer := range copyPointers {
		if inputPointers[pointer] {
			t.Fatalf("identity copy shares mutable storage at %x", pointer)
		}
	}
	external := Expr{Kind: "app", Opcode: "Tup", Args: []Expr{{Kind: "name", Name: "external"}}}
	replacer := IRTransformer{EnterExpression: func(e Expr) (Expr, error) {
		if e.Kind == "bool" {
			return external, nil
		}
		return e, nil
	}}
	result, err := replacer.TransformExpression(Expr{Kind: "bool"})
	if err != nil {
		t.Fatal(err)
	}
	result.Args[0].Name = "changed"
	if external.Args[0].Name != "external" {
		t.Fatal("replacement graph mutated")
	}
}

func TestIRTransformerDefinitionRepresentations(t *testing.T) {
	t.Parallel()
	operator := OperatorDef{Kind: "def", Expr: &Expr{Kind: "bool"}}
	declaration := Declaration{Kind: "def", Expr: &Expr{Kind: "bool"}}
	fromOperator := IRTransformer{ExitDefinition: func(d IRDefinition) (IRDefinition, error) {
		copy := irTransformDeclaration(*d.Operator)
		return IRDefinition{Declaration: &copy}, nil
	}}
	if _, err := fromOperator.TransformExpression(Expr{Kind: "let", Opdef: &operator, Body: &Expr{Kind: "bool"}}); err != nil {
		t.Fatal(err)
	}
	toOperator := IRTransformer{ExitDefinition: func(d IRDefinition) (IRDefinition, error) {
		copy := irTransformOperator(*d.Declaration)
		return IRDefinition{Operator: &copy}, nil
	}}
	if _, err := toOperator.TransformDeclaration(declaration); err != nil {
		t.Fatal(err)
	}
	invalid := IRTransformer{ExitDefinition: func(IRDefinition) (IRDefinition, error) { return IRDefinition{}, nil }}
	if _, err := invalid.TransformDefinition(IRDefinition{Declaration: &declaration}); err == nil {
		t.Fatal("accepted missing exit replacement")
	}
	// OperatorDef is the def representation, even if handwritten input has a
	// different Kind. Source union variants are represented by Declaration.
	operator.Kind = "other"
	definition, err := (&IRTransformer{}).TransformDefinition(IRDefinition{Operator: &operator})
	if err != nil || definition.Operator.Kind != "def" {
		t.Fatal("local definition representation was not normalized", err)
	}
}

func TestIRTransformerExitReplacementIsNotValidatedOrTraversed(t *testing.T) {
	t.Parallel()
	childCalls := 0
	transformer := IRTransformer{ExitExpression: func(expression Expr) (Expr, error) {
		childCalls++
		return Expr{Kind: "app", Opcode: "unfinished", Args: []Expr{{Kind: "unknown"}}}, nil
	}}
	output, err := transformer.TransformExpression(Expr{Kind: "bool"})
	if err != nil || childCalls != 1 || output.Args[0].Kind != "unknown" {
		t.Fatal("exit replacement was traversed", output, childCalls, err)
	}
}

func TestIRTransformerMalformedTrees(t *testing.T) {
	t.Parallel()
	badType := &Type{Kind: "bad"}
	badExpression := &Expr{Kind: "bad"}
	goodExpression := &Expr{Kind: "bool"}
	cases := []struct {
		name string
		run  func(*IRTransformer) error
	}{
		{"declaration", func(x *IRTransformer) error { _, e := x.TransformDeclaration(Declaration{Kind: "bad"}); return e }},
		{"definition", func(x *IRTransformer) error { _, e := x.TransformDefinition(IRDefinition{}); return e }},
		{"definition_kind", func(x *IRTransformer) error {
			_, e := x.TransformDefinition(IRDefinition{Declaration: &Declaration{Kind: "import"}})
			return e
		}},
		{"definition_body", func(x *IRTransformer) error {
			_, e := x.TransformDefinition(IRDefinition{Declaration: &Declaration{Kind: "def"}})
			return e
		}},
		{"definition_annotation", func(x *IRTransformer) error {
			_, e := x.TransformDefinition(IRDefinition{Declaration: &Declaration{Kind: "const", TypeAnnotation: badType}})
			return e
		}},
		{"alias_type", func(x *IRTransformer) error {
			_, e := x.TransformDefinition(IRDefinition{Declaration: &Declaration{Kind: "typedef", Type: badType}})
			return e
		}},
		{"expression", func(x *IRTransformer) error { _, e := x.TransformExpression(*badExpression); return e }},
		{"lambda_body", func(x *IRTransformer) error { _, e := x.TransformExpression(Expr{Kind: "lambda"}); return e }},
		{"lambda_annotation", func(x *IRTransformer) error {
			_, e := x.TransformExpression(Expr{Kind: "lambda", Params: []Parameter{{TypeAnnotation: badType}}, Body: goodExpression})
			return e
		}},
		{"lambda_nested", func(x *IRTransformer) error {
			_, e := x.TransformExpression(Expr{Kind: "lambda", Body: badExpression})
			return e
		}},
		{"type", func(x *IRTransformer) error { _, e := x.TransformType(*badType); return e }},
		{"set_element", func(x *IRTransformer) error { _, e := x.TransformType(Type{Kind: "set"}); return e }},
		{"function_child", func(x *IRTransformer) error { _, e := x.TransformType(Type{Kind: "fun"}); return e }},
		{"operator_result", func(x *IRTransformer) error { _, e := x.TransformType(Type{Kind: "oper"}); return e }},
		{"row_type", func(x *IRTransformer) error { _, e := x.TransformType(Type{Kind: "rec"}); return e }},
		{"row", func(x *IRTransformer) error { _, e := x.TransformRow(Row{Kind: "bad"}); return e }},
		{"row_tail", func(x *IRTransformer) error { _, e := x.TransformRow(Row{Kind: "row"}); return e }},
		{"row_nested", func(x *IRTransformer) error {
			_, e := x.TransformRow(Row{Kind: "row", Other: &Row{Kind: "bad"}})
			return e
		}},
	}
	for _, item := range cases {
		t.Run(item.name, func(t *testing.T) {
			if err := item.run(&IRTransformer{}); err == nil {
				t.Fatal("accepted malformed tree")
			}
		})
	}
}
