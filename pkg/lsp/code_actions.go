package lsp

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	quint "github.com/glycerine/quint-go/pkg"
)

type codeAction struct {
	Title       string                `json:"title"`
	Kind        string                `json:"kind"`
	IsPreferred bool                  `json:"isPreferred"`
	Diagnostics []json.RawMessage     `json:"diagnostics"`
	Edit        semanticWorkspaceEdit `json:"edit"`
}

// codeActionRequest ports server.ts's supplied-diagnostic quick fixes. It uses
// the current open buffer, without requiring a completed analysis snapshot.
func codeActionRequest(params json.RawMessage, snapshot *Snapshot) (any, *ResponseError) {
	actions := []codeAction{}
	var request struct {
		TextDocument struct {
			URI string `json:"uri"`
		} `json:"textDocument"`
		Range   *Range `json:"range"`
		Context *struct {
			Diagnostics []json.RawMessage `json:"diagnostics"`
		} `json:"context"`
	}
	if json.Unmarshal(params, &request) != nil || request.TextDocument.URI == "" || request.Range == nil || request.Context == nil {
		return nil, semanticError("Expected textDocument.uri, range, and diagnostic context")
	}
	if snapshot == nil || request.TextDocument.URI != snapshot.URI {
		return actions, nil
	}
	file, err := quint.NewSourceFile(snapshot.URI, snapshot.Text)
	if err != nil {
		return nil, semanticError(err.Error())
	}
	if !codeActionValidRange(file, *request.Range) {
		return nil, semanticError("Invalid code action range")
	}
	start, _ := file.ByteOffset(request.Range.Start.Line, 0)
	line := snapshot.Text[start:]
	if end := strings.IndexByte(line, '\n'); end >= 0 {
		line = line[:end+1]
	}
	var definitions []codeActionDefinition
	definitionsParsed := false
	for _, encoded := range request.Context.Diagnostics {
		var diagnostic struct {
			Range *Range          `json:"range"`
			Data  json.RawMessage `json:"data"`
			Code  json.RawMessage `json:"code"`
		}
		if json.Unmarshal(encoded, &diagnostic) != nil || diagnostic.Range == nil || !codeActionValidRange(file, *diagnostic.Range) || diagnostic.Range.Start.Line != request.Range.Start.Line {
			continue
		}
		var data struct {
			Fix *struct {
				Kind        string  `json:"kind"`
				Original    *string `json:"original"`
				Replacement *string `json:"replacement"`
			} `json:"fix"`
		}
		if json.Unmarshal(diagnostic.Data, &data) != nil || data.Fix == nil || data.Fix.Kind != "replace" || data.Fix.Original == nil || data.Fix.Replacement == nil {
			continue
		}
		fix := data.Fix
		if strings.TrimFunc(*fix.Original, codeActionWhitespace) == "" {
			continue
		}
		pattern, err := regexp.Compile(codeActionPattern(*fix.Original))
		if err != nil {
			continue
		}
		if pattern.FindStringIndex(line) == nil {
			continue
		}
		span, valid := codeActionMatchedRange(file, snapshot.Text, diagnostic.Range.Start, pattern)
		if string(diagnostic.Code) == `"QNT200"` {
			if !definitionsParsed {
				definitions = codeActionDefinitions(snapshot)
				definitionsParsed = true
			}
			if qualifier, owned, verified := codeActionQualifierRange(file, snapshot.Text, definitions, diagnostic.Range.Start, *fix.Original, pattern); owned {
				span, valid = qualifier, verified
			}
		}
		if !valid {
			continue
		}
		edit := semanticTextEdit{Range: span, NewText: *fix.Replacement}
		uri, _ := json.Marshal(snapshot.URI)
		edits, _ := json.Marshal([]semanticTextEdit{edit})
		changes := append([]byte{'{'}, uri...)
		changes = append(changes, ':')
		changes = append(changes, edits...)
		changes = append(changes, '}')
		actions = append(actions, codeAction{
			Title: fmt.Sprintf("Replace %s with %s", *fix.Original, *fix.Replacement), Kind: "quickfix", IsPreferred: true,
			Diagnostics: []json.RawMessage{append(json.RawMessage(nil), encoded...)}, Edit: semanticWorkspaceEdit{Changes: changes},
		})
	}
	return actions, nil
}

type codeActionDefinition struct {
	nameStart Position
	qualifier string
}

