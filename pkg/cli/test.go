package cli

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"math/big"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/glycerine/insdict"
	quint "github.com/glycerine/quint-go/pkg"
)

type testCommandOptions struct {
	input, main, match, out, itf, seed string
	maxSamples, verbosity              int
	seedSet, samplesSet, help          bool
}

type testOutputWriter struct {
	writer io.Writer
	err    error
}

func (w *testOutputWriter) Write(data []byte) (int, error) {
	if w.err != nil {
		return 0, w.err
	}
	n, err := w.writer.Write(data)
	if err == nil && n < len(data) {
		err = io.ErrShortWrite
	}
	w.err = err
	return n, err
}

const testUsage = `Usage: quint test <input> [options]
  --main NAME          main module (defaults to the source module or filename)
  --match REGEX        select test names; defaults to names ending in Test
                       supports the shared JavaScript/RE2 regular-expression subset
  --max-samples N      successful samples per randomized test (default 10000, or 1 with --seed)
  --seed INTEGER       replay an arbitrary-precision decimal/hexadecimal seed
  --verbosity N        output level from 0 to 5 (default 2)
  --out FILE           write stage JSON and suppress test reports
  --out-itf TEMPLATE   write passing/false-result traces; supports {test} and {seq}
  --output TEMPLATE    alias for --out-itf
  --backend go         use the Go evaluator
`

func testParseOptions(args []string) (testCommandOptions, error) {
	o := testCommandOptions{maxSamples: 10000, verbosity: 2}
	seen := insdict.NewDict[string, bool]()
	positional := false
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if !positional && arg == "--" {
			positional = true
			continue
		}
		if positional || !strings.HasPrefix(arg, "-") || arg == "-" {
			if o.input != "" {
				return o, fmt.Errorf("test accepts exactly one input file")
			}
			o.input = arg
			continue
		}
		if arg == "--help" || arg == "-h" {
			o.help = true
			continue
		}
		name, value, hasValue := strings.Cut(arg, "=")
		canonical := name
		if canonical == "--output" {
			canonical = "--out-itf"
		}
		if seen.Get(canonical) {
			return o, fmt.Errorf("%s cannot be specified more than once", canonical)
		}
		seen.Set(canonical, true)
		switch name {
		case "--main", "--match", "--max-samples", "--seed", "--verbosity", "--out", "--out-itf", "--output", "--backend":
		default:
			return o, fmt.Errorf("unknown test option %s", name)
		}
		if !hasValue {
			i++
			if i >= len(args) || strings.HasPrefix(args[i], "--") {
				return o, fmt.Errorf("%s requires a value", name)
			}
			value = args[i]
		}
		switch name {
		case "--main":
			o.main = value
		case "--match":
			o.match = value
		case "--out":
			o.out = value
		case "--out-itf", "--output":
			o.itf = value
		case "--seed":
			o.seed = value
			o.seedSet = true
		case "--max-samples":
			n, err := strconv.Atoi(value)
			if err != nil || n < 0 {
				return o, fmt.Errorf("--max-samples must be a nonnegative integer")
			}
			o.maxSamples = n
			o.samplesSet = true
		case "--verbosity":
			n, err := strconv.Atoi(value)
			if err != nil || n < 0 || n > 5 {
				return o, fmt.Errorf("--verbosity must be an integer from 0 to 5")
			}
			o.verbosity = n
		case "--backend":
			if value != "go" {
				return o, fmt.Errorf("unsupported test backend %q: the Go port uses --backend=go", value)
			}
		}
	}
	if o.seedSet && !o.samplesSet {
		o.maxSamples = 1
	}
	if !o.help && o.input == "" {
		return o, fmt.Errorf("test requires an input file")
	}
	if o.seedSet {
		if _, err := quint.ParseSeed(o.seed); err != nil {
			return o, err
		}
	}
	return o, nil
}

