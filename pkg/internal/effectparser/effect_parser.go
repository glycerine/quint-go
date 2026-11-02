// Code generated from quint/src/generated/Effect.g4 by ANTLR 4.13.2. DO NOT EDIT.

package effectparser // Effect
import (
	"fmt"
	"strconv"
	"sync"

	"github.com/antlr4-go/antlr/v4"
)

// Suppress unused import errors
var _ = fmt.Printf
var _ = strconv.Itoa
var _ = sync.Once{}

type EffectParser struct {
	*antlr.BaseParser
}

var EffectParserStaticData struct {
	once                   sync.Once
	serializedATN          []int32
	LiteralNames           []string
	SymbolicNames          []string
	RuleNames              []string
	PredictionContextCache *antlr.PredictionContextCache
	atn                    *antlr.ATN
	decisionToDFA          []*antlr.DFA
}

func effectParserInit() {
	staticData := &EffectParserStaticData
	staticData.LiteralNames = []string{
		"", "'('", "','", "')'", "'=>'", "'Read'", "'['", "']'", "'Update'",
		"'Temporal'", "'&'", "'Pure'", "'''",
	}
	staticData.SymbolicNames = []string{
		"", "", "", "", "", "", "", "", "", "", "", "", "", "IDENTIFIER", "WS",
	}
	staticData.RuleNames = []string{
		"effect", "read", "update", "temporal", "concrete", "component", "entity",
		"stateVarRef",
	}
	staticData.PredictionContextCache = antlr.NewPredictionContextCache()
	staticData.serializedATN = []int32{
		4, 1, 14, 86, 2, 0, 7, 0, 2, 1, 7, 1, 2, 2, 7, 2, 2, 3, 7, 3, 2, 4, 7,
		4, 2, 5, 7, 5, 2, 6, 7, 6, 2, 7, 7, 7, 1, 0, 1, 0, 1, 0, 1, 0, 1, 0, 5,
		0, 22, 8, 0, 10, 0, 12, 0, 25, 9, 0, 3, 0, 27, 8, 0, 1, 0, 1, 0, 1, 0,
		1, 0, 3, 0, 33, 8, 0, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 2, 1, 2, 1, 2, 1,
		2, 1, 2, 1, 3, 1, 3, 1, 3, 1, 3, 1, 3, 1, 4, 1, 4, 1, 4, 5, 4, 53, 8, 4,
		10, 4, 12, 4, 56, 9, 4, 1, 4, 3, 4, 59, 8, 4, 1, 5, 1, 5, 1, 5, 3, 5, 64,
		8, 5, 1, 6, 1, 6, 3, 6, 68, 8, 6, 1, 6, 1, 6, 1, 6, 3, 6, 73, 8, 6, 5,
		6, 75, 8, 6, 10, 6, 12, 6, 78, 9, 6, 3, 6, 80, 8, 6, 1, 7, 1, 7, 1, 7,
		1, 7, 1, 7, 0, 0, 8, 0, 2, 4, 6, 8, 10, 12, 14, 0, 0, 89, 0, 32, 1, 0,
		0, 0, 2, 34, 1, 0, 0, 0, 4, 39, 1, 0, 0, 0, 6, 44, 1, 0, 0, 0, 8, 58, 1,
		0, 0, 0, 10, 63, 1, 0, 0, 0, 12, 79, 1, 0, 0, 0, 14, 81, 1, 0, 0, 0, 16,
		33, 3, 8, 4, 0, 17, 26, 5, 1, 0, 0, 18, 23, 3, 0, 0, 0, 19, 20, 5, 2, 0,
		0, 20, 22, 3, 0, 0, 0, 21, 19, 1, 0, 0, 0, 22, 25, 1, 0, 0, 0, 23, 21,
		1, 0, 0, 0, 23, 24, 1, 0, 0, 0, 24, 27, 1, 0, 0, 0, 25, 23, 1, 0, 0, 0,
		26, 18, 1, 0, 0, 0, 26, 27, 1, 0, 0, 0, 27, 28, 1, 0, 0, 0, 28, 29, 5,
		3, 0, 0, 29, 30, 5, 4, 0, 0, 30, 33, 3, 0, 0, 0, 31, 33, 5, 13, 0, 0, 32,
		16, 1, 0, 0, 0, 32, 17, 1, 0, 0, 0, 32, 31, 1, 0, 0, 0, 33, 1, 1, 0, 0,
		0, 34, 35, 5, 5, 0, 0, 35, 36, 5, 6, 0, 0, 36, 37, 3, 12, 6, 0, 37, 38,
		5, 7, 0, 0, 38, 3, 1, 0, 0, 0, 39, 40, 5, 8, 0, 0, 40, 41, 5, 6, 0, 0,
		41, 42, 3, 12, 6, 0, 42, 43, 5, 7, 0, 0, 43, 5, 1, 0, 0, 0, 44, 45, 5,
		9, 0, 0, 45, 46, 5, 6, 0, 0, 46, 47, 3, 12, 6, 0, 47, 48, 5, 7, 0, 0, 48,
		7, 1, 0, 0, 0, 49, 54, 3, 10, 5, 0, 50, 51, 5, 10, 0, 0, 51, 53, 3, 10,
		5, 0, 52, 50, 1, 0, 0, 0, 53, 56, 1, 0, 0, 0, 54, 52, 1, 0, 0, 0, 54, 55,
		1, 0, 0, 0, 55, 59, 1, 0, 0, 0, 56, 54, 1, 0, 0, 0, 57, 59, 5, 11, 0, 0,
		58, 49, 1, 0, 0, 0, 58, 57, 1, 0, 0, 0, 59, 9, 1, 0, 0, 0, 60, 64, 3, 2,
		1, 0, 61, 64, 3, 4, 2, 0, 62, 64, 3, 6, 3, 0, 63, 60, 1, 0, 0, 0, 63, 61,
		1, 0, 0, 0, 63, 62, 1, 0, 0, 0, 64, 11, 1, 0, 0, 0, 65, 68, 3, 14, 7, 0,
		66, 68, 5, 13, 0, 0, 67, 65, 1, 0, 0, 0, 67, 66, 1, 0, 0, 0, 68, 76, 1,
		0, 0, 0, 69, 72, 5, 2, 0, 0, 70, 73, 3, 14, 7, 0, 71, 73, 5, 13, 0, 0,
		72, 70, 1, 0, 0, 0, 72, 71, 1, 0, 0, 0, 73, 75, 1, 0, 0, 0, 74, 69, 1,
		0, 0, 0, 75, 78, 1, 0, 0, 0, 76, 74, 1, 0, 0, 0, 76, 77, 1, 0, 0, 0, 77,
		80, 1, 0, 0, 0, 78, 76, 1, 0, 0, 0, 79, 67, 1, 0, 0, 0, 79, 80, 1, 0, 0,
		0, 80, 13, 1, 0, 0, 0, 81, 82, 5, 12, 0, 0, 82, 83, 5, 13, 0, 0, 83, 84,
		5, 12, 0, 0, 84, 15, 1, 0, 0, 0, 10, 23, 26, 32, 54, 58, 63, 67, 72, 76,
		79,
	}
	deserializer := antlr.NewATNDeserializer(nil)
	staticData.atn = deserializer.Deserialize(staticData.serializedATN)
	atn := staticData.atn
	staticData.decisionToDFA = make([]*antlr.DFA, len(atn.DecisionToState))
	decisionToDFA := staticData.decisionToDFA
	for index, state := range atn.DecisionToState {
		decisionToDFA[index] = antlr.NewDFA(state, index)
	}
}

