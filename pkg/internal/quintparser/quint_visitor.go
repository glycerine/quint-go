// Code generated from pkg/grammar/Quint.g4 by ANTLR 4.13.2. DO NOT EDIT.

package quintparser // Quint
import "github.com/antlr4-go/antlr/v4"

// A complete Visitor for a parse tree produced by QuintParser.
type QuintVisitor interface {
	antlr.ParseTreeVisitor

	// Visit a parse tree produced by QuintParser#modules.
	VisitModules(ctx *ModulesContext) interface{}

	// Visit a parse tree produced by QuintParser#module.
	VisitModule(ctx *ModuleContext) interface{}

	// Visit a parse tree produced by QuintParser#moduleItem.
	VisitModuleItem(ctx *ModuleItemContext) interface{}

	// Visit a parse tree produced by QuintParser#const.
	VisitConst(ctx *ConstContext) interface{}

	// Visit a parse tree produced by QuintParser#var.
	VisitVar(ctx *VarContext) interface{}

	// Visit a parse tree produced by QuintParser#assume.
	VisitAssume(ctx *AssumeContext) interface{}

	// Visit a parse tree produced by QuintParser#instance.
	VisitInstance(ctx *InstanceContext) interface{}

	// Visit a parse tree produced by QuintParser#oper.
	VisitOper(ctx *OperContext) interface{}

	// Visit a parse tree produced by QuintParser#typeDefs.
	VisitTypeDefs(ctx *TypeDefsContext) interface{}

	// Visit a parse tree produced by QuintParser#importDef.
	VisitImportDef(ctx *ImportDefContext) interface{}

	// Visit a parse tree produced by QuintParser#exportDef.
	VisitExportDef(ctx *ExportDefContext) interface{}

	// Visit a parse tree produced by QuintParser#annotatedOperDef.
	VisitAnnotatedOperDef(ctx *AnnotatedOperDefContext) interface{}

	// Visit a parse tree produced by QuintParser#deprecatedOperDef.
	VisitDeprecatedOperDef(ctx *DeprecatedOperDefContext) interface{}

	// Visit a parse tree produced by QuintParser#valDestructuring.
	VisitValDestructuring(ctx *ValDestructuringContext) interface{}

	// Visit a parse tree produced by QuintParser#pureValDestructuring.
	VisitPureValDestructuring(ctx *PureValDestructuringContext) interface{}

	// Visit a parse tree produced by QuintParser#typeAbstractDef.
	VisitTypeAbstractDef(ctx *TypeAbstractDefContext) interface{}

	// Visit a parse tree produced by QuintParser#typeSumDef.
	VisitTypeSumDef(ctx *TypeSumDefContext) interface{}

	// Visit a parse tree produced by QuintParser#typeAliasDef.
	VisitTypeAliasDef(ctx *TypeAliasDefContext) interface{}

	// Visit a parse tree produced by QuintParser#typeDefHead.
	VisitTypeDefHead(ctx *TypeDefHeadContext) interface{}

	// Visit a parse tree produced by QuintParser#sumTypeDefinition.
	VisitSumTypeDefinition(ctx *SumTypeDefinitionContext) interface{}

	// Visit a parse tree produced by QuintParser#typeSumVariant.
	VisitTypeSumVariant(ctx *TypeSumVariantContext) interface{}

	// Visit a parse tree produced by QuintParser#qualifier.
	VisitQualifier(ctx *QualifierContext) interface{}

	// Visit a parse tree produced by QuintParser#importMod.
	VisitImportMod(ctx *ImportModContext) interface{}

	// Visit a parse tree produced by QuintParser#exportMod.
	VisitExportMod(ctx *ExportModContext) interface{}

	// Visit a parse tree produced by QuintParser#instanceMod.
	VisitInstanceMod(ctx *InstanceModContext) interface{}

	// Visit a parse tree produced by QuintParser#moduleName.
	VisitModuleName(ctx *ModuleNameContext) interface{}

	// Visit a parse tree produced by QuintParser#name.
	VisitName(ctx *NameContext) interface{}

	// Visit a parse tree produced by QuintParser#qualifiedName.
	VisitQualifiedName(ctx *QualifiedNameContext) interface{}

	// Visit a parse tree produced by QuintParser#fromSource.
	VisitFromSource(ctx *FromSourceContext) interface{}

	// Visit a parse tree produced by QuintParser#typeConst.
	VisitTypeConst(ctx *TypeConstContext) interface{}

	// Visit a parse tree produced by QuintParser#typeApp.
	VisitTypeApp(ctx *TypeAppContext) interface{}

	// Visit a parse tree produced by QuintParser#typeFun.
	VisitTypeFun(ctx *TypeFunContext) interface{}

	// Visit a parse tree produced by QuintParser#typeOper.
	VisitTypeOper(ctx *TypeOperContext) interface{}

	// Visit a parse tree produced by QuintParser#typeParen.
	VisitTypeParen(ctx *TypeParenContext) interface{}

	// Visit a parse tree produced by QuintParser#typeBool.
	VisitTypeBool(ctx *TypeBoolContext) interface{}

	// Visit a parse tree produced by QuintParser#typeSet.
	VisitTypeSet(ctx *TypeSetContext) interface{}

	// Visit a parse tree produced by QuintParser#typeUnit.
	VisitTypeUnit(ctx *TypeUnitContext) interface{}

	// Visit a parse tree produced by QuintParser#typeRec.
	VisitTypeRec(ctx *TypeRecContext) interface{}

	// Visit a parse tree produced by QuintParser#typeStr.
	VisitTypeStr(ctx *TypeStrContext) interface{}

	// Visit a parse tree produced by QuintParser#typeInt.
	VisitTypeInt(ctx *TypeIntContext) interface{}

	// Visit a parse tree produced by QuintParser#typeList.
	VisitTypeList(ctx *TypeListContext) interface{}

	// Visit a parse tree produced by QuintParser#typeTuple.
	VisitTypeTuple(ctx *TypeTupleContext) interface{}

	// Visit a parse tree produced by QuintParser#wrongTypeApp.
	VisitWrongTypeApp(ctx *WrongTypeAppContext) interface{}

	// Visit a parse tree produced by QuintParser#typeVarCase.
	VisitTypeVarCase(ctx *TypeVarCaseContext) interface{}

	// Visit a parse tree produced by QuintParser#typeVar.
	VisitTypeVar(ctx *TypeVarContext) interface{}

	// Visit a parse tree produced by QuintParser#row.
	VisitRow(ctx *RowContext) interface{}

	// Visit a parse tree produced by QuintParser#rowLabel.
	VisitRowLabel(ctx *RowLabelContext) interface{}

	// Visit a parse tree produced by QuintParser#typeArgs.
	VisitTypeArgs(ctx *TypeArgsContext) interface{}

	// Visit a parse tree produced by QuintParser#typeApplication.
	VisitTypeApplication(ctx *TypeApplicationContext) interface{}

	// Visit a parse tree produced by QuintParser#wrongTypeApplication.
	VisitWrongTypeApplication(ctx *WrongTypeApplicationContext) interface{}

	// Visit a parse tree produced by QuintParser#letIn.
	VisitLetIn(ctx *LetInContext) interface{}

	// Visit a parse tree produced by QuintParser#uminus.
	VisitUminus(ctx *UminusContext) interface{}

	// Visit a parse tree produced by QuintParser#operApp.
	VisitOperApp(ctx *OperAppContext) interface{}

	// Visit a parse tree produced by QuintParser#actionAny.
	VisitActionAny(ctx *ActionAnyContext) interface{}

	// Visit a parse tree produced by QuintParser#errorEq.
	VisitErrorEq(ctx *ErrorEqContext) interface{}

	// Visit a parse tree produced by QuintParser#braces.
	VisitBraces(ctx *BracesContext) interface{}

	// Visit a parse tree produced by QuintParser#lambdaCons.
	VisitLambdaCons(ctx *LambdaConsContext) interface{}

	// Visit a parse tree produced by QuintParser#tuple.
	VisitTuple(ctx *TupleContext) interface{}

	// Visit a parse tree produced by QuintParser#paren.
	VisitParen(ctx *ParenContext) interface{}

	// Visit a parse tree produced by QuintParser#multDiv.
	VisitMultDiv(ctx *MultDivContext) interface{}

	// Visit a parse tree produced by QuintParser#and.
	VisitAnd(ctx *AndContext) interface{}

	// Visit a parse tree produced by QuintParser#record.
	VisitRecord(ctx *RecordContext) interface{}

	// Visit a parse tree produced by QuintParser#pow.
	VisitPow(ctx *PowContext) interface{}

	// Visit a parse tree produced by QuintParser#actionAll.
	VisitActionAll(ctx *ActionAllContext) interface{}

	// Visit a parse tree produced by QuintParser#dotCall.
	VisitDotCall(ctx *DotCallContext) interface{}

	// Visit a parse tree produced by QuintParser#ifElse.
	VisitIfElse(ctx *IfElseContext) interface{}

	// Visit a parse tree produced by QuintParser#plusMinus.
	VisitPlusMinus(ctx *PlusMinusContext) interface{}

	// Visit a parse tree produced by QuintParser#or.
	VisitOr(ctx *OrContext) interface{}

	// Visit a parse tree produced by QuintParser#match.
	VisitMatch(ctx *MatchContext) interface{}

	// Visit a parse tree produced by QuintParser#iff.
	VisitIff(ctx *IffContext) interface{}

	// Visit a parse tree produced by QuintParser#orExpr.
	VisitOrExpr(ctx *OrExprContext) interface{}

	// Visit a parse tree produced by QuintParser#list.
	VisitList(ctx *ListContext) interface{}

	// Visit a parse tree produced by QuintParser#pair.
	VisitPair(ctx *PairContext) interface{}

	// Visit a parse tree produced by QuintParser#unit.
	VisitUnit(ctx *UnitContext) interface{}

	// Visit a parse tree produced by QuintParser#leadsTo.
	VisitLeadsTo(ctx *LeadsToContext) interface{}

	// Visit a parse tree produced by QuintParser#asgn.
	VisitAsgn(ctx *AsgnContext) interface{}

	// Visit a parse tree produced by QuintParser#literalOrId.
	VisitLiteralOrId(ctx *LiteralOrIdContext) interface{}

	// Visit a parse tree produced by QuintParser#listApp.
	VisitListApp(ctx *ListAppContext) interface{}

	// Visit a parse tree produced by QuintParser#relations.
	VisitRelations(ctx *RelationsContext) interface{}

	// Visit a parse tree produced by QuintParser#implies.
	VisitImplies(ctx *ImpliesContext) interface{}

	// Visit a parse tree produced by QuintParser#andExpr.
	VisitAndExpr(ctx *AndExprContext) interface{}

	// Visit a parse tree produced by QuintParser#matchSumExpr.
	VisitMatchSumExpr(ctx *MatchSumExprContext) interface{}

	// Visit a parse tree produced by QuintParser#matchSumCase.
	VisitMatchSumCase(ctx *MatchSumCaseContext) interface{}

	// Visit a parse tree produced by QuintParser#matchSumVariant.
	VisitMatchSumVariant(ctx *MatchSumVariantContext) interface{}

	// Visit a parse tree produced by QuintParser#declarationOrExpr.
	VisitDeclarationOrExpr(ctx *DeclarationOrExprContext) interface{}

	// Visit a parse tree produced by QuintParser#lambda.
	VisitLambda(ctx *LambdaContext) interface{}

	// Visit a parse tree produced by QuintParser#lambdaUnsugared.
	VisitLambdaUnsugared(ctx *LambdaUnsugaredContext) interface{}

	// Visit a parse tree produced by QuintParser#lambdaTupleSugar.
	VisitLambdaTupleSugar(ctx *LambdaTupleSugarContext) interface{}

	// Visit a parse tree produced by QuintParser#identOrHole.
	VisitIdentOrHole(ctx *IdentOrHoleContext) interface{}

	// Visit a parse tree produced by QuintParser#parameter.
	VisitParameter(ctx *ParameterContext) interface{}

	// Visit a parse tree produced by QuintParser#annotatedParameter.
	VisitAnnotatedParameter(ctx *AnnotatedParameterContext) interface{}

	// Visit a parse tree produced by QuintParser#destructuringPattern.
	VisitDestructuringPattern(ctx *DestructuringPatternContext) interface{}

	// Visit a parse tree produced by QuintParser#tuplePattern.
	VisitTuplePattern(ctx *TuplePatternContext) interface{}

	// Visit a parse tree produced by QuintParser#recordPattern.
	VisitRecordPattern(ctx *RecordPatternContext) interface{}

	// Visit a parse tree produced by QuintParser#identOrStar.
	VisitIdentOrStar(ctx *IdentOrStarContext) interface{}

	// Visit a parse tree produced by QuintParser#argList.
	VisitArgList(ctx *ArgListContext) interface{}

	// Visit a parse tree produced by QuintParser#recElem.
	VisitRecElem(ctx *RecElemContext) interface{}

	// Visit a parse tree produced by QuintParser#normalCallName.
	VisitNormalCallName(ctx *NormalCallNameContext) interface{}

	// Visit a parse tree produced by QuintParser#nameAfterDot.
	VisitNameAfterDot(ctx *NameAfterDotContext) interface{}

	// Visit a parse tree produced by QuintParser#operator.
	VisitOperator(ctx *OperatorContext) interface{}

	// Visit a parse tree produced by QuintParser#literal.
	VisitLiteral(ctx *LiteralContext) interface{}

	// Visit a parse tree produced by QuintParser#qualId.
	VisitQualId(ctx *QualIdContext) interface{}

	// Visit a parse tree produced by QuintParser#simpleId.
	VisitSimpleId(ctx *SimpleIdContext) interface{}

	// Visit a parse tree produced by QuintParser#identifier.
	VisitIdentifier(ctx *IdentifierContext) interface{}

	// Visit a parse tree produced by QuintParser#keywordAsID.
	VisitKeywordAsID(ctx *KeywordAsIDContext) interface{}

	// Visit a parse tree produced by QuintParser#reserved.
	VisitReserved(ctx *ReservedContext) interface{}
}
