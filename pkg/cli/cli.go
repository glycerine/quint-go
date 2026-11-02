// Package cli provides the Go Quint command line with injectable output streams.
package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"math/big"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/glycerine/insdict"
	quint "github.com/glycerine/quint-go/pkg"
	"github.com/glycerine/quint-go/pkg/backend/apalache"
)

// Version may be set at build time using -ldflags=-X=.../cli.Version=VERSION.
var Version = "0.33.1-go"

// Execute runs one invocation. Commands report failures with status 1 and never
// terminate the process, allowing applications and tests to own their streams.
func Execute(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if stdout == nil {
		stdout = io.Discard
	}
	if stderr == nil {
		stderr = io.Discard
	}
	if err := ctx.Err(); err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return 1
	}
	help, version, args := coreInformationalOptions(args)
	if help {
		command := ""
		if len(args) > 0 {
			command = args[0]
		}
		switch command {
		case "parse", "typecheck", "compile":
			return coreHelp(stdout, stderr, command)
		case "run":
			return commandRun(ctx, []string{"--help"}, stdout, stderr)
		case "test":
			return commandTest(ctx, []string{"--help"}, stdout, stderr)
		case "verify":
			return commandVerify(ctx, []string{"--help"}, stdout, stderr)
		case "docs":
			return commandDocs(ctx, []string{"--help"}, stdout, stderr)
		case "repl":
			return commandREPL(ctx, []string{"--help"}, stdout, stderr)
		default:
			return coreHelp(stdout, stderr, "")
		}
	}
	if version {
		if _, err := fmt.Fprintln(stdout, Version); err != nil {
			fmt.Fprintln(stderr, "error:", err)
			return 1
		}
		return 0
	}
	if len(args) == 0 {
		return commandREPL(ctx, nil, stdout, stderr)
	}
	switch args[0] {
	case "parse", "typecheck", "compile":
		return coreFrontendCommand(ctx, args[0], args[1:], stdout, stderr)
	case "run":
		return commandRun(ctx, args[1:], stdout, stderr)
	case "test":
		return commandTest(ctx, args[1:], stdout, stderr)
	case "verify":
		return commandVerify(ctx, args[1:], stdout, stderr)
	case "repl":
		return commandREPL(ctx, args[1:], stdout, stderr)
	case "docs":
		return commandDocs(ctx, args[1:], stdout, stderr)
	default:
		// The reference's default command is the REPL, including optional
		// expressions and commands supplied without an explicit "repl".
		return commandREPL(ctx, args, stdout, stderr)
	}
}

// Help/version are handled before required arguments and scalar validation by
// yargs. A positional terminator protects literal filenames named --help.
func coreInformationalOptions(args []string) (help, version bool, remaining []string) {
	remaining = make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		argument := args[i]
		if argument == "--" {
			remaining = append(remaining, args[i:]...)
			break
		}
		name, value, inline := strings.Cut(argument, "=")
		if name == "--help" || name == "-h" || name == "--version" || name == "--no-help" || name == "--no-version" {
			if !inline {
				value = "true"
				if strings.HasPrefix(name, "--no-") {
					value = "false"
				} else if i+1 < len(args) && (args[i+1] == "true" || args[i+1] == "false") {
					i++
					value = args[i]
				}
			}
			if name == "--version" || name == "--no-version" {
				version = value == "true"
			} else {
				help = value == "true"
			}
			continue
		}
		remaining = append(remaining, argument)
		if argument == "help" {
			help = true
		}
		if !strings.HasPrefix(argument, "-") || inline {
			continue
		}
		boolean := false
		switch name {
		case "--flatten", "--no-flatten", "--mbt", "--no-mbt", "--quiet", "--no-quiet", "-q", "--random-transitions", "--no-random-transitions":
			boolean = true
		}
		if i+1 < len(args) && ((!boolean && coreOptionValueArgument(args[i+1])) || (boolean && (args[i+1] == "true" || args[i+1] == "false"))) {
			i++
			remaining = append(remaining, args[i])
			if name == "--invariants" || name == "--witnesses" || name == "--hide" {
				for i+1 < len(args) && coreOptionValueArgument(args[i+1]) {
					i++
					remaining = append(remaining, args[i])
				}
			}
		}
	}
	return help, version, remaining
}

