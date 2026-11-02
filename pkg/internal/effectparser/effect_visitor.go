// Code generated from quint/src/generated/Effect.g4 by ANTLR 4.13.2. DO NOT EDIT.

package effectparser // Effect
import "github.com/antlr4-go/antlr/v4"

// A complete Visitor for a parse tree produced by EffectParser.
type EffectVisitor interface {
	antlr.ParseTreeVisitor

	// Visit a parse tree produced by EffectParser#concreteEffect.
	VisitConcreteEffect(ctx *ConcreteEffectContext) interface{}

	// Visit a parse tree produced by EffectParser#arrowEffect.
	VisitArrowEffect(ctx *ArrowEffectContext) interface{}

	// Visit a parse tree produced by EffectParser#variableEffect.
	VisitVariableEffect(ctx *VariableEffectContext) interface{}

	// Visit a parse tree produced by EffectParser#read.
	VisitRead(ctx *ReadContext) interface{}

	// Visit a parse tree produced by EffectParser#update.
	VisitUpdate(ctx *UpdateContext) interface{}

	// Visit a parse tree produced by EffectParser#temporal.
	VisitTemporal(ctx *TemporalContext) interface{}

	// Visit a parse tree produced by EffectParser#concreteComponents.
	VisitConcreteComponents(ctx *ConcreteComponentsContext) interface{}

	// Visit a parse tree produced by EffectParser#pure.
	VisitPure(ctx *PureContext) interface{}

	// Visit a parse tree produced by EffectParser#readComponent.
	VisitReadComponent(ctx *ReadComponentContext) interface{}

	// Visit a parse tree produced by EffectParser#updateComponent.
	VisitUpdateComponent(ctx *UpdateComponentContext) interface{}

	// Visit a parse tree produced by EffectParser#temporalComponent.
	VisitTemporalComponent(ctx *TemporalComponentContext) interface{}

	// Visit a parse tree produced by EffectParser#entity.
	VisitEntity(ctx *EntityContext) interface{}

	// Visit a parse tree produced by EffectParser#stateVarRef.
	VisitStateVarRef(ctx *StateVarRefContext) interface{}
}
