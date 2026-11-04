package quint

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

// The generator builds valid projects rather than skipping random syntax
// failures. All arithmetic is bounded, and a closed-form oracle is independent
// of both name resolution and the evaluator. Imports, exports, type aliases,
// shadowing, dependency order, and independent constant instances can vary.
func FuzzFlatteningSemanticPreservation(f *testing.F) {
	for _, seed := range []struct {
		shape uint8
		x, c  int16
		depth uint8
	}{
		{0, 0, 0, 0}, {5, -4, 5, 1}, {10, 2, -3, 2}, {15, -2, 7, 1},
	} {
		f.Add(seed.shape, seed.x, seed.c, seed.depth)
	}
	f.Fuzz(func(t *testing.T, shape uint8, x, c int16, depth uint8) {
		source, want := flattenPropertyProject(shape, int(x)%17, int(c)%17, int(depth%3))
		ids := NewIDGenerator()
		modules, err := parseModulesWithIDs(source, ids)
		if err != nil {
			t.Fatal("invalid generated project", source, err)
		}
		sorted, diagnostics := SortModules(modules)
		if len(diagnostics) != 0 {
			t.Fatal("generated module dependency order", source, diagnostics)
		}
		linked, err := ResolveLinkedModules(sorted)
		if err != nil || len(linked.Errors) != 0 {
			t.Fatal("generated name resolution", source, err, linked.Errors)
		}
		sorted, diagnostics = SortModuleDeclarations(sorted, linked.Table)
		if len(diagnostics) != 0 {
			t.Fatal("generated declaration dependency order", source, diagnostics)
		}
		analyzed, err := AnalyzeModules(sorted, linked.Table)
		if err != nil || len(analyzed.Diagnostics) != 0 {
			t.Fatal("generated analysis", source, err, analyzed.Diagnostics)
		}
		input := flattenPropertyJSON(t, analyzed.Modules)
		analysis := AnalysisOutput{Types: analyzed.Types, Effects: analyzed.Effects, Modes: analyzed.Modes}
		// Each call owns its working lookup/names. IDs intentionally advance;
		// start both passes at the same parser position to test determinism.
		leftIDs, rightIDs := NewIDGenerator(), NewIDGenerator()
		leftIDs.next.Set(&ids.next)
		rightIDs.next.Set(&ids.next)
		left, err := FlattenModules(analyzed.Modules, linked.Table, leftIDs, nil, analysis)
		if err != nil {
			t.Fatal("flattening", source, err)
		}
		if after := flattenPropertyJSON(t, analyzed.Modules); !bytes.Equal(input, after) {
			t.Fatal("flattening mutated input modules", source, string(input), string(after))
		}
		wire := flattenPropertyJSON(t, left.Modules)
		right, err := FlattenModules(analyzed.Modules, linked.Table, rightIDs, nil, analysis)
		if err != nil {
			t.Fatal("repeated flattening", source, err)
		}
		if !bytes.Equal(wire, flattenPropertyJSON(t, left.Modules)) || !bytes.Equal(input, flattenPropertyJSON(t, analyzed.Modules)) {
			t.Fatal("repeated flattening mutated an earlier public input or result", source)
		}
		if !bytes.Equal(wire, flattenPropertyJSON(t, right.Modules)) || leftIDs.next.Cmp(&rightIDs.next) != 0 {
			t.Fatal("flattening depends on prior pass", source)
		}
		// A retained runtimeSource graph could execute the original source and
		// conceal a broken public flattened tree. Evaluate only transported IR.
		var transported []Module
		if err := json.Unmarshal(wire, &transported); err != nil {
			t.Fatal("flattened public IR does not decode", source, err)
		}
		resolved, err := ResolveLinkedModules(transported)
		if err != nil || len(resolved.Errors) != 0 {
			t.Fatal("flattened public IR has unresolved references", source, err, resolved.Errors)
		}
		foundMain := false
		for _, module := range transported {
			for _, declaration := range module.Declarations {
				switch declaration.Kind {
				case "import", "export", "instance":
					t.Fatal("flattening retained a module operation", source, module.Name, declaration.Kind)
				}
			}
			if module.Name != "Main" {
				continue
			}
			foundMain = true
			got, err := EvaluatePureDefinition(context.Background(), module, "answer", nil)
			if err != nil || got.expression.Kind != "int" || got.expression.Int.String() != fmt.Sprint(want) {
				t.Fatalf("public flattened arithmetic: got %s / %v, want %d\n%s\n%s", got.String(), err, want, source, wire)
			}
		}
		if !foundMain {
			t.Fatal("flattening removed the main module", source)
		}
	})
}

func flattenPropertyJSON(t *testing.T, value any) []byte {
	t.Helper()
	wire, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return wire
}

func flattenPropertyProject(shape uint8, x, c, depth int) (string, int) {
	mode := shape % 4
	constant := fmt.Sprintf("(%d)", c)
	declarations := "type Number=int "
	if mode >= 2 {
		declarations += "const offset:int "
		constant = "offset"
	}
	declarations += fmt.Sprintf("pure def f0(x:Number):Number=List(x+%s).foldl(0,(x,y)=>x+y)*2 ", constant)
	for i := 1; i <= depth; i++ {
		declarations += fmt.Sprintf("pure def f%d(x:Number):Number=f%d(x)+%s ", i, i-1, constant)
	}
	modules := []string{"module Base { " + declarations + "}"}
	call := fmt.Sprintf("B::f%d(x)", depth)
	imports := "import Base as B "
	want := 2*(x+c) + depth*c
	switch mode {
	case 1:
		modules = append(modules, "module Bridge { import Base as B export B }")
		imports = "import Bridge.* "
	case 2:
		imports = fmt.Sprintf("import Base(offset=(%d)) as B ", c)
	case 3:
		imports = fmt.Sprintf("import Base(offset=(%d)) as L import Base(offset=(%d)) as R ", c, -c)
		call = fmt.Sprintf("L::f%d(x)+R::f%d(x+1)", depth, depth)
		want = 4*x + 2
	}
	compute := fmt.Sprintf("pure def compute(x:int):int=%s ", call)
	answer := fmt.Sprintf("pure val answer=compute((%d)) ", x)
	if shape&8 != 0 {
		compute, answer = answer, compute
	}
	modules = append(modules, "module Main { "+imports+compute+answer+"}")
	if shape&4 != 0 {
		for i, j := 0, len(modules)-1; i < j; i, j = i+1, j-1 {
			modules[i], modules[j] = modules[j], modules[i]
		}
	}
	return strings.Join(modules, "\n"), want
}
