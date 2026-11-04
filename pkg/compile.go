package quint

import "fmt"

// CompilationOptions selects a main module and the declarations needed by the
// requested output. Flatten is explicit; TLA+ always forces flattening.
type CompilationOptions struct {
	CompilationEntries
	Main, DefaultModule, Input string
	Invariant                  string
}

// CompilationResult retains the selected module and compilation analysis.
// Diagnostics represent name/argument failures; structural failures return an
// error. AnalysisDiagnostics retains incremental diagnostics separately because
// upstream compile does not reject the generated definitions on analysis errors.
type CompilationResult struct {
	FlatteningResult
	// UnflattenedModules is the analyzed source module list after q::* entry
	// synthesis. The CLI stage output includes this view even when the target
	// contains only the flattened main module. Module/declaration slices are
	// copied; nested IR remains read-only, as in the other compilation views.
	UnflattenedModules   []Module
	MainModule           Module
	Main                 string
	Diagnostics          []Diagnostic
	AnalysisDiagnostics  []Diagnostic
	InvariantReportNames []string
}

// CompileModules prepares already typechecked, dependency-ordered modules for
// output. ids must continue the parser's sequence. As in upstream compilation,
// incremental analysis may extend the supplied analysis tables and source map.
func CompileModules(modules []Module, analysis AnalysisOutput, ids *IDGenerator, sources SourceMap, options CompilationOptions) (CompilationResult, error) {
	result := CompilationResult{Diagnostics: []Diagnostic{}, InvariantReportNames: append([]string(nil), options.InvariantReportNames...)}
	index, err := SelectMainModule(modules, options.Main, options.DefaultModule, options.Input)
	if err != nil {
		return result, err
	}
	result.Main = modules[index].Name
	linked, err := ResolveLinkedModules(modules)
	if err != nil {
		return result, err
	}
	if len(linked.Errors) > 0 {
		result.Diagnostics = linked.Errors
		return result, nil
	}
	entries := options.CompilationEntries
	_, entries.Invariants = CompilationInvariants(options.Invariant, entries.Invariants)
	if entries.Target != "json" || entries.Flatten {
		if entries.Init == nil {
			value := "init"
			entries.Init = &value
		}
		if entries.Step == nil {
			value := "step"
			entries.Step = &value
		}
	}
	resolver := linked.ExpressionResolver(result.Main)
	// Keep successful flag references on failures, matching the CLI resolver's
	// shared-table behavior without mutating the caller's lookup table.
	result.Table = resolver.Table
	for _, flag := range []struct {
		name  string
		value *string
	}{{"init", entries.Init}, {"step", entries.Step}} {
		if flag.value == nil {
			continue
		}
		parsed, locations, parseErr := ParseREPLWithSource("<input>", *flag.value, ids)
		for id, loc := range locations {
			if sources != nil {
				sources[id] = loc
			}
		}
		if parseErr != nil || parsed.Kind != "expr" {
			result.Diagnostics = append(result.Diagnostics, Diagnostic{Code: QNT501, Message: fmt.Sprintf("Expected %s to be a valid expression", *flag.value)})
			return result, nil
		}
		resolver.Resolve(parsed.Expr, 0)
		if len(resolver.Errors) > 0 {
			result.Diagnostics = append(result.Diagnostics, resolver.Errors[0])
			return result, nil
		}
		if parsed.Expr.Kind == "name" {
			def, ok := resolver.Table.Get2(parsed.Expr.ID)
			if ok && def.Declaration != nil && (def.Declaration.Kind != "def" || def.Declaration.Qualifier != Action) {
				kind := def.Declaration.Kind
				if kind == "def" {
					kind = string(def.Declaration.Qualifier)
				}
				result.Diagnostics = append(result.Diagnostics, Diagnostic{Code: QNT501, Message: fmt.Sprintf("default --%s action name '%s' is used by a %s declaration", flag.name, *flag.value, kind)})
				return result, nil
			}
		}
	}
	extra, err := SynthesizeCompilationEntries(entries, ids)
	if err != nil {
		return result, err
	}
	working := append([]Module{}, modules...)
	working[index].Declarations = append(append([]Declaration{}, modules[index].Declarations...), extra...)
	result.UnflattenedModules = append([]Module{}, working...)
	for i := range result.UnflattenedModules {
		result.UnflattenedModules[i].Declarations = append([]Declaration{}, working[i].Declarations...)
	}
	linked, err = ResolveLinkedModules(working)
	if err != nil {
		return result, err
	}
	if len(linked.Errors) > 0 {
		result.Diagnostics = linked.Errors
		return result, nil
	}
	incremental, err := AnalyzeIncremental(analysis, linked.Table, extra)
	if err != nil {
		return result, err
	}
	result.AnalysisDiagnostics = incremental.Diagnostics
	output := AnalysisOutput{Types: incremental.Types, Effects: incremental.Effects, Modes: incremental.Modes}
	if !entries.Flatten && entries.Target != "tlaplus" {
		result.FlatteningResult = FlatteningResult{Modules: working, Table: linked.Table, Analysis: output}
		result.MainModule = working[index]
		return result, nil
	}
	flat, err := FlattenModules(working, linked.Table, ids, sources, output)
	if err != nil {
		return result, err
	}
	result.FlatteningResult = flat
	for _, m := range flat.Modules {
		if m.Name == result.Main {
			result.MainModule = m
			return result, nil
		}
	}
	return result, fmt.Errorf("compiled main module %s not found", result.Main)
}