func coreHelp(stdout, stderr io.Writer, command string) int {
	text := "quint [command]\n\nCommands:\n  parse <input>       parse and resolve a specification\n  typecheck <input>   check types and effects\n  compile <input>     compile to JSON or TLA+ using Go Apalache\n  run <input>         simulate a specification\n  test <input>        run named tests\n  verify <input>      verify with Go Apalache\n  repl [commands..]   evaluate interactively (default)\n  docs <input>        produce documentation\n\nOptions:\n  --help              show help\n  --version           show version\n"
	if command != "" {
		text = "quint " + command + " <input> [options]\n\nOptions:\n  --out <file>        write stage JSON to a file\n  --help              show help\n"
		if command == "parse" {
			text += "  --source-map <file> write compact source locations\n"
		}
		if command == "compile" {
			text += "  --main <module>    select the main module\n  --target <target>  json (default) or tlaplus\n  --flatten <bool>   flatten imports and instances (default true)\n  --init <expr>      initializer action\n  --step <expr>      next-state action\n  --invariant <expr> invariant expression\n  --temporal <expr>  temporal properties\n  --verbosity <0-5>  output verbosity\n"
		}
	}
	if _, err := io.WriteString(stdout, text); err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return 1
	}
	return 0
}

type coreOptions struct {
	input, out, sourceMap, main, target, invariant, temporal string
	init, step                                               *string
	flatten                                                  bool
	verbosity                                                float64
	help                                                     bool
}

// coreParseOptions accepts interspersed flags like yargs, including --name=value,
// and rejects repeated scalar flags before any source or output file is touched.
func coreParseOptions(command string, args []string) (coreOptions, error) {
	o := coreOptions{target: "json", flatten: true, verbosity: 2}
	seen := insdict.NewDict[string, bool]()
	positionals := []string{}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			positionals = append(positionals, args[i+1:]...)
			break
		}
		if coreOptionValueArgument(arg) {
			positionals = append(positionals, arg)
			continue
		}
		if arg == "-h" {
			arg = "--help"
		}
		if !strings.HasPrefix(arg, "--") {
			return o, coreUnknownShortArguments(arg)
		}
		name, value, inline := strings.Cut(strings.TrimPrefix(arg, "--"), "=")
		switch name {
		case "sourceMap":
			name = "source-map"
		case "apalacheVersion":
			name = "apalache-version"
		case "serverEndpoint":
			name = "server-endpoint"
		}
		if name == "no-flatten" {
			if inline {
				return o, fmt.Errorf("Unknown argument: %s", name)
			}
			name, value, inline = "flatten", "false", true
		}
		if seen.Get(name) && name != "help" && name != "flatten" {
			return o, fmt.Errorf("--%s can not be specified more than once", name)
		}
		seen.Set(name, true)
		allowed := name == "help" || name == "out" || (command == "parse" && name == "source-map")
		if command == "compile" {
			switch name {
			case "main", "target", "flatten", "init", "step", "invariant", "temporal", "verbosity", "apalache-version", "server-endpoint":
				allowed = true
			}
		}
		if !allowed {
			return o, fmt.Errorf("Unknown argument: %s", name)
		}
		if name == "help" || name == "flatten" {
			if !inline && i+1 < len(args) && (args[i+1] == "true" || args[i+1] == "false") {
				i++
				value, inline = args[i], true
			}
			if !inline {
				value = "true"
			}
			boolean := value == "true"
			if name == "help" {
				o.help = boolean
			} else {
				o.flatten = boolean
			}
			continue
		}
		if !inline {
			if i+1 == len(args) || !coreOptionValueArgument(args[i+1]) {
				// String options default to the empty string; an omitted
				// numeric option retains its declared default.
				if name == "verbosity" {
					continue
				}
				value = ""
			} else {
				i++
				value = args[i]
			}
		}
		switch name {
		case "out":
			o.out = value
		case "source-map":
			o.sourceMap = value
		case "main":
			o.main = value
		case "target":
			if value != "json" && value != "tlaplus" {
				return o, fmt.Errorf("Invalid value for --target: %s; expected json or tlaplus", value)
			}
			o.target = value
		case "init":
			o.init = &value
		case "step":
			o.step = &value
		case "invariant":
			o.invariant = value
		case "temporal":
			o.temporal = value
		case "verbosity":
			o.verbosity = coreNumberOption(value)
		case "apalache-version", "server-endpoint":
			return o, fmt.Errorf("--%s is unsupported: this CLI uses in-process Go Apalache", name)
		}
	}
	if o.help {
		return o, nil
	}
	if len(positionals) != 1 {
		return o, fmt.Errorf("%s requires exactly one input file", command)
	}
	o.input = positionals[0]
	return o, nil
}

