// Code generated from pkg/grammar/Quint.g4 by ANTLR 4.13.2. DO NOT EDIT.

package quintparser // Quint
import "github.com/antlr4-go/antlr/v4"

// BaseQuintListener is a complete listener for a parse tree produced by QuintParser.
type BaseQuintListener struct{}

var _ QuintListener = &BaseQuintListener{}

// VisitTerminal is called when a terminal node is visited.
func (s *BaseQuintListener) VisitTerminal(node antlr.TerminalNode) {}

// VisitErrorNode is called when an error node is visited.
func (s *BaseQuintListener) VisitErrorNode(node antlr.ErrorNode) {}

// EnterEveryRule is called when any rule is entered.
func (s *BaseQuintListener) EnterEveryRule(ctx antlr.ParserRuleContext) {}

// ExitEveryRule is called when any rule is exited.
func (s *BaseQuintListener) ExitEveryRule(ctx antlr.ParserRuleContext) {}

// EnterModules is called when production modules is entered.
func (s *BaseQuintListener) EnterModules(ctx *ModulesContext) {}

// ExitModules is called when production modules is exited.
func (s *BaseQuintListener) ExitModules(ctx *ModulesContext) {}

// EnterModule is called when production module is entered.
func (s *BaseQuintListener) EnterModule(ctx *ModuleContext) {}

// ExitModule is called when production module is exited.
func (s *BaseQuintListener) ExitModule(ctx *ModuleContext) {}

// EnterModuleItem is called when production moduleItem is entered.
func (s *BaseQuintListener) EnterModuleItem(ctx *ModuleItemContext) {}

// ExitModuleItem is called when production moduleItem is exited.
func (s *BaseQuintListener) ExitModuleItem(ctx *ModuleItemContext) {}

// EnterConst is called when production const is entered.
func (s *BaseQuintListener) EnterConst(ctx *ConstContext) {}

// ExitConst is called when production const is exited.
func (s *BaseQuintListener) ExitConst(ctx *ConstContext) {}

// EnterVar is called when production var is entered.
func (s *BaseQuintListener) EnterVar(ctx *VarContext) {}

// ExitVar is called when production var is exited.
func (s *BaseQuintListener) ExitVar(ctx *VarContext) {}

// EnterAssume is called when production assume is entered.
func (s *BaseQuintListener) EnterAssume(ctx *AssumeContext) {}

// ExitAssume is called when production assume is exited.
func (s *BaseQuintListener) ExitAssume(ctx *AssumeContext) {}

// EnterInstance is called when production instance is entered.
func (s *BaseQuintListener) EnterInstance(ctx *InstanceContext) {}

// ExitInstance is called when production instance is exited.
func (s *BaseQuintListener) ExitInstance(ctx *InstanceContext) {}

// EnterOper is called when production oper is entered.
func (s *BaseQuintListener) EnterOper(ctx *OperContext) {}

// ExitOper is called when production oper is exited.
func (s *BaseQuintListener) ExitOper(ctx *OperContext) {}

// EnterTypeDefs is called when production typeDefs is entered.
func (s *BaseQuintListener) EnterTypeDefs(ctx *TypeDefsContext) {}

// ExitTypeDefs is called when production typeDefs is exited.
func (s *BaseQuintListener) ExitTypeDefs(ctx *TypeDefsContext) {}

// EnterImportDef is called when production importDef is entered.
func (s *BaseQuintListener) EnterImportDef(ctx *ImportDefContext) {}

// ExitImportDef is called when production importDef is exited.
func (s *BaseQuintListener) ExitImportDef(ctx *ImportDefContext) {}

// EnterExportDef is called when production exportDef is entered.
func (s *BaseQuintListener) EnterExportDef(ctx *ExportDefContext) {}

// ExitExportDef is called when production exportDef is exited.
func (s *BaseQuintListener) ExitExportDef(ctx *ExportDefContext) {}

// EnterAnnotatedOperDef is called when production annotatedOperDef is entered.
func (s *BaseQuintListener) EnterAnnotatedOperDef(ctx *AnnotatedOperDefContext) {}

