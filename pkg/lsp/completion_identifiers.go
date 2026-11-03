package lsp

import (
	"strings"

	quint "github.com/glycerine/quint-go/pkg"
)

// Invoked completion on a complete, resolved identifier offers operators for
// its type. Upstream's declaration search admits variables and declarations
// inside expressions (parameters and local definitions), while top-level
// definitions remain available through Go's existing name-prefix completion.
func semanticTypedIdentifierCompletions(snapshot *Snapshot, index semanticIndex, position Position) (semanticCompletionList, bool) {
	out := semanticCompletionList{Items: []semanticCompletionItem{}}
	file, err := quint.NewSourceFile(snapshot.URI, snapshot.Text)
	if err != nil {
		return out, false
	}
	offset, err := file.ByteOffset(position.Line, position.Character)
	if err != nil {
		return out, false
	}
	start := offset
	for start > 0 && semanticIdentifierPart(snapshot.Text[start-1]) {
		start--
	}
	if start == offset || !semanticIdentifierStart(snapshot.Text[start]) || start > 0 && snapshot.Text[start-1] == ':' || offset < len(snapshot.Text) && semanticIdentifierPart(snapshot.Text[offset]) {
		return out, false
	}
	name := snapshot.Text[start:offset]
	code := false
	for _, token := range semanticNameTokens(snapshot.Text, 0, offset) {
		if token.start == start && token.end == offset && token.text == name {
			code = true
			break
		}
	}
	if !code {
		return out, false
	}
	occurrence, found := index.at(snapshot.URI, position)
	if !found || !occurrence.Resolved || occurrence.Name != name || occurrence.Location.Range.End != position {
		return out, false
	}
	definition, found := index.definitions.Get2(occurrence.Target.String())
	if !found || definition.Location.URI != snapshot.URI || (definition.Kind != "var" && definition.Kind != "parameter" && definition.Kind != "local") {
		return out, false
	}
	// A reference to an imported variable does not turn it into a declaration
	// in the current module. Keep the upstream module-scope search boundary.
	local := false
	for _, module := range snapshot.Frontend.Modules {
		span, ok := snapshot.Frontend.Sources[module.ID]
		if ok && module.Name == definition.Module && uriForSource(span.Source) == snapshot.URI && semanticContains(semanticRange(span), position) {
			local = true
			break
		}
	}
	if !local {
		return out, false
	}
	if snapshot.Analysis.Types == nil {
		return out, true
	}
	scheme, found := snapshot.Analysis.Types.Get2(occurrence.Target.String())
	if !found {
		return out, true
	}
	typ := scheme.Type
	if snapshot.Frontend.Table != nil {
		typ, err = quint.ResolveTypeAlias(snapshot.Frontend.Table, typ)
		if err != nil {
			return out, true
		}
	}
	// An unconstrained type variable supplies no meaningful operator family.
	if typ.Kind == "var" {
		return out, true
	}
	out.Items = completionItemsForType(typ, "")
	return out, true
}

func completionItemsForType(typ quint.Type, prefix string) []semanticCompletionItem {
	items := []semanticCompletionItem{}
	docs, _ := quint.BuiltinDocs()
	for _, name := range suggestedMemberBuiltins(typ) {
		if !strings.HasPrefix(name, prefix) {
			continue
		}
		item := semanticCompletionItem{Label: name, Kind: 3}
		if docs != nil {
			if entry, ok := docs.Get2(name); ok {
				item.Detail = entry.Signature
				if entry.Documentation != nil && *entry.Documentation != "" {
					item.Documentation = &semanticMarkup{Kind: "markdown", Value: *entry.Documentation}
				}
			}
		}
		items = append(items, item)
	}
	for _, item := range completionFieldsForType(typ) {
		if strings.HasPrefix(item.Label, prefix) {
			items = append(items, item)
		}
	}
	return items
}
