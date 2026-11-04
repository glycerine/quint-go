package quint

import (
	"bytes"
	"encoding/json"
	"reflect"
	"sort"
	"strconv"
	"testing"
)

// Fuzz inputs select bounded, valid structural trees, not arbitrary replacement
// programs. Acyclic chains and disjoint ground domains make the algebraic laws
// independent of a second implementation of either substitution algorithm.
func FuzzTypeSubstitutionLaws(f *testing.F) {
	for i := byte(0); i < 11; i++ {
		f.Add(bytes.Repeat([]byte{i}, 24))
	}
	f.Add([]byte{})
	f.Add([]byte{9, 3, 1, 8, 7, 0, 5, 2, 4, 6})
	f.Fuzz(func(t *testing.T, input []byte) {
		if len(input) > 128 {
			t.Skip()
		}
		substitutionPropertyTypeLaws(t, input)
	})
}

func FuzzEffectSubstitutionLaws(f *testing.F) {
	for i := byte(0); i < 9; i++ {
		f.Add(bytes.Repeat([]byte{i}, 24))
	}
	f.Add([]byte{})
	f.Add([]byte{2, 7, 1, 8, 6, 3, 0, 5, 4})
	f.Fuzz(func(t *testing.T, input []byte) {
		if len(input) > 128 {
			t.Skip()
		}
		substitutionPropertyEffectLaws(t, input)
	})
}

// These fixed mixed shapes exercise nested rows/arrows during ordinary tests;
// native fuzzing is optional and does not add wall-clock work to the fast suite.
func TestSubstitutionBoundedStructuralLaws(t *testing.T) {
	for i := byte(0); i < 12; i++ {
		seed := make([]byte, 32)
		for j := range seed {
			seed[j] = i*19 + byte(j)*7
		}
		substitutionPropertyTypeLaws(t, seed)
		substitutionPropertyEffectLaws(t, seed)
	}
}

type substitutionPropertyCursor struct {
	data  []byte
	index int
}

func (c *substitutionPropertyCursor) next() byte {
	if c.index >= len(c.data) {
		return 0
	}
	value := c.data[c.index]
	c.index++
	return value
}

func (c *substitutionPropertyCursor) typ(depth int) Type {
	kind := c.next()
	if depth == 0 {
		kind %= 6
	} else {
		kind %= 13
	}
	var result Type
	switch kind {
	case 0, 1, 2:
		result = Type{Kind: "var", Name: []string{"a", "b", "freeType"}[kind]}
	case 3, 4, 5:
		result = Type{Kind: []string{"int", "bool", "str"}[kind-3]}
	case 6, 7:
		element := c.typ(depth - 1)
		result = Type{Kind: []string{"set", "list"}[kind-6], Elem: &element}
	case 8:
		argument, value := c.typ(depth-1), c.typ(depth-1)
		result = Type{Kind: "fun", Arg: &argument, Res: &value}
	case 9:
		value := c.typ(depth - 1)
		result = Type{Kind: "oper", Args: []Type{c.typ(depth - 1), c.typ(depth - 1)}, Res: &value}
	case 10:
		constructor := Type{Kind: "const", Name: "Box"}
		result = Type{Kind: "app", Ctor: &constructor, Args: []Type{c.typ(depth - 1)}}
	case 11, 12:
		row := c.row(depth - 1)
		result = Type{Kind: []string{"rec", "tup"}[kind-11], Fields: &row}
	}
	if c.next()%3 == 1 {
		id, _ := ParseID(strconv.Itoa(int(c.next()) + 1))
		result.ID = &id
	}
	return result
}

func (c *substitutionPropertyCursor) row(depth int) Row {
	tail := Row{Kind: "empty"}
	if selected := c.next() % 4; selected < 3 {
		tail = Row{Kind: "var", Name: []string{"r", "s", "freeRow"}[selected]}
	}
	fields := make([]RowField, int(c.next()%3))
	for i := range fields {
		fields[i] = RowField{Name: "field" + strconv.Itoa(i), Type: c.typ(depth)}
	}
	return Row{Kind: "row", Fields: fields, Other: &tail}
}