// EffectParserInit initializes any static state used to implement EffectParser. By default the
// static state used to implement the parser is lazily initialized during the first call to
// NewEffectParser(). You can call this function if you wish to initialize the static state ahead
// of time.
func EffectParserInit() {
	staticData := &EffectParserStaticData
	staticData.once.Do(effectParserInit)
}

// NewEffectParser produces a new parser instance for the optional input antlr.TokenStream.
func NewEffectParser(input antlr.TokenStream) *EffectParser {
	EffectParserInit()
	this := new(EffectParser)
	this.BaseParser = antlr.NewBaseParser(input)
	staticData := &EffectParserStaticData
	this.Interpreter = antlr.NewParserATNSimulator(this, staticData.atn, staticData.decisionToDFA, staticData.PredictionContextCache)
	this.RuleNames = staticData.RuleNames
	this.LiteralNames = staticData.LiteralNames
	this.SymbolicNames = staticData.SymbolicNames
	this.GrammarFileName = "Effect.g4"

	return this
}

// EffectParser tokens.
const (
	EffectParserEOF        = antlr.TokenEOF
	EffectParserT__0       = 1
	EffectParserT__1       = 2
	EffectParserT__2       = 3
	EffectParserT__3       = 4
	EffectParserT__4       = 5
	EffectParserT__5       = 6
	EffectParserT__6       = 7
	EffectParserT__7       = 8
	EffectParserT__8       = 9
	EffectParserT__9       = 10
	EffectParserT__10      = 11
	EffectParserT__11      = 12
	EffectParserIDENTIFIER = 13
	EffectParserWS         = 14
)

// EffectParser rules.
const (
	EffectParserRULE_effect      = 0
	EffectParserRULE_read        = 1
	EffectParserRULE_update      = 2
	EffectParserRULE_temporal    = 3
	EffectParserRULE_concrete    = 4
	EffectParserRULE_component   = 5
	EffectParserRULE_entity      = 6
	EffectParserRULE_stateVarRef = 7
)

// IEffectContext is an interface to support dynamic dispatch.
type IEffectContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser
	// IsEffectContext differentiates from other interfaces.
	IsEffectContext()
}

type EffectContext struct {
	antlr.BaseParserRuleContext
	parser antlr.Parser
}

func NewEmptyEffectContext() *EffectContext {
	var p = new(EffectContext)
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = EffectParserRULE_effect
	return p
}

func InitEmptyEffectContext(p *EffectContext) {
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = EffectParserRULE_effect
}

func (*EffectContext) IsEffectContext() {}

func NewEffectContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *EffectContext {
	var p = new(EffectContext)

	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, parent, invokingState)

	p.parser = parser
	p.RuleIndex = EffectParserRULE_effect

	return p
}

func (s *EffectContext) GetParser() antlr.Parser { return s.parser }

func (s *EffectContext) CopyAll(ctx *EffectContext) {
	s.CopyFrom(&ctx.BaseParserRuleContext)
}

