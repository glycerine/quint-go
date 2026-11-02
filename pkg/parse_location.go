package quint

import (
	"fmt"
	"github.com/antlr4-go/antlr/v4"
)

// RuneLocation converts a half-open range of Unicode code-point offsets into
// source coordinates. ANTLR Go indexes its input by code points rather than bytes.
func (f *SourceFile) RuneLocation(start, end int) (Location, error) {
	if start < 0 || end < start || end >= len(f.positions) {
		return Location{}, fmt.Errorf("invalid rune span %d:%d", start, end)
	}
	finish := f.positions[end]
	return Location{Source: f.name, Start: f.positions[start], End: &finish}, nil
}

// parserLocation adapts inclusive ANTLR token ends to half-open source spans.
// An absent stop token leaves the end unknown; EOF has zero width. Invalid or
// synthetic token offsets produce errors instead of misleading locations.
func parserLocation(file *SourceFile, ctx antlr.ParserRuleContext) (Location, error) {
	start := ctx.GetStart()
	if start == nil {
		return Location{}, fmt.Errorf("parse context has no start token")
	}
	begin := start.GetStart()
	if begin < 0 || begin >= len(file.positions) {
		return Location{}, fmt.Errorf("invalid start token offset %d", begin)
	}
	stop := ctx.GetStop()
	if stop == nil {
		return Location{Source: file.name, Start: file.positions[begin]}, nil
	}
	end := stop.GetStop() + 1
	if stop.GetTokenType() == antlr.TokenEOF {
		end = stop.GetStart()
	}
	return file.RuneLocation(begin, end)
}