// ExitAnnotatedOperDef is called when production annotatedOperDef is exited.
func (s *BaseQuintListener) ExitAnnotatedOperDef(ctx *AnnotatedOperDefContext) {}

// EnterDeprecatedOperDef is called when production deprecatedOperDef is entered.
func (s *BaseQuintListener) EnterDeprecatedOperDef(ctx *DeprecatedOperDefContext) {}

// ExitDeprecatedOperDef is called when production deprecatedOperDef is exited.
func (s *BaseQuintListener) ExitDeprecatedOperDef(ctx *DeprecatedOperDefContext) {}

// EnterValDestructuring is called when production valDestructuring is entered.
func (s *BaseQuintListener) EnterValDestructuring(ctx *ValDestructuringContext) {}

// ExitValDestructuring is called when production valDestructuring is exited.
func (s *BaseQuintListener) ExitValDestructuring(ctx *ValDestructuringContext) {}

// EnterPureValDestructuring is called when production pureValDestructuring is entered.
func (s *BaseQuintListener) EnterPureValDestructuring(ctx *PureValDestructuringContext) {}

// ExitPureValDestructuring is called when production pureValDestructuring is exited.
func (s *BaseQuintListener) ExitPureValDestructuring(ctx *PureValDestructuringContext) {}

// EnterTypeAbstractDef is called when production typeAbstractDef is entered.
func (s *BaseQuintListener) EnterTypeAbstractDef(ctx *TypeAbstractDefContext) {}

// ExitTypeAbstractDef is called when production typeAbstractDef is exited.
func (s *BaseQuintListener) ExitTypeAbstractDef(ctx *TypeAbstractDefContext) {}

// EnterTypeSumDef is called when production typeSumDef is entered.
func (s *BaseQuintListener) EnterTypeSumDef(ctx *TypeSumDefContext) {}

// ExitTypeSumDef is called when production typeSumDef is exited.
func (s *BaseQuintListener) ExitTypeSumDef(ctx *TypeSumDefContext) {}

// EnterTypeAliasDef is called when production typeAliasDef is entered.
func (s *BaseQuintListener) EnterTypeAliasDef(ctx *TypeAliasDefContext) {}

// ExitTypeAliasDef is called when production typeAliasDef is exited.
func (s *BaseQuintListener) ExitTypeAliasDef(ctx *TypeAliasDefContext) {}

// EnterTypeDefHead is called when production typeDefHead is entered.
func (s *BaseQuintListener) EnterTypeDefHead(ctx *TypeDefHeadContext) {}

// ExitTypeDefHead is called when production typeDefHead is exited.
func (s *BaseQuintListener) ExitTypeDefHead(ctx *TypeDefHeadContext) {}

// EnterSumTypeDefinition is called when production sumTypeDefinition is entered.
func (s *BaseQuintListener) EnterSumTypeDefinition(ctx *SumTypeDefinitionContext) {}

// ExitSumTypeDefinition is called when production sumTypeDefinition is exited.
func (s *BaseQuintListener) ExitSumTypeDefinition(ctx *SumTypeDefinitionContext) {}

// EnterTypeSumVariant is called when production typeSumVariant is entered.
func (s *BaseQuintListener) EnterTypeSumVariant(ctx *TypeSumVariantContext) {}

// ExitTypeSumVariant is called when production typeSumVariant is exited.
func (s *BaseQuintListener) ExitTypeSumVariant(ctx *TypeSumVariantContext) {}

// EnterQualifier is called when production qualifier is entered.
func (s *BaseQuintListener) EnterQualifier(ctx *QualifierContext) {}

// ExitQualifier is called when production qualifier is exited.
func (s *BaseQuintListener) ExitQualifier(ctx *QualifierContext) {}

// EnterImportMod is called when production importMod is entered.
func (s *BaseQuintListener) EnterImportMod(ctx *ImportModContext) {}

// ExitImportMod is called when production importMod is exited.
func (s *BaseQuintListener) ExitImportMod(ctx *ImportModContext) {}

// EnterExportMod is called when production exportMod is entered.
func (s *BaseQuintListener) EnterExportMod(ctx *ExportModContext) {}

