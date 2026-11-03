package quint

import (
	"fmt"
	"testing"
)

func BenchmarkEffectSubstitutionPreparation(b *testing.B) {
	for _, kind := range []string{"variables", "identified", "bare", "arrow"} {
		for _, count := range []int{1, 16, 64} {
			subs := make([]EffectSubstitution, count)
			for i := range subs {
				state := StateVariable{Name: "x", Reference: effectPreparationID(fmt.Sprint(i))}
				if kind != "bare" {
					state = NewStateVariable(state.Name, state.Reference)
				}
				entity := Entity{Kind: "concrete", StateVariables: []StateVariable{state}}
				if kind == "variables" {
					entity = Entity{Kind: "variable", Name: fmt.Sprintf("unbound%d", i)}
				}
				subs[i] = EffectSubstitution{Kind: "entity", Name: fmt.Sprintf("r%d", i), Entity: entity}
				if kind == "arrow" {
					value := effectPreparationConcrete(entity, "read")
					subs[i] = EffectSubstitution{Kind: "effect", Name: fmt.Sprintf("e%d", i), Effect: effectPreparationArrow(value, value)}
				}
			}
			input := effectPreparationConcrete(Entity{Kind: "variable", Name: "r0"}, "read")
			if kind == "arrow" {
				input = Effect{Kind: "variable", Name: "e0"}
			}
			for _, optimized := range []bool{false, true} {
				b.Run(fmt.Sprintf("%s/%d/guard=%v", kind, count, optimized), func(b *testing.B) {
					apply := effectSubstitutionPreparedOracle
					if optimized {
						apply = ApplyEffectSubstitution
					}
					b.ReportAllocs()
					for b.Loop() {
						if _, err := apply(subs, input); err != nil {
							b.Fatal(err)
						}
					}
				})
			}
		}
	}
}
