package lsp

import (
	"encoding/json"

	quint "github.com/glycerine/quint-go/pkg"
)

// Signature help follows server.ts:onSignatureHelp: show the source signature
// for the smallest enclosing name/application/type reference. Upstream always
// selects signature and parameter zero, including at later call arguments.
type signatureHelp struct {
	Signatures      []signatureInformation `json:"signatures"`
	ActiveSignature int                    `json:"activeSignature"`
	ActiveParameter int                    `json:"activeParameter"`
}
type signatureInformation struct {
	Name          string          `json:"name"`
	Label         string          `json:"label"`
	Documentation *semanticMarkup `json:"documentation,omitempty"`
}

func emptySignatureHelp() signatureHelp {
	return signatureHelp{Signatures: []signatureInformation{}}
}

func signatureHelpRequest(params json.RawMessage, snapshot *Snapshot) (any, *ResponseError) {
	var p struct {
		TextDocument struct {
			URI string `json:"uri"`
		} `json:"textDocument"`
		Position *Position `json:"position"`
	}
	if json.Unmarshal(params, &p) != nil || p.TextDocument.URI == "" || p.Position == nil {
		return nil, semanticError("Expected textDocument.uri and position")
	}
	out := emptySignatureHelp()
	if snapshot == nil || p.TextDocument.URI != snapshot.URI {
		return out, nil
	}
	file, err := quint.NewSourceFile(snapshot.URI, snapshot.Text)
	if err != nil {
		return nil, semanticError(err.Error())
	}
	if _, err := file.ByteOffset(p.Position.Line, p.Position.Character); err != nil {
		return nil, semanticError(err.Error())
	}
	name, id, found := signatureHelpName(snapshot, *p.Position)
	if !found {
		return out, nil
	}
	var target quint.ID
	resolved := false
	if table := snapshot.Frontend.Table; table != nil {
		if definition, ok := table.Get2(id); ok {
			target, resolved = definition.ID, true
		} else {
			// Application nodes occasionally have no direct table entry. Match
			// the upstream positional/declaration/first-name fallback order.
			var first, declaration quint.ID
			firstFound, declarationFound := false, false
			for ref, definition := range table.All() {
				if definition.Name != name {
					continue
				}
				if !firstFound {
					first, firstFound = definition.ID, true
				}
				if !declarationFound && ref == definition.ID {
					declaration, declarationFound = definition.ID, true
				}
				if loc, ok := snapshot.Frontend.Sources[ref]; ok && uriForSource(loc.Source) == snapshot.URI && signatureLocationContains(loc, *p.Position) {
					target, resolved = definition.ID, true
					break
				}
			}
			if !resolved {
				if declarationFound {
					target, resolved = declaration, true
				} else if firstFound {
					target, resolved = first, true
				}
			}
		}
	}
	var entry quint.DocumentationEntry
	available := false
	if resolved {
		if snapshot.Documentation != nil {
			entry, available = snapshot.Documentation.Get2(target.String())
		}
	} else if docs, err := quint.BuiltinDocs(); err == nil {
		entry, available = docs.Get2(name)
	}
	if !available {
		return out, nil
	}
	information := signatureInformation{Name: entry.Name, Label: entry.Signature}
	if entry.Documentation != nil && *entry.Documentation != "" {
		information.Documentation = &semanticMarkup{Kind: "markdown", Value: *entry.Documentation}
	}
	out.Signatures = append(out.Signatures, information)
	return out, nil
}

func signatureLocationContains(location quint.Location, position Position) bool {
	if location.End == nil {
		return position == (Position{Line: location.Start.Line, Character: location.Start.Col})
	}
	return semanticContains(semanticRange(location), position)
}

// Search parser spans rather than lexical name ranges: hovering over a literal
// argument or a call delimiter still selects its enclosing application. UTF-16
// editor spans preserve correct positions after astral characters and newlines.
func signatureHelpName(snapshot *Snapshot, position Position) (string, quint.ID, bool) {
	var name string
	var id quint.ID
	bestSize := int(^uint(0) >> 1)
	found := false
	add := func(candidate quint.ID, candidateName string) {
		loc, ok := snapshot.Frontend.Sources[candidate]
		if !ok || uriForSource(loc.Source) != snapshot.URI || !signatureLocationContains(loc, position) {
			return
		}
		size := 0
		if loc.End != nil {
			size = loc.End.Index - loc.Start.Index
		}
		if !found || size <= bestSize {
			name, id, bestSize, found = candidateName, candidate, size, true
		}
	}
	visitType := func(root *quint.Type) {
		_ = quint.WalkType(root, func(typ *quint.Type) (bool, error) {
			if typ.Kind == "const" && typ.ID != nil {
				add(*typ.ID, typ.Name)
			}
			return true, nil
		})
	}
	visitExpr := func(root *quint.Expr) {
		_ = quint.WalkExpr(root, func(expr *quint.Expr) (bool, error) {
			switch expr.Kind {
			case "name":
				add(expr.ID, expr.Name)
			case "app":
				add(expr.ID, expr.Opcode)
			case "lambda":
				for _, parameter := range expr.Params {
					visitType(parameter.TypeAnnotation)
				}
			case "let":
				if expr.Opdef != nil {
					visitType(expr.Opdef.TypeAnnotation)
				}
			}
			return true, nil
		})
	}
	for _, module := range snapshot.Frontend.Modules {
		for _, declaration := range module.Declarations {
			visitType(declaration.TypeAnnotation)
			visitType(declaration.Type)
			visitExpr(declaration.Expr)
			visitExpr(declaration.Assumption)
			for _, override := range declaration.Overrides {
				visitType(override.Parameter.TypeAnnotation)
				visitExpr(&override.Expr)
			}
		}
	}
	return name, id, found
}