// ExitExportMod is called when production exportMod is exited.
func (s *BaseQuintListener) ExitExportMod(ctx *ExportModContext) {}

// EnterInstanceMod is called when production instanceMod is entered.
func (s *BaseQuintListener) EnterInstanceMod(ctx *InstanceModContext) {}

// ExitInstanceMod is called when production instanceMod is exited.
func (s *BaseQuintListener) ExitInstanceMod(ctx *InstanceModContext) {}

// EnterModuleName is called when production moduleName is entered.
func (s *BaseQuintListener) EnterModuleName(ctx *ModuleNameContext) {}

// ExitModuleName is called when production moduleName is exited.
func (s *BaseQuintListener) ExitModuleName(ctx *ModuleNameContext) {}

// EnterName is called when production name is entered.
func (s *BaseQuintListener) EnterName(ctx *NameContext) {}

// ExitName is called when production name is exited.
func (s *BaseQuintListener) ExitName(ctx *NameContext) {}

// EnterQualifiedName is called when production qualifiedName is entered.
func (s *BaseQuintListener) EnterQualifiedName(ctx *QualifiedNameContext) {}

// ExitQualifiedName is called when production qualifiedName is exited.
func (s *BaseQuintListener) ExitQualifiedName(ctx *QualifiedNameContext) {}

// EnterFromSource is called when production fromSource is entered.
func (s *BaseQuintListener) EnterFromSource(ctx *FromSourceContext) {}

// ExitFromSource is called when production fromSource is exited.
func (s *BaseQuintListener) ExitFromSource(ctx *FromSourceContext) {}

// EnterTypeConst is called when production typeConst is entered.
func (s *BaseQuintListener) EnterTypeConst(ctx *TypeConstContext) {}

// ExitTypeConst is called when production typeConst is exited.
func (s *BaseQuintListener) ExitTypeConst(ctx *TypeConstContext) {}

// EnterTypeApp is called when production typeApp is entered.
func (s *BaseQuintListener) EnterTypeApp(ctx *TypeAppContext) {}

// ExitTypeApp is called when production typeApp is exited.
func (s *BaseQuintListener) ExitTypeApp(ctx *TypeAppContext) {}

// EnterTypeFun is called when production typeFun is entered.
func (s *BaseQuintListener) EnterTypeFun(ctx *TypeFunContext) {}

// ExitTypeFun is called when production typeFun is exited.
func (s *BaseQuintListener) ExitTypeFun(ctx *TypeFunContext) {}

// EnterTypeOper is called when production typeOper is entered.
func (s *BaseQuintListener) EnterTypeOper(ctx *TypeOperContext) {}

// ExitTypeOper is called when production typeOper is exited.
func (s *BaseQuintListener) ExitTypeOper(ctx *TypeOperContext) {}

// EnterTypeParen is called when production typeParen is entered.
func (s *BaseQuintListener) EnterTypeParen(ctx *TypeParenContext) {}

// ExitTypeParen is called when production typeParen is exited.
func (s *BaseQuintListener) ExitTypeParen(ctx *TypeParenContext) {}

// EnterTypeBool is called when production typeBool is entered.
func (s *BaseQuintListener) EnterTypeBool(ctx *TypeBoolContext) {}

// ExitTypeBool is called when production typeBool is exited.
func (s *BaseQuintListener) ExitTypeBool(ctx *TypeBoolContext) {}

// EnterTypeSet is called when production typeSet is entered.
func (s *BaseQuintListener) EnterTypeSet(ctx *TypeSetContext) {}

// ExitTypeSet is called when production typeSet is exited.
func (s *BaseQuintListener) ExitTypeSet(ctx *TypeSetContext) {}

// EnterTypeUnit is called when production typeUnit is entered.
func (s *BaseQuintListener) EnterTypeUnit(ctx *TypeUnitContext) {}

// ExitTypeUnit is called when production typeUnit is exited.
func (s *BaseQuintListener) ExitTypeUnit(ctx *TypeUnitContext) {}

