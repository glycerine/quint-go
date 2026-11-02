package quint

// TypeNames returns distinct type and row variable names in encounter order.
// The two namespaces stay separate even when a name occurs in both.
func TypeNames(typ *Type) (typeVariables, rowVariables []string, err error) {
	typeVariables = []string{}
	rowVariables = []string{}
	seenTypes, seenRows := map[string]bool{}, map[string]bool{}
	err = WalkType(typ, func(t *Type) (bool, error) {
		if t.Kind == "var" && !seenTypes[t.Name] {
			seenTypes[t.Name] = true
			typeVariables = append(typeVariables, t.Name)
		}
		// Row tails are visited after their field types by the upstream visitor.
		return true, nil
	})
	if err != nil {
		return nil, nil, err
	}
	var collect func(*Type) error
	collect = func(t *Type) error {
		if t == nil {
			return nil
		}
		switch t.Kind {
		case "set", "list":
			return collect(t.Elem)
		case "fun":
			if err := collect(t.Arg); err != nil {
				return err
			}
			return collect(t.Res)
		case "oper", "app":
			if err := collect(t.Ctor); err != nil {
				return err
			}
			for i := range t.Args {
				if err := collect(&t.Args[i]); err != nil {
					return err
				}
			}
			return collect(t.Res)
		case "rec", "tup", "sum":
			for row := t.Fields; row != nil; row = row.Other {
				for i := range row.Fields {
					if err := collect(&row.Fields[i].Type); err != nil {
						return err
					}
				}
				if row.Kind == "var" && !seenRows[row.Name] {
					seenRows[row.Name] = true
					rowVariables = append(rowVariables, row.Name)
				}
			}
		}
		return nil
	}
	err = collect(typ)
	return
}

// RowFieldNames follows concrete row tails, retaining source order.
func RowFieldNames(row *Row) []string {
	names := []string{}
	for row != nil && row.Kind == "row" {
		for _, f := range row.Fields {
			names = append(names, f.Name)
		}
		row = row.Other
	}
	return names
}

// RowNames collects only tail variables, not variables inside field types.
func RowNames(row *Row) []string {
	for row != nil {
		if row.Kind == "var" {
			return []string{row.Name}
		}
		if row.Kind != "row" {
			break
		}
		row = row.Other
	}
	return []string{}
}
