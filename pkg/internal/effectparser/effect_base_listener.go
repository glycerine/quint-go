// Code generated from quint/src/generated/Effect.g4 by ANTLR 4.13.2. DO NOT EDIT.

package effectparser // Effect
import "github.com/antlr4-go/antlr/v4"

// BaseEffectListener is a complete listener for a parse tree produced by EffectParser.
type BaseEffectListener struct{}

var _ EffectListener = &BaseEffectListener{}

// VisitTerminal is called when a terminal node is visited.
func (s *BaseEffectListener) VisitTerminal(node antlr.TerminalNode) {}

// VisitErrorNode is called when an error node is visited.
func (s *BaseEffectListener) VisitErrorNode(node antlr.ErrorNode) {}

// EnterEveryRule is called when any rule is entered.
func (s *BaseEffectListener) EnterEveryRule(ctx antlr.ParserRuleContext) {}

// ExitEveryRule is called when any rule is exited.
func (s *BaseEffectListener) ExitEveryRule(ctx antlr.ParserRuleContext) {}

// EnterConcreteEffect is called when production concreteEffect is entered.
func (s *BaseEffectListener) EnterConcreteEffect(ctx *ConcreteEffectContext) {}

// ExitConcreteEffect is called when production concreteEffect is exited.
func (s *BaseEffectListener) ExitConcreteEffect(ctx *ConcreteEffectContext) {}

// EnterArrowEffect is called when production arrowEffect is entered.
func (s *BaseEffectListener) EnterArrowEffect(ctx *ArrowEffectContext) {}

// ExitArrowEffect is called when production arrowEffect is exited.
func (s *BaseEffectListener) ExitArrowEffect(ctx *ArrowEffectContext) {}

// EnterVariableEffect is called when production variableEffect is entered.
func (s *BaseEffectListener) EnterVariableEffect(ctx *VariableEffectContext) {}

// ExitVariableEffect is called when production variableEffect is exited.
func (s *BaseEffectListener) ExitVariableEffect(ctx *VariableEffectContext) {}

// EnterRead is called when production read is entered.
func (s *BaseEffectListener) EnterRead(ctx *ReadContext) {}

// ExitRead is called when production read is exited.
func (s *BaseEffectListener) ExitRead(ctx *ReadContext) {}

// EnterUpdate is called when production update is entered.
func (s *BaseEffectListener) EnterUpdate(ctx *UpdateContext) {}

// ExitUpdate is called when production update is exited.
func (s *BaseEffectListener) ExitUpdate(ctx *UpdateContext) {}

// EnterTemporal is called when production temporal is entered.
func (s *BaseEffectListener) EnterTemporal(ctx *TemporalContext) {}

// ExitTemporal is called when production temporal is exited.
func (s *BaseEffectListener) ExitTemporal(ctx *TemporalContext) {}

// EnterConcreteComponents is called when production concreteComponents is entered.
func (s *BaseEffectListener) EnterConcreteComponents(ctx *ConcreteComponentsContext) {}

// ExitConcreteComponents is called when production concreteComponents is exited.
func (s *BaseEffectListener) ExitConcreteComponents(ctx *ConcreteComponentsContext) {}

// EnterPure is called when production pure is entered.
func (s *BaseEffectListener) EnterPure(ctx *PureContext) {}

// ExitPure is called when production pure is exited.
func (s *BaseEffectListener) ExitPure(ctx *PureContext) {}

// EnterReadComponent is called when production readComponent is entered.
func (s *BaseEffectListener) EnterReadComponent(ctx *ReadComponentContext) {}

// ExitReadComponent is called when production readComponent is exited.
func (s *BaseEffectListener) ExitReadComponent(ctx *ReadComponentContext) {}

// EnterUpdateComponent is called when production updateComponent is entered.
func (s *BaseEffectListener) EnterUpdateComponent(ctx *UpdateComponentContext) {}

// ExitUpdateComponent is called when production updateComponent is exited.
func (s *BaseEffectListener) ExitUpdateComponent(ctx *UpdateComponentContext) {}

// EnterTemporalComponent is called when production temporalComponent is entered.
func (s *BaseEffectListener) EnterTemporalComponent(ctx *TemporalComponentContext) {}

// ExitTemporalComponent is called when production temporalComponent is exited.
func (s *BaseEffectListener) ExitTemporalComponent(ctx *TemporalComponentContext) {}

// EnterEntity is called when production entity is entered.
func (s *BaseEffectListener) EnterEntity(ctx *EntityContext) {}

// ExitEntity is called when production entity is exited.
func (s *BaseEffectListener) ExitEntity(ctx *EntityContext) {}

// EnterStateVarRef is called when production stateVarRef is entered.
func (s *BaseEffectListener) EnterStateVarRef(ctx *StateVarRefContext) {}

// ExitStateVarRef is called when production stateVarRef is exited.
func (s *BaseEffectListener) ExitStateVarRef(ctx *StateVarRefContext) {}
