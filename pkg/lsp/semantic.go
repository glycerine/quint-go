package lsp

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/glycerine/insdict"
	quint "github.com/glycerine/quint-go/pkg"
)

// These responses deliberately use plain LSP types rather than frontend IR.
// No mutable analysis tables are exposed through the protocol.
type semanticHover struct {
	Contents semanticMarkup `json:"contents"`
	Range    Range          `json:"range"`
}
type semanticMarkup struct {
	Kind  string `json:"kind"`
	Value string `json:"value"`
}
type semanticSymbol struct {
	Name           string           `json:"name"`
	Detail         string           `json:"detail,omitempty"`
	Kind           int              `json:"kind"`
	Range          Range            `json:"range"`
	SelectionRange Range            `json:"selectionRange"`
	Children       []semanticSymbol `json:"children,omitempty"`
}
type semanticCompletionItem struct {
	Label         string          `json:"label"`
	Kind          int             `json:"kind"`
	Detail        string          `json:"detail,omitempty"`
	Documentation *semanticMarkup `json:"documentation,omitempty"`
}
type semanticCompletionList struct {
	IsIncomplete bool                     `json:"isIncomplete"`
	Items        []semanticCompletionItem `json:"items"`
}
type semanticTextEdit struct {
	Range   Range  `json:"range"`
	NewText string `json:"newText"`
}
type semanticWorkspaceEdit struct {
	Changes json.RawMessage `json:"changes"`
}
type semanticDefinition struct {
	ID        quint.ID
	Name      string
	Kind      string
	Qualifier quint.Qualifier
	Doc       *string
	Module    string
	Location  Location
	FullRange Range
}
type semanticOccurrence struct {
	ID          quint.ID
	Target      quint.ID
	Resolved    bool
	Name        string
	Location    Location
	Declaration bool
}
type semanticIndex struct {
	definitions *insdict.Dict[string, semanticDefinition]
	occurrences []semanticOccurrence
}

func semanticRequest(method string, params json.RawMessage, snapshot *Snapshot) (any, *ResponseError) {
	var p struct {
		TextDocument struct {
			URI string `json:"uri"`
		} `json:"textDocument"`
		Position *Position `json:"position"`
		NewName  string    `json:"newName"`
		Context  struct {
			IncludeDeclaration bool `json:"includeDeclaration"`
		} `json:"context"`
	}
	if err := json.Unmarshal(params, &p); err != nil || p.TextDocument.URI == "" {
		return nil, semanticError("Expected textDocument.uri and valid request parameters")
	}
	if snapshot == nil || p.TextDocument.URI != snapshot.URI {
		return nil, semanticError("Document is not open")
	}
	index := buildSemanticIndex(snapshot)
	if method == "textDocument/documentSymbol" {
		return semanticDocumentSymbols(snapshot, index), nil
	}
	if p.Position == nil {
		return nil, semanticError("Expected position")
	}
	file, err := quint.NewSourceFile(snapshot.URI, snapshot.Text)
	if err != nil {
		return nil, semanticError(err.Error())
	}
	if _, err := file.ByteOffset(p.Position.Line, p.Position.Character); err != nil {
		return nil, semanticError(err.Error())
	}
	if method == "textDocument/completion" {
		return semanticCompletions(snapshot, index, *p.Position), nil
	}
	occurrence, ok := index.at(snapshot.URI, *p.Position)
	if !ok {
		switch method {
		case "textDocument/references":
			return []Location{}, nil
		case "textDocument/rename":
			return nil, semanticError("No renameable symbol at position")
		}
		return nil, nil
	}
	switch method {
	case "textDocument/hover":
		return semanticHoverAt(snapshot, index, occurrence), nil
	case "textDocument/definition":
		if !occurrence.Resolved {
			return nil, nil
		}
		definition, ok := index.definitions.Get2(occurrence.Target.String())
		if !ok {
			return nil, nil
		}
		return []Location{definition.Location}, nil
	case "textDocument/references":
		return index.references(occurrence, p.Context.IncludeDeclaration), nil
	case "textDocument/prepareRename":
		if !occurrence.Resolved {
			return nil, nil
		}
		return semanticNameSuffixRange(occurrence), nil
	case "textDocument/rename":
		return semanticRename(snapshot, index, occurrence, p.NewName)
	}
	return nil, &ResponseError{Code: -32601, Message: "Method not found"}
}