// EnterTypeRec is called when production typeRec is entered.
func (s *BaseQuintListener) EnterTypeRec(ctx *TypeRecContext) {}

// ExitTypeRec is called when production typeRec is exited.
func (s *BaseQuintListener) ExitTypeRec(ctx *TypeRecContext) {}

// EnterTypeStr is called when production typeStr is entered.
func (s *BaseQuintListener) EnterTypeStr(ctx *TypeStrContext) {}

// ExitTypeStr is called when production typeStr is exited.
func (s *BaseQuintListener) ExitTypeStr(ctx *TypeStrContext) {}

// EnterTypeInt is called when production typeInt is entered.
func (s *BaseQuintListener) EnterTypeInt(ctx *TypeIntContext) {}

// ExitTypeInt is called when production typeInt is exited.
func (s *BaseQuintListener) ExitTypeInt(ctx *TypeIntContext) {}

// EnterTypeList is called when production typeList is entered.
func (s *BaseQuintListener) EnterTypeList(ctx *TypeListContext) {}

// ExitTypeList is called when production typeList is exited.
func (s *BaseQuintListener) ExitTypeList(ctx *TypeListContext) {}

// EnterTypeTuple is called when production typeTuple is entered.
func (s *BaseQuintListener) EnterTypeTuple(ctx *TypeTupleContext) {}

// ExitTypeTuple is called when production typeTuple is exited.
func (s *BaseQuintListener) ExitTypeTuple(ctx *TypeTupleContext) {}

// EnterWrongTypeApp is called when production wrongTypeApp is entered.
func (s *BaseQuintListener) EnterWrongTypeApp(ctx *WrongTypeAppContext) {}

// ExitWrongTypeApp is called when production wrongTypeApp is exited.
func (s *BaseQuintListener) ExitWrongTypeApp(ctx *WrongTypeAppContext) {}

// EnterTypeVarCase is called when production typeVarCase is entered.
func (s *BaseQuintListener) EnterTypeVarCase(ctx *TypeVarCaseContext) {}

// ExitTypeVarCase is called when production typeVarCase is exited.
func (s *BaseQuintListener) ExitTypeVarCase(ctx *TypeVarCaseContext) {}

// EnterTypeVar is called when production typeVar is entered.
func (s *BaseQuintListener) EnterTypeVar(ctx *TypeVarContext) {}

// ExitTypeVar is called when production typeVar is exited.
func (s *BaseQuintListener) ExitTypeVar(ctx *TypeVarContext) {}

// EnterRow is called when production row is entered.
func (s *BaseQuintListener) EnterRow(ctx *RowContext) {}

// ExitRow is called when production row is exited.
func (s *BaseQuintListener) ExitRow(ctx *RowContext) {}

// EnterRowLabel is called when production rowLabel is entered.
func (s *BaseQuintListener) EnterRowLabel(ctx *RowLabelContext) {}

// ExitRowLabel is called when production rowLabel is exited.
func (s *BaseQuintListener) ExitRowLabel(ctx *RowLabelContext) {}

// EnterTypeArgs is called when production typeArgs is entered.
func (s *BaseQuintListener) EnterTypeArgs(ctx *TypeArgsContext) {}

// ExitTypeArgs is called when production typeArgs is exited.
func (s *BaseQuintListener) ExitTypeArgs(ctx *TypeArgsContext) {}

// EnterTypeApplication is called when production typeApplication is entered.
func (s *BaseQuintListener) EnterTypeApplication(ctx *TypeApplicationContext) {}

// ExitTypeApplication is called when production typeApplication is exited.
func (s *BaseQuintListener) ExitTypeApplication(ctx *TypeApplicationContext) {}

// EnterWrongTypeApplication is called when production wrongTypeApplication is entered.
func (s *BaseQuintListener) EnterWrongTypeApplication(ctx *WrongTypeApplicationContext) {}

// ExitWrongTypeApplication is called when production wrongTypeApplication is exited.
func (s *BaseQuintListener) ExitWrongTypeApplication(ctx *WrongTypeApplicationContext) {}

// EnterLetIn is called when production letIn is entered.
func (s *BaseQuintListener) EnterLetIn(ctx *LetInContext) {}

