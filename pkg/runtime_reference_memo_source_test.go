package quint

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"testing"
)

func TestUpstreamReferenceMemoSourceHistory(t *testing.T) {
	t.Parallel()
	var fixture struct {
		Cases []struct {
			Name, Source  string
			Input, Output Expr
		}
	}
	data, err := os.ReadFile("testdata/parser/runtime_reference_memo_source.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	if len(fixture.Cases) != 14 {
		t.Fatalf("memo source oracle has %d cases, want 14", len(fixture.Cases))
	}
	for _, test := range fixture.Cases {
		t.Run(test.Name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			value, err := EvaluatePureExpression(ctx, test.Input, nil)
			if err != nil {
				t.Fatal(err)
			}
			output, err := value.ExpressionContext(ctx)
			if err != nil {
				t.Fatal(err)
			}
			got, _ := json.Marshal(output)
			want, _ := json.Marshal(test.Output)
			if !bytes.Equal(got, want) {
				t.Fatalf("source %s\ngot %s\nwant %s", test.Source, got, want)
			}
		})
	}
}