func codeActionDefinitions(snapshot *Snapshot) []codeActionDefinition {
	modules, sources, err := quint.ParseModulesWithSource(snapshot.URI, snapshot.Text)
	if err != nil {
		return nil
	}
	definitions := []codeActionDefinition{}
	add := func(id quint.ID, qualifier quint.Qualifier) {
		if location, ok := sources[id]; ok {
			definitions = append(definitions, codeActionDefinition{nameStart: Position{Line: location.Start.Line, Character: location.Start.Col}, qualifier: quint.QualifierString(qualifier)})
		}
	}
	for _, module := range modules {
		for _, declaration := range module.Declarations {
			if declaration.Kind == "def" {
				add(declaration.ID, declaration.Qualifier)
			}
			_ = quint.WalkExpr(declaration.Expr, func(expression *quint.Expr) (bool, error) {
				if expression.Opdef != nil {
					add(expression.Opdef.ID, expression.Opdef.Qualifier)
				}
				return true, nil
			})
		}
	}
	return definitions
}

// The upstream mode diagnostic points at the operator name, but its fix
// replaces a qualifier. Verify the current declaration and qualifier tokens
// before moving the edit, so stale diagnostics cannot replace a name or body.
func codeActionQualifierRange(file *quint.SourceFile, text string, definitions []codeActionDefinition, start Position, original string, pattern *regexp.Regexp) (Range, bool, bool) {
	for _, definition := range definitions {
		if definition.nameStart != start {
			continue
		}
		if definition.qualifier != original {
			return Range{}, true, false
		}
		name, _ := file.ByteOffset(start.Line, start.Character)
		tokens := semanticNameTokens(text, 0, name)
		words := strings.Fields(original)
		if len(tokens) < len(words) {
			return Range{}, true, false
		}
		tokens = tokens[len(tokens)-len(words):]
		for index, word := range words {
			if tokens[index].text != word {
				return Range{}, true, false
			}
		}
		first, _ := file.PositionAt(tokens[0].start)
		span, valid := codeActionMatchedRange(file, text, Position{Line: first.Line, Character: first.Col}, pattern)
		end, err := file.ByteOffset(span.End.Line, span.End.Character)
		return span, true, valid && err == nil && end == tokens[len(tokens)-1].end && span.Start.Line == start.Line
	}
	return Range{}, false, false
}

func codeActionMatchedRange(file *quint.SourceFile, text string, start Position, pattern *regexp.Regexp) (Range, bool) {
	offset, err := file.ByteOffset(start.Line, start.Character)
	if err != nil {
		return Range{}, false
	}
	line := text[offset:]
	if end := strings.IndexByte(line, '\n'); end >= 0 {
		line = line[:end+1]
	}
	match := pattern.FindStringIndex(line)
	if match == nil || match[0] != 0 || match[1] == 0 {
		return Range{}, false
	}
	location, err := file.Location(offset, offset+match[1])
	if err != nil || location.End.Line != start.Line {
		return Range{}, false
	}
	return semanticRange(location), true
}

func codeActionValidRange(file *quint.SourceFile, span Range) bool {
	start, err := file.ByteOffset(span.Start.Line, span.Start.Character)
	if err != nil {
		return false
	}
	end, err := file.ByteOffset(span.End.Line, span.End.Character)
	return err == nil && end >= start
}

// JavaScript's \s includes Unicode separators, BOM, and vertical tab. Go's
// regexp \s covers a smaller set, so use the ECMAScript character class.
func codeActionWhitespace(character rune) bool {
	return character == '\t' || character == '\n' || character == '\v' || character == '\f' || character == '\r' || character == ' ' || character == '\u00a0' || character == '\u1680' || (character >= '\u2000' && character <= '\u200a') || character == '\u2028' || character == '\u2029' || character == '\u202f' || character == '\u205f' || character == '\u3000' || character == '\ufeff'
}

func codeActionPattern(original string) string {
	var pattern strings.Builder
	space := false
	for _, character := range original {
		if codeActionWhitespace(character) {
			if !space {
				pattern.WriteString(`[\t\n\v\f\r \x{00a0}\x{1680}\x{2000}-\x{200a}\x{2028}\x{2029}\x{202f}\x{205f}\x{3000}\x{feff}]*`)
			}
			space = true
			continue
		}
		space = false
		pattern.WriteString(regexp.QuoteMeta(string(character)))
	}
	return pattern.String()
}