func (s *EffectContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *EffectContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

type VariableEffectContext struct {
	EffectContext
}

func NewVariableEffectContext(parser antlr.Parser, ctx antlr.ParserRuleContext) *VariableEffectContext {
	var p = new(VariableEffectContext)

	InitEmptyEffectContext(&p.EffectContext)
	p.parser = parser
	p.CopyAll(ctx.(*EffectContext))

	return p
}

func (s *VariableEffectContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *VariableEffectContext) IDENTIFIER() antlr.TerminalNode {
	return s.GetToken(EffectParserIDENTIFIER, 0)
}

func (s *VariableEffectContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(EffectListener); ok {
		listenerT.EnterVariableEffect(s)
	}
}

func (s *VariableEffectContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(EffectListener); ok {
		listenerT.ExitVariableEffect(s)
	}
}

func (s *VariableEffectContext) Accept(visitor antlr.ParseTreeVisitor) interface{} {
	switch t := visitor.(type) {
	case EffectVisitor:
		return t.VisitVariableEffect(s)

	default:
		return t.VisitChildren(s)
	}
}

type ArrowEffectContext struct {
	EffectContext
}

func NewArrowEffectContext(parser antlr.Parser, ctx antlr.ParserRuleContext) *ArrowEffectContext {
	var p = new(ArrowEffectContext)

	InitEmptyEffectContext(&p.EffectContext)
	p.parser = parser
	p.CopyAll(ctx.(*EffectContext))

	return p
}

func (s *ArrowEffectContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *ArrowEffectContext) AllEffect() []IEffectContext {
	children := s.GetChildren()
	len := 0
	for _, ctx := range children {
		if _, ok := ctx.(IEffectContext); ok {
			len++
		}
	}

	tst := make([]IEffectContext, len)
	i := 0
	for _, ctx := range children {
		if t, ok := ctx.(IEffectContext); ok {
			tst[i] = t.(IEffectContext)
			i++
		}
	}

	return tst
}

func (s *ArrowEffectContext) Effect(i int) IEffectContext {
	var t antlr.RuleContext
	j := 0
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IEffectContext); ok {
			if j == i {
				t = ctx.(antlr.RuleContext)
				break
			}
			j++
		}
	}

	if t == nil {
		return nil
	}

	return t.(IEffectContext)
}

func (s *ArrowEffectContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(EffectListener); ok {
		listenerT.EnterArrowEffect(s)
	}
}

func (s *ArrowEffectContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(EffectListener); ok {
		listenerT.ExitArrowEffect(s)
	}
}

func (s *ArrowEffectContext) Accept(visitor antlr.ParseTreeVisitor) interface{} {
	switch t := visitor.(type) {
	case EffectVisitor:
		return t.VisitArrowEffect(s)

	default:
		return t.VisitChildren(s)
	}
}

type ConcreteEffectContext struct {
	EffectContext
}

func NewConcreteEffectContext(parser antlr.Parser, ctx antlr.ParserRuleContext) *ConcreteEffectContext {
	var p = new(ConcreteEffectContext)

	InitEmptyEffectContext(&p.EffectContext)
	p.parser = parser
	p.CopyAll(ctx.(*EffectContext))

	return p
}

func (s *ConcreteEffectContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *ConcreteEffectContext) Concrete() IConcreteContext {
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IConcreteContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

	if t == nil {
		return nil
	}

	return t.(IConcreteContext)
}

func (s *ConcreteEffectContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(EffectListener); ok {
		listenerT.EnterConcreteEffect(s)
	}
}

func (s *ConcreteEffectContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(EffectListener); ok {
		listenerT.ExitConcreteEffect(s)
	}
}

func (s *ConcreteEffectContext) Accept(visitor antlr.ParseTreeVisitor) interface{} {
	switch t := visitor.(type) {
	case EffectVisitor:
		return t.VisitConcreteEffect(s)

	default:
		return t.VisitChildren(s)
	}
}

func (p *EffectParser) Effect() (localctx IEffectContext) {
	localctx = NewEffectContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 0, EffectParserRULE_effect)
	var _la int

	p.SetState(32)
	p.GetErrorHandler().Sync(p)
	if p.HasError() {
		goto errorExit
	}

	switch p.GetTokenStream().LA(1) {
	case EffectParserT__4, EffectParserT__7, EffectParserT__8, EffectParserT__10:
		localctx = NewConcreteEffectContext(p, localctx)
		p.EnterOuterAlt(localctx, 1)
		{
			p.SetState(16)
			p.Concrete()
		}

	case EffectParserT__0:
		localctx = NewArrowEffectContext(p, localctx)
		p.EnterOuterAlt(localctx, 2)
		{
			p.SetState(17)
			p.Match(EffectParserT__0)
			if p.HasError() {
				// Recognition error - abort rule
				goto errorExit
			}
		}
		p.SetState(26)
		p.GetErrorHandler().Sync(p)
		if p.HasError() {
			goto errorExit
		}
		_la = p.GetTokenStream().LA(1)

		if (int64(_la) & ^0x3f) == 0 && ((int64(1)<<_la)&11042) != 0 {
			{
				p.SetState(18)
				p.Effect()
			}
			p.SetState(23)
			p.GetErrorHandler().Sync(p)
			if p.HasError() {
				goto errorExit
			}
			_la = p.GetTokenStream().LA(1)

			for _la == EffectParserT__1 {
				{
					p.SetState(19)
					p.Match(EffectParserT__1)
					if p.HasError() {
						// Recognition error - abort rule
						goto errorExit
					}
				}
				{
					p.SetState(20)
					p.Effect()
				}

				p.SetState(25)
				p.GetErrorHandler().Sync(p)
				if p.HasError() {
					goto errorExit
				}
				_la = p.GetTokenStream().LA(1)
			}

		}
		{
			p.SetState(28)
			p.Match(EffectParserT__2)
			if p.HasError() {
				// Recognition error - abort rule
				goto errorExit
			}
		}
		{
			p.SetState(29)
			p.Match(EffectParserT__3)
			if p.HasError() {
				// Recognition error - abort rule
				goto errorExit
			}
		}
		{
			p.SetState(30)
			p.Effect()
		}

	case EffectParserIDENTIFIER:
		localctx = NewVariableEffectContext(p, localctx)
		p.EnterOuterAlt(localctx, 3)
		{
			p.SetState(31)
			p.Match(EffectParserIDENTIFIER)
			if p.HasError() {
				// Recognition error - abort rule
				goto errorExit
			}
		}

	default:
		p.SetError(antlr.NewNoViableAltException(p, nil, nil, nil, nil, nil))
		goto errorExit
	}