func semanticError(message string) *ResponseError {
	return &ResponseError{Code: -32602, Message: message}
}

func semanticSource(snapshot *Snapshot, source string) (string, bool) {
	if snapshot.Sources != nil {
		if text, ok := snapshot.Sources.Get2(source); ok {
			return text, true
		}
		if path, err := sourceForURI(source); err == nil {
			if text, ok := snapshot.Sources.Get2(path); ok {
				return text, true
			}
		}
	}
	if source == snapshot.URI || uriForSource(source) == snapshot.URI {
		return snapshot.Text, true
	}
	return "", false
}

func semanticRange(location quint.Location) Range {
	end := location.Start
	if location.End != nil {
		end = *location.End
	}
	return Range{Start: Position{Line: location.Start.Line, Character: location.Start.Col}, End: Position{Line: end.Line, Character: end.Col}}
}

// semanticTokenRange finds a lexical name within a parser span. Application
// spans often include arguments, so using the whole node would rename too much.
// Strings and comments are skipped and source positions remain UTF-16 based.
func semanticTokenRange(snapshot *Snapshot, id quint.ID, name string) (Location, bool) {
	return semanticTokenRangeExcluding(snapshot, id, name, nil)
}
func semanticTokenRangeExcluding(snapshot *Snapshot, id quint.ID, name string, excluded []quint.ID) (Location, bool) {
	location, ok := snapshot.Frontend.Sources[id]
	if !ok || location.End == nil || name == "" {
		return Location{}, false
	}
	text, ok := semanticSource(snapshot, location.Source)
	if !ok {
		return Location{}, false
	}
	file, err := quint.NewSourceFile(location.Source, text)
	if err != nil {
		return Location{}, false
	}
	start, err := file.ByteOffset(location.Start.Line, location.Start.Col)
	if err != nil {
		return Location{}, false
	}
	end, err := file.ByteOffset(location.End.Line, location.End.Col)
	if err != nil {
		return Location{}, false
	}
	for _, token := range semanticNameTokens(text, start, end) {
		if token.text == name || strings.HasSuffix(name, "::"+token.text) {
			loc, err := file.Location(token.start, token.end)
			if err == nil {
				insideChild := false
				for _, childID := range excluded {
					child, ok := snapshot.Frontend.Sources[childID]
					if ok && child.Source == location.Source && semanticContains(semanticRange(child), Position{Line: loc.Start.Line, Character: loc.Start.Col}) {
						insideChild = true
						break
					}
				}
				if insideChild {
					continue
				}
				return Location{URI: uriForSource(location.Source), Range: semanticRange(loc)}, true
			}
		}
	}
	return Location{}, false
}

type semanticToken struct {
	text       string
	start, end int
}

func semanticNameTokens(text string, start, end int) []semanticToken {
	out := []semanticToken{}
	for i := start; i < end; {
		if text[i] == '"' {
			i++
			for i < end {
				if text[i] == '\\' && i+1 < end {
					i += 2
					continue
				}
				if text[i] == '"' {
					i++
					break
				}
				i++
			}
			continue
		}
		if i+1 < end && text[i:i+2] == "//" {
			for i < end && text[i] != '\n' {
				i++
			}
			continue
		}
		if i+1 < end && text[i:i+2] == "/*" {
			i += 2
			for i+1 < end && text[i:i+2] != "*/" {
				i++
			}
			if i+1 < end {
				i += 2
			}
			continue
		}
		if !semanticIdentifierStart(text[i]) {
			i++
			continue
		}
		a := i
		for i < end && semanticIdentifierPart(text[i]) {
			i++
		}
		for i+2 < end && text[i:i+2] == "::" && semanticIdentifierStart(text[i+2]) {
			i += 2
			for i < end && semanticIdentifierPart(text[i]) {
				i++
			}
		}
		out = append(out, semanticToken{text: text[a:i], start: a, end: i})
	}
	return out
}
func semanticIdentifierStart(b byte) bool {
	return b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' || b == '_'
}
func semanticIdentifierPart(b byte) bool {
	return semanticIdentifierStart(b) || b >= '0' && b <= '9'
}

