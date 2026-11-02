package quint

import "strings"

func checkTypeDeclaration(d Declaration, checkName bool) error {
	if checkName {
		parts := strings.Split(d.Name, "::")
		name := parts[len(parts)-1]
		if len(name) > 0 && name[0] >= 'a' && name[0] <= 'z' {
			parts[len(parts)-1] = strings.ToUpper(name[:1]) + name[1:]
			return Diagnostic{Code: QNT007, Message: "type names must start with an uppercase letter", Reference: &d.ID, Data: &DiagnosticData{Fix: &DiagnosticFix{Kind: "replace", Original: "type " + d.Name, Replacement: "type " + strings.Join(parts, "::")}}}
		}
	}
	if d.Type == nil {
		return nil
	}
	types, rows, err := TypeNames(d.Type)
	if err != nil {
		return err
	}
	declared := map[string]bool{}
	for _, name := range d.Params {
		declared[name] = true
	}
	unbound := []string{}
	for _, name := range append(types, rows...) {
		if !declared[name] {
			unbound = append(unbound, name)
		}
	}
	if len(unbound) == 0 {
		return nil
	}
	return Diagnostic{Code: QNT014, Reference: &d.ID, Data: &DiagnosticData{}, Message: "the type variables " + strings.Join(unbound, ", ") + ` are unbound.
E.g., in

   type T = List[a]

type variable 'a' is unbound. To fix it, write

   type T[a] = List[a]`}
}
