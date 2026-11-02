// Code generated from pkg/grammar/Quint.g4 by ANTLR 4.13.2. DO NOT EDIT.

package quintparser // Quint
import "github.com/antlr4-go/antlr/v4"

type BaseQuintVisitor struct {
	*antlr.BaseParseTreeVisitor
}

func (v *BaseQuintVisitor) VisitModules(ctx *ModulesContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseQuintVisitor) VisitModule(ctx *ModuleContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseQuintVisitor) VisitModuleItem(ctx *ModuleItemContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseQuintVisitor) VisitConst(ctx *ConstContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseQuintVisitor) VisitVar(ctx *VarContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseQuintVisitor) VisitAssume(ctx *AssumeContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseQuintVisitor) VisitInstance(ctx *InstanceContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseQuintVisitor) VisitOper(ctx *OperContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseQuintVisitor) VisitTypeDefs(ctx *TypeDefsContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseQuintVisitor) VisitImportDef(ctx *ImportDefContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseQuintVisitor) VisitExportDef(ctx *ExportDefContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseQuintVisitor) VisitAnnotatedOperDef(ctx *AnnotatedOperDefContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseQuintVisitor) VisitDeprecatedOperDef(ctx *DeprecatedOperDefContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseQuintVisitor) VisitValDestructuring(ctx *ValDestructuringContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseQuintVisitor) VisitPureValDestructuring(ctx *PureValDestructuringContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseQuintVisitor) VisitTypeAbstractDef(ctx *TypeAbstractDefContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseQuintVisitor) VisitTypeSumDef(ctx *TypeSumDefContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseQuintVisitor) VisitTypeAliasDef(ctx *TypeAliasDefContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseQuintVisitor) VisitTypeDefHead(ctx *TypeDefHeadContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseQuintVisitor) VisitSumTypeDefinition(ctx *SumTypeDefinitionContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseQuintVisitor) VisitTypeSumVariant(ctx *TypeSumVariantContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseQuintVisitor) VisitQualifier(ctx *QualifierContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseQuintVisitor) VisitImportMod(ctx *ImportModContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseQuintVisitor) VisitExportMod(ctx *ExportModContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseQuintVisitor) VisitInstanceMod(ctx *InstanceModContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseQuintVisitor) VisitModuleName(ctx *ModuleNameContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseQuintVisitor) VisitName(ctx *NameContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseQuintVisitor) VisitQualifiedName(ctx *QualifiedNameContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseQuintVisitor) VisitFromSource(ctx *FromSourceContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseQuintVisitor) VisitTypeConst(ctx *TypeConstContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseQuintVisitor) VisitTypeApp(ctx *TypeAppContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseQuintVisitor) VisitTypeFun(ctx *TypeFunContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseQuintVisitor) VisitTypeOper(ctx *TypeOperContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseQuintVisitor) VisitTypeParen(ctx *TypeParenContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseQuintVisitor) VisitTypeBool(ctx *TypeBoolContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseQuintVisitor) VisitTypeSet(ctx *TypeSetContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseQuintVisitor) VisitTypeUnit(ctx *TypeUnitContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseQuintVisitor) VisitTypeRec(ctx *TypeRecContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseQuintVisitor) VisitTypeStr(ctx *TypeStrContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseQuintVisitor) VisitTypeInt(ctx *TypeIntContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseQuintVisitor) VisitTypeList(ctx *TypeListContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseQuintVisitor) VisitTypeTuple(ctx *TypeTupleContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseQuintVisitor) VisitWrongTypeApp(ctx *WrongTypeAppContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseQuintVisitor) VisitTypeVarCase(ctx *TypeVarCaseContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseQuintVisitor) VisitTypeVar(ctx *TypeVarContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseQuintVisitor) VisitRow(ctx *RowContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseQuintVisitor) VisitRowLabel(ctx *RowLabelContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseQuintVisitor) VisitTypeArgs(ctx *TypeArgsContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseQuintVisitor) VisitTypeApplication(ctx *TypeApplicationContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseQuintVisitor) VisitWrongTypeApplication(ctx *WrongTypeApplicationContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseQuintVisitor) VisitLetIn(ctx *LetInContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseQuintVisitor) VisitUminus(ctx *UminusContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseQuintVisitor) VisitOperApp(ctx *OperAppContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseQuintVisitor) VisitActionAny(ctx *ActionAnyContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseQuintVisitor) VisitErrorEq(ctx *ErrorEqContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseQuintVisitor) VisitBraces(ctx *BracesContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseQuintVisitor) VisitLambdaCons(ctx *LambdaConsContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseQuintVisitor) VisitTuple(ctx *TupleContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseQuintVisitor) VisitParen(ctx *ParenContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseQuintVisitor) VisitMultDiv(ctx *MultDivContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseQuintVisitor) VisitAnd(ctx *AndContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseQuintVisitor) VisitRecord(ctx *RecordContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseQuintVisitor) VisitPow(ctx *PowContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseQuintVisitor) VisitActionAll(ctx *ActionAllContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseQuintVisitor) VisitDotCall(ctx *DotCallContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseQuintVisitor) VisitIfElse(ctx *IfElseContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseQuintVisitor) VisitPlusMinus(ctx *PlusMinusContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseQuintVisitor) VisitOr(ctx *OrContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseQuintVisitor) VisitMatch(ctx *MatchContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseQuintVisitor) VisitIff(ctx *IffContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseQuintVisitor) VisitOrExpr(ctx *OrExprContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseQuintVisitor) VisitList(ctx *ListContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseQuintVisitor) VisitPair(ctx *PairContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseQuintVisitor) VisitUnit(ctx *UnitContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseQuintVisitor) VisitLeadsTo(ctx *LeadsToContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseQuintVisitor) VisitAsgn(ctx *AsgnContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseQuintVisitor) VisitLiteralOrId(ctx *LiteralOrIdContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseQuintVisitor) VisitListApp(ctx *ListAppContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseQuintVisitor) VisitRelations(ctx *RelationsContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseQuintVisitor) VisitImplies(ctx *ImpliesContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseQuintVisitor) VisitAndExpr(ctx *AndExprContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseQuintVisitor) VisitMatchSumExpr(ctx *MatchSumExprContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseQuintVisitor) VisitMatchSumCase(ctx *MatchSumCaseContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseQuintVisitor) VisitMatchSumVariant(ctx *MatchSumVariantContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseQuintVisitor) VisitDeclarationOrExpr(ctx *DeclarationOrExprContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseQuintVisitor) VisitLambda(ctx *LambdaContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseQuintVisitor) VisitLambdaUnsugared(ctx *LambdaUnsugaredContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseQuintVisitor) VisitLambdaTupleSugar(ctx *LambdaTupleSugarContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseQuintVisitor) VisitIdentOrHole(ctx *IdentOrHoleContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseQuintVisitor) VisitParameter(ctx *ParameterContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseQuintVisitor) VisitAnnotatedParameter(ctx *AnnotatedParameterContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseQuintVisitor) VisitDestructuringPattern(ctx *DestructuringPatternContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseQuintVisitor) VisitTuplePattern(ctx *TuplePatternContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseQuintVisitor) VisitRecordPattern(ctx *RecordPatternContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseQuintVisitor) VisitIdentOrStar(ctx *IdentOrStarContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseQuintVisitor) VisitArgList(ctx *ArgListContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseQuintVisitor) VisitRecElem(ctx *RecElemContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseQuintVisitor) VisitNormalCallName(ctx *NormalCallNameContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseQuintVisitor) VisitNameAfterDot(ctx *NameAfterDotContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseQuintVisitor) VisitOperator(ctx *OperatorContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseQuintVisitor) VisitLiteral(ctx *LiteralContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseQuintVisitor) VisitQualId(ctx *QualIdContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseQuintVisitor) VisitSimpleId(ctx *SimpleIdContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseQuintVisitor) VisitIdentifier(ctx *IdentifierContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseQuintVisitor) VisitKeywordAsID(ctx *KeywordAsIDContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseQuintVisitor) VisitReserved(ctx *ReservedContext) interface{} {
	return v.VisitChildren(ctx)
}
