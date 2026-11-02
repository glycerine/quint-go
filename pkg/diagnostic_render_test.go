package quint

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/glycerine/insdict"
)

func TestUpstreamDiagnosticRendering(t *testing.T) {
	t.Parallel()
	var fixtures []struct {
		Name    string
		Code    []struct{ Source, Text string }
		Message ErrorMessage
		DiagnosticRenderOptions
		Output string
	}
	data, err := os.ReadFile("testdata/parser/diagnostic_render.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	for _, fixture := range fixtures {
		t.Run(fixture.Name, func(t *testing.T) {
			code := insdict.NewDict[string, string]()
			for _, source := range fixture.Code {
				code.Set(source.Source, source.Text)
			}
			options := fixture.DiagnosticRenderOptions
			options.InclusiveEnd = true
			if got := RenderErrorMessage(fixture.Message, code, options); got != fixture.Output {
				t.Fatalf("got %q\nwant %q", got, fixture.Output)
			}
		})
	}
}

func TestDiagnosticRenderingHalfOpenSourceSpans(t *testing.T) {
	t.Parallel()
	text := "val x = \"😀\"\nval y = missing\n"
	file, err := NewSourceFile("unicode.qnt", text)
	if err != nil {
		t.Fatal(err)
	}
	code := insdict.NewDict[string, string]()
	code.Set(file.Name(), file.Text())
	for _, test := range []struct {
		name        string
		first, last int
		want        string
	}{
		{"astral_utf16_width", 9, 13, "\n Error [QNT404]: missing\n\n  at unicode.qnt:1:10\n  1: val x = \"😀\"\n              ^^\n"},
		{"line_boundary_excludes_next_line", 8, 15, "\n Error [QNT404]: missing\n\n  at unicode.qnt:1:9\n  1: val x = \"😀\"\n             ^^^^\n"},
		{"multiline_partial_end", 9, 20, "\n Error [QNT404]: missing\n\n  at unicode.qnt:1:10\n  1: val x = \"😀\"\n              ^^^\n  2: val y = missing\n     ^^^^^\n"},
		{"zero_width_point", 15, 15, "\n Error [QNT404]: missing\n\n  at unicode.qnt:2:1\n  2: val y = missing\n     ^\n"},
		{"trailing_eof_point", len(text), len(text), "\n Error [QNT404]: missing\n\n  at unicode.qnt:3:1\n  3: \n     ^\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			loc, err := file.Location(test.first, test.last)
			if err != nil {
				t.Fatal(err)
			}
			message := ErrorMessage{Explanation: "[QNT404] missing", Locations: []Location{loc}}
			if got := RenderErrorMessage(message, code); got != test.want {
				t.Fatalf("got %q\nwant %q", got, test.want)
			}
		})
	}
}

func TestDiagnosticRenderingResolutionAndOrder(t *testing.T) {
	t.Parallel()
	code := insdict.NewDict[string, string]()
	code.Set("calls.qnt", "one()\ntwo()\nthree()")
	a, _ := ParseID("1")
	b, _ := ParseID("2")
	c, _ := ParseID("3")
	missing, _ := ParseID("4")
	span := func(line int, end int) Location {
		return Location{Source: "calls.qnt", Start: Position{Line: line}, End: &Position{Line: line, Col: end}}
	}
	sources := SourceMap{a: span(0, 5), b: span(1, 5), c: span(2, 7)}
	trace := Diagnostic{Code: "QNT508", Message: "Assertion failed", Reference: &c, Trace: []ID{a, missing, b, a}}
	want := "\n Error [QNT508]: Assertion failed\n\n  at calls.qnt:1:1\n  1: one()\n     ^^^^^\n    at two() (calls.qnt:2:1)\n    at one() (calls.qnt:1:1)\n"
	if got := RenderDiagnostic(trace, sources, code); got != want {
		t.Fatalf("got %q\nwant %q", got, want)
	}
	trace.Trace[0] = missing
	if got := RenderDiagnostic(trace, sources, code); got != "error: [QNT508] Assertion failed" {
		t.Fatal("missing initial trace frame must suppress source and trace rendering", got)
	}
	diagnostics := []Diagnostic{{Code: "QNT404", Message: "first"}, {Code: "QNT503", Message: "second"}}
	if got := RenderDiagnostics(diagnostics, nil, code); got != "error: [QNT404] first\nerror: [QNT503] second" {
		t.Fatal(got)
	}
	if got := RenderDiagnostics(nil, nil, nil); got != "" {
		t.Fatal(got)
	}
	if code.Len() != 1 || code.Get("calls.qnt") != "one()\ntwo()\nthree()" {
		t.Fatal("mutated source dictionary")
	}
}

