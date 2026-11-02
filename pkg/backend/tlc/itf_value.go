package tlc

import (
	"context"
	"fmt"
	"reflect"
	"sort"
	"strconv"

	quint "github.com/glycerine/quint-go/pkg"
	native "github.com/glycerine/tlago/tlc"
)

// nativeITFExpr snapshots a concrete native value while the caller owns TLC.
// Checked Quint types are essential: TLC equates sequences, tuples and integer
// functions, and represents variants as singleton string-key functions.
func nativeITFExpr(ctx context.Context, value native.Value, typ quint.Type) (expr quint.Expr, err error) {
	defer func() {
		if failure := recover(); failure != nil {
			expr = quint.Expr{}
			err = fmt.Errorf("TLC ITF value: %v", failure)
		}
	}()
	return nativeITFExprAt(ctx, value, typ, 0)
}

func nativeITFExprAt(ctx context.Context, value native.Value, typ quint.Type, depth int) (quint.Expr, error) {
	if err := ctx.Err(); err != nil {
		return quint.Expr{}, err
	}
	if value == nil || reflect.ValueOf(value).Kind() == reflect.Pointer && reflect.ValueOf(value).IsNil() {
		return quint.Expr{}, fmt.Errorf("TLC ITF value is nil")
	}
	if depth >= 256 {
		return quint.Expr{}, fmt.Errorf("TLC ITF value exceeds nesting limit")
	}
	child := func(value native.Value, typ quint.Type) (quint.Expr, error) {
		return nativeITFExprAt(ctx, value, typ, depth+1)
	}
	app := func(op string, args []quint.Expr) quint.Expr {
		return quint.Expr{Kind: "app", Opcode: op, Args: args}
	}
	mismatch := func() (quint.Expr, error) {
		return quint.Expr{}, fmt.Errorf("TLC ITF expected %s, found %T", typ.Kind, value)
	}
	switch typ.Kind {
	case "bool":
		v, ok := value.(*native.BoolValue)
		if !ok {
			return mismatch()
		}
		return quint.Expr{Kind: "bool", Bool: v.GetVal()}, nil
	case "int":
		v, ok := value.(*native.IntValue)
		if !ok {
			return mismatch()
		}
		// Native TLC currently restricts integers to int32. Preserve its exact
		// decimal representation; the common ITF encoder always uses #bigint.
		n, err := quint.ParseInteger(v.String())
		if err != nil {
			return quint.Expr{}, err
		}
		return quint.Expr{Kind: "int", Int: n}, nil
	case "str":
		v, ok := value.(*native.StringValue)
		if !ok {
			return mismatch()
		}
		return quint.Expr{Kind: "str", Str: v.RawString()}, nil
	case "list", "tup":
		var fields []quint.RowField
		var err error
		if typ.Kind == "tup" {
			fields, err = itfConcreteFields(typ.Fields)
			if err != nil {
				return quint.Expr{}, err
			}
			// Match the checker's tuple importer, which orders numeric row
			// fields independently of their encounter order.
			for _, field := range fields {
				index, err := strconv.Atoi(field.Name)
				if err != nil || index < 0 || strconv.Itoa(index) != field.Name {
					return quint.Expr{}, fmt.Errorf("TLC ITF invalid tuple field %q", field.Name)
				}
			}
			sort.Slice(fields, func(i, j int) bool {
				a, _ := strconv.Atoi(fields[i].Name)
				b, _ := strconv.Atoi(fields[j].Name)
				return a < b
			})
			if len(fields) == 0 {
				// The Quint importer encodes Tup() as [tag |-> "UNIT"].
				if f := native.ValueToFcnRcd(value); f != nil {
					domain := f.GetDomainAsValues()
					if len(domain) == 1 {
						key, ok := domain[0].(*native.StringValue)
						if ok && key.RawString() == "tag" {
							unit, err := f.ApplyWithControl(domain[0], native.EvalClear)
							if err != nil {
								return quint.Expr{}, err
							}
							if tag, ok := unit.(*native.StringValue); ok && tag.RawString() == "UNIT" {
								return app("Tup", []quint.Expr{}), nil
							}
						}
					}
				}
			}
		} else if typ.Elem == nil {
			return quint.Expr{}, fmt.Errorf("TLC ITF list type has no element")
		}
		tuple := native.ValueToTuple(value)
		if tuple == nil {
			return mismatch()
		}
		values := tuple.GetElems()
		if typ.Kind == "tup" && len(values) != len(fields) {
			return quint.Expr{}, fmt.Errorf("TLC ITF tuple has %d values, type requires %d", len(values), len(fields))
		}
		args := make([]quint.Expr, 0, len(values))
		for i, v := range values {
			var element quint.Type
			if typ.Kind == "list" {
				element = *typ.Elem
			} else {
				element = fields[i].Type
			}
			expr, err := child(v, element)
			if err != nil {
				return quint.Expr{}, fmt.Errorf("element %d: %w", i, err)
			}
			args = append(args, expr)
		}
		if typ.Kind == "list" {
			return app("List", args), nil
		}
		return app("Tup", args), nil
	case "set":
		if typ.Elem == nil {
			return quint.Expr{}, fmt.Errorf("TLC ITF set type has no element")
		}
		set, ok := value.(interface {
			Elements() native.ValueEnumeration
		})
		if !ok {
			return mismatch()
		}
		finite, err := value.IsFinite()
		if err != nil {
			return quint.Expr{}, err
		}
		if !finite {
			return quint.Expr{}, fmt.Errorf("TLC ITF cannot enumerate an infinite set")
		}
		args := []quint.Expr{}
		enumeration := set.Elements()
		if enumeration == nil {
			return quint.Expr{}, fmt.Errorf("TLC ITF set has no enumeration")
		}
		for {
			if err := ctx.Err(); err != nil {
				return quint.Expr{}, err
			}
			v := enumeration.NextElement()
			if v == nil {
				break
			}
			expr, err := child(v, *typ.Elem)
			if err != nil {
				return quint.Expr{}, err
			}
			args = append(args, expr)
		}
		if err := enumeration.Err(); err != nil {
			return quint.Expr{}, err
		}
		return app("Set", args), nil
	case "fun", "rec", "sum":
		f := native.ValueToFcnRcd(value)
		if f == nil {
			return mismatch()
		}
		domain := f.GetDomainAsValues()
		if typ.Kind == "fun" {
			if typ.Arg == nil || typ.Res == nil {
				return quint.Expr{}, fmt.Errorf("TLC ITF function type has no domain or range")
			}
			pairs := make([]quint.Expr, 0, len(domain))
			for _, key := range domain {
				k, err := child(key, *typ.Arg)
				if err != nil {
					return quint.Expr{}, fmt.Errorf("map key: %w", err)
				}
				value, err := f.ApplyWithControl(key, native.EvalClear)
				if err != nil {
					return quint.Expr{}, err
				}
				v, err := child(value, *typ.Res)
				if err != nil {
					return quint.Expr{}, fmt.Errorf("map value: %w", err)
				}
				pairs = append(pairs, app("Tup", []quint.Expr{k, v}))
			}
			return app("Map", pairs), nil
		}
		fields, err := itfConcreteFields(typ.Fields)
		if err != nil {
			return quint.Expr{}, err
		}
		if typ.Kind == "sum" {
			if len(domain) != 1 {
				return quint.Expr{}, fmt.Errorf("TLC ITF variant must have exactly one tag")
			}
			tag, ok := domain[0].(*native.StringValue)
			if !ok {
				return mismatch()
			}
			for _, field := range fields {
				if field.Name != tag.RawString() {
					continue
				}
				payload, err := f.ApplyWithControl(domain[0], native.EvalClear)
				if err != nil {
					return quint.Expr{}, err
				}
				expr, err := child(payload, field.Type)
				if err != nil {
					return quint.Expr{}, fmt.Errorf("variant %s: %w", field.Name, err)
				}
				return app("variant", []quint.Expr{{Kind: "str", Str: field.Name}, expr}), nil
			}
			return quint.Expr{}, fmt.Errorf("TLC ITF variant tag %q is absent from checked type", tag.RawString())
		}
		if len(domain) != len(fields) {
			return quint.Expr{}, fmt.Errorf("TLC ITF record has %d fields, type requires %d", len(domain), len(fields))
		}
		args := make([]quint.Expr, 0, len(fields)*2)
		for _, field := range fields {
			key := native.NewStringValue(field.Name)
			value, err := f.ApplyWithControl(key, native.EvalClear)
			if err != nil {
				return quint.Expr{}, fmt.Errorf("record field %s: %w", field.Name, err)
			}
			expr, err := child(value, field.Type)
			if err != nil {
				return quint.Expr{}, fmt.Errorf("record field %s: %w", field.Name, err)
			}
			args = append(args, quint.Expr{Kind: "str", Str: field.Name}, expr)
		}
		return app("Rec", args), nil
	default:
		return quint.Expr{}, fmt.Errorf("TLC ITF requires a concrete checked type, found %q", typ.Kind)
	}
}

func itfConcreteFields(row *quint.Row) ([]quint.RowField, error) {
	if row == nil {
		return nil, fmt.Errorf("TLC ITF type is missing its row")
	}
	fields := []quint.RowField{}
	seen := map[string]bool{}
	for depth := 0; row != nil; row = row.Other {
		if depth >= 256 {
			return nil, fmt.Errorf("TLC ITF row exceeds nesting limit")
		}
		depth++
		if row.Kind == "empty" {
			if row.Other != nil || len(row.Fields) > 0 {
				return nil, fmt.Errorf("TLC ITF empty row has unexpected fields or tail")
			}
			return fields, nil
		}
		if row.Kind != "row" {
			return nil, fmt.Errorf("TLC ITF requires a closed row, found %q", row.Kind)
		}
		for _, field := range row.Fields {
			if seen[field.Name] {
				return nil, fmt.Errorf("TLC ITF type has duplicate field %q", field.Name)
			}
			seen[field.Name] = true
			fields = append(fields, field)
		}
	}
	return nil, fmt.Errorf("TLC ITF row has no closing empty tail")
}
