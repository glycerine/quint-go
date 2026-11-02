package cli

import (
	"context"
	"crypto/rand"
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
	"time"

	"github.com/glycerine/insdict"
	quint "github.com/glycerine/quint-go/pkg"
)

type runOptions struct {
	input, main, init, step, invariant, seed, out, outITF string
	invariants, witnesses, hide                           []string
	maxSamples, maxSteps, traces, verbosity               int
	seedSet, samplesSet, metadata                         bool
}

func runParseOptions(args []string) (runOptions, error) {
	o := runOptions{init: "init", step: "step", invariant: "true", maxSamples: 10000, maxSteps: 20, traces: 1, verbosity: 2}
	for i := 0; i < len(args); i++ {
		token := args[i]
		if token == "--" {
			if i+2 != len(args) || o.input != "" {
				return o, fmt.Errorf("run requires exactly one input file")
			}
			o.input = args[i+1]
			break
		}
		if !strings.HasPrefix(token, "-") {
			if o.input != "" {
				return o, fmt.Errorf("unexpected argument: %s", token)
			}
			o.input = token
			continue
		}
		name, value, hasValue := strings.Cut(token, "=")
		take := func() (string, error) {
			if hasValue {
				return value, nil
			}
			if i+1 == len(args) || strings.HasPrefix(args[i+1], "--") {
				return "", fmt.Errorf("%s requires a value", name)
			}
			i++
			return args[i], nil
		}
		switch name {
		case "--help", "-h":
			return o, errRunHelp
		case "--main", "--init", "--step", "--invariant", "--seed", "--out", "--out-itf", "--backend":
			v, err := take()
			if err != nil {
				return o, err
			}
			switch name {
			case "--main":
				o.main = v
			case "--init":
				o.init = v
			case "--step":
				o.step = v
			case "--invariant":
				o.invariant = v
			case "--seed":
				o.seed = v
				o.seedSet = true
			case "--out":
				o.out = v
			case "--out-itf":
				o.outITF = v
			case "--backend":
				if v != "go" {
					return o, fmt.Errorf("--backend=%s is unsupported; this port uses --backend=go", v)
				}
			}
		case "--max-samples", "--max-steps", "--n-traces", "--verbosity", "--n-threads":
			v, err := take()
			if err != nil {
				return o, err
			}
			n, err := strconv.Atoi(v)
			if err != nil || n < 0 {
				return o, fmt.Errorf("%s must be a nonnegative integer, found: %s", name, v)
			}
			switch name {
			case "--max-samples":
				o.maxSamples = n
				o.samplesSet = true
			case "--max-steps":
				o.maxSteps = n
			case "--n-traces":
				o.traces = n
			case "--verbosity":
				if n > 5 {
					return o, fmt.Errorf("--verbosity must be between 0 and 5")
				}
				o.verbosity = n
			case "--n-threads":
				if n != 1 {
					return o, fmt.Errorf("--n-threads supports only 1 with the Go sequential simulator")
				}
			}
		case "--mbt":
			o.metadata = true
			if hasValue {
				v, err := strconv.ParseBool(value)
				if err != nil {
					return o, fmt.Errorf("--mbt must be a Boolean")
				}
				o.metadata = v
			}
		case "--no-mbt":
			if hasValue {
				return o, fmt.Errorf("--no-mbt does not take a value")
			}
			o.metadata = false
		case "--invariants", "--witnesses", "--hide":
			values := []string{}
			if hasValue {
				values = append(values, value)
			} else {
				for i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
					i++
					values = append(values, args[i])
				}
			}
			if len(values) == 0 {
				return o, fmt.Errorf("%s requires at least one value", name)
			}
			switch name {
			case "--invariants":
				o.invariants = append(o.invariants, values...)
			case "--witnesses":
				o.witnesses = append(o.witnesses, values...)
			case "--hide":
				o.hide = append(o.hide, values...)
			}
		default:
			return o, fmt.Errorf("unknown run option: %s", name)
		}
	}
	if o.input == "" {
		return o, fmt.Errorf("run requires an input file")
	}
	if o.seedSet && !o.samplesSet {
		o.maxSamples = 1
	}
	if o.traces > o.maxSamples {
		return o, fmt.Errorf("--n-traces (%d) cannot be greater than --max-samples (%d).", o.traces, o.maxSamples)
	}
	return o, nil
}

