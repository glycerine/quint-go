package quint

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/glycerine/insdict"
)

// REPLResult describes an accepted input. Kind is none, declaration, or value.
// A true value commits pending variable updates, just as the interactive
// TypeScript evaluator does. Missing lists variables left unset by that commit.
// OldState and NewState retain immutable snapshots immediately before and after
// a commit; they are valid only when Shifted is true.
type REPLResult struct {
	Kind     string
	Value    RuntimeValue
	Type     *TypeScheme
	Shifted  bool
	Missing  []string
	OldState RuntimeValue
	NewState RuntimeValue
}

// REPLSession retains declarations, static analysis, runtime state, and unique
// input IDs across sequential inputs. It is not safe for concurrent use.
// Invalid declarations never enter the session; statically accepted expressions
// enter its history even when evaluation reports a runtime error.
type REPLSession struct {
	ids      *IDGenerator
	modules  []Module
	main     string
	analysis AnalysisOutput
	runtime  *ModuleEvaluator
	sources  SourceMap
	code     *insdict.Dict[string, string]
	history  []string
	input    uint64
	loaded   string
	loadMain string
	saved    replSessionArchive
}

func NewREPLSession() *REPLSession {
	s := &REPLSession{}
	s.Reset()
	return s
}

// Reset clears declarations and state while preserving the current RNG state
// and last loaded file, so .reload remains available after .clear.
func (s *REPLSession) Reset() {
	seed := Integer{}
	if s.runtime != nil {
		seed = s.runtime.RNG.State()
	}
	s.ids = NewIDGenerator()
	s.main = "__repl__"
	s.modules = []Module{{ID: ID{}, Name: s.main, Declarations: []Declaration{}}}
	s.analysis = replCopyAnalysis(AnalysisOutput{})
	s.runtime, _ = NewModuleEvaluator(s.modules[0], NewRNG(seed), false)
	s.sources = SourceMap{}
	s.code = insdict.NewDict[string, string]()
	s.history = []string{}
	s.input = 0
	s.saved = replSessionArchive{}
}

func (s *REPLSession) Seed() Integer                { return s.runtime.RNG.State() }
func (s *REPLSession) SetSeed(seed Integer)         { s.runtime.RNG.SetState(seed) }
func (s *REPLSession) History() []string            { return append([]string{}, s.history...) }
func (s *REPLSession) State() (RuntimeValue, error) { return s.runtime.Storage.Record() }

// Evaluate parses, resolves and analyzes one expression or declaration batch.
// Inputs have distinct source names so diagnostics from separate inputs cannot
// accidentally resolve to an earlier line. Failed inputs may consume IDs.
func (s *REPLSession) Evaluate(ctx context.Context, text string) (REPLResult, error) {
	if err := ctx.Err(); err != nil {
		return REPLResult{}, err
	}
	s.saved.ensure()
	source := fmt.Sprintf("<input-%d>", s.input)
	s.input++
	s.code.Set(source, text)
	parsed, sources, err := ParseREPLWithSource(source, text, s.ids)
	for id, location := range sources {
		s.sources[id] = location
	}
	if err != nil {
		return REPLResult{}, err
	}
	if parsed.Kind == "none" {
		return REPLResult{Kind: "none"}, nil
	}
	if parsed.Kind == "declaration" {
		return s.addDeclarations(ctx, parsed.Declarations, text)
	}
	input := Declaration{ID: s.ids.Next(), Kind: "def", Qualifier: Action, Name: "q::input", Expr: parsed.Expr}
	candidate := append([]Module{}, s.modules...)
	for i, module := range candidate {
		if module.Name == s.main {
			candidate[i].Declarations = append(append([]Declaration{}, module.Declarations...), input)
		}
	}
	linked, err := ResolveLinkedModules(candidate)
	if err != nil {
		return REPLResult{}, err
	}
	if len(linked.Errors) > 0 {
		return REPLResult{}, linked.Errors[0]
	}
	analysis, err := AnalyzeIncremental(replCopyAnalysis(s.analysis), linked.Table, []Declaration{input})
	if err != nil {
		return REPLResult{}, err
	}
	if len(analysis.Diagnostics) > 0 {
		return REPLResult{}, analysis.Diagnostics[0]
	}
	output := AnalysisOutput{Types: analysis.Types, Effects: analysis.Effects, Modes: analysis.Modes}
	runtime, err := s.prepareRuntime(ctx, candidate, linked, output)
	if err != nil {
		return REPLResult{}, err
	}
	s.adoptRuntime(runtime)
	result := REPLResult{Kind: "value", Missing: []string{}}
	if scheme, ok := analysis.Types.Get2(parsed.Expr.ID.String()); ok {
		copied := SimplifyScheme(scheme)
		result.Type = &copied
	}
	s.history = append(s.history, strings.TrimSpace(text))
	s.saved.expressions = append(s.saved.expressions, strings.TrimSpace(text))
	result.Value, err = s.runtime.Evaluate(ctx, "q::input", nil)
	if err != nil {
		return result, err
	}
	if result.Value.expression.Kind == "bool" && result.Value.expression.Bool {
		updated := false
		for _, register := range s.runtime.Storage.next.All() {
			if register.Value.Valid() {
				updated = true
			} else {
				result.Missing = append(result.Missing, register.Name)
			}
		}
		if updated {
			oldState, err := s.runtime.Storage.Record()
			if err != nil {
				return result, err
			}
			if err = s.runtime.Shift(); err != nil {
				return result, err
			}
			newState, err := s.runtime.Storage.Record()
			if err != nil {
				return result, err
			}
			result.Shifted = true
			result.OldState, result.NewState = oldState, newState
		} else {
			result.Missing = []string{}
		}
	}
	return result, nil
}