errorExit:
	if p.HasError() {
		v := p.GetError()
		localctx.SetException(v)
		p.GetErrorHandler().ReportError(p, v)
		p.GetErrorHandler().Recover(p, v)
		p.SetError(nil)
	}
	p.ExitRule()
	return localctx
	goto errorExit // Trick to prevent compiler error if the label is not used
}

// IReadContext is an interface to support dynamic dispatch.
type IReadContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// Getter signatures
	Entity() IEntityContext

	// IsReadContext differentiates from other interfaces.
	IsReadContext()
}

type ReadContext struct {
	antlr.BaseParserRuleContext
	parser antlr.Parser
}

func NewEmptyReadContext() *ReadContext {
	var p = new(ReadContext)
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = EffectParserRULE_read
	return p
}

func InitEmptyReadContext(p *ReadContext) {
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = EffectParserRULE_read
}

func (*ReadContext) IsReadContext() {}

func NewReadContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *ReadContext {
	var p = new(ReadContext)

	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, parent, invokingState)

	p.parser = parser
	p.RuleIndex = EffectParserRULE_read

	return p
}

func (s *ReadContext) GetParser() antlr.Parser { return s.parser }

func (s *ReadContext) Entity() IEntityContext {
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IEntityContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

	if t == nil {
		return nil
	}

	return t.(IEntityContext)
}

func (s *ReadContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *ReadContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

func (s *ReadContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(EffectListener); ok {
		listenerT.EnterRead(s)
	}
}

func (s *ReadContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(EffectListener); ok {
		listenerT.ExitRead(s)
	}
}

func (s *ReadContext) Accept(visitor antlr.ParseTreeVisitor) interface{} {
	switch t := visitor.(type) {
	case EffectVisitor:
		return t.VisitRead(s)

	default:
		return t.VisitChildren(s)
	}
}

func (p *EffectParser) Read() (localctx IReadContext) {
	localctx = NewReadContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 2, EffectParserRULE_read)
	p.EnterOuterAlt(localctx, 1)
	{
		p.SetState(34)
		p.Match(EffectParserT__4)
		if p.HasError() {
			// Recognition error - abort rule
			goto errorExit
		}
	}
	{
		p.SetState(35)
		p.Match(EffectParserT__5)
		if p.HasError() {
			// Recognition error - abort rule
			goto errorExit
		}
	}
	{
		p.SetState(36)
		p.Entity()
	}
	{
		p.SetState(37)
		p.Match(EffectParserT__6)
		if p.HasError() {
			// Recognition error - abort rule
			goto errorExit
		}
	}

errorExit:
	if p.HasError() {
		v := p.GetError()
		localctx.SetException(v)
		p.GetErrorHandler().ReportError(p, v)
		p.GetErrorHandler().Recover(p, v)
		p.SetError(nil)
	}
	p.ExitRule()
	return localctx
	goto errorExit // Trick to prevent compiler error if the label is not used
}

// IUpdateContext is an interface to support dynamic dispatch.
type IUpdateContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// Getter signatures
	Entity() IEntityContext

	// IsUpdateContext differentiates from other interfaces.
	IsUpdateContext()
}

type UpdateContext struct {
	antlr.BaseParserRuleContext
	parser antlr.Parser
}

func NewEmptyUpdateContext() *UpdateContext {
	var p = new(UpdateContext)
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = EffectParserRULE_update
	return p
}

func InitEmptyUpdateContext(p *UpdateContext) {
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = EffectParserRULE_update
}

func (*UpdateContext) IsUpdateContext() {}

func NewUpdateContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *UpdateContext {
	var p = new(UpdateContext)

	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, parent, invokingState)

	p.parser = parser
	p.RuleIndex = EffectParserRULE_update

	return p
}

func (s *UpdateContext) GetParser() antlr.Parser { return s.parser }

func (s *UpdateContext) Entity() IEntityContext {
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IEntityContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

	if t == nil {
		return nil
	}

	return t.(IEntityContext)
}

func (s *UpdateContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *UpdateContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

func (s *UpdateContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(EffectListener); ok {
		listenerT.EnterUpdate(s)
	}
}

func (s *UpdateContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(EffectListener); ok {
		listenerT.ExitUpdate(s)
	}
}

func (s *UpdateContext) Accept(visitor antlr.ParseTreeVisitor) interface{} {
	switch t := visitor.(type) {
	case EffectVisitor:
		return t.VisitUpdate(s)

	default:
		return t.VisitChildren(s)
	}
}

func (p *EffectParser) Update() (localctx IUpdateContext) {
	localctx = NewUpdateContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 4, EffectParserRULE_update)
	p.EnterOuterAlt(localctx, 1)
	{
		p.SetState(39)
		p.Match(EffectParserT__7)
		if p.HasError() {
			// Recognition error - abort rule
			goto errorExit
		}
	}
	{
		p.SetState(40)
		p.Match(EffectParserT__5)
		if p.HasError() {
			// Recognition error - abort rule
			goto errorExit
		}
	}
	{
		p.SetState(41)
		p.Entity()
	}
	{
		p.SetState(42)
		p.Match(EffectParserT__6)
		if p.HasError() {
			// Recognition error - abort rule
			goto errorExit
		}
	}

errorExit:
	if p.HasError() {
		v := p.GetError()
		localctx.SetException(v)
		p.GetErrorHandler().ReportError(p, v)
		p.GetErrorHandler().Recover(p, v)
		p.SetError(nil)
	}
	p.ExitRule()
	return localctx
	goto errorExit // Trick to prevent compiler error if the label is not used
}

// ITemporalContext is an interface to support dynamic dispatch.
type ITemporalContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// Getter signatures
	Entity() IEntityContext

	// IsTemporalContext differentiates from other interfaces.
	IsTemporalContext()
}