// yargs consumes decimal negative values, while negative exponent/hex strings
// and other leading-dash values remain flags unless supplied with '='.
func coreOptionValueArgument(value string) bool {
	if !strings.HasPrefix(value, "-") || value == "-" {
		return true
	}
	digits, decimal := false, false
	for i := 1; i < len(value); i++ {
		if value[i] >= '0' && value[i] <= '9' {
			digits = true
		} else if value[i] == '.' && !decimal && i+1 < len(value) {
			decimal = true
		} else {
			return false
		}
	}
	return digits
}

func coreUnknownShortArguments(argument string) error {
	cluster, _, _ := strings.Cut(strings.TrimPrefix(argument, "-"), "=")
	seen := map[rune]bool{}
	names := []string{}
	for index, name := range cluster {
		if !seen[name] {
			seen[name] = true
			names = append(names, string(name))
		}
		if index+1 < len(cluster) {
			if _, err := strconv.ParseFloat(cluster[index+1:], 64); err == nil {
				break
			}
		}
	}
	if len(names) == 1 {
		return fmt.Errorf("Unknown argument: %s", names[0])
	}
	return fmt.Errorf("Unknown arguments: %s", strings.Join(names, ", "))
}

func coreNumberOption(value string) float64 {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	if value == "Infinity" || value == "+Infinity" {
		return math.Inf(1)
	}
	if value == "-Infinity" {
		return math.Inf(-1)
	}
	if strings.HasPrefix(value, "0x") || strings.HasPrefix(value, "0X") || strings.HasPrefix(value, "0b") || strings.HasPrefix(value, "0B") || strings.HasPrefix(value, "0o") || strings.HasPrefix(value, "0O") {
		integer, ok := new(big.Int).SetString(value, 0)
		if ok {
			number, _ := integer.Float64()
			return number
		}
		return math.NaN()
	}
	// ParseFloat accepts Go-only spellings of infinity; JavaScript does not.
	if strings.Contains(value, "Inf") || strings.Contains(value, "_") {
		return math.NaN()
	}
	number, err := strconv.ParseFloat(value, 64)
	if err != nil && !errors.Is(err, strconv.ErrRange) {
		return math.NaN()
	}
	return number
}