// Quint names use ASCII identifiers. The shared JavaScript/RE2 subset therefore
// covers ordinary name filtering without claiming support for backreferences,
// lookaround, Unicode classes, or Go-only inline flags.
func testMatcher(pattern string) (func(string) bool, error) {
	if pattern == "" {
		return nil, nil
	}
	for _, unsupported := range []string{`\p`, `\P`, `(?=`, `(?!`, `(?<`, `(?P`, `(?i`, `(?m`, `(?s`, `(?U`, `(?-`, `\A`, `\z`, `\Q`, `\E`, `\a`, `\x{`, `[[:`} {
		if strings.Contains(pattern, unsupported) {
			return nil, fmt.Errorf("unsupported --match expression: the Go CLI supports the shared JavaScript/RE2 subset (no lookaround, backreferences, Unicode classes, or inline flags)")
		}
	}
	compiled, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("invalid or unsupported --match expression %q: %w", pattern, err)
	}
	return compiled.MatchString, nil
}

type testSelection struct {
	definition             quint.LookupDefinition
	reference, name, entry string
	variables              []string
}

func testSelections(front quint.FrontendResult, main string, match func(string) bool) ([]testSelection, error) {
	resolution := quint.LinkedNameResolution{Table: front.Table, Modules: front.Scopes}
	selected, err := quint.DiscoverTests(resolution, main, match)
	if err != nil {
		return nil, err
	}
	out := make([]testSelection, 0, len(selected))
	for _, definition := range selected {
		reference := ""
		for key, entries := range front.Scopes.Get(main).Definitions.All() {
			for _, candidate := range entries {
				if candidate.ID == definition.ID && candidate.Name == definition.Name && slices.Equal(candidate.Namespaces, definition.Namespaces) {
					reference = key
					break
				}
			}
			if reference != "" {
				break
			}
		}
		if reference == "" {
			return nil, fmt.Errorf("cannot resolve selected test %s", definition.Name)
		}
		parts := append([]string{}, definition.Namespaces...)
		slices.Reverse(parts)
		parts = append(parts, definition.Name)
		out = append(out, testSelection{definition: definition, reference: reference, name: strings.Join(parts, "::")})
	}
	return out, nil
}

// Synthetic references keep unreferenced imported tests in the flattened main
// module, while preserving discovery order, namespaces, dependencies, and
// independent constant-overridden instances. They are private execution entries
// and never appear in the output stage.
func testPrepareEvaluator(ctx context.Context, front quint.FrontendResult, main string, selections []testSelection, rng *quint.RNG) (*quint.ModuleEvaluator, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	working := append([]quint.Module{}, front.Modules...)
	for i, module := range working {
		if module.Name != main {
			continue
		}
		working[i].Declarations = append([]quint.Declaration{}, module.Declarations...)
		for j := range selections {
			id := front.IDs.Next()
			name := "q::cliTest_" + id.String()
			for _, exists := front.Scopes.Get(main).Lookup(name); exists; _, exists = front.Scopes.Get(main).Lookup(name) {
				name += "_"
			}
			selections[j].entry = name
			reference := quint.Expr{ID: front.IDs.Next(), Kind: "name", Name: selections[j].reference}
			working[i].Declarations = append(working[i].Declarations, quint.Declaration{ID: id, Kind: "def", Qualifier: selections[j].definition.Declaration.Qualifier, Name: name, Expr: &reference})
		}
	}
	linked, err := quint.ResolveLinkedModules(working)
	if err != nil {
		return nil, err
	}
	if len(linked.Errors) > 0 {
		return nil, linked.Errors[0]
	}
	working, diagnostics := quint.SortModuleDeclarations(working, linked.Table)
	if len(diagnostics) > 0 {
		return nil, diagnostics[0]
	}
	analysis, err := quint.AnalyzeModules(working, linked.Table)
	if err != nil {
		return nil, err
	}
	if len(analysis.Diagnostics) > 0 {
		return nil, analysis.Diagnostics[0]
	}
	flat, err := quint.FlattenModules(analysis.Modules, linked.Table, front.IDs, front.Sources, quint.AnalysisOutput{Types: analysis.Types, Effects: analysis.Effects, Modes: analysis.Modes})
	if err != nil {
		return nil, err
	}
	if err = ctx.Err(); err != nil {
		return nil, err
	}
	for _, module := range flat.Modules {
		if module.Name != main {
			continue
		}
		testVariableOrder(module, flat.Table, selections)
		return quint.NewModuleEvaluator(module, rng, false)
	}
	return nil, fmt.Errorf("flattened module %s does not exist", main)
}

