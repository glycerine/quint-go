package lsp

import (
	"context"
	"testing"

	"github.com/glycerine/insdict"
	quint "github.com/glycerine/quint-go/pkg"
)

func TestIncrementalUTF16Edits(t *testing.T) {
	t.Parallel()
	text := "a😀b\r\nsecond"
	length := 2
	got, err := applyChanges(text, []contentChange{{Range: &Range{Start: Position{Character: 1}, End: Position{Character: 3}}, RangeLength: &length, Text: "é"}, {Range: &Range{Start: Position{Line: 1, Character: 0}, End: Position{Line: 1, Character: 6}}, Text: "next"}})
	if err != nil || got != "aéb\r\nnext" {
		t.Fatal(got, err)
	}
	got, err = applyChanges(text, []contentChange{{Text: "whole"}, {Range: &Range{Start: Position{Character: 5}, End: Position{Character: 5}}, Text: "!"}})
	if err != nil || got != "whole!" {
		t.Fatal(got, err)
	}
	for _, span := range []Range{{Start: Position{Character: 2}, End: Position{Character: 3}}, {Start: Position{Character: 4}, End: Position{Character: 1}}, {Start: Position{Line: -1}, End: Position{}}, {Start: Position{Character: 99}, End: Position{Character: 99}}} {
		if _, err := applyChanges(text, []contentChange{{Range: &span, Text: "x"}}); err == nil {
			t.Fatal("invalid UTF16 span", span)
		}
	}
	wrong := 1
	if _, err := applyChanges(text, []contentChange{{Range: &Range{Start: Position{Character: 1}, End: Position{Character: 3}}, RangeLength: &wrong}}); err == nil {
		t.Fatal("wrong rangeLength accepted")
	}
	if _, err := applyChanges("ok", []contentChange{{Text: string([]byte{0xff})}}); err == nil {
		t.Fatal("invalid UTF8 accepted")
	}
}
func TestDocumentURIs(t *testing.T) {
	t.Parallel()
	path, err := sourceForURI("file:///tmp/my%20model.qnt")
	if err != nil || path != "/tmp/my model.qnt" {
		t.Fatal(path, err)
	}
	if got := uriForSource(path); got != "file:///tmp/my%20model.qnt" {
		t.Fatal(got)
	}
	if got, err := sourceForURI("untitled:Untitled-1"); err != nil || got != "untitled:Untitled-1" {
		t.Fatal(got, err)
	}
	for _, uri := range []string{"relative.qnt", "file://remote-host/tmp/a.qnt", "file://", "%"} {
		if _, err := sourceForURI(uri); err == nil {
			t.Fatal("accepted URI", uri)
		}
	}
}
func TestGoDocumentAnalysis(t *testing.T) {
	t.Parallel()
	for _, example := range []struct {
		text string
		code string
	}{
		{`module M { pure val x = 1 }`, ""},
		{`module M { pure val x = unknown }`, "QNT404"},
		{`module M { pure val x = true + 1 }`, "QNT000"},
		{`module M { pure val x = }`, "syntax"},
	} {
		doc := Document{URI: "file:///tmp/main.qnt", Version: 3, Text: example.text}
		overlays := insdict.NewDict[string, string]()
		overlays.Set("/tmp/main.qnt", doc.Text)
		snapshot, err := analyzeDocument(context.Background(), doc, overlays)
		if err != nil || snapshot.Version != 3 || snapshot.Sources.Get("/tmp/main.qnt") != doc.Text {
			t.Fatal(err, snapshot)
		}
		if example.code == "" {
			if len(snapshot.Diagnostics) != 0 || snapshot.Analysis.Types == nil {
				t.Fatal(snapshot.Diagnostics)
			}
			continue
		}
		if len(snapshot.Diagnostics) == 0 {
			t.Fatal("missing diagnostic", example)
		}
		if example.code != "syntax" && snapshot.Diagnostics[0].Code != example.code {
			t.Fatal(snapshot.Diagnostics)
		}
	}
	doc := Document{URI: "file:///tmp/main.qnt", Version: 1, Text: `module M { import A.* from "a" pure val x = value + 1 }`}
	overlays := insdict.NewDict[string, string]()
	overlays.Set("/tmp/main.qnt", doc.Text)
	overlays.Set("/tmp/a.qnt", `module A { pure val value = 2 }`)
	snapshot, err := analyzeDocument(context.Background(), doc, overlays)
	if err != nil || len(snapshot.Diagnostics) != 0 || snapshot.Sources.Len() != 2 {
		t.Fatal(err, snapshot.Diagnostics)
	}
	overlays.Set("/tmp/a.qnt", `module A { pure val value = }`)
	snapshot, err = analyzeDocument(context.Background(), doc, overlays)
	// Upstream drops the import's phase-one diagnostic, but its recovered
	// Boolean placeholder still produces a type error at this main-file use.
	if err != nil || len(snapshot.Diagnostics) != 1 || snapshot.Diagnostics[0].Code != "QNT000" || len(snapshot.Diagnostics[0].RelatedInformation) != 0 {
		t.Fatal(err, snapshot.Diagnostics)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := analyzeDocument(ctx, doc, overlays); err != context.Canceled {
		t.Fatal(err)
	}
}
func TestDiagnosticRanges(t *testing.T) {
	t.Parallel()
	id, _ := quint.ParseID("1")
	other, _ := quint.ParseID("2")
	end := quint.Position{Line: 1, Col: 8}
	sources := quint.SourceMap{id: {Source: "/tmp/main.qnt", Start: quint.Position{Line: 1, Col: 4}, End: &end}, other: {Source: "/tmp/other.qnt", Start: quint.Position{Col: 2}}}
	groups := diagnosticsBySource(sources, []quint.Diagnostic{{Code: "QNT123", Message: "details", Reference: &id, Trace: []quint.ID{id, other}}})
	got := diagnosticsForSource(groups, "/tmp/main.qnt")[0]
	if got.Range.Start.Line != 1 || got.Range.Start.Character != 4 || got.Range.End.Character != 8 || got.Code != "QNT123" || len(got.RelatedInformation) != 0 {
		t.Fatal(got)
	}
}
