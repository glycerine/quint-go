package quint

import (
	"encoding/json"
	"os"
	"testing"

	p "github.com/glycerine/quint-go/pkg/internal/quintparser"
)

func BenchmarkModulePrediction(b *testing.B) {
	var corpus []struct{ Name, Source string }
	data, err := os.ReadFile("testdata/parser/corpus.json")
	if err != nil {
		b.Fatal(err)
	}
	if err = json.Unmarshal(data, &corpus); err != nil {
		b.Fatal(err)
	}
	cases := []struct{ Name, Source string }{
		{"small", "module M { val x=1 }"},
		{"syntax-fallback", "module M { val x=1+ }"},
	}
	for _, fixture := range corpus {
		if fixture.Name == "SuperSpec.qnt" || fixture.Name == "nestedParameterizedCallsWithLet.qnt" {
			cases = append(cases, fixture)
		}
	}
	for _, fixture := range cases {
		b.Run(fixture.Name, func(b *testing.B) {
			for _, fast := range []bool{false, true} {
				name := "LL"
				if fast {
					name = "SLL"
				}
				b.Run(name, func(b *testing.B) {
					parse := func() {
						listener := newSyntaxErrors(fixture.Source)
						var tree p.IModulesContext
						if fast {
							tree = parseModuleTree(fixture.Source, listener)
						} else {
							tree = moduleTreeLL(fixture.Source, listener)
						}
						modulesFromContext(tree, listener, NewIDGenerator())
					}
					parse() // Warm the shared DFA cache before measurement.
					b.ReportAllocs()
					for b.Loop() {
						parse()
					}
				})
			}
		})
	}
}
