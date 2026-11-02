// Code generated from quint/src/generated/Effect.g4 by ANTLR 4.13.2. DO NOT EDIT.

package effectparser // Effect
import "github.com/antlr4-go/antlr/v4"

// EffectListener is a complete listener for a parse tree produced by EffectParser.
type EffectListener interface {
	antlr.ParseTreeListener

	// EnterConcreteEffect is called when entering the concreteEffect production.
	EnterConcreteEffect(c *ConcreteEffectContext)

	// EnterArrowEffect is called when entering the arrowEffect production.
	EnterArrowEffect(c *ArrowEffectContext)

	// EnterVariableEffect is called when entering the variableEffect production.
	EnterVariableEffect(c *VariableEffectContext)

	// EnterRead is called when entering the read production.
	EnterRead(c *ReadContext)

	// EnterUpdate is called when entering the update production.
	EnterUpdate(c *UpdateContext)

	// EnterTemporal is called when entering the temporal production.
	EnterTemporal(c *TemporalContext)

	// EnterConcreteComponents is called when entering the concreteComponents production.
	EnterConcreteComponents(c *ConcreteComponentsContext)

	// EnterPure is called when entering the pure production.
	EnterPure(c *PureContext)

	// EnterReadComponent is called when entering the readComponent production.
	EnterReadComponent(c *ReadComponentContext)

	// EnterUpdateComponent is called when entering the updateComponent production.
	EnterUpdateComponent(c *UpdateComponentContext)

	// EnterTemporalComponent is called when entering the temporalComponent production.
	EnterTemporalComponent(c *TemporalComponentContext)

	// EnterEntity is called when entering the entity production.
	EnterEntity(c *EntityContext)

	// EnterStateVarRef is called when entering the stateVarRef production.
	EnterStateVarRef(c *StateVarRefContext)

	// ExitConcreteEffect is called when exiting the concreteEffect production.
	ExitConcreteEffect(c *ConcreteEffectContext)

	// ExitArrowEffect is called when exiting the arrowEffect production.
	ExitArrowEffect(c *ArrowEffectContext)

	// ExitVariableEffect is called when exiting the variableEffect production.
	ExitVariableEffect(c *VariableEffectContext)

	// ExitRead is called when exiting the read production.
	ExitRead(c *ReadContext)

	// ExitUpdate is called when exiting the update production.
	ExitUpdate(c *UpdateContext)

	// ExitTemporal is called when exiting the temporal production.
	ExitTemporal(c *TemporalContext)

	// ExitConcreteComponents is called when exiting the concreteComponents production.
	ExitConcreteComponents(c *ConcreteComponentsContext)

	// ExitPure is called when exiting the pure production.
	ExitPure(c *PureContext)

	// ExitReadComponent is called when exiting the readComponent production.
	ExitReadComponent(c *ReadComponentContext)

	// ExitUpdateComponent is called when exiting the updateComponent production.
	ExitUpdateComponent(c *UpdateComponentContext)

	// ExitTemporalComponent is called when exiting the temporalComponent production.
	ExitTemporalComponent(c *TemporalComponentContext)

	// ExitEntity is called when exiting the entity production.
	ExitEntity(c *EntityContext)

	// ExitStateVarRef is called when exiting the stateVarRef production.
	ExitStateVarRef(c *StateVarRefContext)
}