// ExitLetIn is called when production letIn is exited.
func (s *BaseQuintListener) ExitLetIn(ctx *LetInContext) {}

// EnterUminus is called when production uminus is entered.
func (s *BaseQuintListener) EnterUminus(ctx *UminusContext) {}

// ExitUminus is called when production uminus is exited.
func (s *BaseQuintListener) ExitUminus(ctx *UminusContext) {}

// EnterOperApp is called when production operApp is entered.
func (s *BaseQuintListener) EnterOperApp(ctx *OperAppContext) {}

// ExitOperApp is called when production operApp is exited.
func (s *BaseQuintListener) ExitOperApp(ctx *OperAppContext) {}

// EnterActionAny is called when production actionAny is entered.
func (s *BaseQuintListener) EnterActionAny(ctx *ActionAnyContext) {}

// ExitActionAny is called when production actionAny is exited.
func (s *BaseQuintListener) ExitActionAny(ctx *ActionAnyContext) {}

// EnterErrorEq is called when production errorEq is entered.
func (s *BaseQuintListener) EnterErrorEq(ctx *ErrorEqContext) {}

// ExitErrorEq is called when production errorEq is exited.
func (s *BaseQuintListener) ExitErrorEq(ctx *ErrorEqContext) {}

// EnterBraces is called when production braces is entered.
func (s *BaseQuintListener) EnterBraces(ctx *BracesContext) {}

// ExitBraces is called when production braces is exited.
func (s *BaseQuintListener) ExitBraces(ctx *BracesContext) {}

// EnterLambdaCons is called when production lambdaCons is entered.
func (s *BaseQuintListener) EnterLambdaCons(ctx *LambdaConsContext) {}

// ExitLambdaCons is called when production lambdaCons is exited.
func (s *BaseQuintListener) ExitLambdaCons(ctx *LambdaConsContext) {}

// EnterTuple is called when production tuple is entered.
func (s *BaseQuintListener) EnterTuple(ctx *TupleContext) {}

// ExitTuple is called when production tuple is exited.
func (s *BaseQuintListener) ExitTuple(ctx *TupleContext) {}

// EnterParen is called when production paren is entered.
func (s *BaseQuintListener) EnterParen(ctx *ParenContext) {}

// ExitParen is called when production paren is exited.
func (s *BaseQuintListener) ExitParen(ctx *ParenContext) {}

// EnterMultDiv is called when production multDiv is entered.
func (s *BaseQuintListener) EnterMultDiv(ctx *MultDivContext) {}

// ExitMultDiv is called when production multDiv is exited.
func (s *BaseQuintListener) ExitMultDiv(ctx *MultDivContext) {}

// EnterAnd is called when production and is entered.
func (s *BaseQuintListener) EnterAnd(ctx *AndContext) {}

// ExitAnd is called when production and is exited.
func (s *BaseQuintListener) ExitAnd(ctx *AndContext) {}

// EnterRecord is called when production record is entered.
func (s *BaseQuintListener) EnterRecord(ctx *RecordContext) {}

// ExitRecord is called when production record is exited.
func (s *BaseQuintListener) ExitRecord(ctx *RecordContext) {}

// EnterPow is called when production pow is entered.
func (s *BaseQuintListener) EnterPow(ctx *PowContext) {}

// ExitPow is called when production pow is exited.
func (s *BaseQuintListener) ExitPow(ctx *PowContext) {}

// EnterActionAll is called when production actionAll is entered.
func (s *BaseQuintListener) EnterActionAll(ctx *ActionAllContext) {}

// ExitActionAll is called when production actionAll is exited.
func (s *BaseQuintListener) ExitActionAll(ctx *ActionAllContext) {}

// EnterDotCall is called when production dotCall is entered.
func (s *BaseQuintListener) EnterDotCall(ctx *DotCallContext) {}

// ExitDotCall is called when production dotCall is exited.
func (s *BaseQuintListener) ExitDotCall(ctx *DotCallContext) {}

// EnterIfElse is called when production ifElse is entered.
func (s *BaseQuintListener) EnterIfElse(ctx *IfElseContext) {}