type TemporalContext struct {
	antlr.BaseParserRuleContext
	parser antlr.Parser
}

func NewEmptyTemporalContext() *TemporalContext {
	var p = new(TemporalContext)
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = EffectParserRULE_temporal
	return p
}

func InitEmptyTemporalContext(p *TemporalContext) {
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = EffectParserRULE_temporal
}

func (*TemporalContext) IsTemporalContext() {}

func NewTemporalContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *TemporalContext {
	var p = new(TemporalContext)

	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, parent, invokingState)

	p.parser = parser
	p.RuleIndex = EffectParserRULE_temporal

	return p
}

func (s *TemporalContext) GetParser() antlr.Parser { return s.parser }

func (s *TemporalContext) Entity() IEntityContext {
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IEntityContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

	if t == nil {
		return nil
	}

	return t.(IEntityContext)
}

func (s *TemporalContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *TemporalContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

func (s *TemporalContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(EffectListener); ok {
		listenerT.EnterTemporal(s)
	}
}

func (s *TemporalContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(EffectListener); ok {
		listenerT.ExitTemporal(s)
	}
}

func (s *TemporalContext) Accept(visitor antlr.ParseTreeVisitor) interface{} {
	switch t := visitor.(type) {
	case EffectVisitor:
		return t.VisitTemporal(s)

	default:
		return t.VisitChildren(s)
	}
}

func (p *EffectParser) Temporal() (localctx ITemporalContext) {
	localctx = NewTemporalContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 6, EffectParserRULE_temporal)
	p.EnterOuterAlt(localctx, 1)
	{
		p.SetState(44)
		p.Match(EffectParserT__8)
		if p.HasError() {
			// Recognition error - abort rule
			goto errorExit
		}
	}
	{
		p.SetState(45)
		p.Match(EffectParserT__5)
		if p.HasError() {
			// Recognition error - abort rule
			goto errorExit
		}
	}
	{
		p.SetState(46)
		p.Entity()
	}
	{
		p.SetState(47)
		p.Match(EffectParserT__6)
		if p.HasError() {
			// Recognition error - abort rule
			goto errorExit
		}
	}

errorExit:
	if p.HasError() {
		v := p.GetError()
		localctx.SetException(v)
		p.GetErrorHandler().ReportError(p, v)
		p.GetErrorHandler().Recover(p, v)
		p.SetError(nil)
	}
	p.ExitRule()
	return localctx
	goto errorExit // Trick to prevent compiler error if the label is not used
}

// IConcreteContext is an interface to support dynamic dispatch.
type IConcreteContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser
	// IsConcreteContext differentiates from other interfaces.
	IsConcreteContext()
}

type ConcreteContext struct {
	antlr.BaseParserRuleContext
	parser antlr.Parser
}

func NewEmptyConcreteContext() *ConcreteContext {
	var p = new(ConcreteContext)
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = EffectParserRULE_concrete
	return p
}

func InitEmptyConcreteContext(p *ConcreteContext) {
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = EffectParserRULE_concrete
}

func (*ConcreteContext) IsConcreteContext() {}

func NewConcreteContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *ConcreteContext {
	var p = new(ConcreteContext)

	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, parent, invokingState)

	p.parser = parser
	p.RuleIndex = EffectParserRULE_concrete

	return p
}

func (s *ConcreteContext) GetParser() antlr.Parser { return s.parser }

func (s *ConcreteContext) CopyAll(ctx *ConcreteContext) {
	s.CopyFrom(&ctx.BaseParserRuleContext)
}

func (s *ConcreteContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *ConcreteContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

type ConcreteComponentsContext struct {
	ConcreteContext
}

func NewConcreteComponentsContext(parser antlr.Parser, ctx antlr.ParserRuleContext) *ConcreteComponentsContext {
	var p = new(ConcreteComponentsContext)

	InitEmptyConcreteContext(&p.ConcreteContext)
	p.parser = parser
	p.CopyAll(ctx.(*ConcreteContext))

	return p
}

func (s *ConcreteComponentsContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *ConcreteComponentsContext) AllComponent() []IComponentContext {
	children := s.GetChildren()
	len := 0
	for _, ctx := range children {
		if _, ok := ctx.(IComponentContext); ok {
			len++
		}
	}

	tst := make([]IComponentContext, len)
	i := 0
	for _, ctx := range children {
		if t, ok := ctx.(IComponentContext); ok {
			tst[i] = t.(IComponentContext)
			i++
		}
	}

	return tst
}

func (s *ConcreteComponentsContext) Component(i int) IComponentContext {
	var t antlr.RuleContext
	j := 0
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IComponentContext); ok {
			if j == i {
				t = ctx.(antlr.RuleContext)
				break
			}
			j++
		}
	}

	if t == nil {
		return nil
	}

	return t.(IComponentContext)
}

func (s *ConcreteComponentsContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(EffectListener); ok {
		listenerT.EnterConcreteComponents(s)
	}
}

func (s *ConcreteComponentsContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(EffectListener); ok {
		listenerT.ExitConcreteComponents(s)
	}
}

func (s *ConcreteComponentsContext) Accept(visitor antlr.ParseTreeVisitor) interface{} {
	switch t := visitor.(type) {
	case EffectVisitor:
		return t.VisitConcreteComponents(s)

	default:
		return t.VisitChildren(s)
	}
}

type PureContext struct {
	ConcreteContext
}

func NewPureContext(parser antlr.Parser, ctx antlr.ParserRuleContext) *PureContext {
	var p = new(PureContext)

	InitEmptyConcreteContext(&p.ConcreteContext)
	p.parser = parser
	p.CopyAll(ctx.(*ConcreteContext))

	return p
}

