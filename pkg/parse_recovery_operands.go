package quint

import (
	"strings"

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
	case *p.RecordContext, *p.ParenContext, *p.BracesContext, *p.LetInContext, *p.LambdaConsContext,
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

// The listener keeps temporary field/spread tuples on the shared expression
// stack until the record exits. Missing later bodies can consume those tuples;
// their zero IDs are intentional upstream temporaries, not allocated IR IDs.
func recoveryRecordFromContext(ctx *p.RecordContext, ids *recoveringParserIDs) (Expr, error) {
	elems := []Expr{}
	defer recoveryOperandFrame(ids, &elems)()
	for _, elem := range ctx.AllRecElem() {
		value, err := expressionFromContext(elem.Expr(), ids)
		if err != nil {
			return Expr{}, err
		}
		if value.Kind == "" && recoveryOperandCount(ids) > 0 {
			value = recoveryTakeOperands(ids, 1, elem)[0]
		}
		args := []Expr{value}
		if elem.SimpleId() != nil {
			field := Expr{ID: parserIDAt(ids, elem), Kind: "str", Str: elem.SimpleId().GetText()}
			args = append([]Expr{field}, args...)
		}
		if value.Kind == "" {
			// A named field's bare stack pop may be undefined upstream.
			// Only the isolated raw replay may retain that JSON null; it
			// never enters the typed recovery result or normal serializers.
			if ids.rawNullOperands && elem.SimpleId() != nil {
				args[len(args)-1] = Expr{Kind: rawRecoveryNullKind}
			} else {
				return Expr{}, &nullRecordOperandError{message: unexpectedRecoveryNode(elem).Error()}
			}
		}
		elems = append(elems, Expr{Kind: "app", Opcode: "Tup", Args: args})
	}
	elems = recoveryTakeOperands(ids, len(ctx.AllRecElem()), ctx)
	pairs, spreads := []Expr{}, []Expr{}
	seen, duplicateSet := map[string]bool{}, map[string]bool{}
	duplicates := []string{}
	for _, elem := range elems {
		if elem.Kind != "app" {
			continue
		}
		switch len(elem.Args) {
		case 1:
			spreads = append(spreads, elem.Args[0])
		case 2:
			pairs = append(pairs, elem.Args...)
			field := elem.Args[0]
			if field.Kind == "str" {
				if seen[field.Str] && !duplicateSet[field.Str] {
					duplicates = append(duplicates, field.Str)
					duplicateSet[field.Str] = true
				}
				seen[field.Str] = true
			}
		}
	}
	if len(duplicates) > 0 {
		plural := "Field"
		if len(duplicates) > 1 {
			plural = "Fields"
		}
		id := ids.Next()
		reportParserDiagnostic(ids, Diagnostic{Code: QNT016, Message: plural + " " + strings.Join(duplicates, ", ") + " cannot be defined more than once", Reference: &id, Data: &DiagnosticData{}})
	}
	if len(spreads) == 0 {
		return Expr{ID: ids.Next(), Kind: "app", Opcode: "Rec", Args: pairs}, nil
	}
	if len(spreads) > 1 {
		id := ids.Next()
		reportParserDiagnostic(ids, Diagnostic{Code: QNT012, Message: "... may be used once in { ...record, <fields> }", Reference: &id, Data: &DiagnosticData{}})
		return Expr{}, nil
	}
	record := spreads[0]
	for i := 0; i < len(pairs); i += 2 {
		record = Expr{ID: ids.Next(), Kind: "app", Opcode: "with", Args: []Expr{record, pairs[i], pairs[i+1]}}
	}
	return record, nil
}