var errRunHelp = errors.New("run help")

// commandRun connects source preparation, sequential Go simulation and ITF output.
// All caller-visible output is written through the supplied streams or --out.
func commandRun(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	o, err := runParseOptions(args)
	if errors.Is(err, errRunHelp) {
		fmt.Fprintln(stdout, "Usage: quint run <input> [--main M] [--init init] [--step step] [--invariant true] [--invariants names...] [--max-steps 20] [--max-samples N] [--n-traces 1] [--seed N] [--out file] [--out-itf template] [--verbosity 0..5] [--mbt] [--backend go]")
		return 0
	}
	if err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return 1
	}
	ctx = quint.WithRuntimeHooks(ctx, quint.RuntimeHooks{DebugWriter: stdout})
	fields := insdict.NewDict[string, any]()
	fields.Set("stage", "loading")
	fields.Set("errors", []quint.ErrorMessage{})
	sources := quint.SourceMap{}
	sourceCode := insdict.NewDict[string, string]()
	fail := func(err error) int {
		var diagnostic quint.Diagnostic
		message := quint.ErrorMessage{Explanation: err.Error(), Locations: []quint.Location{}}
		if errors.As(err, &diagnostic) {
			message = quint.ResolveDiagnostic(sources, diagnostic)
		}
		fields.Set("errors", []quint.ErrorMessage{message})
		if o.out != "" {
			if writeErr := runWriteStage(o.out, fields); writeErr != nil {
				fmt.Fprintln(stderr, "error:", writeErr)
			}
		} else {
			fmt.Fprintln(stderr, quint.RenderErrorMessage(message, sourceCode, quint.DiagnosticRenderOptions{InclusiveEnd: true}))
		}
		return 1
	}
	if err := ctx.Err(); err != nil {
		return fail(err)
	}
	seed := quint.Integer{}
	if o.seedSet {
		seed, err = quint.ParseSeed(o.seed)
	} else {
		var bytes [8]byte
		if _, err = rand.Read(bytes[:]); err == nil {
			seed = quint.IntegerFromBig(new(big.Int).SetBytes(bytes[:]))
		}
	}
	if err != nil {
		return fail(err)
	}
	path, err := filepath.Abs(o.input)
	if err != nil {
		return fail(err)
	}
	loadedSources := map[string]string{}
	resolver := quint.NewFileSourceResolver(loadedSources, nil)
	frontend, err := quint.ParseProject(resolver, quint.SourceLookupPath{NormalizedPath: path, SourceName: path})
	paths := make([]string, 0, len(loadedSources))
	for name := range loadedSources {
		paths = append(paths, name)
	}
	sort.Strings(paths)
	for _, name := range paths {
		sourceCode.Set(name, loadedSources[name])
	}
	sources = coreCompatibilitySources(frontend)
	fields = coreStageFields("parsing", frontend, nil)
	if err != nil {
		return fail(err)
	}
	if len(frontend.Diagnostics) > 0 {
		return fail(frontend.Diagnostics[0])
	}
	if err = ctx.Err(); err != nil {
		return fail(err)
	}
	sourceDefault := ""
	for _, module := range frontend.Modules {
		if location, ok := sources[module.ID]; ok && location.Source == path {
			if sourceDefault != "" {
				sourceDefault = ""
				break
			}
			sourceDefault = module.Name
		}
	}
	index, err := quint.SelectMainModule(frontend.Modules, o.main, sourceDefault, o.input)
	if err != nil {
		return fail(quint.Diagnostic{Code: "QNT405", Message: "Main module " + quint.GuessMainModule(o.main, sourceDefault, o.input) + " not found"})
	}
	o.main = frontend.Modules[index].Name
	// Analyze original source before adding CLI expressions, preserving its stage
	// output. NewProjectEvaluator separately checks and flattens the added wrappers.
	analysis, err := quint.AnalyzeModules(frontend.Modules, frontend.Table)
	fields = coreStageFields("typechecking", frontend, &analysis)
	if err != nil {
		return fail(err)
	}
	if len(analysis.Diagnostics) > 0 {
		return fail(analysis.Diagnostics[0])
	}
	fields.Set("stage", "running")
	if err = runValidateAction(frontend, o.main, "init", o.init); err != nil {
		return fail(err)
	}
	if err = runValidateAction(frontend, o.main, "step", o.step); err != nil {
		return fail(err)
	}
	combined, individualInvariants := quint.CompilationInvariants(o.invariant, o.invariants)
	declarations, err := quint.SynthesizeCompilationEntries(quint.CompilationEntries{Target: "json", Init: &o.init, Step: &o.step, Invariants: []string{combined}}, frontend.IDs)
	if err != nil {
		return fail(err)
	}
	// Runtime selector wrappers must not claim the action metadata before the
	// user action runs. Run admits action effects without naming that action.
	for i := range declarations {
		if declarations[i].Name == "q::init" || declarations[i].Name == "q::step" {
			declarations[i].Qualifier = quint.Run
		}
	}
	witnessNames := []string{}
	for i, expression := range o.witnesses {
		name := fmt.Sprintf("q::runWitness%d", i)
		parsed, parseErr := quint.ParseExpressionOrDeclaration("val "+name+" = "+expression, frontend.IDs)
		if parseErr != nil {
			return fail(parseErr)
		}
		declarations = append(declarations, parsed.Declarations...)
		witnessNames = append(witnessNames, name)
	}
	invariantNames := []string{}
	if len(individualInvariants) > 1 {
		for i, expression := range individualInvariants {
			name := fmt.Sprintf("q::runInvariant%d", i)
			parsed, parseErr := quint.ParseExpressionOrDeclaration("val "+name+" = "+expression, frontend.IDs)
			if parseErr != nil {
				return fail(parseErr)
			}
			declarations = append(declarations, parsed.Declarations...)
			invariantNames = append(invariantNames, name)
		}
	}
	modules := append([]quint.Module{}, analysis.Modules...)
	modules[index].Declarations = append(append([]quint.Declaration{}, modules[index].Declarations...), declarations...)
	evaluator, err := quint.NewProjectEvaluator(ctx, modules, frontend.IDs, o.main, quint.NewRNG(seed), o.metadata)
	if err != nil {
		return fail(err)
	}
	started := time.Now()
	outcome, err := evaluator.Simulate(ctx, quint.SimulationOptions{Init: "q::init", Step: "q::step", Invariant: "q::inv", Witnesses: witnessNames, MaxSamples: o.maxSamples, MaxSteps: o.maxSteps, NumberOfTraces: o.traces})
	if err != nil {
		return fail(err)
	}
	fields.Set("status", outcome.Status)
	messages := make([]quint.ErrorMessage, len(outcome.Errors))
	for i, d := range outcome.Errors {
		messages[i] = quint.ResolveDiagnostic(sources, d)
	}
	fields.Set("errors", messages)
	serializedTraces := make([][]quint.Expr, len(outcome.BestTraces))
	hashes := evaluator.ObjectHashState()
	for i, trace := range outcome.BestTraces {
		serializedTraces[i], err = runExpressions(ctx, trace.States, hashes)
		if err != nil {
			return fail(err)
		}
	}
	if len(outcome.BestTraces) > 0 {
		fields.Set("trace", serializedTraces[0])
		fields.Set("seed", outcome.BestTraces[0].Seed)
	}
	elapsed := time.Since(started).Milliseconds()
	if o.outITF != "" {
		now := time.Now()
		writer := quint.TraceWriter{Template: o.outITF, TraceCount: o.traces, Metadata: o.metadata, Header: quint.ITFHeader{Source: o.input, Timestamp: now.UnixMilli(), CreatedAt: now.Format(time.RFC1123)}}
		for i, trace := range outcome.BestTraces {
			states := serializedTraces[i]
			status := "ok"
			if !trace.Result {
				status = "violation"
			}
			if _, err = writer.Write(i, status, evaluator.Storage.VarNames(), states, ""); err != nil {
				return fail(err)
			}
		}
	}
	if o.out != "" {
		if err = runWriteStage(o.out, fields); err != nil {
			fmt.Fprintln(stderr, "error:", err)
			return 1
		}
	} else {
		if o.verbosity >= 2 && len(serializedTraces) > 0 && len(serializedTraces[0]) > 0 {
			fmt.Fprint(stdout, "An example execution:\n\n")
			if err = runPrintTrace(stdout, serializedTraces[0], o.hide); err != nil {
				return fail(err)
			}
		}
		if o.verbosity >= 1 {
			summary := "No violation found"
			if outcome.Status == "violation" {
				summary = "Found an issue"
			} else if outcome.Status == "error" {
				summary = "Runtime error"
			}
			fmt.Fprintf(stdout, "[%s] %s (%dms at %s traces/second).\n", outcome.Status, summary, elapsed, runThroughput(outcome.Samples, elapsed))
			if outcome.Status == "violation" && len(invariantNames) > 1 && len(serializedTraces) > 0 && len(serializedTraces[0]) > 0 {
				states := serializedTraces[0]
				violations, reportErr := runViolatedInvariants(ctx, modules, frontend.IDs, o.main, invariantNames, states[len(states)-1])
				if reportErr != nil {
					return fail(reportErr)
				}
				for _, index := range violations {
					fmt.Fprintf(stdout, "  ❌ %s\n", individualInvariants[index])
				}
			}
			if o.verbosity >= 2 {
				if outcome.Status == "ok" {
					s := outcome.TraceStatistics
					fmt.Fprintf(stdout, "Trace length statistics: max=%s, min=%s, average=%.2f\n", runNumber(s.MaxTraceLength), runNumber(s.MinTraceLength), s.AverageTraceLength)
					fmt.Fprintln(stdout, "You may increase --max-samples and --max-steps.")
					fmt.Fprintln(stdout, "Use --verbosity to produce more (or less) output.")
				} else if outcome.Status == "violation" {
					fmt.Fprintln(stdout, "Use --verbosity=3 to show executions.")
				}
			}
			if outcome.Status != "error" && len(o.witnesses) > 0 {
				fmt.Fprintln(stdout, "Witnesses:")
				for i, witness := range o.witnesses {
					percentage := math.NaN()
					if outcome.Samples > 0 {
						percentage = 100 * float64(outcome.WitnessingTraces[i]) / float64(outcome.Samples)
					}
					fmt.Fprintf(stdout, "%s was witnessed in %d trace(s) out of %d explored (%.2f%%)\n", witness, outcome.WitnessingTraces[i], outcome.Samples, percentage)
				}
			}
		}
		for _, message := range runUniqueMessages(messages) {
			fmt.Fprintln(stderr, quint.RenderErrorMessage(message, sourceCode, quint.DiagnosticRenderOptions{InclusiveEnd: true}))
		}
		if o.verbosity >= 1 && o.outITF == "" && len(outcome.BestTraces) > 0 && outcome.BestTraces[0].Seed.Big().Sign() != 0 {
			fmt.Fprintf(stdout, "Use --seed=0x%s --backend=go to reproduce.\n", outcome.BestTraces[0].Seed.Big().Text(16))
		}
		switch outcome.Status {
		case "violation":
			fmt.Fprintln(stderr, "error: Invariant violated")
		case "error":
			fmt.Fprintln(stderr, "error: Runtime error")
		}
	}
	if outcome.Status != "ok" {
		return 1
	}
	return 0
}