func (s *PureContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *PureContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(EffectListener); ok {
		listenerT.EnterPure(s)
	}
}

func (s *PureContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(EffectListener); ok {
		listenerT.ExitPure(s)
	}
}

func (s *PureContext) Accept(visitor antlr.ParseTreeVisitor) interface{} {
	switch t := visitor.(type) {
	case EffectVisitor:
		return t.VisitPure(s)

	default:
		return t.VisitChildren(s)
	}
}

func (p *EffectParser) Concrete() (localctx IConcreteContext) {
	localctx = NewConcreteContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 8, EffectParserRULE_concrete)
	var _la int

	p.SetState(58)
	p.GetErrorHandler().Sync(p)
	if p.HasError() {
		goto errorExit
	}

	switch p.GetTokenStream().LA(1) {
	case EffectParserT__4, EffectParserT__7, EffectParserT__8:
		localctx = NewConcreteComponentsContext(p, localctx)
		p.EnterOuterAlt(localctx, 1)
		{
			p.SetState(49)
			p.Component()
		}
		p.SetState(54)
		p.GetErrorHandler().Sync(p)
		if p.HasError() {
			goto errorExit
		}
		_la = p.GetTokenStream().LA(1)

		for _la == EffectParserT__9 {
			{
				p.SetState(50)
				p.Match(EffectParserT__9)
				if p.HasError() {
					// Recognition error - abort rule
					goto errorExit
				}
			}
			{
				p.SetState(51)
				p.Component()
			}

			p.SetState(56)
			p.GetErrorHandler().Sync(p)
			if p.HasError() {
				goto errorExit
			}
			_la = p.GetTokenStream().LA(1)
		}

	case EffectParserT__10:
		localctx = NewPureContext(p, localctx)
		p.EnterOuterAlt(localctx, 2)
		{
			p.SetState(57)
			p.Match(EffectParserT__10)
			if p.HasError() {
				// Recognition error - abort rule
				goto errorExit
			}
		}

	default:
		p.SetError(antlr.NewNoViableAltException(p, nil, nil, nil, nil, nil))
		goto errorExit
	}

errorExit:
	if p.HasError() {
		v := p.GetError()
		localctx.SetException(v)
		p.GetErrorHandler().ReportError(p, v)
		p.GetErrorHandler().Recover(p, v)
		p.SetError(nil)
	}
	p.ExitRule()
	return localctx
	goto errorExit // Trick to prevent compiler error if the label is not used
}

// IComponentContext is an interface to support dynamic dispatch.
type IComponentContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser
	// IsComponentContext differentiates from other interfaces.
	IsComponentContext()
}

type ComponentContext struct {
	antlr.BaseParserRuleContext
	parser antlr.Parser
}

func NewEmptyComponentContext() *ComponentContext {
	var p = new(ComponentContext)
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = EffectParserRULE_component
	return p
}

func InitEmptyComponentContext(p *ComponentContext) {
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = EffectParserRULE_component
}

func (*ComponentContext) IsComponentContext() {}

func NewComponentContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *ComponentContext {
	var p = new(ComponentContext)

	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, parent, invokingState)

	p.parser = parser
	p.RuleIndex = EffectParserRULE_component

	return p
}

func (s *ComponentContext) GetParser() antlr.Parser { return s.parser }

func (s *ComponentContext) CopyAll(ctx *ComponentContext) {
	s.CopyFrom(&ctx.BaseParserRuleContext)
}

func (s *ComponentContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *ComponentContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

type TemporalComponentContext struct {
	ComponentContext
}

func NewTemporalComponentContext(parser antlr.Parser, ctx antlr.ParserRuleContext) *TemporalComponentContext {
	var p = new(TemporalComponentContext)

	InitEmptyComponentContext(&p.ComponentContext)
	p.parser = parser
	p.CopyAll(ctx.(*ComponentContext))

	return p
}

func (s *TemporalComponentContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *TemporalComponentContext) Temporal() ITemporalContext {
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(ITemporalContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

	if t == nil {
		return nil
	}

	return t.(ITemporalContext)
}

func (s *TemporalComponentContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(EffectListener); ok {
		listenerT.EnterTemporalComponent(s)
	}
}

func (s *TemporalComponentContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(EffectListener); ok {
		listenerT.ExitTemporalComponent(s)
	}
}

func (s *TemporalComponentContext) Accept(visitor antlr.ParseTreeVisitor) interface{} {
	switch t := visitor.(type) {
	case EffectVisitor:
		return t.VisitTemporalComponent(s)

	default:
		return t.VisitChildren(s)
	}
}

type UpdateComponentContext struct {
	ComponentContext
}

func NewUpdateComponentContext(parser antlr.Parser, ctx antlr.ParserRuleContext) *UpdateComponentContext {
	var p = new(UpdateComponentContext)

	InitEmptyComponentContext(&p.ComponentContext)
	p.parser = parser
	p.CopyAll(ctx.(*ComponentContext))

	return p
}

func (s *UpdateComponentContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *UpdateComponentContext) Update() IUpdateContext {
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IUpdateContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

	if t == nil {
		return nil
	}

	return t.(IUpdateContext)
}

func (s *UpdateComponentContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(EffectListener); ok {
		listenerT.EnterUpdateComponent(s)
	}
}

func (s *UpdateComponentContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(EffectListener); ok {
		listenerT.ExitUpdateComponent(s)
	}
}

func (s *UpdateComponentContext) Accept(visitor antlr.ParseTreeVisitor) interface{} {
	switch t := visitor.(type) {
	case EffectVisitor:
		return t.VisitUpdateComponent(s)

	default:
		return t.VisitChildren(s)
	}
}

type ReadComponentContext struct {
	ComponentContext
}

func NewReadComponentContext(parser antlr.Parser, ctx antlr.ParserRuleContext) *ReadComponentContext {
	var p = new(ReadComponentContext)

	InitEmptyComponentContext(&p.ComponentContext)
	p.parser = parser
	p.CopyAll(ctx.(*ComponentContext))

	return p
}

func (s *ReadComponentContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *ReadComponentContext) Read() IReadContext {
	var t antlr.RuleContext
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IReadContext); ok {
			t = ctx.(antlr.RuleContext)
			break
		}
	}

	if t == nil {
		return nil
	}

	return t.(IReadContext)
}

