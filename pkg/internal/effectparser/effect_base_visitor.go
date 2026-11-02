// Code generated from quint/src/generated/Effect.g4 by ANTLR 4.13.2. DO NOT EDIT.

package effectparser // Effect
import "github.com/antlr4-go/antlr/v4"

type BaseEffectVisitor struct {
	*antlr.BaseParseTreeVisitor
}

func (v *BaseEffectVisitor) VisitConcreteEffect(ctx *ConcreteEffectContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseEffectVisitor) VisitArrowEffect(ctx *ArrowEffectContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseEffectVisitor) VisitVariableEffect(ctx *VariableEffectContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseEffectVisitor) VisitRead(ctx *ReadContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseEffectVisitor) VisitUpdate(ctx *UpdateContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseEffectVisitor) VisitTemporal(ctx *TemporalContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseEffectVisitor) VisitConcreteComponents(ctx *ConcreteComponentsContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseEffectVisitor) VisitPure(ctx *PureContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseEffectVisitor) VisitReadComponent(ctx *ReadComponentContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseEffectVisitor) VisitUpdateComponent(ctx *UpdateComponentContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseEffectVisitor) VisitTemporalComponent(ctx *TemporalComponentContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseEffectVisitor) VisitEntity(ctx *EntityContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseEffectVisitor) VisitStateVarRef(ctx *StateVarRefContext) interface{} {
	return v.VisitChildren(ctx)
}