func coreFrontendCommand(ctx context.Context, command string, args []string, stdout, stderr io.Writer) int {
	o, err := coreParseOptions(command, args)
	if err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return 1
	}
	if o.help {
		return coreHelp(stdout, stderr, command)
	}
	fields := insdict.NewDict[string, any]()
	fields.Set("stage", "loading")
	fields.Set("errors", []quint.ErrorMessage{})
	code := insdict.NewDict[string, string]()
	fail := func(summary string, failure error) int {
		if failure != nil {
			fields.Set("errors", []quint.ErrorMessage{{Explanation: failure.Error(), Locations: []quint.Location{}}})
		}
		if o.out != "" {
			if err := coreWriteStage(o.out, fields); err != nil {
				fmt.Fprintln(stderr, "error:", err)
			}
			return 1
		}
		if diagnostics, ok := fields.Get("errors").([]quint.ErrorMessage); ok {
			for _, diagnostic := range diagnostics {
				fmt.Fprintln(stderr, quint.RenderErrorMessage(diagnostic, code, quint.DiagnosticRenderOptions{InclusiveEnd: true}))
			}
		}
		if failure == nil || summary != failure.Error() {
			fmt.Fprintln(stderr, "error:", summary)
		}
		return 1
	}
	abs, err := filepath.Abs(o.input)
	if err != nil {
		return fail(err.Error(), err)
	}
	if _, err = os.Stat(abs); errors.Is(err, os.ErrNotExist) {
		return fail("file "+o.input+" does not exist", nil)
	} else if err != nil {
		return fail("file "+o.input+" could not be opened due to "+err.Error(), nil)
	}
	if err = ctx.Err(); err != nil {
		return fail(err.Error(), err)
	}
	sourceCode := map[string]string{}
	resolver := quint.NewFileSourceResolver(sourceCode, nil)
	front, parseErr := quint.ParseProjectRecovering(resolver, resolver.LookupPath(filepath.Dir(abs), filepath.Base(abs)))
	paths := make([]string, 0, len(sourceCode))
	for path := range sourceCode {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	for _, path := range paths {
		code.Set(path, sourceCode[path])
	}
	var loadError *quint.SourceLoadError
	if errors.As(parseErr, &loadError) && loadError.Path == abs {
		return fail("file "+o.input+" could not be opened due to "+loadError.Error(), nil)
	}
	fields = coreStageFields("parsing", front, nil)
	if o.sourceMap != "" {
		encoded, err := coreCompactSourceMap(front.UpstreamSources)
		if err != nil {
			return fail(err.Error(), err)
		}
		if err = os.WriteFile(o.sourceMap, encoded, 0666); err != nil {
			return fail(err.Error(), err)
		}
	}
	if parseErr != nil {
		return fail("parsing failed", parseErr)
	}
	if len(front.Diagnostics) > 0 {
		return fail("parsing failed", nil)
	}
	if err = ctx.Err(); err != nil {
		return fail(err.Error(), err)
	}
	if command == "parse" {
		if o.out != "" {
			if err = coreWriteStage(o.out, fields); err != nil {
				return fail(err.Error(), err)
			}
		}
		return 0
	}
	analysis, err := quint.AnalyzeModules(front.Modules, front.Table)
	fields = coreStageFields("typechecking", front, &analysis)
	if err != nil {
		return fail("typechecking failed", err)
	}
	if len(analysis.Diagnostics) > 0 {
		return fail("typechecking failed", nil)
	}
	if command == "typecheck" {
		if o.out != "" {
			if err = coreWriteStage(o.out, fields); err != nil {
				return fail(err.Error(), err)
			}
		}
		return 0
	}
	if err = ctx.Err(); err != nil {
		return fail(err.Error(), err)
	}
	defaultModule := coreDefaultModule(front, abs)
	options := quint.CompilationOptions{CompilationEntries: quint.CompilationEntries{Target: o.target, Flatten: o.flatten, Init: o.init, Step: o.step, Temporal: o.temporal}, Main: o.main, DefaultModule: defaultModule, Input: o.input, Invariant: o.invariant}
	compiled, err := quint.CompileModules(analysis.Modules, quint.AnalysisOutput{Types: analysis.Types, Effects: analysis.Effects, Modes: analysis.Modes}, front.IDs, front.Sources, options)
	if err != nil {
		if compiled.Main == "" {
			return fail(err.Error(), nil)
		}
		return fail(err.Error(), err)
	}
	if len(compiled.Diagnostics) > 0 {
		if compiled.Table != nil {
			fields.Set("table", coreLookupTable(compiled.Table))
		}
		if compiled.UnflattenedModules == nil {
			// toExpr's argument errors are deliberately resolved against an
			// empty map upstream, even though parsing the flag allocated IDs.
			fields.Set("errors", coreResolveDiagnostics(nil, compiled.Diagnostics))
			return fail("Argument error", nil)
		}
		fields.Set("modules", coreModules(compiled.UnflattenedModules))
		fields.Set("errors", coreResolveDiagnostics(coreCompatibilitySources(front), compiled.Diagnostics))
		return fail("name resolution failed", nil)
	}
	fields.Set("stage", "compiling")
	fields.Set("modules", coreModules(compiled.UnflattenedModules))
	fields.Set("table", coreLookupTable(compiled.Table))
	fields.Set("types", coreStringTable(compiled.Analysis.Types))
	fields.Set("effects", coreStringTable(coreCompilationEffects(compiled, analysis.Effects)))
	fields.Set("main", compiled.Main)
	if o.target == "tlaplus" {
		if !o.flatten {
			fmt.Fprintln(stderr, "Warning: flattening is required for TLA+ output, ignoring --flatten=false option.")
		}
		tla, err := apalache.New().TLA(ctx, compiled)
		if err != nil {
			fields.Set("stage", "outputting target")
			fields.Set("status", "error")
			return fail(err.Error(), err)
		}
		if _, err = io.WriteString(stdout, tla); err != nil {
			return fail(err.Error(), err)
		}
		if o.out != "" {
			if err = coreWriteStage(o.out, fields); err != nil {
				return fail(err.Error(), err)
			}
		}
		return 0
	}
	fields.Set("modules", coreModules([]quint.Module{compiled.MainModule}))
	encoded, err := coreMarshalStage(fields)
	if err != nil {
		return fail(err.Error(), err)
	}
	if _, err = stdout.Write(encoded); err != nil {
		return fail(err.Error(), err)
	}
	if o.out != "" {
		fields.Set("modules", coreModules(compiled.UnflattenedModules))
		if err = coreWriteStage(o.out, fields); err != nil {
			return fail(err.Error(), err)
		}
	}
	return 0
}