func (s *ReadComponentContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(EffectListener); ok {
		listenerT.EnterReadComponent(s)
	}
}

func (s *ReadComponentContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(EffectListener); ok {
		listenerT.ExitReadComponent(s)
	}
}

func (s *ReadComponentContext) Accept(visitor antlr.ParseTreeVisitor) interface{} {
	switch t := visitor.(type) {
	case EffectVisitor:
		return t.VisitReadComponent(s)

	default:
		return t.VisitChildren(s)
	}
}

func (p *EffectParser) Component() (localctx IComponentContext) {
	localctx = NewComponentContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 10, EffectParserRULE_component)
	p.SetState(63)
	p.GetErrorHandler().Sync(p)
	if p.HasError() {
		goto errorExit
	}

	switch p.GetTokenStream().LA(1) {
	case EffectParserT__4:
		localctx = NewReadComponentContext(p, localctx)
		p.EnterOuterAlt(localctx, 1)
		{
			p.SetState(60)
			p.Read()
		}

	case EffectParserT__7:
		localctx = NewUpdateComponentContext(p, localctx)
		p.EnterOuterAlt(localctx, 2)
		{
			p.SetState(61)
			p.Update()
		}

	case EffectParserT__8:
		localctx = NewTemporalComponentContext(p, localctx)
		p.EnterOuterAlt(localctx, 3)
		{
			p.SetState(62)
			p.Temporal()
		}

	default:
		p.SetError(antlr.NewNoViableAltException(p, nil, nil, nil, nil, nil))
		goto errorExit
	}

errorExit:
	if p.HasError() {
		v := p.GetError()
		localctx.SetException(v)
		p.GetErrorHandler().ReportError(p, v)
		p.GetErrorHandler().Recover(p, v)
		p.SetError(nil)
	}
	p.ExitRule()
	return localctx
	goto errorExit // Trick to prevent compiler error if the label is not used
}

// IEntityContext is an interface to support dynamic dispatch.
type IEntityContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// Getter signatures
	AllStateVarRef() []IStateVarRefContext
	StateVarRef(i int) IStateVarRefContext
	AllIDENTIFIER() []antlr.TerminalNode
	IDENTIFIER(i int) antlr.TerminalNode

	// IsEntityContext differentiates from other interfaces.
	IsEntityContext()
}

type EntityContext struct {
	antlr.BaseParserRuleContext
	parser antlr.Parser
}

func NewEmptyEntityContext() *EntityContext {
	var p = new(EntityContext)
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = EffectParserRULE_entity
	return p
}

func InitEmptyEntityContext(p *EntityContext) {
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = EffectParserRULE_entity
}

func (*EntityContext) IsEntityContext() {}

func NewEntityContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *EntityContext {
	var p = new(EntityContext)

	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, parent, invokingState)

	p.parser = parser
	p.RuleIndex = EffectParserRULE_entity

	return p
}

func (s *EntityContext) GetParser() antlr.Parser { return s.parser }

func (s *EntityContext) AllStateVarRef() []IStateVarRefContext {
	children := s.GetChildren()
	len := 0
	for _, ctx := range children {
		if _, ok := ctx.(IStateVarRefContext); ok {
			len++
		}
	}

	tst := make([]IStateVarRefContext, len)
	i := 0
	for _, ctx := range children {
		if t, ok := ctx.(IStateVarRefContext); ok {
			tst[i] = t.(IStateVarRefContext)
			i++
		}
	}

	return tst
}

func (s *EntityContext) StateVarRef(i int) IStateVarRefContext {
	var t antlr.RuleContext
	j := 0
	for _, ctx := range s.GetChildren() {
		if _, ok := ctx.(IStateVarRefContext); ok {
			if j == i {
				t = ctx.(antlr.RuleContext)
				break
			}
			j++
		}
	}

	if t == nil {
		return nil
	}

	return t.(IStateVarRefContext)
}

func (s *EntityContext) AllIDENTIFIER() []antlr.TerminalNode {
	return s.GetTokens(EffectParserIDENTIFIER)
}

func (s *EntityContext) IDENTIFIER(i int) antlr.TerminalNode {
	return s.GetToken(EffectParserIDENTIFIER, i)
}