func substitutionPropertyTypeLaws(t *testing.T, seed []byte) {
	t.Helper()
	cursor := substitutionPropertyCursor{data: seed}
	input := cursor.typ(3)
	ground := Type{Kind: []string{"int", "bool", "str"}[cursor.next()%3]}
	variableB := Type{Kind: "var", Name: "b"}
	empty := Row{Kind: "empty"}
	rowS := Row{Kind: "var", Name: "s"}
	bindings := []Substitution{
		{Kind: "type", Name: "a", Type: Type{Kind: "list", Elem: &variableB}},
		{Kind: "type", Name: "b", Type: ground},
		{Kind: "row", Name: "r", Row: Row{Kind: "row", Fields: []RowField{{Name: "fromR", Type: Type{Kind: "var", Name: "a"}}}, Other: &rowS}},
		{Kind: "row", Name: "s", Row: Row{Kind: "row", Fields: []RowField{{Name: "fromS", Type: ground}}, Other: &empty}},
	}
	beforeInput := substitutionPropertyWire(t, input)
	beforeBindings := substitutionPropertyTypeBindings(t, bindings)
	result, err := ApplySubstitution(bindings, input)
	if err != nil {
		t.Fatal("acyclic valid type substitution failed", err)
	}
	if err := result.Validate(); err != nil {
		t.Fatal("substitution broke recursive type validation", err)
	}
	again, err := ApplySubstitution(bindings, result)
	if err != nil || !reflect.DeepEqual(result, again) {
		t.Fatal("fully resolved type substitution is not idempotent", err)
	}
	types, rows, err := TypeNames(&result)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range append(types, rows...) {
		if name == "a" || name == "b" || name == "r" || name == "s" {
			t.Fatal("acyclic substitution left a bound variable", name)
		}
	}
	identity, err := ApplySubstitution(nil, input)
	if err != nil || !reflect.DeepEqual(identity, input) {
		t.Fatal("empty substitution is not an identity", err)
	}
	// Ground replacement domains are disjoint, so order cannot affect meaning.
	left := []Substitution{bindings[1], {Kind: "row", Name: "r", Row: Row{Kind: "row", Fields: []RowField{{Name: "left", Type: ground}}, Other: &empty}}}
	right := []Substitution{{Kind: "type", Name: "a", Type: Type{Kind: "set", Elem: &ground}}, {Kind: "row", Name: "s", Row: empty}}
	first, err := ApplySubstitution(left, input)
	if err != nil {
		t.Fatal(err)
	}
	first, err = ApplySubstitution(right, first)
	if err != nil {
		t.Fatal(err)
	}
	second, err := ApplySubstitution(right, input)
	if err != nil {
		t.Fatal(err)
	}
	second, err = ApplySubstitution(left, second)
	if err != nil || !reflect.DeepEqual(first, second) {
		t.Fatal("disjoint ground type substitutions do not commute", err)
	}
	if !bytes.Equal(beforeInput, substitutionPropertyWire(t, input)) || !bytes.Equal(beforeBindings, substitutionPropertyTypeBindings(t, bindings)) {
		t.Fatal("type substitution mutated an input or replacement")
	}
	// Check mutable structural children separately from metadata. This property
	// does not impose an ownership guarantee on returned ID pointers.
	substitutionPropertyMutateType(&result)
	if !bytes.Equal(beforeInput, substitutionPropertyWire(t, input)) || !bytes.Equal(beforeBindings, substitutionPropertyTypeBindings(t, bindings)) {
		t.Fatal("type result aliases structural input/replacement storage")
	}
}

func (c *substitutionPropertyCursor) entity(depth int) Entity {
	selected := c.next()
	if depth == 0 {
		selected %= 4
	} else {
		selected %= 5
	}
	if selected < 3 {
		id, _ := ParseID(strconv.Itoa(int(c.next()) + 1))
		return Entity{Kind: "variable", Name: []string{"v0", "v1", "freeEntity"}[selected], Reference: &id}
	}
	if selected == 4 {
		return Entity{Kind: "union", Entities: []Entity{c.entity(depth - 1), c.entity(depth - 1)}}
	}
	variables := make([]StateVariable, int(c.next()%3))
	for i := range variables {
		id, _ := ParseID(strconv.Itoa(int(c.next()) + 1))
		variables[i] = NewStateVariable([]string{"x", "y"}[c.next()%2], id)
	}
	return Entity{Kind: "concrete", StateVariables: variables}
}