func TestDiagnosticRenderingFallback(t *testing.T) {
	t.Parallel()
	code := insdict.NewDict[string, string]()
	code.Set("file", "abc\ndef")
	code.Set("invalid", string([]byte{0xff}))
	for _, loc := range []Location{
		{Source: "unknown"},
		{Source: "invalid"},
		{Source: "file", Start: Position{Line: -1}},
		{Source: "file", Start: Position{Line: 2}},
		{Source: "file", Start: Position{Col: -1}},
		{Source: "file", Start: Position{Line: 1}, End: &Position{Line: 0}},
		{Source: "file", End: &Position{Line: 3}},
		{Source: "file", End: &Position{Col: -1}},
		{Source: "file", Start: Position{Col: 3}, End: &Position{Col: 1}},
		{Source: "file", Start: Position{Col: 99}, End: &Position{Line: 1, Col: 1}},
	} {
		message := ErrorMessage{Explanation: "[QNT000] original", Locations: []Location{loc}}
		if got := RenderErrorMessage(message, code); got != "error: [QNT000] original" {
			t.Fatal(loc, got)
		}
	}
	message := ErrorMessage{Explanation: "[QNT000] original", Locations: []Location{{Source: "file"}}}
	if got := RenderErrorMessage(message, nil); got != "error: [QNT000] original" {
		t.Fatal(got)
	}
	for _, trace := range []Location{
		{Source: "invalid"},
		{Source: "file", Start: Position{Line: -1}},
		{Source: "file", Start: Position{Col: -1}},
	} {
		message.TraceLocations = []Location{trace}
		if got := RenderErrorMessage(message, code); got != "error: [QNT000] original" {
			t.Fatal(trace, got)
		}
	}
}

func TestDiagnosticRenderingMessageAndTraceEdges(t *testing.T) {
	t.Parallel()
	code := insdict.NewDict[string, string]()
	code.Set("file", "😀abc")
	message := ErrorMessage{Explanation: "[QNT000]\ufeff\u2003message", Locations: []Location{{Source: "file"}}, TraceLocations: []Location{{Source: "file", Start: Position{Col: 2}, End: &Position{Col: 99}}}}
	got := RenderErrorMessage(message, code)
	if !strings.HasPrefix(got, "\n Error [QNT000]: message\n") || !strings.HasSuffix(got, "    at abc (file:1:3)\n") {
		t.Fatal(got)
	}
	message.TraceLocations = []Location{{Source: "file", Start: Position{Col: 99}}}
	// Upstream String.slice clamps positions beyond the line length.
	if got := RenderErrorMessage(message, code); !strings.HasSuffix(got, "    at  (file:1:100)\n") {
		t.Fatal(got)
	}
	message.TraceLocations = []Location{{Source: "file", Start: Position{Col: 3}, End: &Position{Col: 1}}}
	if got := RenderErrorMessage(message, code); !strings.HasSuffix(got, "    at  (file:1:4)\n") {
		t.Fatal(got)
	}
	message.TraceLocations = nil
	message.Explanation = "[] [QNT000] first"
	if got := RenderErrorMessage(message, code); !strings.HasPrefix(got, "\n Error [QNT000]: [] first\n") {
		t.Fatal(got)
	}
}