func (s *EntityContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *EntityContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

func (s *EntityContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(EffectListener); ok {
		listenerT.EnterEntity(s)
	}
}

func (s *EntityContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(EffectListener); ok {
		listenerT.ExitEntity(s)
	}
}

func (s *EntityContext) Accept(visitor antlr.ParseTreeVisitor) interface{} {
	switch t := visitor.(type) {
	case EffectVisitor:
		return t.VisitEntity(s)

	default:
		return t.VisitChildren(s)
	}
}

func (p *EffectParser) Entity() (localctx IEntityContext) {
	localctx = NewEntityContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 12, EffectParserRULE_entity)
	var _la int

	p.EnterOuterAlt(localctx, 1)
	p.SetState(79)
	p.GetErrorHandler().Sync(p)
	if p.HasError() {
		goto errorExit
	}
	_la = p.GetTokenStream().LA(1)

	if _la == EffectParserT__11 || _la == EffectParserIDENTIFIER {
		p.SetState(67)
		p.GetErrorHandler().Sync(p)
		if p.HasError() {
			goto errorExit
		}

		switch p.GetTokenStream().LA(1) {
		case EffectParserT__11:
			{
				p.SetState(65)
				p.StateVarRef()
			}

		case EffectParserIDENTIFIER:
			{
				p.SetState(66)
				p.Match(EffectParserIDENTIFIER)
				if p.HasError() {
					// Recognition error - abort rule
					goto errorExit
				}
			}

		default:
			p.SetError(antlr.NewNoViableAltException(p, nil, nil, nil, nil, nil))
			goto errorExit
		}
		p.SetState(76)
		p.GetErrorHandler().Sync(p)
		if p.HasError() {
			goto errorExit
		}
		_la = p.GetTokenStream().LA(1)

		for _la == EffectParserT__1 {
			{
				p.SetState(69)
				p.Match(EffectParserT__1)
				if p.HasError() {
					// Recognition error - abort rule
					goto errorExit
				}
			}
			p.SetState(72)
			p.GetErrorHandler().Sync(p)
			if p.HasError() {
				goto errorExit
			}

			switch p.GetTokenStream().LA(1) {
			case EffectParserT__11:
				{
					p.SetState(70)
					p.StateVarRef()
				}

			case EffectParserIDENTIFIER:
				{
					p.SetState(71)
					p.Match(EffectParserIDENTIFIER)
					if p.HasError() {
						// Recognition error - abort rule
						goto errorExit
					}
				}

			default:
				p.SetError(antlr.NewNoViableAltException(p, nil, nil, nil, nil, nil))
				goto errorExit
			}

			p.SetState(78)
			p.GetErrorHandler().Sync(p)
			if p.HasError() {
				goto errorExit
			}
			_la = p.GetTokenStream().LA(1)
		}

	}

errorExit:
	if p.HasError() {
		v := p.GetError()
		localctx.SetException(v)
		p.GetErrorHandler().ReportError(p, v)
		p.GetErrorHandler().Recover(p, v)
		p.SetError(nil)
	}
	p.ExitRule()
	return localctx
	goto errorExit // Trick to prevent compiler error if the label is not used
}

// IStateVarRefContext is an interface to support dynamic dispatch.
type IStateVarRefContext interface {
	antlr.ParserRuleContext

	// GetParser returns the parser.
	GetParser() antlr.Parser

	// Getter signatures
	IDENTIFIER() antlr.TerminalNode

	// IsStateVarRefContext differentiates from other interfaces.
	IsStateVarRefContext()
}

type StateVarRefContext struct {
	antlr.BaseParserRuleContext
	parser antlr.Parser
}

func NewEmptyStateVarRefContext() *StateVarRefContext {
	var p = new(StateVarRefContext)
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = EffectParserRULE_stateVarRef
	return p
}

func InitEmptyStateVarRefContext(p *StateVarRefContext) {
	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, nil, -1)
	p.RuleIndex = EffectParserRULE_stateVarRef
}

func (*StateVarRefContext) IsStateVarRefContext() {}

func NewStateVarRefContext(parser antlr.Parser, parent antlr.ParserRuleContext, invokingState int) *StateVarRefContext {
	var p = new(StateVarRefContext)

	antlr.InitBaseParserRuleContext(&p.BaseParserRuleContext, parent, invokingState)

	p.parser = parser
	p.RuleIndex = EffectParserRULE_stateVarRef

	return p
}

func (s *StateVarRefContext) GetParser() antlr.Parser { return s.parser }

func (s *StateVarRefContext) IDENTIFIER() antlr.TerminalNode {
	return s.GetToken(EffectParserIDENTIFIER, 0)
}

func (s *StateVarRefContext) GetRuleContext() antlr.RuleContext {
	return s
}

func (s *StateVarRefContext) ToStringTree(ruleNames []string, recog antlr.Recognizer) string {
	return antlr.TreesStringTree(s, ruleNames, recog)
}

func (s *StateVarRefContext) EnterRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(EffectListener); ok {
		listenerT.EnterStateVarRef(s)
	}
}

func (s *StateVarRefContext) ExitRule(listener antlr.ParseTreeListener) {
	if listenerT, ok := listener.(EffectListener); ok {
		listenerT.ExitStateVarRef(s)
	}
}

func (s *StateVarRefContext) Accept(visitor antlr.ParseTreeVisitor) interface{} {
	switch t := visitor.(type) {
	case EffectVisitor:
		return t.VisitStateVarRef(s)

	default:
		return t.VisitChildren(s)
	}
}

func (p *EffectParser) StateVarRef() (localctx IStateVarRefContext) {
	localctx = NewStateVarRefContext(p, p.GetParserRuleContext(), p.GetState())
	p.EnterRule(localctx, 14, EffectParserRULE_stateVarRef)
	p.EnterOuterAlt(localctx, 1)
	{
		p.SetState(81)
		p.Match(EffectParserT__11)
		if p.HasError() {
			// Recognition error - abort rule
			goto errorExit
		}
	}
	{
		p.SetState(82)
		p.Match(EffectParserIDENTIFIER)
		if p.HasError() {
			// Recognition error - abort rule
			goto errorExit
		}
	}
	{
		p.SetState(83)
		p.Match(EffectParserT__11)
		if p.HasError() {
			// Recognition error - abort rule
			goto errorExit
		}
	}

errorExit:
	if p.HasError() {
		v := p.GetError()
		localctx.SetException(v)
		p.GetErrorHandler().ReportError(p, v)
		p.GetErrorHandler().Recover(p, v)
		p.SetError(nil)
	}
	p.ExitRule()
	return localctx
	goto errorExit // Trick to prevent compiler error if the label is not used
}
