package quint

// SimplifyType copies a type tree and flattens nested rows without reordering
// fields. This is a structural operation, not type checking or unification.
func SimplifyType(t Type) Type {
	out := t
	if t.ID != nil {
		id := *t.ID
		out.ID = &id
	}
	copyType := func(p *Type) *Type {
		if p == nil {
			return nil
		}
		v := SimplifyType(*p)
		return &v
	}
	out.Elem = copyType(t.Elem)
	out.Arg = copyType(t.Arg)
	out.Res = copyType(t.Res)
	out.Ctor = copyType(t.Ctor)
	if t.Args != nil {
		out.Args = make([]Type, len(t.Args))
		for i, a := range t.Args {
			out.Args[i] = SimplifyType(a)
		}
	}
	if t.Fields != nil {
		row := SimplifyRow(*t.Fields)
		out.Fields = &row
	}
	return out
}
func SimplifyRow(row Row) Row {
	out := row
	if row.Fields != nil {
		out.Fields = make([]RowField, len(row.Fields))
		for i, f := range row.Fields {
			out.Fields[i] = RowField{Name: f.Name, Type: SimplifyType(f.Type)}
		}
	}
	if row.Other != nil {
		tail := SimplifyRow(*row.Other)
		out.Other = &tail
	}
	if out.Kind != "row" || out.Other == nil {
		return out
	}
	if len(out.Fields) == 0 {
		return *out.Other
	}
	if out.Other.Kind == "row" {
		out.Fields = append(out.Fields, out.Other.Fields...)
		out.Other = out.Other.Other
	}
	return out
}
func SimplifyScheme(s TypeScheme) TypeScheme {
	out := s
	out.Type = SimplifyType(s.Type)
	if s.TypeVariables != nil {
		out.TypeVariables = append([]string{}, s.TypeVariables...)
	}
	if s.RowVariables != nil {
		out.RowVariables = append([]string{}, s.RowVariables...)
	}
	return out
}