// A selective directive has no expression lookup-table entry. Resolve its
// selected member against the resulting module binding, rather than matching
// its spelling across unrelated modules. Source aliases can be reused by later
// modules, so consulting the final global alias scope would be ambiguous. The
// token is the name after the dot; the prototype may have the same spelling.
func semanticSelectedMember(snapshot *Snapshot, module string, declaration quint.Declaration) (semanticOccurrence, bool) {
	if (declaration.Kind != "import" && declaration.Kind != "export") || declaration.DefName == nil || *declaration.DefName == "*" || snapshot.Frontend.Scopes == nil {
		return semanticOccurrence{}, false
	}
	scope, ok := snapshot.Frontend.Scopes.Get2(module)
	if !ok {
		return semanticOccurrence{}, false
	}
	var target quint.LookupDefinition
	found := false
	bindings := scope.Definitions.Get(*declaration.DefName)
	for _, definition := range bindings {
		if definition.ImportedFrom != nil && definition.ImportedFrom.ID == declaration.ID {
			target, found = definition, true
			break
		}
	}
	// A later directive for this same identity may replace only provenance.
	if !found && len(bindings) == 1 {
		target, found = bindings[0], true
	}
	if !found {
		return semanticOccurrence{}, false
	}
	span, ok := snapshot.Frontend.Sources[declaration.ID]
	if !ok || span.End == nil {
		return semanticOccurrence{}, false
	}
	text, ok := semanticSource(snapshot, span.Source)
	if !ok {
		return semanticOccurrence{}, false
	}
	file, err := quint.NewSourceFile(span.Source, text)
	if err != nil {
		return semanticOccurrence{}, false
	}
	start, err := file.ByteOffset(span.Start.Line, span.Start.Col)
	if err != nil {
		return semanticOccurrence{}, false
	}
	end, err := file.ByteOffset(span.End.Line, span.End.Col)
	if err != nil {
		return semanticOccurrence{}, false
	}
	tokens := semanticNameTokens(text, start, end)
	if len(tokens) < 3 || tokens[0].text != declaration.Kind || tokens[1].text != declaration.ProtoName || tokens[2].text != *declaration.DefName {
		return semanticOccurrence{}, false
	}
	member, err := file.Location(tokens[2].start, tokens[2].end)
	if err != nil {
		return semanticOccurrence{}, false
	}
	return semanticOccurrence{ID: declaration.ID, Target: target.ID, Resolved: true, Name: *declaration.DefName, Location: Location{URI: uriForSource(span.Source), Range: semanticRange(member)}}, true
}

func buildSemanticIndex(snapshot *Snapshot) semanticIndex {
	index := semanticIndex{definitions: insdict.NewDict[string, semanticDefinition](), occurrences: []semanticOccurrence{}}
	addDefinition := func(id quint.ID, name, kind string, qualifier quint.Qualifier, doc *string, module string) {
		location, ok := semanticTokenRange(snapshot, id, name)
		if !ok {
			return
		}
		full := semanticRange(snapshot.Frontend.Sources[id])
		definition := semanticDefinition{ID: id, Name: name, Kind: kind, Qualifier: qualifier, Doc: doc, Module: module, Location: location, FullRange: full}
		index.definitions.Set(id.String(), definition)
		index.occurrences = append(index.occurrences, semanticOccurrence{ID: id, Target: id, Resolved: true, Name: name, Location: location, Declaration: true})
	}
	addReference := func(id quint.ID, name string, excluded ...quint.ID) {
		location, ok := semanticTokenRangeExcluding(snapshot, id, name, excluded)
		if !ok {
			return
		}
		occurrence := semanticOccurrence{ID: id, Name: name, Location: location}
		if snapshot.Frontend.Table != nil {
			if target, ok := snapshot.Frontend.Table.Get2(id); ok {
				occurrence.Target, occurrence.Resolved = target.ID, true
			}
		}
		index.occurrences = append(index.occurrences, occurrence)
	}
	visitType := func(typ *quint.Type) {
		_ = quint.WalkType(typ, func(t *quint.Type) (bool, error) {
			if t.Kind == "const" && t.ID != nil {
				addReference(*t.ID, t.Name)
			}
			return true, nil
		})
	}
	for _, module := range snapshot.Frontend.Modules {
		for _, declaration := range module.Declarations {
			if selected, ok := semanticSelectedMember(snapshot, module.Name, declaration); ok {
				index.occurrences = append(index.occurrences, selected)
			}
			if declaration.Name != "" {
				addDefinition(declaration.ID, declaration.Name, declaration.Kind, declaration.Qualifier, declaration.Doc, module.Name)
			}
			visitType(declaration.TypeAnnotation)
			visitType(declaration.Type)
			for _, root := range []*quint.Expr{declaration.Expr, declaration.Assumption} {
				_ = quint.WalkExpr(root, func(e *quint.Expr) (bool, error) {
					switch e.Kind {
					case "name":
						addReference(e.ID, e.Name)
					case "app":
						children := make([]quint.ID, len(e.Args))
						for i := range e.Args {
							children[i] = e.Args[i].ID
						}
						addReference(e.ID, e.Opcode, children...)
					case "lambda":
						for _, parameter := range e.Params {
							addDefinition(parameter.ID, parameter.Name, "parameter", "", nil, module.Name)
							visitType(parameter.TypeAnnotation)
						}
					case "let":
						if e.Opdef != nil {
							addDefinition(e.Opdef.ID, e.Opdef.Name, "local", e.Opdef.Qualifier, e.Opdef.Doc, module.Name)
							visitType(e.Opdef.TypeAnnotation)
						}
					}
					return true, nil
				})
			}
			for _, override := range declaration.Overrides {
				addReference(override.Parameter.ID, override.Parameter.Name)
				_ = quint.WalkExpr(&override.Expr, func(e *quint.Expr) (bool, error) {
					if e.Kind == "name" {
						addReference(e.ID, e.Name)
					} else if e.Kind == "app" {
						children := make([]quint.ID, len(e.Args))
						for i := range e.Args {
							children[i] = e.Args[i].ID
						}
						addReference(e.ID, e.Opcode, children...)
					}
					return true, nil
				})
			}
		}
	}
	return index
}

