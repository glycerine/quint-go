package quint

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/glycerine/insdict"
	"github.com/glycerine/quint-go/pkg/terminal"
)

func TestUpstreamTerminalDiagnosticStyles(t *testing.T) {
	t.Parallel()
	var fixtures struct {
		Cases []struct {
			Name    string
			Level   int
			Code    []struct{ Source, Text string }
			Message ErrorMessage
			DiagnosticRenderOptions
			Output, Wrapped string
		}
	}
	data, err := os.ReadFile("testdata/parser/diagnostic_terminal.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	for _, fixture := range fixtures.Cases {
		code := insdict.NewDict[string, string]()
		for _, source := range fixture.Code {
			code.Set(source.Source, source.Text)
		}
		options := fixture.DiagnosticRenderOptions
		options.InclusiveEnd = true
		options.Decorate = func(text string, styles ...string) string { return terminal.Style(text, fixture.Level, styles...) }
		got := RenderErrorMessage(fixture.Message, code, options)
		if got != fixture.Output {
			t.Fatalf("%s got=%q want=%q", fixture.Name, got, fixture.Output)
		}
		if got = terminal.Style(got, fixture.Level, "red"); got != fixture.Wrapped {
			t.Fatalf("%s outer got=%q want=%q", fixture.Name, got, fixture.Wrapped)
		}
	}
}