// The reference's builder registers variables while compiling each selected
// definition, rather than enumerating module declarations. Registrations
// accumulate across the test suite, including variables in unevaluated branches.
// Application arguments compile before the operator body; assignment targets
// compile after the right-hand side. Retain that order in each ITF header.
func testVariableOrder(module quint.Module, table *insdict.Dict[quint.ID, quint.LookupDefinition], selections []testSelection) {
	definitions := insdict.NewDict[string, quint.Declaration]()
	for _, declaration := range module.Declarations {
		if declaration.Kind == "def" {
			definitions.Set(declaration.Name, declaration)
		}
	}
	seenDefinitions := map[quint.ID]bool{}
	variables := insdict.NewDict[string, string]()
	var expression func(quint.Expr)
	var definition func(quint.LookupDefinition)
	definition = func(lookup quint.LookupDefinition) {
		if lookup.Declaration == nil {
			return
		}
		declaration := lookup.Declaration
		if declaration.Kind == "var" {
			variables.Set(declaration.ID.String(), declaration.Name)
			return
		}
		if seenDefinitions[declaration.ID] {
			return
		}
		seenDefinitions[declaration.ID] = true
		if declaration.Expr != nil {
			expression(*declaration.Expr)
		}
	}
	reference := func(id quint.ID) {
		if lookup, found := table.Get2(id); found {
			definition(lookup)
		}
	}
	expression = func(value quint.Expr) {
		switch value.Kind {
		case "name":
			reference(value.ID)
		case "app":
			if value.Opcode == "assign" && len(value.Args) == 2 {
				expression(value.Args[1])
				reference(value.Args[0].ID)
				return
			}
			for _, argument := range value.Args {
				expression(argument)
			}
			reference(value.ID)
		case "lambda":
			if value.Body != nil {
				expression(*value.Body)
			}
		case "let":
			if value.Opdef != nil && value.Opdef.Expr != nil {
				expression(*value.Opdef.Expr)
			}
			if value.Body != nil {
				expression(*value.Body)
			}
		}
	}
	for i := range selections {
		if declaration, found := definitions.Get2(selections[i].entry); found && declaration.Expr != nil {
			expression(*declaration.Expr)
		}
		selections[i].variables = make([]string, 0, variables.Len())
		for _, name := range variables.All() {
			selections[i].variables = append(selections[i].variables, name)
		}
	}
}

