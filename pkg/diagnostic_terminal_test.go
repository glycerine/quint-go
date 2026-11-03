package quint

import (
	"encoding/json"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/glycerine/insdict"
)

func TestUpstreamStyledDiagnosticPlainBoundary(t *testing.T) {
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
	ansi := regexp.MustCompile("\x1b\\[[0-?]*[ -/]*[@-~]")
	for _, fixture := range fixtures.Cases {
		t.Run(fixture.Name, func(t *testing.T) {
			t.Parallel()
			code := insdict.NewDict[string, string]()
			for _, source := range fixture.Code {
				code.Set(source.Source, source.Text)
			}
			options := fixture.DiagnosticRenderOptions
			options.InclusiveEnd = true
			got := RenderErrorMessage(fixture.Message, code, options)
			if strings.Contains(got, "\x1b") || got != ansi.ReplaceAllString(fixture.Output, "") || got != ansi.ReplaceAllString(fixture.Wrapped, "") {
				t.Fatalf("plain rendering=%q, upstream=%q wrapped=%q", got, fixture.Output, fixture.Wrapped)
			}
			if fixture.Level > 0 && (!strings.Contains(fixture.Output, "\x1b[90m") || !strings.Contains(fixture.Output, "\x1b[1m") || !strings.Contains(fixture.Wrapped, "\x1b[31m")) {
				t.Fatal("styled oracle lost the gray/bold stack or outer red diagnostic", fixture.Output, fixture.Wrapped)
			}
		})
	}
}