func (s *REPLSession) addDeclarations(ctx context.Context, declarations []Declaration, text string) (REPLResult, error) {
	candidate := append([]Module{}, s.modules...)
	for i, module := range candidate {
		if module.Name == s.main {
			candidate[i].Declarations = append(append([]Declaration{}, module.Declarations...), declarations...)
		}
	}
	linked, err := ResolveLinkedModules(candidate)
	if err != nil {
		return REPLResult{}, err
	}
	if len(linked.Errors) > 0 {
		return REPLResult{}, linked.Errors[0]
	}
	analysis, err := AnalyzeIncremental(replCopyAnalysis(s.analysis), linked.Table, declarations)
	if err != nil {
		return REPLResult{}, err
	}
	if len(analysis.Diagnostics) > 0 {
		return REPLResult{}, analysis.Diagnostics[0]
	}
	output := AnalysisOutput{Types: analysis.Types, Effects: analysis.Effects, Modes: analysis.Modes}
	runtime, err := s.prepareRuntime(ctx, candidate, linked, output)
	if err != nil {
		return REPLResult{}, err
	}
	s.adoptRuntime(runtime)
	s.modules, s.analysis = candidate, output
	s.history = append(s.history, strings.TrimSpace(text))
	s.saved.declaration(text)
	return REPLResult{Kind: "declaration"}, nil
}

// adoptRuntime updates the compiled definitions while keeping the storage and
// trace owners stable. Previously returned lexical closures continue to see
// subsequent state shifts. Imported variables remain registered even when a
// later input does not reference them and flattening omits their declarations.
func (s *REPLSession) adoptRuntime(runtime *ModuleEvaluator) {
	for key, register := range runtime.Storage.current.All() {
		found := false
		for _, old := range s.runtime.Storage.current.All() {
			if old.Name == register.Name {
				found = true
				break
			}
		}
		if !found {
			s.runtime.Storage.Register(key, register.Name)
		}
	}
	s.runtime.module = runtime.module
	s.runtime.definitions = runtime.definitions
}

func (s *REPLSession) prepareRuntime(ctx context.Context, modules []Module, linked LinkedNameResolution, analysis AnalysisOutput) (*ModuleEvaluator, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	flat, err := FlattenModules(modules, linked.Table, s.ids, s.sources, replCopyAnalysis(analysis))
	if err != nil {
		return nil, err
	}
	for _, module := range flat.Modules {
		if module.Name == s.main {
			return NewModuleEvaluator(module, s.runtime.RNG, false)
		}
	}
	return nil, fmt.Errorf("REPL module %s not found", s.main)
}