// ExitIfElse is called when production ifElse is exited.
func (s *BaseQuintListener) ExitIfElse(ctx *IfElseContext) {}

// EnterPlusMinus is called when production plusMinus is entered.
func (s *BaseQuintListener) EnterPlusMinus(ctx *PlusMinusContext) {}

// ExitPlusMinus is called when production plusMinus is exited.
func (s *BaseQuintListener) ExitPlusMinus(ctx *PlusMinusContext) {}

// EnterOr is called when production or is entered.
func (s *BaseQuintListener) EnterOr(ctx *OrContext) {}

// ExitOr is called when production or is exited.
func (s *BaseQuintListener) ExitOr(ctx *OrContext) {}

// EnterMatch is called when production match is entered.
func (s *BaseQuintListener) EnterMatch(ctx *MatchContext) {}

// ExitMatch is called when production match is exited.
func (s *BaseQuintListener) ExitMatch(ctx *MatchContext) {}

// EnterIff is called when production iff is entered.
func (s *BaseQuintListener) EnterIff(ctx *IffContext) {}

// ExitIff is called when production iff is exited.
func (s *BaseQuintListener) ExitIff(ctx *IffContext) {}

// EnterOrExpr is called when production orExpr is entered.
func (s *BaseQuintListener) EnterOrExpr(ctx *OrExprContext) {}

// ExitOrExpr is called when production orExpr is exited.
func (s *BaseQuintListener) ExitOrExpr(ctx *OrExprContext) {}

// EnterList is called when production list is entered.
func (s *BaseQuintListener) EnterList(ctx *ListContext) {}

// ExitList is called when production list is exited.
func (s *BaseQuintListener) ExitList(ctx *ListContext) {}

// EnterPair is called when production pair is entered.
func (s *BaseQuintListener) EnterPair(ctx *PairContext) {}

// ExitPair is called when production pair is exited.
func (s *BaseQuintListener) ExitPair(ctx *PairContext) {}

// EnterUnit is called when production unit is entered.
func (s *BaseQuintListener) EnterUnit(ctx *UnitContext) {}

// ExitUnit is called when production unit is exited.
func (s *BaseQuintListener) ExitUnit(ctx *UnitContext) {}

// EnterLeadsTo is called when production leadsTo is entered.
func (s *BaseQuintListener) EnterLeadsTo(ctx *LeadsToContext) {}

// ExitLeadsTo is called when production leadsTo is exited.
func (s *BaseQuintListener) ExitLeadsTo(ctx *LeadsToContext) {}

// EnterAsgn is called when production asgn is entered.
func (s *BaseQuintListener) EnterAsgn(ctx *AsgnContext) {}

// ExitAsgn is called when production asgn is exited.
func (s *BaseQuintListener) ExitAsgn(ctx *AsgnContext) {}

// EnterLiteralOrId is called when production literalOrId is entered.
func (s *BaseQuintListener) EnterLiteralOrId(ctx *LiteralOrIdContext) {}

// ExitLiteralOrId is called when production literalOrId is exited.
func (s *BaseQuintListener) ExitLiteralOrId(ctx *LiteralOrIdContext) {}

// EnterListApp is called when production listApp is entered.
func (s *BaseQuintListener) EnterListApp(ctx *ListAppContext) {}

// ExitListApp is called when production listApp is exited.
func (s *BaseQuintListener) ExitListApp(ctx *ListAppContext) {}

// EnterRelations is called when production relations is entered.
func (s *BaseQuintListener) EnterRelations(ctx *RelationsContext) {}

// ExitRelations is called when production relations is exited.
func (s *BaseQuintListener) ExitRelations(ctx *RelationsContext) {}

// EnterImplies is called when production implies is entered.
func (s *BaseQuintListener) EnterImplies(ctx *ImpliesContext) {}

// ExitImplies is called when production implies is exited.
func (s *BaseQuintListener) ExitImplies(ctx *ImpliesContext) {}

// EnterAndExpr is called when production andExpr is entered.
func (s *BaseQuintListener) EnterAndExpr(ctx *AndExprContext) {}