func (c *substitutionPropertyCursor) effect(depth int) Effect {
	selected := c.next() % 5
	if selected < 3 {
		return Effect{Kind: "variable", Name: []string{"e0", "e1", "freeEffect"}[selected]}
	}
	if selected == 4 && depth > 0 {
		result := c.effect(depth - 1)
		return Effect{Kind: "arrow", Params: []Effect{c.effect(depth - 1), c.effect(depth - 1)}, Result: &result}
	}
	components := make([]EffectComponent, int(c.next()%4))
	for i := range components {
		components[i] = EffectComponent{Kind: []string{"read", "update", "temporal"}[c.next()%3], Entity: c.entity(2)}
	}
	return Effect{Kind: "concrete", Components: components}
}

func substitutionPropertyEffectLaws(t *testing.T, seed []byte) {
	t.Helper()
	cursor := substitutionPropertyCursor{data: seed}
	input := cursor.effect(2)
	id, _ := ParseID("17")
	ground := Entity{Kind: "concrete", StateVariables: []StateVariable{NewStateVariable("x", id)}}
	read := Effect{Kind: "concrete", Components: []EffectComponent{{Kind: "read", Entity: ground}}}
	update := Effect{Kind: "concrete", Components: []EffectComponent{{Kind: "update", Entity: Entity{Kind: "variable", Name: "v0"}}}}
	variableE1 := Effect{Kind: "variable", Name: "e1"}
	bindings := []EffectSubstitution{
		{Kind: "effect", Name: "e0", Effect: Effect{Kind: "arrow", Params: []Effect{variableE1}, Result: &update}},
		{Kind: "effect", Name: "e1", Effect: read},
		{Kind: "entity", Name: "v0", Entity: Entity{Kind: "variable", Name: "v1"}},
		{Kind: "entity", Name: "v1", Entity: ground},
	}
	beforeInput := substitutionPropertyWire(t, input)
	beforeBindings := substitutionPropertyEffectBindings(t, bindings)
	result, err := ApplyEffectSubstitution(bindings, input)
	if err != nil {
		t.Fatal("acyclic valid effect substitution failed", err)
	}
	again, err := ApplyEffectSubstitution(bindings, result)
	if err != nil || !bytes.Equal(substitutionPropertyWire(t, result), substitutionPropertyWire(t, again)) {
		t.Fatal("effect substitution did not reach a fixed point", err)
	}
	effects, entities := EffectNames(result)
	for _, name := range append(effects, entities...) {
		if name == "e0" || name == "e1" || name == "v0" || name == "v1" {
			t.Fatal("effect substitution left an acyclic bound name", name)
		}
	}
	single := ApplyEntitySubstitution(bindings, Entity{Kind: "variable", Name: "v0"})
	if single.Kind != "variable" || single.Name != "v1" {
		t.Fatal("entity substitution unexpectedly took transitive closure", single)
	}
	left := []EffectSubstitution{{Kind: "effect", Name: "e0", Effect: read}, {Kind: "entity", Name: "v0", Entity: ground}}
	right := []EffectSubstitution{{Kind: "effect", Name: "e1", Effect: Effect{Kind: "concrete"}}, {Kind: "entity", Name: "v1", Entity: Entity{Kind: "concrete"}}}
	first, err := ApplyEffectSubstitution(left, input)
	if err != nil {
		t.Fatal(err)
	}
	first, err = ApplyEffectSubstitution(right, first)
	if err != nil {
		t.Fatal(err)
	}
	second, err := ApplyEffectSubstitution(right, input)
	if err != nil {
		t.Fatal(err)
	}
	second, err = ApplyEffectSubstitution(left, second)
	if err != nil || !bytes.Equal(substitutionPropertyWire(t, first), substitutionPropertyWire(t, second)) {
		t.Fatal("disjoint ground effect substitutions do not commute", err)
	}
	composition, err := ComposeEffectSubstitutions(left, right)
	if err != nil {
		t.Fatal(err)
	}
	combined, err := ApplyEffectSubstitution(composition, input)
	if err != nil || !bytes.Equal(substitutionPropertyWire(t, combined), substitutionPropertyWire(t, second)) {
		t.Fatal("ground effect composition disagrees with sequential application", err)
	}
	if !bytes.Equal(beforeInput, substitutionPropertyWire(t, input)) || !bytes.Equal(beforeBindings, substitutionPropertyEffectBindings(t, bindings)) {
		t.Fatal("effect substitution mutated an input or replacement")
	}
	substitutionPropertyMutateEffect(&result)
	if !bytes.Equal(beforeInput, substitutionPropertyWire(t, input)) || !bytes.Equal(beforeBindings, substitutionPropertyEffectBindings(t, bindings)) {
		t.Fatal("effect result aliases structural input/replacement storage")
	}
}

