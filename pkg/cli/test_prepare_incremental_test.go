package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"

	quint "github.com/glycerine/quint-go/pkg"
)

func TestTestPrepareEvaluatorPreservesAnalyzedSource(t *testing.T) {
	t.Parallel()
	input := testCommandFile(t, `module A {
  const N: int
  type Box[a] = { item: a }
  pure def identity(x) = x
  val box: Box[int] = { item: N }
  val importedTest = identity(box.item) == 1
}
module M { import A(N=1) as I import A(N=2) as J }`)
	front, err := quint.ParseProject(quint.NewFileSourceResolver(nil, nil), quint.SourceLookupPath{NormalizedPath: input, SourceName: input})
	if err != nil || len(front.Diagnostics) != 0 {
		t.Fatal(err, front.Diagnostics)
	}
	analysis, err := quint.AnalyzeModules(front.Modules, front.Table)
	if err != nil || len(analysis.Diagnostics) != 0 {
		t.Fatal(err, analysis.Diagnostics)
	}
	before, err := coreMarshalStage(coreStageFields("typechecking", front, &analysis))
	if err != nil {
		t.Fatal(err)
	}
	selections, err := testSelections(front, "M", func(name string) bool { return strings.HasSuffix(name, "Test") })
	if err != nil || len(selections) != 2 {
		t.Fatal(err, selections)
	}
	seed, _ := quint.ParseSeed("0")
	evaluator, err := testPrepareEvaluator(context.Background(), front, analysis, "M", selections, quint.NewRNG(seed))
	if err != nil {
		t.Fatal(err)
	}
	after, err := coreMarshalStage(coreStageFields("typechecking", front, &analysis))
	if err != nil || !bytes.Equal(after, before) {
		t.Fatalf("test preparation changed source IR, lookup metadata, inferred schemes, raw IDs, or stage diagnostics: %v\nbefore %s\nafter %s", err, before, after)
	}
	for i, want := range []struct{ name, status string }{
		{"M::I::importedTest", "passed"},
		{"M::J::importedTest", "failed"},
	} {
		if selections[i].name != want.name || !strings.HasPrefix(selections[i].entry, "q::cliTest_") {
			t.Fatal("test discovery order or private execution entry changed", selections)
		}
		result, err := evaluator.Test(context.Background(), selections[i].entry, 1)
		if err != nil || result.Status != want.status {
			t.Fatal("constant-overridden instance lost its independent typed execution", want.name, result.Status, err)
		}
	}
}
