package quint

import (
	"fmt"
	"strings"
)

// CompilationEntries describes the extra declarations synthesized by compile.
// Expression strings are parsed as Quint, as in the upstream CLI.
type CompilationEntries struct {
	Target                       string
	Flatten                      bool
	Init, Step                   *string
	Invariants                   []string
	InductiveInvariant, Temporal string
	// InvariantReports preserves separately flattened predicates for reporting
	// individual failures at a counterexample's final state.
	InvariantReports bool
}

// SynthesizeCompilationEntries parses q::* wrappers using the compilation's ID
// generator. Callers must resolve names and analyze the returned declarations.
func SynthesizeCompilationEntries(options CompilationEntries, ids *IDGenerator) ([]Declaration, error) {
	init, step := options.Init, options.Step
	if options.Target != "json" || options.Flatten {
		if init == nil {
			v := "init"
			init = &v
		}
		if step == nil {
			v := "step"
			step = &v
		}
	}
	texts := []string{}
	if init != nil {
		texts = append(texts, "action q::init = "+*init)
	}
	if step != nil {
		texts = append(texts, "action q::step = "+*step)
	}
	if len(options.Invariants) > 0 {
		texts = append(texts, "val q::inv = and("+strings.Join(options.Invariants, " and ")+")")
		if options.InvariantReports {
			for i, invariant := range options.Invariants {
				texts = append(texts, "val "+InvariantReportName(i)+" = "+invariant)
			}
		}
	}
	if options.InductiveInvariant != "" {
		texts = append(texts, "val q::inductiveInv = "+options.InductiveInvariant)
	}
	if options.Temporal != "" {
		texts = append(texts, "temporal q::temporalProps = and("+options.Temporal+")")
	}
	if len(texts) == 0 {
		return []Declaration{}, nil
	}
	declarations := []Declaration{}
	for _, text := range texts {
		parsed, err := ParseExpressionOrDeclaration(text, ids)
		if err != nil {
			return nil, err
		}
		declarations = append(declarations, parsed.Declarations...)
	}
	return declarations, nil
}

// InvariantReportName identifies a predicate generated with InvariantReports.
func InvariantReportName(index int) string {
	return fmt.Sprintf("q::reportInvariant%d", index)
}