func coreDefaultModule(front quint.FrontendResult, source string) string {
	names := []string{}
	for _, module := range front.Modules {
		if location, ok := front.Sources[module.ID]; ok && location.Source == source {
			names = append(names, module.Name)
		}
	}
	if len(names) == 1 {
		return names[0]
	}
	return ""
}

func coreResolveDiagnostics(sources quint.SourceMap, diagnostics []quint.Diagnostic) []quint.ErrorMessage {
	result := make([]quint.ErrorMessage, 0, len(diagnostics))
	for _, diagnostic := range diagnostics {
		result = append(result, quint.ResolveDiagnostic(sources, diagnostic))
	}
	return result
}

// Shared stage serialization keeps CLI JSON separate from the checker envelope.
// These helpers never expose native insdict fields or Go lookup wrapper fields.
func coreStageFields(stage string, front quint.FrontendResult, analysis *quint.AnalysisResult) *insdict.Dict[string, any] {
	fields := insdict.NewDict[string, any]()
	fields.Set("stage", stage)
	fields.Set("warnings", []any{})
	if front.Modules != nil {
		fields.Set("modules", coreModules(front.Modules))
	}
	if front.Table != nil {
		fields.Set("table", coreLookupTable(front.Table))
	}
	diagnostics := append([]quint.Diagnostic{}, front.Diagnostics...)
	if analysis != nil {
		fields.Set("modules", coreModules(analysis.Modules))
		fields.Set("types", coreStringTable(analysis.Types))
		fields.Set("effects", coreStringTable(analysis.Effects))
		diagnostics = append(diagnostics, analysis.Diagnostics...)
	}
	fields.Set("errors", coreResolveDiagnostics(coreCompatibilitySources(front), diagnostics))
	return fields
}