// substitutionPropertyWire compares effects by value. Upstream treats a concrete
// entity's state variables as a set (effects/base.ts compares them by name), so
// their order is canonicalized here; every other list keeps its order.
func substitutionPropertyWire(t *testing.T, value any) []byte {
	t.Helper()
	wire, err := json.Marshal(value)
	if err != nil {
		t.Fatal("invalid property-generated structure", err)
	}
	var generic any
	if err = json.Unmarshal(wire, &generic); err != nil {
		t.Fatal("invalid property-generated wire", err)
	}
	canonicalizeStateVariables(generic)
	canonical, err := json.Marshal(generic)
	if err != nil {
		t.Fatal("invalid canonical wire", err)
	}
	return canonical
}

func canonicalizeStateVariables(value any) {
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			if list, ok := child.([]any); ok && key == "stateVariables" {
				typed[key] = substitutionPropertySet(list)
			}
			canonicalizeStateVariables(child)
		}
	case []any:
		for _, child := range typed {
			canonicalizeStateVariables(child)
		}
	}
}

// substitutionPropertySet sorts a state-variable list and drops exact duplicates.
// Upstream compares these lists by name membership, so repetition is not a value.
func substitutionPropertySet(list []any) []any {
	keys := make([]string, len(list))
	for i, item := range list {
		encoded, _ := json.Marshal(item)
		keys[i] = string(encoded)
	}
	sort.Sort(substitutionPropertySorted{values: list, keys: keys})
	out := make([]any, 0, len(list))
	for i := range list {
		if i == 0 || keys[i] != keys[i-1] {
			out = append(out, list[i])
		}
	}
	return out
}

type substitutionPropertySorted struct {
	values []any
	keys   []string
}

func (s substitutionPropertySorted) Len() int           { return len(s.values) }
func (s substitutionPropertySorted) Less(i, j int) bool { return s.keys[i] < s.keys[j] }
func (s substitutionPropertySorted) Swap(i, j int) {
	s.values[i], s.values[j] = s.values[j], s.values[i]
	s.keys[i], s.keys[j] = s.keys[j], s.keys[i]
}

type substitutionPropertyBinding struct {
	Kind, Name string
	Value      any
}

func substitutionPropertyTypeBindings(t *testing.T, bindings []Substitution) []byte {
	t.Helper()
	values := make([]substitutionPropertyBinding, len(bindings))
	for i, binding := range bindings {
		values[i] = substitutionPropertyBinding{Kind: binding.Kind, Name: binding.Name, Value: binding.Type}
		if binding.Kind == "row" {
			values[i].Value = binding.Row
		}
	}
	return substitutionPropertyWire(t, values)
}

func substitutionPropertyEffectBindings(t *testing.T, bindings []EffectSubstitution) []byte {
	t.Helper()
	values := make([]substitutionPropertyBinding, len(bindings))
	for i, binding := range bindings {
		values[i] = substitutionPropertyBinding{Kind: binding.Kind, Name: binding.Name, Value: binding.Effect}
		if binding.Kind == "entity" {
			values[i].Value = binding.Entity
		}
	}
	return substitutionPropertyWire(t, values)
}

func substitutionPropertyMutateType(value *Type) {
	if value == nil {
		return
	}
	value.Name = "changed"
	for _, child := range []*Type{value.Elem, value.Arg, value.Res, value.Ctor} {
		substitutionPropertyMutateType(child)
	}
	for i := range value.Args {
		substitutionPropertyMutateType(&value.Args[i])
	}
	for row := value.Fields; row != nil; row = row.Other {
		row.Name = "changed"
		for i := range row.Fields {
			row.Fields[i].Name = "changed"
			substitutionPropertyMutateType(&row.Fields[i].Type)
		}
	}
}

func substitutionPropertyMutateEffect(value *Effect) {
	value.Name = "changed"
	for i := range value.Params {
		substitutionPropertyMutateEffect(&value.Params[i])
	}
	if value.Result != nil {
		substitutionPropertyMutateEffect(value.Result)
	}
	for i := range value.Components {
		value.Components[i].Kind = "changed"
		substitutionPropertyMutateEntity(&value.Components[i].Entity)
	}
}

func substitutionPropertyMutateEntity(value *Entity) {
	value.Name = "changed"
	for i := range value.StateVariables {
		value.StateVariables[i].Name = "changed"
	}
	for i := range value.Entities {
		substitutionPropertyMutateEntity(&value.Entities[i])
	}
}
