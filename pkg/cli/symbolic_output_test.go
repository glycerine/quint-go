package cli

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	quint "github.com/glycerine/quint-go/pkg"
)

func TestREPLStateFormatsSymbolicValues(t *testing.T) {
	t.Parallel()
	// .state is a Go REPL extension, while ordinary expression results are
	// covered by the saved upstream symbolic-state transcript.
	var stdout, stderr bytes.Buffer
	input := "var x:Set[int]\nx'=9.to(12)\n.state\n.exit\n"
	status := RunREPL(context.Background(), strings.NewReader(input), &stdout, &stderr, REPLLoopOptions{Quiet: true})
	if status != 0 || stderr.Len() != 0 || stdout.String() != "\ntrue\n{ x: Set(9, 10, 11, 12) }\n" {
		t.Fatalf("status=%d stdout=%q stderr=%q", status, stdout.String(), stderr.String())
	}
}

func TestREPLSymbolicDisplayErrorsKeepSessionUsable(t *testing.T) {
	t.Parallel()
	for _, test := range []struct{ name, input string }{
		{"debug", "q::debug(\"infinite\",Int.powerset())\n2+3\n.exit\n"},
		{"state", "var x:Set[Set[int]]\nx'=Int.powerset()\n.state\n2+3\n.exit\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			var stdout, stderr bytes.Buffer
			status := RunREPL(context.Background(), strings.NewReader(test.input), &stdout, &stderr, REPLLoopOptions{Quiet: true})
			if status != 0 || !strings.HasSuffix(stdout.String(), "5\n") || strings.Contains(stdout.String(), "infinite") || strings.Count(stderr.String(), "Either state is Left") != 1 {
				t.Fatalf("status=%d stdout=%q stderr=%q", status, stdout.String(), stderr.String())
			}
		})
	}
}

func TestRunExpressionsPreservesCancellation(t *testing.T) {
	t.Parallel()
	expression, err := quint.ParseExpression("0.to(1000000000000000000000000000000)")
	if err != nil {
		t.Fatal(err)
	}
	value, err := quint.EvaluatePureExpression(context.Background(), expression, nil)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for _, values := range [][]quint.RuntimeValue{nil, {value}} {
		objects := &quint.RuntimeObjectHashState{}
		if _, err = runExpressions(ctx, values, objects); !errors.Is(err, context.Canceled) || objects.Counter() != 0 {
			t.Fatalf("canceled trace serialization: counter=%d error=%v", objects.Counter(), err)
		}
	}
}