// The reference collector adds depth to non-operator source definitions in
// place. Keep the Go IR immutable and project that metadata at the CLI boundary.
func coreModules(modules []quint.Module) []quint.Module {
	result := append([]quint.Module{}, modules...)
	for i := range result {
		result[i].Declarations = append([]quint.Declaration{}, modules[i].Declarations...)
		for j := range result[i].Declarations {
			declaration := &result[i].Declarations[j]
			switch declaration.Kind {
			case "var", "const", "typedef", "assume":
				if declaration.Depth == nil {
					depth := 0
					declaration.Depth = &depth
				}
			}
		}
	}
	return result
}

// CLI JSON uses upstream's inclusive code-point coordinates; the public editor
// source map continues to expose half-open UTF-16 spans.
func coreCompatibilitySources(front quint.FrontendResult) quint.SourceMap {
	sources := make(quint.SourceMap, len(front.Sources))
	for id, location := range front.Sources {
		sources[id] = location
	}
	for id, location := range front.UpstreamSources {
		position := func(p quint.UpstreamPosition) quint.Position {
			return quint.Position{Line: p.Line, Col: p.Col, Index: p.Index}
		}
		compatible := quint.Location{Source: location.Source, Start: position(location.Start)}
		if location.End != nil {
			end := position(*location.End)
			compatible.End = &end
		}
		sources[id] = compatible
	}
	return sources
}

// analyzeInc shares the type table but copies effects; the reference compile
// command ignores its returned effects. Keep the library's complete analysis
// while projecting the CLI's original effects plus refreshed instance entries.
func coreCompilationEffects(compiled quint.CompilationResult, original *insdict.Dict[string, quint.EffectScheme]) *insdict.Dict[string, quint.EffectScheme] {
	generated := map[string]bool{}
	for _, modules := range [][]quint.Module{compiled.UnflattenedModules, compiled.Modules} {
		for _, module := range modules {
			for _, declaration := range module.Declarations {
				name := declaration.Name
				wrapper := false
				for _, suffix := range []string{"q::init", "q::step", "q::inv", "q::inductiveInv", "q::temporalProps"} {
					if name == suffix || strings.HasSuffix(name, "::"+suffix) {
						wrapper = true
						break
					}
				}
				if !wrapper {
					continue
				}
				generated[declaration.ID.String()] = true
				_ = quint.WalkExpr(declaration.Expr, func(expression *quint.Expr) (bool, error) {
					generated[expression.ID.String()] = true
					for _, parameter := range expression.Params {
						generated[parameter.ID.String()] = true
					}
					if expression.Opdef != nil {
						generated[expression.Opdef.ID.String()] = true
					}
					return true, nil
				})
			}
		}
	}
	result := insdict.NewDict[string, quint.EffectScheme]()
	if compiled.Analysis.Effects != nil {
		for id, effect := range compiled.Analysis.Effects.All() {
			source := false
			if original != nil {
				_, source = original.Get2(id)
			}
			if source || !generated[id] {
				result.Set(id, effect)
			}
		}
	}
	return result
}

type coreObject struct{ fields *insdict.Dict[string, any] }

func (object coreObject) MarshalJSON() ([]byte, error) { return coreMarshalStage(object.fields) }

func coreMarshalStage(fields *insdict.Dict[string, any]) ([]byte, error) {
	var buffer bytes.Buffer
	buffer.WriteByte('{')
	first := true
	if fields != nil {
		keys := make([]string, 0, fields.Len())
		for key := range fields.All() {
			keys = append(keys, key)
		}
		// JSON.stringify(Object.fromEntries(map)) enumerates array-index
		// properties numerically before the remaining insertion-ordered keys.
		// IR IDs in lookup/type/effect tables are commonly such properties.
		sort.SliceStable(keys, func(i, j int) bool {
			a, aIndex := coreObjectIndex(keys[i])
			b, bIndex := coreObjectIndex(keys[j])
			if aIndex != bIndex {
				return aIndex
			}
			return aIndex && a < b
		})
		for _, key := range keys {
			value := fields.Get(key)
			encoded, err := json.Marshal(value)
			if err != nil {
				return nil, err
			}
			name, _ := json.Marshal(key)
			if !first {
				buffer.WriteByte(',')
			}
			first = false
			buffer.Write(name)
			buffer.WriteByte(':')
			buffer.Write(encoded)
		}
	}
	buffer.WriteByte('}')
	return buffer.Bytes(), nil
}

