package quint

import (
	"github.com/antlr4-go/antlr/v4"
	p "github.com/glycerine/quint-go/pkg/internal/effectparser"
)

// ParseEffect parses a complete effect signature, assigning state occurrence
// references from one in encounter order, independently for each parse.
func ParseEffect(text string) (Effect, error) {
	listener := newSyntaxErrors(text)
	lexer := p.NewEffectLexer(antlr.NewInputStream(text))
	lexer.RemoveErrorListeners()
	lexer.AddErrorListener(listener)
	tokens := antlr.NewCommonTokenStream(lexer, antlr.TokenDefaultChannel)
	parser := p.NewEffectParser(tokens)
	parser.RemoveErrorListeners()
	parser.AddErrorListener(listener)
	tree := parser.Effect()
	if tokens.LA(1) != antlr.TokenEOF {
		listener.trailing(tokens.LT(1))
	}
	if len(listener.issues) > 0 {
		return Effect{}, listener.err("effect")
	}
	ids := NewIDGenerator()
	entity := func(ctx p.IEntityContext) Entity {
		parts := []Entity{}
		for _, name := range ctx.AllIDENTIFIER() {
			parts = append(parts, Entity{Kind: "variable", Name: name.GetText()})
		}
		vars := []StateVariable{}
		for _, v := range ctx.AllStateVarRef() {
			vars = append(vars, StateVariable{Name: v.IDENTIFIER().GetText(), Reference: ids.Next()})
		}
		if len(vars) > 0 {
			parts = append(parts, Entity{Kind: "concrete", StateVariables: vars})
		}
		if len(parts) == 0 {
			return Entity{Kind: "concrete", StateVariables: []StateVariable{}}
		}
		if len(parts) == 1 {
			return parts[0]
		}
		return Entity{Kind: "union", Entities: parts}
	}
	var convert func(p.IEffectContext) Effect
	convert = func(ctx p.IEffectContext) Effect {
		switch c := ctx.(type) {
		case *p.VariableEffectContext:
			return Effect{Kind: "variable", Name: c.IDENTIFIER().GetText()}
		case *p.ArrowEffectContext:
			children := c.AllEffect()
			params := make([]Effect, len(children)-1)
			for i := range params {
				params[i] = convert(children[i])
			}
			result := convert(children[len(children)-1])
			return Effect{Kind: "arrow", Params: params, Result: &result}
		case *p.ConcreteEffectContext:
			components := []EffectComponent{}
			if concrete, ok := c.Concrete().(*p.ConcreteComponentsContext); ok {
				for _, component := range concrete.AllComponent() {
					switch v := component.(type) {
					case *p.ReadComponentContext:
						components = append(components, EffectComponent{Kind: "read", Entity: entity(v.Read().Entity())})
					case *p.UpdateComponentContext:
						components = append(components, EffectComponent{Kind: "update", Entity: entity(v.Update().Entity())})
					case *p.TemporalComponentContext:
						components = append(components, EffectComponent{Kind: "temporal", Entity: entity(v.Temporal().Entity())})
					}
				}
			}
			return Effect{Kind: "concrete", Components: components}
		}
		panic("unreachable effect parse node")
	}
	return convert(tree), nil
}