// Load clears the session and reads a project with imports relative to its
// source file. An explicit main selects that module. Without main, the upstream
// // @mainModule annotation is honored; otherwise a fresh __repl__ scope is
// created and the loaded modules are available to subsequent import inputs.
func (s *REPLSession) loadProject(ctx context.Context, path, main string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	seed := s.Seed()
	s.Reset()
	s.SetSeed(seed)
	absolute, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	raw, err := os.ReadFile(absolute)
	if err != nil {
		return err
	}
	loadedText := decodeSourceUTF8(raw)
	parserText := loadedText
	emptySavedSession := false
	cleaned, _ := replExtractExpressions(loadedText)
	if annotation := replMainAnnotation.FindStringSubmatchIndex(cleaned); annotation != nil && cleaned[annotation[2]:annotation[3]] == "__repl__" && strings.TrimSpace(cleaned[annotation[1]:]) == "" && (main == "" || main == "__repl__") {
		// Saving before the first evaluation produces only this annotation.
		// Supply the empty scope while parsing so that it can be loaded too.
		parserText += replEmptyModuleText
		emptySavedSession = true
	}
	code := map[string]string{}
	resolver := replMainSourceResolver{SourceResolver: NewFileSourceResolver(code, nil), path: absolute, text: parserText, code: code}
	parsed, err := ParseProject(resolver, resolver.LookupPath(filepath.Dir(absolute), filepath.Base(absolute)))
	s.sources = parsed.Sources
	s.ids = parsed.IDs
	if _, found := code[absolute]; found {
		s.loaded = path
	}
	for source, text := range code {
		s.code.Set(source, text)
	}
	if err != nil {
		return err
	}
	if len(parsed.Diagnostics) > 0 {
		return parsed.Diagnostics[0]
	}
	if main == "" {
		annotation := regexp.MustCompile(`^// @mainModule\s+(\w+)\r?\n`).FindStringSubmatch(code[absolute])
		if len(annotation) > 1 {
			main = annotation[1]
		}
	}
	loadMain := main
	modules := parsed.Modules
	synthetic := main == "" || (main == "__repl__" && len(modules) == 0)
	if synthetic {
		main = "__repl__"
		modules = append(append([]Module{}, modules...), Module{ID: parsed.IDs.Next(), Name: main, Declarations: []Declaration{}})
	}
	if _, err = SelectMainModule(modules, main, "", path); err != nil {
		return err
	}
	linked, err := ResolveLinkedModules(modules)
	if err != nil {
		return err
	}
	if len(linked.Errors) > 0 {
		return linked.Errors[0]
	}
	analysis, err := AnalyzeModules(modules, linked.Table)
	if err != nil {
		return err
	}
	if len(analysis.Diagnostics) > 0 {
		return analysis.Diagnostics[0]
	}
	archive, err := newREPLArchive(absolute, loadedText, main, synthetic || emptySavedSession)
	if err != nil {
		return err
	}
	s.ids, s.sources, s.main = parsed.IDs, parsed.Sources, main
	output := AnalysisOutput{Types: analysis.Types, Effects: analysis.Effects, Modes: analysis.Modes}
	runtime, err := s.prepareRuntime(ctx, analysis.Modules, linked, output)
	if err != nil {
		s.main = "__repl__"
		return err
	}
	s.modules, s.analysis, s.runtime = analysis.Modules, output, runtime
	s.loadMain = loadMain
	s.saved = archive
	return nil
}

func (s *REPLSession) Reload(ctx context.Context) error {
	if s.loaded == "" {
		return fmt.Errorf("Nothing to reload. Use: .load filename [moduleName].")
	}
	return s.Load(ctx, s.loaded, s.loadMain)
}

// RenderError reports a session diagnostic against the correct input or loaded
// source. It never reads files, writes output, or changes the session.
func (s *REPLSession) RenderError(err error) string {
	return s.renderError(err, DiagnosticRenderOptions{HideLineNumbers: true})
}

func (s *REPLSession) renderError(err error, options DiagnosticRenderOptions) string {
	if err == nil {
		return ""
	}
	var diagnostic Diagnostic
	if errors.As(err, &diagnostic) {
		return RenderDiagnostic(diagnostic, s.sources, s.code, options)
	}
	var syntax *SyntaxErrors
	if errors.As(err, &syntax) {
		parts := make([]string, 0, len(syntax.Issues))
		for _, issue := range syntax.Issues {
			code := issue.Code
			if code == "" {
				code = "QNT000"
			}
			location := Location{Source: syntax.Source, Start: issue.Position, End: &issue.Position}
			parts = append(parts, RenderErrorMessage(ErrorMessage{Explanation: "[" + code + "] " + issue.Message, Locations: []Location{location}}, s.code, options))
		}
		return strings.Join(parts, "\n")
	}
	return err.Error()
}

func replCopyAnalysis(input AnalysisOutput) AnalysisOutput {
	out := AnalysisOutput{Types: insdict.NewDict[string, TypeScheme](), Effects: insdict.NewDict[string, EffectScheme](), Modes: insdict.NewDict[string, Qualifier]()}
	if input.Types != nil {
		for id, scheme := range input.Types.All() {
			out.Types.Set(id, SimplifyScheme(scheme))
		}
	}
	if input.Effects != nil {
		for id, scheme := range input.Effects.All() {
			scheme.Effect = cloneEffectSubstitutionValue(scheme.Effect)
			scheme.EffectVariables = append([]string{}, scheme.EffectVariables...)
			scheme.EntityVariables = append([]string{}, scheme.EntityVariables...)
			out.Effects.Set(id, scheme)
		}
	}
	if input.Modes != nil {
		for id, mode := range input.Modes.All() {
			out.Modes.Set(id, mode)
		}
	}
	return out
}
