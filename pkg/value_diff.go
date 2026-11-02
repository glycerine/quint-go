package quint

import (
	"fmt"
	"sort"

	"github.com/glycerine/insdict"
)

// ValueDiffRole identifies the meaning of a token in a value comparison.
type ValueDiffRole string

const (
	ValueDiffUnchanged ValueDiffRole = "unchanged"
	ValueDiffRemoved   ValueDiffRole = "removed"
	ValueDiffAdded     ValueDiffRole = "added"
)

// ValueDiffOptions controls comparison rendering. Large unchanged subvalues
// collapse only when nested and larger than a positive CollapseThreshold.
// Decorate may apply terminal colors to a token without changing its visible
// width; nil produces the reference's plain output. FirstColumn is the width
// already occupied before the first token.
type ValueDiffOptions struct {
	CollapseThreshold int
	FirstColumn       int
	Decorate          func(ValueDiffRole, string) string
}

// FormatRuntimeValueDiff compares concrete values using Quint's recursive diff
// layout. Maps and sets retain runtime traversal order, records sort field names,
// and unchanged values use the public Go structural-equality boundary. The
// caller chooses output width and styling; the function performs no terminal IO.
func FormatRuntimeValueDiff(old, new RuntimeValue, width int, options ...ValueDiffOptions) (string, error) {
	var option ValueDiffOptions
	if len(options) > 0 {
		option = options[0]
	}
	if width < 0 || option.FirstColumn < 0 {
		return "", fmt.Errorf("value diff requires nonnegative width and first column")
	}
	if !old.Valid() || !new.Valid() {
		return "", fmt.Errorf("value diff requires valid runtime values")
	}
	differ := valueDiffer{option}
	doc, err := differ.diff(old, new, false)
	if err != nil {
		return "", err
	}
	return formatValueDoc(doc, width, option.FirstColumn), nil
}

type valueDiffer struct{ options ValueDiffOptions }

