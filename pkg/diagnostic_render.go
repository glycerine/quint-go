package quint

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/glycerine/insdict"
)

// DiagnosticRenderOptions controls source presentation without terminal state.
// The default line offset is one and spans use Go's half-open UTF-16 locations.
// InclusiveEnd accepts the inclusive end columns in upstream source maps.
// HideLineNumbers matches REPL output: excerpts omit numbers and locations keep
// their original zero-based line. Source columns are always displayed one-based.
type DiagnosticRenderOptions struct {
	LineOffset      *int
	HideLineNumbers bool
	InclusiveEnd    bool
	// Decorate optionally styles call-stack text; the source excerpt remains
	// plain. It must preserve visible width. Nil leaves the entire message plain.
	Decorate func(string, ...string) string `json:"-"`
}

// RenderDiagnostic resolves a diagnostic and formats source excerpts and call
// frames. Source code is supplied by the caller; rendering never reads files.
func RenderDiagnostic(diagnostic Diagnostic, sources SourceMap, sourceCode *insdict.Dict[string, string], options ...DiagnosticRenderOptions) string {
	return RenderErrorMessage(ResolveDiagnostic(sources, diagnostic), sourceCode, options...)
}

// RenderDiagnostics preserves diagnostic order and separates formatted messages
// with a newline. Empty input produces an empty string.
func RenderDiagnostics(diagnostics []Diagnostic, sources SourceMap, sourceCode *insdict.Dict[string, string], options ...DiagnosticRenderOptions) string {
	messages := make([]string, len(diagnostics))
	for i, diagnostic := range diagnostics {
		messages[i] = RenderDiagnostic(diagnostic, sources, sourceCode, options...)
	}
	return strings.Join(messages, "\n")
}

var diagnosticRenderCode = regexp.MustCompile(`\[([^\]]+)\]`)

// RenderErrorMessage formats resolved primary and trace locations like Quint's
// plain-text reporter. Tabs and CRLF text remain unchanged, and caret columns
// count UTF-16 units. Missing sources or invalid spans fall back to the bare
// explanation, so reporting cannot hide the original error or write to stderr.
func RenderErrorMessage(message ErrorMessage, sourceCode *insdict.Dict[string, string], options ...DiagnosticRenderOptions) string {
	fallback := "error: " + message.Explanation
	if len(message.Locations) == 0 || sourceCode == nil {
		return fallback
	}
	var option DiagnosticRenderOptions
	if len(options) > 0 {
		option = options[0]
	}
	decorate := option.Decorate
	if decorate == nil {
		decorate = func(text string, _ ...string) string { return text }
	}
	offset := 1
	if option.LineOffset != nil {
		offset = *option.LineOffset
	}
	if option.HideLineNumbers {
		offset = 0
	}
	code, explanation := "QNT000", message.Explanation
	if match := diagnosticRenderCode.FindStringSubmatchIndex(explanation); match != nil {
		code = explanation[match[2]:match[3]]
		after := explanation[match[1]:]
		after = strings.TrimLeftFunc(after, func(r rune) bool { return unicode.IsSpace(r) || r == '\ufeff' })
		explanation = explanation[:match[0]] + after
	}
	location := func(loc Location) string {
		return fmt.Sprintf("%s:%d:%d", loc.Source, loc.Start.Line+offset, loc.Start.Col+1)
	}
	var output strings.Builder
	for _, loc := range message.Locations {
		text, found := sourceCode.Get2(loc.Source)
		if !found || !utf8.ValidString(text) {
			return fallback
		}
		lines := strings.Split(text, "\n")
		endLine, endCol, valid := diagnosticRenderEnd(loc, lines, option.InclusiveEnd)
		if !valid {
			return fallback
		}
		fmt.Fprintf(&output, "\n Error [%s]: %s\n\n  at %s\n", code, explanation, location(loc))
		for line := loc.Start.Line; line <= endLine; line++ {
			first, last := 0, diagnosticRenderUTF16Len(lines[line])-1
			if line == loc.Start.Line {
				first = loc.Start.Col
			}
			if line == endLine {
				last = endCol
			}
			if first < 0 || last < first-1 {
				return fallback
			}
			indicator := ""
			if !option.HideLineNumbers {
				indicator = fmt.Sprintf("%d: ", line+offset)
			}
			output.WriteString("  " + indicator + lines[line] + "\n")
			output.WriteString(strings.Repeat(" ", 2+len(indicator)+first))
			output.WriteString(strings.Repeat("^", 1+last-first))
			output.WriteByte('\n')
		}
	}
	for _, loc := range message.TraceLocations {
		text, found := sourceCode.Get2(loc.Source)
		if !found || text == "" {
			continue
		}
		if !utf8.ValidString(text) {
			return fallback
		}
		lines := strings.Split(text, "\n")
		if loc.Start.Line < 0 || loc.Start.Line >= len(lines) || loc.Start.Col < 0 {
			return fallback
		}
		line := lines[loc.Start.Line]
		last := diagnosticRenderUTF16Len(line)
		if loc.End != nil && loc.End.Line == loc.Start.Line {
			last = loc.End.Col
			if option.InclusiveEnd {
				last++
			}
		}
		expression := diagnosticRenderUTF16Slice(line, loc.Start.Col, last)
		output.WriteString(decorate("    at ", "gray"))
		output.WriteString(decorate(expression, "gray", "bold"))
		output.WriteString(decorate(" ("+location(loc)+")\n", "gray"))
	}
	return output.String()
}

func diagnosticRenderEnd(loc Location, lines []string, inclusive bool) (int, int, bool) {
	if loc.Start.Line < 0 || loc.Start.Line >= len(lines) || loc.Start.Col < 0 {
		return 0, 0, false
	}
	if loc.End == nil {
		return loc.Start.Line, loc.Start.Col, true
	}
	endLine, endCol := loc.End.Line, loc.End.Col
	if endLine < loc.Start.Line || endLine >= len(lines) || endCol < 0 {
		return 0, 0, false
	}
	if !inclusive {
		if endLine > loc.Start.Line && endCol == 0 {
			endLine--
			endCol = diagnosticRenderUTF16Len(lines[endLine]) - 1
		} else if endLine != loc.Start.Line || endCol != loc.Start.Col {
			endCol--
		}
	}
	return endLine, endCol, endLine != loc.Start.Line || endCol >= loc.Start.Col-1
}

func diagnosticRenderUTF16Len(text string) int {
	length := 0
	for _, r := range text {
		length++
		if r > 0xffff {
			length++
		}
	}
	return length
}

func diagnosticRenderUTF16Slice(text string, first, last int) string {
	units := utf16.Encode([]rune(text))
	if first > len(units) {
		first = len(units)
	}
	if last > len(units) {
		last = len(units)
	}
	if last < first {
		last = first
	}
	return string(utf16.Decode(units[first:last]))
}