func runValidateAction(frontend quint.FrontendResult, main, flag, text string) error {
	parsed, err := quint.ParseExpressionOrDeclaration(text, quint.NewIDGenerator())
	if err != nil || parsed.Kind != "expr" {
		return fmt.Errorf("Expected %s to be a valid expression", text)
	}
	if parsed.Expr.Kind != "name" {
		return nil
	}
	scope, ok := frontend.Scopes.Get2(main)
	if !ok {
		return nil
	}
	def, ok := scope.Lookup(parsed.Expr.Name)
	if !ok {
		return nil
	}
	if def.Declaration != nil && (def.Declaration.Kind != "def" || def.Declaration.Qualifier != quint.Action) {
		kind := def.Declaration.Kind
		if kind == "def" {
			kind = string(def.Declaration.Qualifier)
		}
		return quint.Diagnostic{Code: "QNT501", Message: fmt.Sprintf("default --%s action name '%s' is used by a %s declaration", flag, text, kind)}
	}
	return nil
}

func runExpressions(ctx context.Context, values []quint.RuntimeValue, hashes *quint.RuntimeObjectHashState) ([]quint.Expr, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	result := make([]quint.Expr, len(values))
	for i, value := range values {
		expression, err := quint.SerializeRuntimeValueContext(ctx, value, hashes)
		if err != nil {
			return nil, err
		}
		result[i] = expression
	}
	return result, nil
}
func runVariables(states []quint.Expr, metadata bool) []string {
	names := []string{}
	if len(states) == 0 {
		return names
	}
	for i := 0; i+1 < len(states[0].Args); i += 2 {
		key := states[0].Args[i]
		if key.Kind == "str" && (!metadata || (key.Str != quint.ITFActionTaken && key.Str != quint.ITFNondetPicks)) {
			names = append(names, key.Str)
		}
	}
	return names
}
func runWriteStage(filename string, fields *insdict.Dict[string, any]) error {
	data, err := coreMarshalStage(fields)
	if err != nil {
		return err
	}
	return os.WriteFile(filename, append(data, '\n'), 0666)
}

