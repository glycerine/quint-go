package quint

import "github.com/antlr4-go/antlr/v4"

// UpstreamPosition uses antlr4ts code-point indexes/columns, not UTF-16 units.
// End columns preserve ToIrListener's span-relative calculation, including its
// multiline behavior, rather than representing an editor cursor position.
type UpstreamPosition struct {
	Line  int `json:"line"`
	Col   int `json:"col"`
	Index int `json:"index"`
}
type UpstreamLocation struct {
	Source string            `json:"source"`
	Start  UpstreamPosition  `json:"start"`
	End    *UpstreamPosition `json:"end,omitempty"`
}
type UpstreamSourceMap map[ID]UpstreamLocation

func upstreamParserLocation(file *SourceFile, ctx antlr.ParserRuleContext) UpstreamLocation {
	start := ctx.GetStart()
	loc := UpstreamLocation{Source: file.name, Start: UpstreamPosition{Line: start.GetLine() - 1, Col: start.GetColumn(), Index: start.GetStart()}}
	if stop := ctx.GetStop(); stop != nil {
		column := stop.GetColumn()
		if stop.GetStop() != 0 {
			column = start.GetColumn() + stop.GetStop() - start.GetStart()
		}
		loc.End = &UpstreamPosition{Line: stop.GetLine() - 1, Col: column, Index: stop.GetStop()}
	}
	return loc
}

// ModuleParseLocations exposes editor-friendly spans and upstream-compatible
// coordinates for the same IDs. Allocation order may differ from TypeScript.
type ModuleParseLocations struct {
	Modules         []Module
	Sources         SourceMap
	UpstreamSources UpstreamSourceMap
	// IDs continues this parse's sequence for compilation and instantiation.
	IDs *IDGenerator
}

func ParseModulesWithLocations(name, text string) (ModuleParseLocations, error) {
	file, err := NewSourceFile(name, text)
	if err != nil {
		return ModuleParseLocations{}, err
	}
	ids := &locatedParserIDs{generator: NewIDGenerator(), file: file, sources: SourceMap{}, upstreamSources: UpstreamSourceMap{}}
	modules, err := parseModulesWithIDs(text, ids)
	result := ModuleParseLocations{Modules: modules, Sources: ids.sources, UpstreamSources: ids.upstreamSources, IDs: ids.generator}
	if err != nil {
		return result, withSyntaxSource(err, name)
	}
	if ids.err != nil {
		return result, ids.err
	}
	return result, nil
}
