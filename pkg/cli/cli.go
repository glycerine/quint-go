// Package cli provides the Go Quint command line with injectable output streams.
package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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
	if len(args) > 0 {
		switch args[0] {
		case "--version":
			if len(args) != 1 {
				fmt.Fprintln(stderr, "error: --version does not accept additional arguments")
				return 1
			}
			if _, err := fmt.Fprintln(stdout, Version); err != nil {
				fmt.Fprintln(stderr, "error:", err)
				return 1
			}
			return 0
		case "--help", "-h":
			return coreHelp(stdout, stderr, "")
		}
	}
	if err := ctx.Err(); err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return 1
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
	verbosity                                                int
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
		if !strings.HasPrefix(arg, "-") || arg == "-" {
			positionals = append(positionals, arg)
			continue
		}
		if arg == "-h" {
			arg = "--help"
		}
		if !strings.HasPrefix(arg, "--") {
			return o, fmt.Errorf("Unknown argument: %s", strings.TrimPrefix(arg, "-"))
		}
		name, value, inline := strings.Cut(strings.TrimPrefix(arg, "--"), "=")
		if name == "no-flatten" {
			if inline {
				return o, fmt.Errorf("Unknown argument: %s", name)
			}
			name, value, inline = "flatten", "false", true
		}
		if seen.Get(name) {
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
			boolean, err := strconv.ParseBool(value)
			if err != nil || (value != "true" && value != "false") {
				return o, fmt.Errorf("--%s must be true or false", name)
			}
			if name == "help" {
				o.help = boolean
			} else {
				o.flatten = boolean
			}
			continue
		}
		if !inline {
			if i+1 == len(args) || strings.HasPrefix(args[i+1], "--") {
				return o, fmt.Errorf("Not enough arguments following: %s", name)
			}
			i++
			value = args[i]
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
			n, err := strconv.Atoi(value)
			if err != nil || n < 0 || n > 5 {
				return o, fmt.Errorf("--verbosity must be an integer between 0 and 5")
			}
			o.verbosity = n
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
	fields.Set("warnings", []any{})
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
				fmt.Fprintln(stderr, quint.RenderErrorMessage(diagnostic, code))
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
		return fail("file "+o.input+" does not exist", fmt.Errorf("file %s does not exist", o.input))
	} else if err != nil {
		return fail(err.Error(), err)
	}
	if err = ctx.Err(); err != nil {
		return fail(err.Error(), err)
	}
	sourceCode := map[string]string{}
	resolver := quint.NewFileSourceResolver(sourceCode, nil)
	front, parseErr := quint.ParseProject(resolver, resolver.LookupPath(filepath.Dir(abs), filepath.Base(abs)))
	paths := make([]string, 0, len(sourceCode))
	for path := range sourceCode {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	for _, path := range paths {
		code.Set(path, sourceCode[path])
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
	fields.Set("stage", "compiling")
	if err != nil {
		return fail(err.Error(), err)
	}
	if len(compiled.Diagnostics) > 0 {
		fields.Set("errors", coreResolveDiagnostics(front.Sources, compiled.Diagnostics))
		return fail("compilation failed", nil)
	}
	fields.Set("modules", compiled.Modules)
	fields.Set("table", coreLookupTable(compiled.Table))
	fields.Set("types", coreStringTable(compiled.Analysis.Types))
	fields.Set("effects", coreStringTable(compiled.Analysis.Effects))
	fields.Set("main", compiled.Main)
	if o.target == "tlaplus" {
		if !o.flatten && o.verbosity > 0 && o.out == "" {
			fmt.Fprintln(stderr, "Warning: flattening is required for TLA+ output, ignoring --flatten=false option.")
		}
		tla, err := apalache.New().TLA(ctx, compiled)
		if err != nil {
			fields.Set("stage", "outputting target")
			fields.Set("status", "error")
			return fail(err.Error(), err)
		}
		if o.out != "" {
			if err = coreWriteStage(o.out, fields); err != nil {
				return fail(err.Error(), err)
			}
			return 0
		}
		if _, err = io.WriteString(stdout, tla); err != nil {
			return fail(err.Error(), err)
		}
		return 0
	}
	if o.out != "" {
		if err = coreWriteStage(o.out, fields); err != nil {
			return fail(err.Error(), err)
		}
		return 0
	}
	fields.Set("modules", []quint.Module{compiled.MainModule})
	encoded, err := coreMarshalStage(fields)
	if err != nil {
		return fail(err.Error(), err)
	}
	if _, err = stdout.Write(encoded); err != nil {
		return fail(err.Error(), err)
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
		fields.Set("modules", front.Modules)
	}
	if front.Table != nil {
		fields.Set("table", coreLookupTable(front.Table))
	}
	diagnostics := append([]quint.Diagnostic{}, front.Diagnostics...)
	if analysis != nil {
		fields.Set("modules", analysis.Modules)
		fields.Set("types", coreStringTable(analysis.Types))
		fields.Set("effects", coreStringTable(analysis.Effects))
		diagnostics = append(diagnostics, analysis.Diagnostics...)
	}
	fields.Set("errors", coreResolveDiagnostics(front.Sources, diagnostics))
	return fields
}

type coreObject struct{ fields *insdict.Dict[string, any] }

func (object coreObject) MarshalJSON() ([]byte, error) { return coreMarshalStage(object.fields) }

func coreMarshalStage(fields *insdict.Dict[string, any]) ([]byte, error) {
	var buffer bytes.Buffer
	buffer.WriteByte('{')
	first := true
	if fields != nil {
		for key, value := range fields.All() {
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
	for _, id := range ids {
		location := sources[id]
		index, present := indexes.Get2(location.Source)
		if !present {
			index = indexes.Len()
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