func coreObjectIndex(key string) (uint64, bool) {
	index, err := strconv.ParseUint(key, 10, 32)
	return index, err == nil && index < 1<<32-1 && strconv.FormatUint(index, 10) == key
}

func coreStringTable[T any](table *insdict.Dict[string, T]) coreObject {
	fields := insdict.NewDict[string, any]()
	if table != nil {
		for key, value := range table.All() {
			fields.Set(key, value)
		}
	}
	return coreObject{fields}
}

func coreLookupTable(table *insdict.Dict[quint.ID, quint.LookupDefinition]) coreObject {
	fields := insdict.NewDict[string, any]()
	if table != nil {
		for id, definition := range table.All() {
			if definition.Declaration != nil {
				declaration := *definition.Declaration
				// Declaration entries retain the import which introduced them;
				// resolved references carry their current lookup provenance.
				if id != declaration.ID {
					declaration.ImportedFrom = nil
				}
				if declaration.Kind == "def" {
					depth := definition.Depth
					declaration.Depth = &depth
				}
				if definition.Hidden {
					hidden := true
					declaration.Hidden = &hidden
				}
				if definition.Shadowing {
					declaration.Shadowing = json.RawMessage("true")
				}
				if definition.Namespaces != nil {
					declaration.Namespaces = definition.Namespaces
				}
				if definition.ImportedFrom != nil {
					data, err := json.Marshal(definition.ImportedFrom)
					if err == nil {
						declaration.ImportedFrom = data
					}
				}
				fields.Set(id.String(), declaration)
			} else if definition.Parameter != nil {
				parameter := *definition.Parameter
				parameter.Kind = "param"
				depth := definition.Depth
				parameter.Depth = &depth
				if definition.Shadowing {
					parameter.Shadowing = json.RawMessage("true")
				}
				fields.Set(id.String(), parameter)
			}
		}
	}
	return coreObject{fields}
}

func coreWriteStage(path string, fields *insdict.Dict[string, any]) error {
	encoded, err := coreMarshalStage(fields)
	if err != nil {
		return err
	}
	return os.WriteFile(path, encoded, 0666)
}

func coreCompactSourceMap(sources quint.UpstreamSourceMap) ([]byte, error) {
	ids := make([]quint.ID, 0, len(sources))
	for id := range sources {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool {
		a, b := ids[i].String(), ids[j].String()
		return len(a) < len(b) || (len(a) == len(b) && a < b)
	})
	indexes := insdict.NewDict[string, int]()
	locations := insdict.NewDict[string, any]()
	indexFields := insdict.NewDict[string, any]()
	for locationIndex, id := range ids {
		location := sources[id]
		index, present := indexes.Get2(location.Source)
		if !present {
			// Upstream indexes the full sequence of locations, so later files
			// start after all preceding locations rather than at file ordinal 1.
			index = locationIndex
			indexes.Set(location.Source, index)
			indexFields.Set(strconv.Itoa(index), location.Source)
		}
		var end any = coreObject{insdict.NewDict[string, any]()}
		if location.End != nil {
			end = *location.End
		}
		locations.Set(id.String(), []any{index, location.Start, end})
	}
	fields := insdict.NewDict[string, any]()
	fields.Set("sourceIndex", coreObject{indexFields})
	fields.Set("map", coreObject{locations})
	return coreMarshalStage(fields)
}
