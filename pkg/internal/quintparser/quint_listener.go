// Code generated from pkg/grammar/Quint.g4 by ANTLR 4.13.2. DO NOT EDIT.

package quintparser // Quint
import "github.com/antlr4-go/antlr/v4"

// QuintListener is a complete listener for a parse tree produced by QuintParser.
type QuintListener interface {
	antlr.ParseTreeListener

	// EnterModules is called when entering the modules production.
	EnterModules(c *ModulesContext)

	// EnterModule is called when entering the module production.
	EnterModule(c *ModuleContext)

	// EnterModuleItem is called when entering the moduleItem production.
	EnterModuleItem(c *ModuleItemContext)

	// EnterConst is called when entering the const production.
	EnterConst(c *ConstContext)

	// EnterVar is called when entering the var production.
	EnterVar(c *VarContext)

	// EnterAssume is called when entering the assume production.
	EnterAssume(c *AssumeContext)

	// EnterInstance is called when entering the instance production.
	EnterInstance(c *InstanceContext)

	// EnterOper is called when entering the oper production.
	EnterOper(c *OperContext)

	// EnterTypeDefs is called when entering the typeDefs production.
	EnterTypeDefs(c *TypeDefsContext)

	// EnterImportDef is called when entering the importDef production.
	EnterImportDef(c *ImportDefContext)

	// EnterExportDef is called when entering the exportDef production.
	EnterExportDef(c *ExportDefContext)

	// EnterAnnotatedOperDef is called when entering the annotatedOperDef production.
	EnterAnnotatedOperDef(c *AnnotatedOperDefContext)

	// EnterDeprecatedOperDef is called when entering the deprecatedOperDef production.
	EnterDeprecatedOperDef(c *DeprecatedOperDefContext)

	// EnterValDestructuring is called when entering the valDestructuring production.
	EnterValDestructuring(c *ValDestructuringContext)

	// EnterPureValDestructuring is called when entering the pureValDestructuring production.
	EnterPureValDestructuring(c *PureValDestructuringContext)

	// EnterTypeAbstractDef is called when entering the typeAbstractDef production.
	EnterTypeAbstractDef(c *TypeAbstractDefContext)

	// EnterTypeSumDef is called when entering the typeSumDef production.
	EnterTypeSumDef(c *TypeSumDefContext)

	// EnterTypeAliasDef is called when entering the typeAliasDef production.
	EnterTypeAliasDef(c *TypeAliasDefContext)

	// EnterTypeDefHead is called when entering the typeDefHead production.
	EnterTypeDefHead(c *TypeDefHeadContext)

	// EnterSumTypeDefinition is called when entering the sumTypeDefinition production.
	EnterSumTypeDefinition(c *SumTypeDefinitionContext)

	// EnterTypeSumVariant is called when entering the typeSumVariant production.
	EnterTypeSumVariant(c *TypeSumVariantContext)

	// EnterQualifier is called when entering the qualifier production.
	EnterQualifier(c *QualifierContext)

	// EnterImportMod is called when entering the importMod production.
	EnterImportMod(c *ImportModContext)

	// EnterExportMod is called when entering the exportMod production.
	EnterExportMod(c *ExportModContext)

	// EnterInstanceMod is called when entering the instanceMod production.
	EnterInstanceMod(c *InstanceModContext)

	// EnterModuleName is called when entering the moduleName production.
	EnterModuleName(c *ModuleNameContext)

	// EnterName is called when entering the name production.
	EnterName(c *NameContext)

	// EnterQualifiedName is called when entering the qualifiedName production.
	EnterQualifiedName(c *QualifiedNameContext)

	// EnterFromSource is called when entering the fromSource production.
	EnterFromSource(c *FromSourceContext)

	// EnterTypeConst is called when entering the typeConst production.
	EnterTypeConst(c *TypeConstContext)

	// EnterTypeApp is called when entering the typeApp production.
	EnterTypeApp(c *TypeAppContext)

	// EnterTypeFun is called when entering the typeFun production.
	EnterTypeFun(c *TypeFunContext)

	// EnterTypeOper is called when entering the typeOper production.
	EnterTypeOper(c *TypeOperContext)

	// EnterTypeParen is called when entering the typeParen production.
	EnterTypeParen(c *TypeParenContext)

	// EnterTypeBool is called when entering the typeBool production.
	EnterTypeBool(c *TypeBoolContext)

	// EnterTypeSet is called when entering the typeSet production.
	EnterTypeSet(c *TypeSetContext)

	// EnterTypeUnit is called when entering the typeUnit production.
	EnterTypeUnit(c *TypeUnitContext)

	// EnterTypeRec is called when entering the typeRec production.
	EnterTypeRec(c *TypeRecContext)

	// EnterTypeStr is called when entering the typeStr production.
	EnterTypeStr(c *TypeStrContext)

	// EnterTypeInt is called when entering the typeInt production.
	EnterTypeInt(c *TypeIntContext)

	// EnterTypeList is called when entering the typeList production.
	EnterTypeList(c *TypeListContext)

	// EnterTypeTuple is called when entering the typeTuple production.
	EnterTypeTuple(c *TypeTupleContext)

	// EnterWrongTypeApp is called when entering the wrongTypeApp production.
	EnterWrongTypeApp(c *WrongTypeAppContext)

	// EnterTypeVarCase is called when entering the typeVarCase production.
	EnterTypeVarCase(c *TypeVarCaseContext)

	// EnterTypeVar is called when entering the typeVar production.
	EnterTypeVar(c *TypeVarContext)

	// EnterRow is called when entering the row production.
	EnterRow(c *RowContext)

	// EnterRowLabel is called when entering the rowLabel production.
	EnterRowLabel(c *RowLabelContext)

	// EnterTypeArgs is called when entering the typeArgs production.
	EnterTypeArgs(c *TypeArgsContext)

	// EnterTypeApplication is called when entering the typeApplication production.
	EnterTypeApplication(c *TypeApplicationContext)

	// EnterWrongTypeApplication is called when entering the wrongTypeApplication production.
	EnterWrongTypeApplication(c *WrongTypeApplicationContext)

	// EnterLetIn is called when entering the letIn production.
	EnterLetIn(c *LetInContext)

	// EnterUminus is called when entering the uminus production.
	EnterUminus(c *UminusContext)

	// EnterOperApp is called when entering the operApp production.
	EnterOperApp(c *OperAppContext)

	// EnterActionAny is called when entering the actionAny production.
	EnterActionAny(c *ActionAnyContext)

	// EnterErrorEq is called when entering the errorEq production.
	EnterErrorEq(c *ErrorEqContext)

	// EnterBraces is called when entering the braces production.
	EnterBraces(c *BracesContext)

	// EnterLambdaCons is called when entering the lambdaCons production.
	EnterLambdaCons(c *LambdaConsContext)

	// EnterTuple is called when entering the tuple production.
	EnterTuple(c *TupleContext)

	// EnterParen is called when entering the paren production.
	EnterParen(c *ParenContext)

	// EnterMultDiv is called when entering the multDiv production.
	EnterMultDiv(c *MultDivContext)

	// EnterAnd is called when entering the and production.
	EnterAnd(c *AndContext)

	// EnterRecord is called when entering the record production.
	EnterRecord(c *RecordContext)

	// EnterPow is called when entering the pow production.
	EnterPow(c *PowContext)

	// EnterActionAll is called when entering the actionAll production.
	EnterActionAll(c *ActionAllContext)

	// EnterDotCall is called when entering the dotCall production.
	EnterDotCall(c *DotCallContext)

	// EnterIfElse is called when entering the ifElse production.
	EnterIfElse(c *IfElseContext)

	// EnterPlusMinus is called when entering the plusMinus production.
	EnterPlusMinus(c *PlusMinusContext)

	// EnterOr is called when entering the or production.
	EnterOr(c *OrContext)

	// EnterMatch is called when entering the match production.
	EnterMatch(c *MatchContext)

	// EnterIff is called when entering the iff production.
	EnterIff(c *IffContext)

	// EnterOrExpr is called when entering the orExpr production.
	EnterOrExpr(c *OrExprContext)

	// EnterList is called when entering the list production.
	EnterList(c *ListContext)

	// EnterPair is called when entering the pair production.
	EnterPair(c *PairContext)

	// EnterUnit is called when entering the unit production.
	EnterUnit(c *UnitContext)

	// EnterLeadsTo is called when entering the leadsTo production.
	EnterLeadsTo(c *LeadsToContext)

	// EnterAsgn is called when entering the asgn production.
	EnterAsgn(c *AsgnContext)

	// EnterLiteralOrId is called when entering the literalOrId production.
	EnterLiteralOrId(c *LiteralOrIdContext)

	// EnterListApp is called when entering the listApp production.
	EnterListApp(c *ListAppContext)

	// EnterRelations is called when entering the relations production.
	EnterRelations(c *RelationsContext)

	// EnterImplies is called when entering the implies production.
	EnterImplies(c *ImpliesContext)

	// EnterAndExpr is called when entering the andExpr production.
	EnterAndExpr(c *AndExprContext)

	// EnterMatchSumExpr is called when entering the matchSumExpr production.
	EnterMatchSumExpr(c *MatchSumExprContext)

	// EnterMatchSumCase is called when entering the matchSumCase production.
	EnterMatchSumCase(c *MatchSumCaseContext)

	// EnterMatchSumVariant is called when entering the matchSumVariant production.
	EnterMatchSumVariant(c *MatchSumVariantContext)

	// EnterDeclarationOrExpr is called when entering the declarationOrExpr production.
	EnterDeclarationOrExpr(c *DeclarationOrExprContext)

	// EnterLambda is called when entering the lambda production.
	EnterLambda(c *LambdaContext)

	// EnterLambdaUnsugared is called when entering the lambdaUnsugared production.
	EnterLambdaUnsugared(c *LambdaUnsugaredContext)

	// EnterLambdaTupleSugar is called when entering the lambdaTupleSugar production.
	EnterLambdaTupleSugar(c *LambdaTupleSugarContext)

	// EnterIdentOrHole is called when entering the identOrHole production.
	EnterIdentOrHole(c *IdentOrHoleContext)

	// EnterParameter is called when entering the parameter production.
	EnterParameter(c *ParameterContext)

	// EnterAnnotatedParameter is called when entering the annotatedParameter production.
	EnterAnnotatedParameter(c *AnnotatedParameterContext)

	// EnterDestructuringPattern is called when entering the destructuringPattern production.
	EnterDestructuringPattern(c *DestructuringPatternContext)

	// EnterTuplePattern is called when entering the tuplePattern production.
	EnterTuplePattern(c *TuplePatternContext)

	// EnterRecordPattern is called when entering the recordPattern production.
	EnterRecordPattern(c *RecordPatternContext)

	// EnterIdentOrStar is called when entering the identOrStar production.
	EnterIdentOrStar(c *IdentOrStarContext)

	// EnterArgList is called when entering the argList production.
	EnterArgList(c *ArgListContext)

	// EnterRecElem is called when entering the recElem production.
	EnterRecElem(c *RecElemContext)

	// EnterNormalCallName is called when entering the normalCallName production.
	EnterNormalCallName(c *NormalCallNameContext)

	// EnterNameAfterDot is called when entering the nameAfterDot production.
	EnterNameAfterDot(c *NameAfterDotContext)

	// EnterOperator is called when entering the operator production.
	EnterOperator(c *OperatorContext)

	// EnterLiteral is called when entering the literal production.
	EnterLiteral(c *LiteralContext)

	// EnterQualId is called when entering the qualId production.
	EnterQualId(c *QualIdContext)

	// EnterSimpleId is called when entering the simpleId production.
	EnterSimpleId(c *SimpleIdContext)

	// EnterIdentifier is called when entering the identifier production.
	EnterIdentifier(c *IdentifierContext)

	// EnterKeywordAsID is called when entering the keywordAsID production.
	EnterKeywordAsID(c *KeywordAsIDContext)

	// EnterReserved is called when entering the reserved production.
	EnterReserved(c *ReservedContext)

	// ExitModules is called when exiting the modules production.
	ExitModules(c *ModulesContext)

	// ExitModule is called when exiting the module production.
	ExitModule(c *ModuleContext)

	// ExitModuleItem is called when exiting the moduleItem production.
	ExitModuleItem(c *ModuleItemContext)

	// ExitConst is called when exiting the const production.
	ExitConst(c *ConstContext)

	// ExitVar is called when exiting the var production.
	ExitVar(c *VarContext)

	// ExitAssume is called when exiting the assume production.
	ExitAssume(c *AssumeContext)

	// ExitInstance is called when exiting the instance production.
	ExitInstance(c *InstanceContext)

	// ExitOper is called when exiting the oper production.
	ExitOper(c *OperContext)

	// ExitTypeDefs is called when exiting the typeDefs production.
	ExitTypeDefs(c *TypeDefsContext)

	// ExitImportDef is called when exiting the importDef production.
	ExitImportDef(c *ImportDefContext)

	// ExitExportDef is called when exiting the exportDef production.
	ExitExportDef(c *ExportDefContext)

	// ExitAnnotatedOperDef is called when exiting the annotatedOperDef production.
	ExitAnnotatedOperDef(c *AnnotatedOperDefContext)

	// ExitDeprecatedOperDef is called when exiting the deprecatedOperDef production.
	ExitDeprecatedOperDef(c *DeprecatedOperDefContext)

	// ExitValDestructuring is called when exiting the valDestructuring production.
	ExitValDestructuring(c *ValDestructuringContext)

	// ExitPureValDestructuring is called when exiting the pureValDestructuring production.
	ExitPureValDestructuring(c *PureValDestructuringContext)

	// ExitTypeAbstractDef is called when exiting the typeAbstractDef production.
	ExitTypeAbstractDef(c *TypeAbstractDefContext)

	// ExitTypeSumDef is called when exiting the typeSumDef production.
	ExitTypeSumDef(c *TypeSumDefContext)

	// ExitTypeAliasDef is called when exiting the typeAliasDef production.
	ExitTypeAliasDef(c *TypeAliasDefContext)

	// ExitTypeDefHead is called when exiting the typeDefHead production.
	ExitTypeDefHead(c *TypeDefHeadContext)

	// ExitSumTypeDefinition is called when exiting the sumTypeDefinition production.
	ExitSumTypeDefinition(c *SumTypeDefinitionContext)

	// ExitTypeSumVariant is called when exiting the typeSumVariant production.
	ExitTypeSumVariant(c *TypeSumVariantContext)

	// ExitQualifier is called when exiting the qualifier production.
	ExitQualifier(c *QualifierContext)

	// ExitImportMod is called when exiting the importMod production.
	ExitImportMod(c *ImportModContext)

	// ExitExportMod is called when exiting the exportMod production.
	ExitExportMod(c *ExportModContext)

	// ExitInstanceMod is called when exiting the instanceMod production.
	ExitInstanceMod(c *InstanceModContext)

	// ExitModuleName is called when exiting the moduleName production.
	ExitModuleName(c *ModuleNameContext)

	// ExitName is called when exiting the name production.
	ExitName(c *NameContext)

	// ExitQualifiedName is called when exiting the qualifiedName production.
	ExitQualifiedName(c *QualifiedNameContext)

	// ExitFromSource is called when exiting the fromSource production.
	ExitFromSource(c *FromSourceContext)

	// ExitTypeConst is called when exiting the typeConst production.
	ExitTypeConst(c *TypeConstContext)

	// ExitTypeApp is called when exiting the typeApp production.
	ExitTypeApp(c *TypeAppContext)

	// ExitTypeFun is called when exiting the typeFun production.
	ExitTypeFun(c *TypeFunContext)

	// ExitTypeOper is called when exiting the typeOper production.
	ExitTypeOper(c *TypeOperContext)

	// ExitTypeParen is called when exiting the typeParen production.
	ExitTypeParen(c *TypeParenContext)

	// ExitTypeBool is called when exiting the typeBool production.
	ExitTypeBool(c *TypeBoolContext)

	// ExitTypeSet is called when exiting the typeSet production.
	ExitTypeSet(c *TypeSetContext)

	// ExitTypeUnit is called when exiting the typeUnit production.
	ExitTypeUnit(c *TypeUnitContext)

	// ExitTypeRec is called when exiting the typeRec production.
	ExitTypeRec(c *TypeRecContext)

	// ExitTypeStr is called when exiting the typeStr production.
	ExitTypeStr(c *TypeStrContext)

	// ExitTypeInt is called when exiting the typeInt production.
	ExitTypeInt(c *TypeIntContext)

	// ExitTypeList is called when exiting the typeList production.
	ExitTypeList(c *TypeListContext)

	// ExitTypeTuple is called when exiting the typeTuple production.
	ExitTypeTuple(c *TypeTupleContext)

	// ExitWrongTypeApp is called when exiting the wrongTypeApp production.
	ExitWrongTypeApp(c *WrongTypeAppContext)

	// ExitTypeVarCase is called when exiting the typeVarCase production.
	ExitTypeVarCase(c *TypeVarCaseContext)

	// ExitTypeVar is called when exiting the typeVar production.
	ExitTypeVar(c *TypeVarContext)

	// ExitRow is called when exiting the row production.
	ExitRow(c *RowContext)

	// ExitRowLabel is called when exiting the rowLabel production.
	ExitRowLabel(c *RowLabelContext)

	// ExitTypeArgs is called when exiting the typeArgs production.
	ExitTypeArgs(c *TypeArgsContext)

	// ExitTypeApplication is called when exiting the typeApplication production.
	ExitTypeApplication(c *TypeApplicationContext)

	// ExitWrongTypeApplication is called when exiting the wrongTypeApplication production.
	ExitWrongTypeApplication(c *WrongTypeApplicationContext)

	// ExitLetIn is called when exiting the letIn production.
	ExitLetIn(c *LetInContext)

	// ExitUminus is called when exiting the uminus production.
	ExitUminus(c *UminusContext)

	// ExitOperApp is called when exiting the operApp production.
	ExitOperApp(c *OperAppContext)

	// ExitActionAny is called when exiting the actionAny production.
	ExitActionAny(c *ActionAnyContext)

	// ExitErrorEq is called when exiting the errorEq production.
	ExitErrorEq(c *ErrorEqContext)

	// ExitBraces is called when exiting the braces production.
	ExitBraces(c *BracesContext)

	// ExitLambdaCons is called when exiting the lambdaCons production.
	ExitLambdaCons(c *LambdaConsContext)

	// ExitTuple is called when exiting the tuple production.
	ExitTuple(c *TupleContext)

	// ExitParen is called when exiting the paren production.
	ExitParen(c *ParenContext)

	// ExitMultDiv is called when exiting the multDiv production.
	ExitMultDiv(c *MultDivContext)

	// ExitAnd is called when exiting the and production.
	ExitAnd(c *AndContext)

	// ExitRecord is called when exiting the record production.
	ExitRecord(c *RecordContext)

	// ExitPow is called when exiting the pow production.
	ExitPow(c *PowContext)

	// ExitActionAll is called when exiting the actionAll production.
	ExitActionAll(c *ActionAllContext)

	// ExitDotCall is called when exiting the dotCall production.
	ExitDotCall(c *DotCallContext)

	// ExitIfElse is called when exiting the ifElse production.
	ExitIfElse(c *IfElseContext)

	// ExitPlusMinus is called when exiting the plusMinus production.
	ExitPlusMinus(c *PlusMinusContext)

	// ExitOr is called when exiting the or production.
	ExitOr(c *OrContext)

	// ExitMatch is called when exiting the match production.
	ExitMatch(c *MatchContext)

	// ExitIff is called when exiting the iff production.
	ExitIff(c *IffContext)

	// ExitOrExpr is called when exiting the orExpr production.
	ExitOrExpr(c *OrExprContext)

	// ExitList is called when exiting the list production.
	ExitList(c *ListContext)

	// ExitPair is called when exiting the pair production.
	ExitPair(c *PairContext)

	// ExitUnit is called when exiting the unit production.
	ExitUnit(c *UnitContext)

	// ExitLeadsTo is called when exiting the leadsTo production.
	ExitLeadsTo(c *LeadsToContext)

	// ExitAsgn is called when exiting the asgn production.
	ExitAsgn(c *AsgnContext)

	// ExitLiteralOrId is called when exiting the literalOrId production.
	ExitLiteralOrId(c *LiteralOrIdContext)

	// ExitListApp is called when exiting the listApp production.
	ExitListApp(c *ListAppContext)

	// ExitRelations is called when exiting the relations production.
	ExitRelations(c *RelationsContext)

	// ExitImplies is called when exiting the implies production.
	ExitImplies(c *ImpliesContext)

	// ExitAndExpr is called when exiting the andExpr production.
	ExitAndExpr(c *AndExprContext)

	// ExitMatchSumExpr is called when exiting the matchSumExpr production.
	ExitMatchSumExpr(c *MatchSumExprContext)

	// ExitMatchSumCase is called when exiting the matchSumCase production.
	ExitMatchSumCase(c *MatchSumCaseContext)

	// ExitMatchSumVariant is called when exiting the matchSumVariant production.
	ExitMatchSumVariant(c *MatchSumVariantContext)

	// ExitDeclarationOrExpr is called when exiting the declarationOrExpr production.
	ExitDeclarationOrExpr(c *DeclarationOrExprContext)

	// ExitLambda is called when exiting the lambda production.
	ExitLambda(c *LambdaContext)

	// ExitLambdaUnsugared is called when exiting the lambdaUnsugared production.
	ExitLambdaUnsugared(c *LambdaUnsugaredContext)

	// ExitLambdaTupleSugar is called when exiting the lambdaTupleSugar production.
	ExitLambdaTupleSugar(c *LambdaTupleSugarContext)

	// ExitIdentOrHole is called when exiting the identOrHole production.
	ExitIdentOrHole(c *IdentOrHoleContext)

	// ExitParameter is called when exiting the parameter production.
	ExitParameter(c *ParameterContext)

	// ExitAnnotatedParameter is called when exiting the annotatedParameter production.
	ExitAnnotatedParameter(c *AnnotatedParameterContext)

	// ExitDestructuringPattern is called when exiting the destructuringPattern production.
	ExitDestructuringPattern(c *DestructuringPatternContext)

	// ExitTuplePattern is called when exiting the tuplePattern production.
	ExitTuplePattern(c *TuplePatternContext)

	// ExitRecordPattern is called when exiting the recordPattern production.
	ExitRecordPattern(c *RecordPatternContext)

	// ExitIdentOrStar is called when exiting the identOrStar production.
	ExitIdentOrStar(c *IdentOrStarContext)

	// ExitArgList is called when exiting the argList production.
	ExitArgList(c *ArgListContext)

	// ExitRecElem is called when exiting the recElem production.
	ExitRecElem(c *RecElemContext)

	// ExitNormalCallName is called when exiting the normalCallName production.
	ExitNormalCallName(c *NormalCallNameContext)

	// ExitNameAfterDot is called when exiting the nameAfterDot production.
	ExitNameAfterDot(c *NameAfterDotContext)

	// ExitOperator is called when exiting the operator production.
	ExitOperator(c *OperatorContext)

	// ExitLiteral is called when exiting the literal production.
	ExitLiteral(c *LiteralContext)

	// ExitQualId is called when exiting the qualId production.
	ExitQualId(c *QualIdContext)

	// ExitSimpleId is called when exiting the simpleId production.
	ExitSimpleId(c *SimpleIdContext)

	// ExitIdentifier is called when exiting the identifier production.
	ExitIdentifier(c *IdentifierContext)

	// ExitKeywordAsID is called when exiting the keywordAsID production.
	ExitKeywordAsID(c *KeywordAsIDContext)

	// ExitReserved is called when exiting the reserved production.
	ExitReserved(c *ReservedContext)
}
