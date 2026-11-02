package quint

import (
	"github.com/antlr4-go/antlr/v4"
	p "github.com/glycerine/quint-go/pkg/internal/quintparser"
)

// Live frames expose earlier operands while recursively converting later
// children. This models the listener's shared expression stack: a missing body
// may consume an operand belonging to an enclosing expression.
func recoveryOperandFrame(ids parserIDAllocator, values *[]Expr) func() {
	recovery, ok := ids.(*recoveringParserIDs)
	if !ok || recovery.syntaxCount == 0 {
		return func() {}
	}
	recovery.operandFrames = append(recovery.operandFrames, values)
	return func() { recovery.operandFrames = recovery.operandFrames[:len(recovery.operandFrames)-1] }
}

func recoveryOperandCount(ids *recoveringParserIDs) int {
	count := 0
	for _, frame := range ids.operandFrames {
		count += len(*frame)
	}
	return count
}

func recoveryTakeOperands(ids *recoveringParserIDs, count int, ctx antlr.ParserRuleContext) []Expr {
	defer parserScope(ids, ctx)()
	available := recoveryOperandCount(ids)
	frame := ids.operandFrames[len(ids.operandFrames)-1]
	for available < count {
		*frame = append(*frame, parserUndefinedExpression(ids))
		available++
	}
	result := make([]Expr, count)
	remaining := count
	for i := len(ids.operandFrames) - 1; i >= 0 && remaining > 0; i-- {
		values := ids.operandFrames[i]
		take := min(remaining, len(*values))
		start := len(*values) - take
		copy(result[remaining-take:remaining], (*values)[start:])
		*values = (*values)[:start]
		remaining -= take
	}
	return result
}

// Only consumer rules supply missing expressions. A generic recovered Expr
// node itself has no listener exit handler and pushes nothing.
func recoveryExpressionBody(ids parserIDAllocator, ctx antlr.ParserRuleContext, value Expr) Expr {
	if value.Kind != "" || !recoveringSyntax(ids) {
		return value
	}
	recovery := ids.(*recoveringParserIDs)
	if recoveryOperandCount(recovery) > 0 {
		return recoveryTakeOperands(recovery, 1, ctx)[0]
	}
	defer parserScope(ids, ctx)()
	return parserUndefinedExpression(ids)
}

func recoveryOperandReducerSupported(ctx p.IExprContext) bool {
	switch ctx.(type) {
	case *p.ParenContext, *p.BracesContext, *p.LetInContext, *p.LambdaConsContext,
		*p.PlusMinusContext, *p.MultDivContext, *p.PowContext, *p.RelationsContext,
		*p.ErrorEqContext, *p.AndContext, *p.OrContext, *p.IffContext,
		*p.ImpliesContext, *p.LeadsToContext, *p.UminusContext, *p.AsgnContext,
		*p.OperAppContext, *p.ListContext, *p.TupleContext, *p.PairContext,
		*p.AndExprContext, *p.OrExprContext, *p.ActionAllContext, *p.ActionAnyContext,
		*p.IfElseContext:
		return true
	}
	return false
}
