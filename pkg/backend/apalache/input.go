package apalache

import (
	"bytes"
	"encoding/json"
	"fmt"

	checker "github.com/glycerine/apalache-go/pkg"
	"github.com/glycerine/insdict"
	quint "github.com/glycerine/quint-go/pkg"
)

// MarshalInput adapts compilation output to Go Apalache's strict Quint JSON
// contract. Lookup provenance and module documentation are frontend metadata;
// the checker does not consume them. Typedef declarations are already inlined
// and are omitted, matching the importer's own treatment of typedefs. The
// checker's unused table field is explicitly empty. All inferred types remain.
// Unflattened directives and unresolved type applications are rejected.
func MarshalInput(compiled quint.CompilationResult) ([]byte, error) {
	if len(compiled.Diagnostics) != 0 {
		return nil, fmt.Errorf("compilation has diagnostics: %v", compiled.Diagnostics)
	}
	if compiled.MainModule.Name == "" || compiled.Analysis.Types == nil {
		return nil, fmt.Errorf("compiled main module and inferred types are required")
	}
	module := compiled.MainModule
	module.Doc = nil
	module.Declarations = []quint.Declaration{}
	var cleanExpr func(*quint.Expr)
	cleanExpr = func(e *quint.Expr) {
		if e == nil {
			return
		}
		for i := range e.Params {
			e.Params[i].Shadowing = nil
		}
		if e.Opdef != nil {
			e.Opdef.Hidden = nil
			e.Opdef.Namespaces = nil
			e.Opdef.Shadowing = nil
			e.Opdef.ImportedFrom = nil
			cleanExpr(e.Opdef.Expr)
		}
		for i := range e.Args {
			cleanExpr(&e.Args[i])
		}
		cleanExpr(e.Body)
	}
	checkType := func(t quint.Type) (quint.Type, error) {
		err := quint.WalkType(&t, func(n *quint.Type) (bool, error) {
			if n.Kind == "app" {
				return false, fmt.Errorf("unresolved type application in checker input")
			}
			return true, nil
		})
		if err != nil {
			return quint.Type{}, err
		}
		return quint.ApplySubstitution(nil, t)
	}
	for _, d := range compiled.MainModule.Declarations {
		switch d.Kind {
		case "typedef":
			continue
		case "def", "const", "var", "assume":
		default:
			return nil, fmt.Errorf("checker input requires flattened definitions, found %s", d.Kind)
		}
		copy, err := quint.TransformDeclarationTypes(d, checkType)
		if err != nil {
			return nil, err
		}
		copy.Hidden = nil
		copy.Namespaces = nil
		copy.Shadowing = nil
		copy.ImportedFrom = nil
		cleanExpr(copy.Expr)
		cleanExpr(copy.Assumption)
		module.Declarations = append(module.Declarations, copy)
	}
	for _, s := range compiled.Analysis.Types.All() {
		if _, err := checkType(s.Type); err != nil {
			return nil, err
		}
	}
	data, err := json.Marshal(struct {
		Stage   string          `json:"stage"`
		Modules []quint.Module  `json:"modules"`
		Types   orderedTypes    `json:"types"`
		Table   json.RawMessage `json:"table"`
	}{"compiling", []quint.Module{module}, orderedTypes{compiled.Analysis.Types}, json.RawMessage(`{}`)})
	if err != nil {
		return nil, err
	}
	if _, err := checker.ParseQuintJSON(data); err != nil {
		return nil, fmt.Errorf("Go Apalache input: %w", err)
	}
	return data, nil
}

// orderedTypes writes the frontend's insertion-ordered table as a JSON object.
type orderedTypes struct {
	values *insdict.Dict[string, quint.TypeScheme]
}

func (t orderedTypes) MarshalJSON() ([]byte, error) {
	var out bytes.Buffer
	out.WriteByte('{')
	first := true
	for id, scheme := range t.values.All() {
		value, err := json.Marshal(scheme)
		if err != nil {
			return nil, err
		}
		key, _ := json.Marshal(id)
		if !first {
			out.WriteByte(',')
		}
		first = false
		out.Write(key)
		out.WriteByte(':')
		out.Write(value)
	}
	out.WriteByte('}')
	return out.Bytes(), nil
}