// ExitAndExpr is called when production andExpr is exited.
func (s *BaseQuintListener) ExitAndExpr(ctx *AndExprContext) {}

// EnterMatchSumExpr is called when production matchSumExpr is entered.
func (s *BaseQuintListener) EnterMatchSumExpr(ctx *MatchSumExprContext) {}

// ExitMatchSumExpr is called when production matchSumExpr is exited.
func (s *BaseQuintListener) ExitMatchSumExpr(ctx *MatchSumExprContext) {}

// EnterMatchSumCase is called when production matchSumCase is entered.
func (s *BaseQuintListener) EnterMatchSumCase(ctx *MatchSumCaseContext) {}

// ExitMatchSumCase is called when production matchSumCase is exited.
func (s *BaseQuintListener) ExitMatchSumCase(ctx *MatchSumCaseContext) {}

// EnterMatchSumVariant is called when production matchSumVariant is entered.
func (s *BaseQuintListener) EnterMatchSumVariant(ctx *MatchSumVariantContext) {}

// ExitMatchSumVariant is called when production matchSumVariant is exited.
func (s *BaseQuintListener) ExitMatchSumVariant(ctx *MatchSumVariantContext) {}

// EnterDeclarationOrExpr is called when production declarationOrExpr is entered.
func (s *BaseQuintListener) EnterDeclarationOrExpr(ctx *DeclarationOrExprContext) {}

// ExitDeclarationOrExpr is called when production declarationOrExpr is exited.
func (s *BaseQuintListener) ExitDeclarationOrExpr(ctx *DeclarationOrExprContext) {}

// EnterLambda is called when production lambda is entered.
func (s *BaseQuintListener) EnterLambda(ctx *LambdaContext) {}

// ExitLambda is called when production lambda is exited.
func (s *BaseQuintListener) ExitLambda(ctx *LambdaContext) {}

// EnterLambdaUnsugared is called when production lambdaUnsugared is entered.
func (s *BaseQuintListener) EnterLambdaUnsugared(ctx *LambdaUnsugaredContext) {}

// ExitLambdaUnsugared is called when production lambdaUnsugared is exited.
func (s *BaseQuintListener) ExitLambdaUnsugared(ctx *LambdaUnsugaredContext) {}

// EnterLambdaTupleSugar is called when production lambdaTupleSugar is entered.
func (s *BaseQuintListener) EnterLambdaTupleSugar(ctx *LambdaTupleSugarContext) {}

// ExitLambdaTupleSugar is called when production lambdaTupleSugar is exited.
func (s *BaseQuintListener) ExitLambdaTupleSugar(ctx *LambdaTupleSugarContext) {}

// EnterIdentOrHole is called when production identOrHole is entered.
func (s *BaseQuintListener) EnterIdentOrHole(ctx *IdentOrHoleContext) {}

// ExitIdentOrHole is called when production identOrHole is exited.
func (s *BaseQuintListener) ExitIdentOrHole(ctx *IdentOrHoleContext) {}

// EnterParameter is called when production parameter is entered.
func (s *BaseQuintListener) EnterParameter(ctx *ParameterContext) {}

// ExitParameter is called when production parameter is exited.
func (s *BaseQuintListener) ExitParameter(ctx *ParameterContext) {}

// EnterAnnotatedParameter is called when production annotatedParameter is entered.
func (s *BaseQuintListener) EnterAnnotatedParameter(ctx *AnnotatedParameterContext) {}

// ExitAnnotatedParameter is called when production annotatedParameter is exited.
func (s *BaseQuintListener) ExitAnnotatedParameter(ctx *AnnotatedParameterContext) {}

// EnterDestructuringPattern is called when production destructuringPattern is entered.
func (s *BaseQuintListener) EnterDestructuringPattern(ctx *DestructuringPatternContext) {}

// ExitDestructuringPattern is called when production destructuringPattern is exited.
func (s *BaseQuintListener) ExitDestructuringPattern(ctx *DestructuringPatternContext) {}

// EnterTuplePattern is called when production tuplePattern is entered.
func (s *BaseQuintListener) EnterTuplePattern(ctx *TuplePatternContext) {}