func semanticPositionLess(a, b Position) bool {
	return a.Line < b.Line || a.Line == b.Line && a.Character < b.Character
}
func semanticContains(r Range, p Position) bool {
	return !semanticPositionLess(p, r.Start) && semanticPositionLess(p, r.End)
}
func (index semanticIndex) at(uri string, position Position) (semanticOccurrence, bool) {
	// Accept a cursor immediately after a name as editor reference requests do.
	for _, allowEnd := range []bool{false, true} {
		for _, occurrence := range index.occurrences {
			if occurrence.Location.URI == uri && (semanticContains(occurrence.Location.Range, position) || allowEnd && occurrence.Location.Range.End == position) {
				return occurrence, true
			}
		}
	}
	return semanticOccurrence{}, false
}

func (index semanticIndex) references(occurrence semanticOccurrence, includeDeclaration bool) []Location {
	locations := []Location{}
	seen := insdict.NewDict[string, bool]()
	if !occurrence.Resolved {
		return locations
	}
	for _, candidate := range index.occurrences {
		if !candidate.Resolved || candidate.Target != occurrence.Target || candidate.Declaration && !includeDeclaration {
			continue
		}
		key := semanticLocationKey(candidate.Location)
		if !seen.Get(key) {
			seen.Set(key, true)
			locations = append(locations, candidate.Location)
		}
	}
	sort.SliceStable(locations, func(i, j int) bool {
		if locations[i].URI != locations[j].URI {
			return locations[i].URI < locations[j].URI
		}
		return semanticPositionLess(locations[i].Range.Start, locations[j].Range.Start)
	})
	return locations
}
func semanticLocationKey(location Location) string {
	return fmt.Sprintf("%s:%d:%d:%d:%d", location.URI, location.Range.Start.Line, location.Range.Start.Character, location.Range.End.Line, location.Range.End.Character)
}

func semanticTypeDetail(snapshot *Snapshot, ids ...quint.ID) string {
	if snapshot.Analysis.Types == nil {
		return ""
	}
	for _, id := range ids {
		if scheme, ok := snapshot.Analysis.Types.Get2(id.String()); ok {
			text, err := quint.TypeSchemeString(scheme)
			if err == nil {
				return text
			}
		}
	}
	return ""
}
func semanticHoverAt(snapshot *Snapshot, index semanticIndex, occurrence semanticOccurrence) any {
	prefix := occurrence.Name
	var doc *string
	if occurrence.Resolved {
		if definition, ok := index.definitions.Get2(occurrence.Target.String()); ok {
			qualifier := definition.Kind
			if definition.Kind == "def" || definition.Kind == "local" {
				qualifier = quint.QualifierString(definition.Qualifier)
			}
			prefix = qualifier + " " + occurrence.Name
			doc = definition.Doc
		}
	}
	detail := semanticTypeDetail(snapshot, occurrence.ID, occurrence.Target)
	if detail != "" {
		prefix += ": " + detail
	}
	if detail == "" && !occurrence.Resolved {
		if docs, err := quint.BuiltinDocs(); err == nil {
			if entry, ok := docs.Get2(occurrence.Name); ok {
				prefix = entry.Signature
				doc = entry.Documentation
			}
		}
	}
	if detail == "" && doc == nil && !occurrence.Resolved {
		return nil
	}
	value := "```quint\n" + prefix + "\n```"
	if doc != nil && *doc != "" {
		value += "\n\n" + *doc
	}
	if snapshot.Analysis.Effects != nil {
		if effect, ok := snapshot.Analysis.Effects.Get2(occurrence.ID.String()); ok {
			if effectText, err := quint.EffectSchemeString(effect); err == nil {
				value += "\n\n**effect**: `" + effectText + "`"
			}
		}
	}
	return semanticHover{Contents: semanticMarkup{Kind: "markdown", Value: value}, Range: occurrence.Location.Range}
}