func commandTest(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if stdout == nil {
		stdout = io.Discard
	}
	if stderr == nil {
		stderr = io.Discard
	}
	output := &testOutputWriter{writer: stdout}
	stdout = output
	ctx = quint.WithRuntimeHooks(ctx, quint.RuntimeHooks{DebugWriter: stdout})
	options, err := testParseOptions(args)
	if err != nil {
		if strings.HasPrefix(err.Error(), "--seed must be a big integer") {
			fmt.Fprintln(stderr, err)
		} else {
			fmt.Fprintln(stderr, "error:", err)
		}
		return 1
	}
	if options.help {
		if _, err := fmt.Fprint(stdout, testUsage); err != nil {
			fmt.Fprintln(stderr, "error: writing test output:", err)
			return 1
		}
		return 0
	}
	fields := insdict.NewDict[string, any]()
	fields.Set("stage", "loading")
	fields.Set("errors", []quint.ErrorMessage{})
	code := insdict.NewDict[string, string]()
	var sources quint.SourceMap
	finish := func(status int) int {
		if output.err != nil {
			fmt.Fprintln(stderr, "error: writing test output:", output.err)
			return 1
		}
		if options.out != "" {
			data, err := coreMarshalStage(fields)
			if err == nil {
				err = os.WriteFile(options.out, data, 0666)
			}
			if err != nil {
				fmt.Fprintln(stderr, "error: writing test output:", err)
				return 1
			}
		}
		return status
	}
	failMessages := func(summary string, messages []quint.ErrorMessage) int {
		fields.Set("errors", messages)
		if options.out == "" {
			duplicate := false
			for _, message := range messages {
				rendered := quint.RenderErrorMessage(message, code)
				fmt.Fprintln(stderr, rendered)
				duplicate = duplicate || rendered == "error: "+summary
			}
			if summary != "" && !duplicate {
				fmt.Fprintln(stderr, "error:", summary)
			}
		}
		return finish(1)
	}
	fail := func(err error) int {
		message := quint.ErrorMessage{Explanation: err.Error(), Locations: []quint.Location{}}
		var diagnostic quint.Diagnostic
		if errors.As(err, &diagnostic) {
			message = quint.ResolveDiagnostic(sources, diagnostic)
		}
		return failMessages("", []quint.ErrorMessage{message})
	}
	if err = ctx.Err(); err != nil {
		return fail(err)
	}
	match, err := testMatcher(options.match)
	if err != nil {
		return fail(err)
	}
	var seed quint.Integer
	if options.seedSet {
		seed, err = quint.ParseSeed(options.seed)
	} else {
		var random *big.Int
		random, err = rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 64))
		if err == nil {
			seed = quint.IntegerFromBig(random)
		}
	}
	if err != nil {
		return fail(err)
	}
	abs, err := filepath.Abs(options.input)
	if err != nil {
		return fail(err)
	}
	sourceCode := map[string]string{}
	resolver := quint.NewFileSourceResolver(sourceCode, nil)
	path := resolver.LookupPath(filepath.Dir(abs), filepath.Base(abs))
	front, err := quint.ParseProject(resolver, path)
	sources = front.Sources
	paths := make([]string, 0, len(sourceCode))
	for path := range sourceCode {
		paths = append(paths, path)
	}
	slices.Sort(paths)
	for _, path := range paths {
		code.Set(path, sourceCode[path])
	}
	fields = coreStageFields("parsing", front, nil)
	if err != nil {
		return fail(err)
	}
	if len(front.Diagnostics) > 0 {
		return failMessages("parsing failed", coreResolveDiagnostics(front.Sources, front.Diagnostics))
	}
	analysis, err := quint.AnalyzeModules(front.Modules, front.Table)
	fields = coreStageFields("typechecking", front, &analysis)
	if err != nil {
		return fail(err)
	}
	if len(analysis.Diagnostics) > 0 {
		return failMessages("typechecking failed", coreResolveDiagnostics(front.Sources, analysis.Diagnostics))
	}
	main := quint.GuessMainModule(options.main, coreDefaultModule(front, abs), options.input)
	if _, err = quint.SelectMainModule(front.Modules, main, "", options.input); err != nil {
		return failMessages("Argument error", []quint.ErrorMessage{{Explanation: "[QNT405] Main module " + main + " not found", Locations: []quint.Location{}}})
	}
	selections, err := testSelections(front, main, match)
	if err != nil {
		return fail(err)
	}
	fields.Set("stage", "testing")
	passed, failed, ignored := []string{}, []string{}, []string{}
	fields.Set("passed", passed)
	fields.Set("failed", failed)
	fields.Set("ignored", ignored)
	if len(selections) == 0 {
		return fail(errors.New(quint.NoTestsMatchedMessage(options.match)))
	}
	evaluator, err := testPrepareEvaluator(ctx, front, main, selections, quint.NewRNG(seed))
	if err != nil {
		return fail(err)
	}
	verbosity := options.verbosity
	if options.out != "" {
		verbosity = 0
	}
	start := time.Now()
	if verbosity > 0 {
		fmt.Fprintf(stdout, "\n  %s\n", main)
	}
	results := make([]quint.RuntimeTestResult, 0, len(selections))
	for index, selection := range selections {
		result, err := evaluator.Test(ctx, selection.entry, options.maxSamples)
		if err != nil {
			return fail(err)
		}
		result.Name = selection.name
		for i, diagnostic := range result.Errors {
			if diagnostic.Code == "QNT511" {
				id := selection.definition.ID
				diagnostic.Reference = &id
				diagnostic.Message = "Test " + selection.name + " returned false"
				result.Errors[i] = diagnostic
			}
		}
		results = append(results, result)
		switch result.Status {
		case "passed":
			passed = append(passed, result.Name)
			if verbosity > 0 {
				fmt.Fprintf(stdout, "    ok %s passed %d test(s)\n", result.Name, result.Samples)
			}
		case "failed":
			failed = append(failed, result.Name)
			if verbosity > 0 {
				fmt.Fprintf(stdout, "    %d) %s failed after %d test(s)\n", len(failed), result.Name, result.Samples)
			}
		case "ignored":
			ignored = append(ignored, result.Name)
		}
		// Upstream calls onTrace for passed and false tests, but not for
		// malformed results or runtime errors.
		if options.itf != "" && (result.Status == "passed" || (result.Status == "failed" && len(result.Errors) > 0 && result.Errors[0].Code == "QNT511")) {
			states := make([]quint.Expr, len(result.States))
			for i, state := range result.States {
				states[i] = state.Expression()
			}
			now := time.Now()
			writer := quint.TraceWriter{Template: options.itf, Header: quint.ITFHeader{Source: options.input, CreatedAt: now.Format(time.RFC1123Z), Timestamp: now.UnixMilli()}}
			if _, err = writer.Write(index, result.Status, selection.variables, states, result.Name); err != nil {
				return fail(err)
			}
		}
	}
	fields.Set("passed", passed)
	fields.Set("failed", failed)
	fields.Set("ignored", ignored)
	if verbosity > 0 {
		fmt.Fprintln(stdout)
		if len(passed) > 0 {
			fmt.Fprintf(stdout, "  %d passing (%dms)\n", len(passed), time.Since(start).Milliseconds())
		}
		if len(failed) > 0 {
			fmt.Fprintf(stdout, "  %d failed\n", len(failed))
		}
		if len(ignored) > 0 {
			fmt.Fprintf(stdout, "  %d ignored\n", len(ignored))
		}
	}
	if verbosity >= 2 && len(failed) > 0 {
		fmt.Fprintln(stdout)
		number := 0
		for index, result := range results {
			if result.Status == "failed" {
				for _, diagnostic := range result.Errors {
					number++
					fmt.Fprintf(stdout, "  %d) %s:\n", number, result.Name)
					for _, line := range strings.Split(quint.RenderDiagnostic(diagnostic, front.Sources, code), "\n") {
						if strings.TrimSpace(line) != "" {
							fmt.Fprintf(stdout, "      %s\n", line)
						}
					}
					// Discovery matches unqualified names; replay keeps this
					// distinction even when result names include namespaces.
					fmt.Fprintf(stdout, "    Use --seed=0x%s --match=^%s$ --backend=go to repeat.\n", result.Seed.Big().Text(16), selections[index].definition.Name)
				}
			}
		}
		fmt.Fprintln(stdout)
		if verbosity < 3 {
			fmt.Fprintln(stdout, "\n  Use --verbosity=3 to show executions.")
			fmt.Fprintf(stdout, "  Further debug with: quint test --verbosity=3 %s\n", options.input)
		}
	}
	if len(failed) > 0 {
		if options.out == "" {
			fmt.Fprintln(stderr, "error: Tests failed")
		}
		return finish(1)
	}
	return finish(0)
}