// ExitTuplePattern is called when production tuplePattern is exited.
func (s *BaseQuintListener) ExitTuplePattern(ctx *TuplePatternContext) {}

// EnterRecordPattern is called when production recordPattern is entered.
func (s *BaseQuintListener) EnterRecordPattern(ctx *RecordPatternContext) {}

// ExitRecordPattern is called when production recordPattern is exited.
func (s *BaseQuintListener) ExitRecordPattern(ctx *RecordPatternContext) {}

// EnterIdentOrStar is called when production identOrStar is entered.
func (s *BaseQuintListener) EnterIdentOrStar(ctx *IdentOrStarContext) {}

// ExitIdentOrStar is called when production identOrStar is exited.
func (s *BaseQuintListener) ExitIdentOrStar(ctx *IdentOrStarContext) {}

// EnterArgList is called when production argList is entered.
func (s *BaseQuintListener) EnterArgList(ctx *ArgListContext) {}

// ExitArgList is called when production argList is exited.
func (s *BaseQuintListener) ExitArgList(ctx *ArgListContext) {}

// EnterRecElem is called when production recElem is entered.
func (s *BaseQuintListener) EnterRecElem(ctx *RecElemContext) {}

// ExitRecElem is called when production recElem is exited.
func (s *BaseQuintListener) ExitRecElem(ctx *RecElemContext) {}

// EnterNormalCallName is called when production normalCallName is entered.
func (s *BaseQuintListener) EnterNormalCallName(ctx *NormalCallNameContext) {}

// ExitNormalCallName is called when production normalCallName is exited.
func (s *BaseQuintListener) ExitNormalCallName(ctx *NormalCallNameContext) {}

// EnterNameAfterDot is called when production nameAfterDot is entered.
func (s *BaseQuintListener) EnterNameAfterDot(ctx *NameAfterDotContext) {}

// ExitNameAfterDot is called when production nameAfterDot is exited.
func (s *BaseQuintListener) ExitNameAfterDot(ctx *NameAfterDotContext) {}

// EnterOperator is called when production operator is entered.
func (s *BaseQuintListener) EnterOperator(ctx *OperatorContext) {}

// ExitOperator is called when production operator is exited.
func (s *BaseQuintListener) ExitOperator(ctx *OperatorContext) {}

// EnterLiteral is called when production literal is entered.
func (s *BaseQuintListener) EnterLiteral(ctx *LiteralContext) {}

// ExitLiteral is called when production literal is exited.
func (s *BaseQuintListener) ExitLiteral(ctx *LiteralContext) {}

// EnterQualId is called when production qualId is entered.
func (s *BaseQuintListener) EnterQualId(ctx *QualIdContext) {}

// ExitQualId is called when production qualId is exited.
func (s *BaseQuintListener) ExitQualId(ctx *QualIdContext) {}

// EnterSimpleId is called when production simpleId is entered.
func (s *BaseQuintListener) EnterSimpleId(ctx *SimpleIdContext) {}

// ExitSimpleId is called when production simpleId is exited.
func (s *BaseQuintListener) ExitSimpleId(ctx *SimpleIdContext) {}

// EnterIdentifier is called when production identifier is entered.
func (s *BaseQuintListener) EnterIdentifier(ctx *IdentifierContext) {}

// ExitIdentifier is called when production identifier is exited.
func (s *BaseQuintListener) ExitIdentifier(ctx *IdentifierContext) {}

// EnterKeywordAsID is called when production keywordAsID is entered.
func (s *BaseQuintListener) EnterKeywordAsID(ctx *KeywordAsIDContext) {}

// ExitKeywordAsID is called when production keywordAsID is exited.
func (s *BaseQuintListener) ExitKeywordAsID(ctx *KeywordAsIDContext) {}

// EnterReserved is called when production reserved is entered.
func (s *BaseQuintListener) EnterReserved(ctx *ReservedContext) {}

// ExitReserved is called when production reserved is exited.
func (s *BaseQuintListener) ExitReserved(ctx *ReservedContext) {}