func runThroughput(samples int, elapsed int64) string {
	return runNumber(math.Round(float64(samples) * 1000 / float64(elapsed)))
}

func runNumber(value float64) string {
	if math.IsInf(value, 1) {
		return "Infinity"
	}
	if math.IsInf(value, -1) {
		return "-Infinity"
	}
	return strconv.FormatFloat(value, 'f', -1, 64)
}

func runUniqueMessages(messages []quint.ErrorMessage) []quint.ErrorMessage {
	seen := insdict.NewDict[string, bool]()
	unique := make([]quint.ErrorMessage, 0, len(messages))
	for _, message := range messages {
		encoded, _ := json.Marshal(message)
		key := string(encoded)
		if !seen.Get(key) {
			seen.Set(key, true)
			unique = append(unique, message)
		}
	}
	return unique
}

// Reporting runs in its own evaluator. Upstream first evaluates each invariant
// to register reads, then restores the selected final state and evaluates again.
// An invariant that fails at runtime is omitted from the list of false values.
func runViolatedInvariants(ctx context.Context, modules []quint.Module, ids *quint.IDGenerator, main string, names []string, state quint.Expr) ([]int, error) {
	reporter, err := quint.NewProjectEvaluator(ctx, modules, ids, main, quint.NewRNG(quint.Integer{}), false)
	if err != nil {
		return nil, err
	}
	record, err := quint.NewRuntimeValue(state)
	if err != nil {
		return nil, err
	}
	violations := []int{}
	for i, name := range names {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		_, _ = reporter.Evaluate(ctx, name, nil)
		if err := reporter.Storage.FromRecord(record); err != nil {
			return nil, err
		}
		value, err := reporter.Evaluate(ctx, name, nil)
		if err != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			continue
		}
		expression := value.Expression()
		if expression.Kind == "bool" && !expression.Bool {
			violations = append(violations, i)
		}
	}
	return violations, nil
}
