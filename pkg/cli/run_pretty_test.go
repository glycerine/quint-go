package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"testing"

	quint "github.com/glycerine/quint-go/pkg"
)

func TestUpstreamRunTraceFormatting(t *testing.T) {
	t.Parallel()
	var fixtures struct {
		TraceCases []struct {
			States []quint.Expr
			Hide   []string
			Output string
		}
	}
	data, err := os.ReadFile("../testdata/parser/execution_format.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	for _, fixture := range fixtures.TraceCases {
		before := make([]string, len(fixture.States))
		for i, state := range fixture.States {
			before[i] = quint.ExpressionString(state)
		}
		var got bytes.Buffer
		if err = runPrintTrace(&got, fixture.States, fixture.Hide); err != nil || got.String() != fixture.Output {
			t.Fatalf("hide %v: got %q want %q: %v", fixture.Hide, got.String(), fixture.Output, err)
		}
		for i, state := range fixture.States {
			if quint.ExpressionString(state) != before[i] {
				t.Fatal("trace formatter changed input", i)
			}
		}
	}
}

type runPrettyFailWriter struct{ err error }

func (w runPrettyFailWriter) Write([]byte) (int, error) { return 0, w.err }

func TestRunPrettyValidationAndWriterErrors(t *testing.T) {
	t.Parallel()
	failure := errors.New("stream failed")
	state := quint.Expr{Kind: "app", Opcode: "Rec"}
	if err := runPrintTrace(runPrettyFailWriter{failure}, []quint.Expr{state}, nil); !errors.Is(err, failure) {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err := runPrintTrace(&output, []quint.Expr{{Kind: "bool"}}, nil); err == nil {
		t.Fatal("nonrecord trace accepted")
	}
	value, err := quint.ParseExpression(`Rec("x", List(1, 2))`)
	if err != nil {
		t.Fatal(err)
	}
	if got := runPrettyExpression(value, 0); got != "{ x: [1, 2] }" {
		t.Fatal(got)
	}
	invalid := quint.Expr{Kind: "app", Opcode: "variant"}
	if got := runPrettyExpression(invalid, 80); got != "variant()" {
		t.Fatal(got)
	}
}