func semanticSymbolKind(kind string) int {
	switch kind {
	case "def", "local":
		return 12 // Function
	case "const":
		return 14 // Constant
	case "typedef":
		return 5 // Class
	}
	return 13 // Variable
}
func semanticDocumentSymbols(snapshot *Snapshot, index semanticIndex) []semanticSymbol {
	out := []semanticSymbol{}
	for _, module := range snapshot.Frontend.Modules {
		location, ok := snapshot.Frontend.Sources[module.ID]
		if !ok || uriForSource(location.Source) != snapshot.URI {
			continue
		}
		selection, ok := semanticTokenRange(snapshot, module.ID, module.Name)
		if !ok {
			continue
		}
		symbol := semanticSymbol{Name: module.Name, Kind: 2, Range: semanticRange(location), SelectionRange: selection.Range}
		for _, declaration := range module.Declarations {
			definition, ok := index.definitions.Get2(declaration.ID.String())
			if !ok || definition.Location.URI != snapshot.URI {
				continue
			}
			symbol.Children = append(symbol.Children, semanticSymbol{Name: declaration.Name, Detail: semanticTypeDetail(snapshot, declaration.ID), Kind: semanticSymbolKind(declaration.Kind), Range: definition.FullRange, SelectionRange: definition.Location.Range})
		}
		sort.SliceStable(symbol.Children, func(i, j int) bool {
			return semanticPositionLess(symbol.Children[i].Range.Start, symbol.Children[j].Range.Start)
		})
		out = append(out, symbol)
	}
	sort.SliceStable(out, func(i, j int) bool { return semanticPositionLess(out[i].Range.Start, out[j].Range.Start) })
	return out
}

func semanticCompletions(snapshot *Snapshot, index semanticIndex, position Position) semanticCompletionList {
	if fields, handled := semanticFieldCompletions(snapshot, index, position); handled {
		return fields
	}
	items := insdict.NewDict[string, semanticCompletionItem]()
	addDefinition := func(name string, id quint.ID) {
		item := semanticCompletionItem{Label: name, Kind: 6, Detail: semanticTypeDetail(snapshot, id)}
		if definition, ok := index.definitions.Get2(id.String()); ok {
			if definition.Kind == "def" || definition.Kind == "local" {
				item.Kind = 3
			}
			if definition.Doc != nil {
				item.Documentation = &semanticMarkup{Kind: "markdown", Value: *definition.Doc}
			}
		}
		items.Set(name, item)
	}
	for _, module := range snapshot.Frontend.Modules {
		location, ok := snapshot.Frontend.Sources[module.ID]
		if !ok || uriForSource(location.Source) != snapshot.URI || !semanticContains(semanticRange(location), position) {
			continue
		}
		if snapshot.Frontend.Scopes != nil {
			if scope, ok := snapshot.Frontend.Scopes.Get2(module.Name); ok {
				for name, definitions := range scope.Definitions.All() {
					if len(definitions) > 0 {
						addDefinition(name, definitions[len(definitions)-1].ID)
					}
				}
			}
		}
		var visitLocal func(*quint.Expr)
		contains := func(id quint.ID) bool {
			location, ok := snapshot.Frontend.Sources[id]
			return ok && semanticContains(semanticRange(location), position)
		}
		visitLocal = func(e *quint.Expr) {
			if e == nil || !contains(e.ID) {
				return
			}
			switch e.Kind {
			case "lambda":
				for _, parameter := range e.Params {
					addDefinition(parameter.Name, parameter.ID)
				}
				visitLocal(e.Body)
			case "let":
				if e.Opdef != nil {
					addDefinition(e.Opdef.Name, e.Opdef.ID)
					visitLocal(e.Opdef.Expr)
				}
				visitLocal(e.Body)
			case "app":
				for i := range e.Args {
					visitLocal(&e.Args[i])
				}
			}
		}
		for _, declaration := range module.Declarations {
			visitLocal(declaration.Expr)
		}
	}
	for _, name := range append(quint.BuiltinOpcodes(), "Bool", "Int", "Nat", "getOnlyElement") {
		if _, ok := items.Get2(name); !ok {
			items.Set(name, semanticCompletionItem{Label: name, Kind: 3})
		}
	}
	file, _ := quint.NewSourceFile(snapshot.URI, snapshot.Text)
	offset, _ := file.ByteOffset(position.Line, position.Character)
	start := offset
	for start > 0 && (semanticIdentifierPart(snapshot.Text[start-1]) || snapshot.Text[start-1] == ':') {
		start--
	}
	prefix := snapshot.Text[start:offset]
	out := semanticCompletionList{Items: []semanticCompletionItem{}}
	for label, item := range items.All() {
		if strings.HasPrefix(label, prefix) {
			out.Items = append(out.Items, item)
		}
	}
	return out
}

