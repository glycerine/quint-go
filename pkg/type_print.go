package quint

import "strings"

// TypeString matches upstream's diagnostic IR printer, including parentheses.
// It expects a validated type tree.
func TypeString(t Type) string {
	switch t.Kind {
	case "bool", "int", "str":
		return t.Kind
	case "const", "var":
		return t.Name
	case "set":
		return "Set[" + TypeString(*t.Elem) + "]"
	case "list":
		return "List[" + TypeString(*t.Elem) + "]"
	case "fun":
		return "(" + TypeString(*t.Arg) + " -> " + TypeString(*t.Res) + ")"
	case "oper", "app":
		args := make([]string, len(t.Args))
		for i, a := range t.Args {
			args[i] = TypeString(a)
		}
		if t.Kind == "oper" {
			return "(" + strings.Join(args, ", ") + ") => " + TypeString(*t.Res)
		}
		return TypeString(*t.Ctor) + "[" + strings.Join(args, ", ") + "]"
	case "rec":
		return RowString(*t.Fields)
	case "tup":
		return "(" + rowFieldsString(*t.Fields, false) + ")"
	case "sum":
		return "(" + sumFieldsString(*t.Fields) + ")"
	}
	return "<invalid type>"
}
func RowString(r Row) string {
	fields := rowFieldsString(r, true)
	if fields == "" {
		return "{}"
	}
	return "{ " + fields + " }"
}
func rowFieldsString(r Row, names bool) string {
	switch r.Kind {
	case "empty":
		return ""
	case "var":
		return "| " + r.Name
	case "row":
		fields := make([]string, len(r.Fields))
		for i, f := range r.Fields {
			prefix := ""
			if names {
				prefix = f.Name + ": "
			}
			fields[i] = prefix + TypeString(f.Type)
		}
		other := rowFieldsString(*r.Other, true)
		switch r.Other.Kind {
		case "row":
			fields = append(fields, other)
		case "var":
			return strings.Join(fields, ", ") + " " + other
		}
		return strings.Join(fields, ", ")
	}
	return "<invalid row>"
}
func sumFieldsString(r Row) string {
	fields := make([]string, len(r.Fields))
	for i, f := range r.Fields {
		fields[i] = f.Name
		if !f.Type.IsUnit() {
			fields[i] += "(" + TypeString(f.Type) + ")"
		}
	}
	if r.Other != nil && r.Other.Kind == "row" {
		fields = append(fields, sumFieldsString(*r.Other))
	}
	return strings.Join(fields, " | ")
}
func QualifierString(q Qualifier) string {
	switch q {
	case PureDef:
		return "pure def"
	case PureVal:
		return "pure val"
	}
	return string(q)
}
func ConstraintString(c Constraint) string {
	switch c.Kind {
	case "empty":
		return "true"
	case "eq":
		return TypeString(c.Types[0]) + " ~ " + TypeString(c.Types[1])
	case "isDefined":
		return TypeString(c.Type) + " is defined"
	case "conjunction":
		parts := make([]string, len(c.Constraints))
		for i, child := range c.Constraints {
			parts[i] = ConstraintString(child)
		}
		return strings.Join(parts, " /\\ ")
	}
	return "<invalid constraint>"
}
