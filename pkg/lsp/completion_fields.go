package lsp

import (
	"context"
	"strconv"
	"strings"

	"github.com/glycerine/insdict"
	quint "github.com/glycerine/quint-go/pkg"
)

type fieldCompletionContext struct {
	receiver                       string
	start, end, cursor, replaceEnd int
	prefix                         string
}

// completionFieldContext recognizes an identifier receiver and its dot suffix
// outside comments and strings. SourceFile converts the UTF-16 cursor before
// the lexical scan, so an astral character earlier on the line is harmless.
func completionFieldContext(snapshot *Snapshot, position Position) (fieldCompletionContext, bool) {
	file, err := quint.NewSourceFile(snapshot.URI, snapshot.Text)
	if err != nil {
		return fieldCompletionContext{}, false
	}
	offset, err := file.ByteOffset(position.Line, position.Character)
	if err != nil {
		return fieldCompletionContext{}, false
	}
	prefixStart := offset
	for prefixStart > 0 && semanticIdentifierPart(snapshot.Text[prefixStart-1]) {
		prefixStart--
	}
	if prefixStart == 0 || snapshot.Text[prefixStart-1] != '.' {
		return fieldCompletionContext{}, false
	}
	dot := prefixStart - 1
	start := dot
	for start > 0 && (semanticIdentifierPart(snapshot.Text[start-1]) || snapshot.Text[start-1] == ':') {
		start--
	}
	if start == dot || start > 0 && snapshot.Text[start-1] == '.' {
		return fieldCompletionContext{}, false
	}
	receiver := snapshot.Text[start:dot]
	for _, part := range strings.Split(receiver, "::") {
		if part == "" || !semanticIdentifierStart(part[0]) {
			return fieldCompletionContext{}, false
		}
		for i := 1; i < len(part); i++ {
			if !semanticIdentifierPart(part[i]) {
				return fieldCompletionContext{}, false
			}
		}
	}
	for _, token := range semanticNameTokens(snapshot.Text, 0, offset) {
		if token.start != start || token.end != dot || token.text != receiver {
			continue
		}
		replaceEnd := offset
		for replaceEnd < len(snapshot.Text) && semanticIdentifierPart(snapshot.Text[replaceEnd]) {
			replaceEnd++
		}
		return fieldCompletionContext{receiver: receiver, start: start, end: dot, cursor: offset, replaceEnd: replaceEnd, prefix: snapshot.Text[prefixStart:offset]}, true
	}
	return fieldCompletionContext{}, false
}

func semanticFieldCompletions(snapshot *Snapshot, index semanticIndex, position Position) (semanticCompletionList, bool) {
	completion, handled := completionFieldContext(snapshot, position)
	if !handled {
		return semanticCompletionList{}, false
	}
	out := semanticCompletionList{Items: []semanticCompletionItem{}}
	working := snapshot
	typ, found := completionReceiverType(working, index, completion)
	if !found {
		// A trailing dot is not a complete Quint expression. Reanalyze this
		// version with just the dot suffix blanked, preserving source offsets.
		// This uses the current source and current unsaved import overlays;
		// results from an earlier document version are never substituted.
		path, err := sourceForURI(snapshot.URI)
		if err != nil {
			return out, true
		}
		overlays := insdict.NewDict[string, string]()
		if snapshot.Sources != nil {
			for name, text := range snapshot.Sources.All() {
				overlays.Set(name, text)
			}
		}
		text := snapshot.Text[:completion.end] + strings.Repeat(" ", completion.replaceEnd-completion.end) + snapshot.Text[completion.replaceEnd:]
		overlays.Set(path, text)
		working, err = analyzeDocument(context.Background(), Document{URI: snapshot.URI, Version: snapshot.Version, Text: text}, overlays)
		if err != nil || working == nil {
			return out, true
		}
		typ, found = completionReceiverType(working, buildSemanticIndex(working), completion)
	}
	if !found {
		return out, true
	}
	if working.Frontend.Table != nil {
		resolved, err := quint.ResolveTypeAlias(working.Frontend.Table, typ)
		if err != nil {
			return out, true
		}
		typ = resolved
	}
	docs, _ := quint.BuiltinDocs()
	for _, name := range suggestedMemberBuiltins(typ) {
		if !strings.HasPrefix(name, completion.prefix) {
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
		out.Items = append(out.Items, item)
	}
	for _, item := range completionFieldsForType(typ) {
		if strings.HasPrefix(item.Label, completion.prefix) {
			out.Items = append(out.Items, item)
		}
	}
	return out, true
}

func completionReceiverType(snapshot *Snapshot, index semanticIndex, completion fieldCompletionContext) (quint.Type, bool) {
	if snapshot.Analysis.Types == nil {
		return quint.Type{}, false
	}
	file, err := quint.NewSourceFile(snapshot.URI, snapshot.Text)
	if err != nil {
		return quint.Type{}, false
	}
	start, err := file.PositionAt(completion.start)
	if err != nil {
		return quint.Type{}, false
	}
	for _, occurrence := range index.occurrences {
		if occurrence.Location.URI != snapshot.URI || occurrence.Name != completion.receiver || occurrence.Location.Range.Start != (Position{Line: start.Line, Character: start.Col}) {
			continue
		}
		for _, id := range []quint.ID{occurrence.ID, occurrence.Target} {
			if scheme, ok := snapshot.Analysis.Types.Get2(id.String()); ok {
				return scheme.Type, true
			}
		}
	}
	return quint.Type{}, false
}

// These type families and their order follow complete.ts and the exported
// operator inventories in quint/src/effects/builtinSignatures.ts.
func suggestedMemberBuiltins(typ quint.Type) []string {
	switch typ.Kind {
	case "bool":
		return []string{"iff", "implies"}
	case "int":
		return []string{"to"}
	case "set":
		return []string{"exists", "forall", "in", "contains", "union", "intersect", "exclude", "subseteq", "filter", "map", "fold", "powerset", "flatten", "allLists", "allListsUpTo", "getOnlyElement", "chooseSome", "oneOf", "isFinite", "size"}
	case "list":
		return []string{"append", "concat", "head", "tail", "length", "indices", "replaceAt", "slice", "range", "select", "foldl", "foldr"}
	case "fun":
		return []string{"get", "keys", "mapBy", "setToMap", "setOfMaps", "set", "setBy", "put"}
	case "rec":
		return []string{"fieldNames", "with"}
	}
	return nil
}
func completionFieldsForType(typ quint.Type) []semanticCompletionItem {
	out := []semanticCompletionItem{}
	if (typ.Kind != "tup" && typ.Kind != "rec") || typ.Fields == nil || typ.Fields.Kind != "row" {
		return out
	}
	for i, field := range typ.Fields.Fields {
		name := field.Name
		if typ.Kind == "tup" {
			name = "_" + strconv.Itoa(i+1)
		}
		out = append(out, semanticCompletionItem{Label: name, Kind: 5, Detail: quint.TypeString(field.Type)})
	}
	return out
}