func (d valueDiffer) token(role ValueDiffRole, text string) valueDoc {
	doc := vdText(text)
	if d.options.Decorate != nil {
		doc.text = d.options.Decorate(role, text)
	}
	return doc
}
func (d valueDiffer) nary(role ValueDiffRole, left string, args []valueDoc, right string, padding valueDoc) valueDoc {
	return vdGroup(d.token(role, left), vdGroup(vdNest("  ", padding, vdJoin(vdConcat(d.token(role, ","), vdLine()), args)), padding), d.token(role, right))
}
func valueDiffOp(v RuntimeValue, ops ...string) bool {
	if v.expression.Kind != "app" {
		return false
	}
	for _, op := range ops {
		if v.expression.Opcode == op {
			return true
		}
	}
	return false
}
func valueDiffPrimitive(v RuntimeValue) bool {
	return v.closure == nil && (v.expression.Kind == "bool" || v.expression.Kind == "int" || v.expression.Kind == "str")
}
func (d valueDiffer) collapse(v RuntimeValue) bool {
	limit := d.options.CollapseThreshold
	if limit <= 0 {
		return false
	}
	switch {
	case valueDiffOp(v, "Map", "Set"):
		return v.payload != nil && v.payload.entries.Len() > limit
	case valueDiffOp(v, "Rec"):
		return v.payload != nil && len(v.payload.children)/2 > limit
	case valueDiffOp(v, "List", "Tup"):
		return v.payload != nil && len(v.payload.children) > limit
	}
	return false
}
func (d valueDiffer) diff(a, b RuntimeValue, nested bool) (valueDoc, error) {
	if a.Equal(b) {
		if nested && d.collapse(a) {
			return d.token(ValueDiffUnchanged, "..."), nil
		}
		return d.pretty(a, ValueDiffUnchanged)
	}
	if valueDiffPrimitive(a) && valueDiffPrimitive(b) {
		return d.arrow(a, b)
	}
	if valueDiffOp(a, "Rec") && valueDiffOp(b, "Rec") {
		return d.records(a, b)
	}
	if valueDiffOp(a, "Map") && valueDiffOp(b, "Map") {
		return d.maps(a, b)
	}
	if valueDiffOp(a, "List", "Tup") && valueDiffOp(b, "List", "Tup") {
		left, err := runtimeChildren(a)
		if err != nil {
			return valueDoc{}, err
		}
		right, err := runtimeChildren(b)
		if err != nil {
			return valueDoc{}, err
		}
		if len(left) != len(right) {
			return d.arrow(a, b)
		}
		items := make([]valueDoc, len(left))
		for i := range left {
			items[i], err = d.diff(left[i], right[i], true)
			if err != nil {
				return valueDoc{}, err
			}
		}
		return d.nary(ValueDiffUnchanged, "[", items, "]", vdBreak()), nil
	}
	if valueDiffOp(a, "variant") && valueDiffOp(b, "variant") {
		left, err := runtimeChildren(a)
		if err != nil {
			return valueDoc{}, err
		}
		right, err := runtimeChildren(b)
		if err != nil {
			return valueDoc{}, err
		}
		if left[0].expression.Str != right[0].expression.Str {
			render := func(children []RuntimeValue, role ValueDiffRole) (valueDoc, error) {
				label := d.token(role, children[0].expression.Str)
				if valueDiffEmptyPayload(children[1]) {
					return label, nil
				}
				payload, err := d.pretty(children[1], role)
				return vdGroup(label, vdText("("), payload, vdText(")")), err
			}
			old, err := render(left, ValueDiffRemoved)
			if err != nil {
				return valueDoc{}, err
			}
			new, err := render(right, ValueDiffAdded)
			return vdGroup(old, vdText(" => "), new), err
		}
		label := d.token(ValueDiffUnchanged, left[0].expression.Str)
		if valueDiffEmptyPayload(left[1]) && valueDiffEmptyPayload(right[1]) {
			return label, nil
		}
		payload, err := d.diff(left[1], right[1], true)
		return vdGroup(label, vdText("("), payload, vdText(")")), err
	}
	if valueDiffSetLike(a) || valueDiffSetLike(b) {
		return d.sets(a, b)
	}
	return d.arrow(a, b)
}
func (d valueDiffer) arrow(a, b RuntimeValue) (valueDoc, error) {
	left, err := d.pretty(a, ValueDiffRemoved)
	if err != nil {
		return valueDoc{}, err
	}
	right, err := d.pretty(b, ValueDiffAdded)
	if err != nil {
		return valueDoc{}, err
	}
	return vdGroup(left, vdText(" => "), right), nil
}
func valueDiffFields(v RuntimeValue) (*insdict.Dict[string, RuntimeValue], error) {
	children, err := runtimeChildren(v)
	if err != nil {
		return nil, err
	}
	fields := insdict.NewDict[string, RuntimeValue]()
	for i := 0; i < len(children); i += 2 {
		fields.Set(children[i].expression.Str, children[i+1])
	}
	return fields, nil
}
func (d valueDiffer) records(a, b RuntimeValue) (valueDoc, error) {
	left, err := valueDiffFields(a)
	if err != nil {
		return valueDoc{}, err
	}
	right, err := valueDiffFields(b)
	if err != nil {
		return valueDoc{}, err
	}
	keys := insdict.NewDict[string, bool]()
	for key := range left.All() {
		keys.Set(key, true)
	}
	for key := range right.All() {
		keys.Set(key, true)
	}
	names := make([]string, 0, keys.Len())
	for key := range keys.All() {
		names = append(names, key)
	}
	sort.Slice(names, func(i, j int) bool { return utf16Less(names[i], names[j]) })
	fields := make([]valueDoc, 0, len(names))
	for _, name := range names {
		old, hasOld := left.Get2(name)
		new, hasNew := right.Get2(name)
		role := ValueDiffUnchanged
		var field valueDoc
		if hasOld && hasNew {
			field, err = d.diff(old, new, true)
		} else if hasOld {
			role = ValueDiffRemoved
			field, err = d.pretty(old, role)
		} else {
			role = ValueDiffAdded
			field, err = d.pretty(new, role)
		}
		if err != nil {
			return valueDoc{}, err
		}
		fields = append(fields, vdGroup(d.token(role, name), d.token(role, ":"), vdNest("  ", vdLine(), field)))
	}
	return d.nary(ValueDiffUnchanged, "{", fields, "}", vdLine()), nil
}
func (d valueDiffer) maps(a, b RuntimeValue) (valueDoc, error) {
	left, err := runtimeEntries(a)
	if err != nil {
		return valueDoc{}, err
	}
	right, err := runtimeEntries(b)
	if err != nil {
		return valueDoc{}, err
	}
	consumed := make([]bool, len(right))
	pairs := make([]valueDoc, 0, len(left)+len(right))
	for _, old := range left {
		matched := -1
		for i, new := range right {
			if old.Key.Equal(new.Key) {
				matched = i
				break
			}
		}
		var key, value valueDoc
		role := ValueDiffUnchanged
		if matched >= 0 {
			consumed[matched] = true
			key, err = d.diff(old.Key, right[matched].Key, true)
			if err != nil {
				return valueDoc{}, err
			}
			value, err = d.diff(old.Value, right[matched].Value, true)
		} else {
			role = ValueDiffRemoved
			key, err = d.pretty(old.Key, role)
			if err != nil {
				return valueDoc{}, err
			}
			value, err = d.pretty(old.Value, role)
		}
		if err != nil {
			return valueDoc{}, err
		}
		pairs = append(pairs, vdGroup(key, d.token(role, " -> "), value))
	}
	for i, new := range right {
		if !consumed[i] {
			key, err := d.pretty(new.Key, ValueDiffAdded)
			if err != nil {
				return valueDoc{}, err
			}
			value, err := d.pretty(new.Value, ValueDiffAdded)
			if err != nil {
				return valueDoc{}, err
			}
			pairs = append(pairs, vdGroup(key, d.token(ValueDiffAdded, " -> "), value))
		}
	}
	return vdGroup(d.token(ValueDiffUnchanged, "Map"), d.nary(ValueDiffUnchanged, "(", pairs, ")", vdBreak())), nil
}
func valueDiffEmptyPayload(v RuntimeValue) bool {
	return valueDiffOp(v, "List", "Tup") && v.payload != nil && len(v.payload.children) == 0
}
func valueDiffSetLike(v RuntimeValue) bool { return valueDiffOp(v, "Set", "List", "Tup", "Map", "Rec") }
func valueDiffElements(v RuntimeValue) ([]RuntimeValue, error) {
	if valueDiffOp(v, "List", "Tup") {
		children, err := runtimeChildren(v)
		if err != nil {
			return nil, err
		}
		v, err = newRuntimeComposite("Set", children)
		if err != nil {
			return nil, err
		}
	} else if !valueDiffOp(v, "Set") {
		return nil, nil
	}
	entries, err := runtimeEntries(v)
	if err != nil {
		return nil, err
	}
	values := make([]RuntimeValue, len(entries))
	for i, entry := range entries {
		values[i] = entry.Value
	}
	return values, nil
}
func (d valueDiffer) sets(a, b RuntimeValue) (valueDoc, error) {
	left, err := valueDiffElements(a)
	if err != nil {
		return valueDoc{}, err
	}
	right, err := valueDiffElements(b)
	if err != nil {
		return valueDoc{}, err
	}
	matched := make([]bool, len(right))
	items := make([]valueDoc, 0, len(left)+len(right))
	for _, old := range left {
		role := ValueDiffRemoved
		for i, new := range right {
			if !matched[i] && old.Equal(new) {
				matched[i] = true
				role = ValueDiffUnchanged
				break
			}
		}
		item, err := d.pretty(old, role)
		if err != nil {
			return valueDoc{}, err
		}
		items = append(items, item)
	}
	for i, new := range right {
		if !matched[i] {
			item, err := d.pretty(new, ValueDiffAdded)
			if err != nil {
				return valueDoc{}, err
			}
			items = append(items, item)
		}
	}
	return vdGroup(d.token(ValueDiffUnchanged, "Set"), d.nary(ValueDiffUnchanged, "(", items, ")", vdBreak())), nil
}
func (d valueDiffer) pretty(v RuntimeValue, role ValueDiffRole) (valueDoc, error) {
	if v.closure != nil {
		return valueDoc{}, fmt.Errorf("value diff does not support lexical closures")
	}
	if valueDiffPrimitive(v) {
		if v.expression.Kind == "str" {
			return vdGroup(d.token(role, "\""), d.token(role, v.expression.Str), d.token(role, "\"")), nil
		}
		return d.token(role, ExpressionString(v.expression)), nil
	}
	switch {
	case valueDiffOp(v, "Map"):
		entries, err := runtimeEntries(v)
		if err != nil {
			return valueDoc{}, err
		}
		pairs := make([]valueDoc, 0, len(entries))
		for _, entry := range entries {
			key, err := d.pretty(entry.Key, role)
			if err != nil {
				return valueDoc{}, err
			}
			value, err := d.pretty(entry.Value, role)
			if err != nil {
				return valueDoc{}, err
			}
			pairs = append(pairs, vdGroup(key, d.token(role, " -> "), value))
		}
		return vdGroup(d.token(role, "Map"), d.nary(role, "(", pairs, ")", vdBreak())), nil
	case valueDiffOp(v, "Set"):
		elements, err := valueDiffElements(v)
		if err != nil {
			return valueDoc{}, err
		}
		items := make([]valueDoc, 0, len(elements))
		for _, element := range elements {
			item, err := d.pretty(element, role)
			if err != nil {
				return valueDoc{}, err
			}
			items = append(items, item)
		}
		return vdGroup(d.token(role, "Set"), d.nary(role, "(", items, ")", vdBreak())), nil
	case valueDiffOp(v, "Rec", "List", "Tup", "variant"):
		children, err := runtimeChildren(v)
		if err != nil {
			return valueDoc{}, err
		}
		if valueDiffOp(v, "variant") {
			label := d.token(role, children[0].expression.Str)
			if valueDiffEmptyPayload(children[1]) {
				return label, nil
			}
			payload, err := d.pretty(children[1], role)
			return vdGroup(label, d.nary(role, "(", []valueDoc{payload}, ")", vdBreak())), err
		}
		items := make([]valueDoc, 0, len(children))
		step := 1
		if valueDiffOp(v, "Rec") {
			step = 2
		}
		for i := 0; i < len(children); i += step {
			child := children[i]
			if step == 2 {
				child = children[i+1]
			}
			item, err := d.pretty(child, role)
			if err != nil {
				return valueDoc{}, err
			}
			if step == 2 {
				item = vdGroup(d.token(role, children[i].expression.Str), d.token(role, ":"), vdNest("  ", vdLine(), item))
			}
			items = append(items, item)
		}
		if step == 2 {
			return d.nary(role, "{", items, "}", vdLine()), nil
		}
		return d.nary(role, "[", items, "]", vdBreak()), nil
	}
	return valueDoc{}, fmt.Errorf("value diff does not support %s", v.expression.Kind)
}