func semanticNameSuffixRange(occurrence semanticOccurrence) Range {
	r := occurrence.Location.Range
	if index := strings.LastIndex(occurrence.Name, "::"); index >= 0 {
		r.Start.Character = r.End.Character - len(occurrence.Name[index+2:])
	}
	return r
}

func semanticRename(snapshot *Snapshot, index semanticIndex, occurrence semanticOccurrence, newName string) (any, *ResponseError) {
	if !occurrence.Resolved {
		return nil, semanticError("Builtins and unresolved names cannot be renamed")
	}
	expression, err := quint.ParseExpression(newName)
	if err != nil || expression.Kind != "name" || expression.Name != newName || strings.Contains(newName, "::") || newName == "_" || quint.IsBuiltinOpcode(newName) || newName == "Bool" || newName == "Int" || newName == "Nat" || newName == "getOnlyElement" {
		return nil, semanticError("newName must be an unqualified, non-reserved Quint identifier")
	}
	definition, ok := index.definitions.Get2(occurrence.Target.String())
	if !ok {
		return nil, semanticError("The symbol's declaration is unavailable")
	}
	if definition.Kind == "typedef" && newName[0] >= 'a' && newName[0] <= 'z' {
		return nil, semanticError("Type names must start with an uppercase letter")
	}
	// Reject possible capture rather than silently changing binding identity.
	// This is conservative for unrelated scopes and modules, including imported
	// bindings whose new spelling could otherwise be captured at a use site.
	for _, other := range index.definitions.All() {
		if other.ID != definition.ID && other.Name == newName {
			return nil, semanticError("The new name is already declared in the project")
		}
	}
	for _, other := range index.occurrences {
		if !other.Resolved && other.Name == newName {
			return nil, semanticError("The new name could capture an unresolved reference")
		}
	}
	changes := insdict.NewDict[string, []semanticTextEdit]()
	seen := insdict.NewDict[string, bool]()
	for _, candidate := range index.occurrences {
		if !candidate.Resolved || candidate.Target != occurrence.Target {
			continue
		}
		location := candidate.Location
		location.Range = semanticNameSuffixRange(candidate)
		key := semanticLocationKey(location)
		if seen.Get(key) {
			continue
		}
		seen.Set(key, true)
		changes.Set(location.URI, append(changes.Get(location.URI), semanticTextEdit{Range: location.Range, NewText: newName}))
	}
	// Dict preserves source discovery order; marshal keys explicitly because
	// the protocol requires an object, not a list of URI/edit pairs.
	encoded := []byte{'{'}
	first := true
	for uri, edits := range changes.All() {
		if !first {
			encoded = append(encoded, ',')
		}
		first = false
		key, _ := json.Marshal(uri)
		value, _ := json.Marshal(edits)
		encoded = append(encoded, key...)
		encoded = append(encoded, ':')
		encoded = append(encoded, value...)
	}
	encoded = append(encoded, '}')
	return semanticWorkspaceEdit{Changes: encoded}, nil
}
