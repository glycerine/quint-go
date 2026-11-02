package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	quint "github.com/glycerine/quint-go/pkg"
	"github.com/glycerine/quint-go/pkg/backend/apalache"
	gotlc "github.com/glycerine/quint-go/pkg/backend/tlc"
)

type verifyBackend interface {
	Verify(context.Context, quint.CompilationResult, json.RawMessage, apalache.VerificationOptions) (apalache.VerificationResult, error)
}

type verifyOptions struct {
	input, main, init, step, invariant, inductive, temporal string
	config, tlcConfig, out, outITF, backend                 string
	maxSteps, verbosity                                     int
	randomTransitions                                       bool
	invariants                                              verifyStringFlags
}

type verifyStringFlags []string

func (s *verifyStringFlags) String() string { return strings.Join(*s, ",") }
func (s *verifyStringFlags) Set(value string) error {
	*s = append(*s, value)
	return nil
}

// verifyStage keeps backend JSON lossless and reports a bounded result without
// presenting bounded checking as an unbounded proof.
type verifyStage struct {
	Status      string                       `json:"status"`
	Input       string                       `json:"input,omitempty"`
	Main        string                       `json:"main,omitempty"`
	Outcome     apalache.CheckOutcome        `json:"outcome,omitempty"`
	Explanation string                       `json:"explanation,omitempty"`
	Errors      []quint.ErrorMessage         `json:"errors"`
	Trace       json.RawMessage              `json:"trace,omitempty"`
	Phases      []apalache.VerificationPhase `json:"phases,omitempty"`
	MaxSteps    *int                         `json:"maxSteps,omitempty"`
	BackendData string                       `json:"backendData,omitempty"`
	TLC         *gotlc.Result                `json:"tlc,omitempty"`
}

func commandVerify(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	return verifyWithBackend(ctx, args, stdout, stderr, apalache.New())
}

func verifyWithBackend(ctx context.Context, args []string, stdout, stderr io.Writer, backend verifyBackend) int {
	return verifyWithCheckers(ctx, args, stdout, stderr, backend, gotlc.Check)
}

type verifyTLCBackend func(context.Context, quint.CompilationResult, gotlc.Options) (gotlc.Result, error)

func verifyWithCheckers(ctx context.Context, args []string, stdout, stderr io.Writer, backend verifyBackend, tlcBackend verifyTLCBackend) int {
	options, help, err := verifyParseOptions(args, stdout, stderr)
	if help {
		return 0
	}
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	stage := verifyStage{Status: "failure", Input: options.input, Errors: []quint.ErrorMessage{}}
	if !verifyUsesTLC(options) {
		stage.MaxSteps = &options.maxSteps
	}
	fail := func(err error) int {
		stage.Explanation = err.Error()
		return verifyOutput(options, stage, stdout, stderr)
	}
	if err = ctx.Err(); err != nil {
		return fail(err)
	}
	tlcOptions := gotlc.Options{}
	if options.tlcConfig != "" {
		data, loadErr := os.ReadFile(options.tlcConfig)
		if loadErr != nil {
			return fail(fmt.Errorf("load TLC configuration: %w", loadErr))
		}
		tlcOptions, err = gotlc.ParseConfig(data)
		if err != nil {
			return fail(err)
		}
	}
	var loaded json.RawMessage
	if options.config != "" {
		loaded, err = os.ReadFile(options.config)
		if err != nil {
			return fail(fmt.Errorf("load Apalache configuration: %w", err))
		}
		if !json.Valid(loaded) || len(bytes.TrimSpace(loaded)) == 0 || bytes.TrimSpace(loaded)[0] != '{' {
			return fail(fmt.Errorf("Apalache configuration must be a JSON object"))
		}
	}
	resolver := quint.NewFileSourceResolver(nil, nil)
	abs, err := filepath.Abs(options.input)
	if err != nil {
		return fail(err)
	}
	frontend, err := quint.ParseProject(resolver, quint.SourceLookupPath{NormalizedPath: abs, SourceName: abs})
	if err != nil {
		return fail(err)
	}
	addDiagnostics := func(diagnostics []quint.Diagnostic) {
		for _, diagnostic := range diagnostics {
			stage.Errors = append(stage.Errors, quint.ResolveDiagnostic(frontend.Sources, diagnostic))
		}
	}
	if len(frontend.Diagnostics) > 0 {
		addDiagnostics(frontend.Diagnostics)
		stage.Explanation = "parsing failed"
		return verifyOutput(options, stage, stdout, stderr)
	}
	analysis, err := quint.AnalyzeModules(frontend.Modules, frontend.Table)
	if err != nil {
		return fail(err)
	}
	if len(analysis.Diagnostics) > 0 {
		addDiagnostics(analysis.Diagnostics)
		stage.Explanation = "typechecking failed"
		return verifyOutput(options, stage, stdout, stderr)
	}
	_, invariants := quint.CompilationInvariants(options.invariant, options.invariants)
	compiled, err := quint.CompileModules(analysis.Modules, quint.AnalysisOutput{Types: analysis.Types, Effects: analysis.Effects, Modes: analysis.Modes}, frontend.IDs, frontend.Sources, quint.CompilationOptions{
		Main: options.main, DefaultModule: coreDefaultModule(frontend, abs), Input: options.input,
		CompilationEntries: quint.CompilationEntries{
			Target: "json", Flatten: true, Init: &options.init, Step: &options.step,
			Invariants: invariants, InductiveInvariant: options.inductive,
			Temporal: options.temporal,
		},
	})
	if err != nil {
		return fail(err)
	}
	stage.Main = compiled.Main
	if len(compiled.Diagnostics) > 0 || len(compiled.AnalysisDiagnostics) > 0 {
		addDiagnostics(compiled.Diagnostics)
		addDiagnostics(compiled.AnalysisDiagnostics)
		stage.Explanation = "compilation failed"
		return verifyOutput(options, stage, stdout, stderr)
	}
	if err = ctx.Err(); err != nil {
		return fail(err)
	}
	if verifyUsesTLC(options) {
		if len(invariants) == 0 {
			tlcOptions.Invariants = []string{}
		}
		result, checkErr := tlcBackend(ctx, compiled, tlcOptions)
		stage.TLC = &result
		if checkErr != nil {
			return fail(checkErr)
		}
		if !result.Complete {
			return fail(fmt.Errorf("TLC did not complete checking"))
		}
		stage.Outcome = apalache.CheckOutcome(result.Outcome)
		switch result.Outcome {
		case gotlc.Safe:
			stage.Status = "ok"
			stage.Explanation = fmt.Sprintf("TLC explored all %d distinct states; no violation found.", result.DistinctStates)
		case gotlc.Violation, gotlc.Deadlock:
			stage.Status = "violation"
			stage.Explanation = fmt.Sprintf("TLC found a %s.\n%s", result.Outcome, result.Log)
		default:
			return fail(fmt.Errorf("invalid TLC outcome %q", result.Outcome))
		}
		return verifyOutput(options, stage, stdout, stderr)
	}
	checkOptions := apalache.CheckOptions{MaxSteps: options.maxSteps, RandomTransitions: options.randomTransitions}
	if len(invariants) == 0 {
		checkOptions.Invariants = []string{}
	}
	if options.temporal != "" {
		checkOptions.Temporal = []string{"q::temporalProps"}
	}
	result, err := backend.Verify(ctx, compiled, loaded, apalache.VerificationOptions{CheckOptions: checkOptions, Inductive: options.inductive != ""})
	report := apalache.ReportVerification(result, err, frontend.Sources)
	stage.Outcome = result.Outcome
	stage.Explanation = report.Explanation
	stage.Errors = report.Errors
	if err != nil {
		stage.BackendData = report.BackendData
	}
	for _, phase := range result.Phases {
		stage.Phases = append(stage.Phases, phase.Phase)
	}
	if len(report.Traces) > 0 {
		stage.Trace = report.Traces[0]
	}
	if err == nil && result.Outcome == apalache.CheckSafe {
		stage.Status = "ok"
	} else if err == nil && (result.Outcome == apalache.CheckViolation || result.Outcome == apalache.CheckDeadlock) {
		stage.Status = "violation"
	}
	return verifyOutput(options, stage, stdout, stderr)
}

func verifyParseOptions(args []string, stdout, stderr io.Writer) (verifyOptions, bool, error) {
	options := verifyOptions{}
	flags := flag.NewFlagSet("verify", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	flags.StringVar(&options.main, "main", "", "main module name")
	flags.StringVar(&options.init, "init", "init", "initializer action or expression")
	flags.StringVar(&options.step, "step", "step", "step action or expression")
	flags.StringVar(&options.invariant, "invariant", "", "comma-separated invariants")
	flags.Var(&options.invariants, "invariants", "space-separated invariant names")
	flags.StringVar(&options.inductive, "inductive-invariant", "", "inductive invariant")
	flags.StringVar(&options.temporal, "temporal", "", "comma-separated temporal properties")
	flags.StringVar(&options.config, "apalache-config", "", "additional Go Apalache JSON configuration")
	flags.StringVar(&options.tlcConfig, "tlc-config", "", "native Go TLC JSON: workers, checkDeadlock, soft timeoutMillis (preparation/builtins may exceed it)")
	flags.StringVar(&options.out, "out", "", "write a JSON verification result and suppress console output")
	flags.StringVar(&options.outITF, "out-itf", "", "write the counterexample as lossless ITF and suppress console output")
	flags.StringVar(&options.backend, "backend", "apalache", "verification backend (apalache, apalache-go, tlc, or tlc-go)")
	flags.IntVar(&options.maxSteps, "max-steps", 10, "maximum number of transitions")
	flags.IntVar(&options.verbosity, "verbosity", 2, "console verbosity (0 to 5)")
	flags.BoolVar(&options.randomTransitions, "random-transitions", false, "choose symbolic transitions at random")
	printHelp := func() {
		fmt.Fprintln(stdout, "Usage: quint verify <input> [options]")
		flags.SetOutput(stdout)
		flags.PrintDefaults()
		flags.SetOutput(io.Discard)
	}
	flags.Usage = func() {}
	normalized, err := verifyInterspersedArgs(args)
	if err != nil {
		return options, false, err
	}
	if err = flags.Parse(normalized); err != nil {
		if err == flag.ErrHelp {
			printHelp()
			return options, true, nil
		}
		return options, false, err
	}
	if flags.NArg() != 1 {
		return options, false, fmt.Errorf("verify requires exactly one input file")
	}
	options.input = flags.Arg(0)
	if options.maxSteps < 0 {
		return options, false, fmt.Errorf("maximum steps cannot be negative")
	}
	if options.verbosity < 0 || options.verbosity > 5 {
		return options, false, fmt.Errorf("verbosity must be between 0 and 5")
	}
	if options.backend != "apalache" && options.backend != "apalache-go" && !verifyUsesTLC(options) {
		return options, false, fmt.Errorf("unsupported verification backend %q; choose a Go Apalache or Go TLC backend", options.backend)
	}
	if verifyUsesTLC(options) {
		explicitBound := false
		flags.Visit(func(f *flag.Flag) {
			if f.Name == "max-steps" {
				explicitBound = true
			}
		})
		if explicitBound {
			return options, false, fmt.Errorf("--max-steps is unsupported by exhaustive Go TLC checking")
		}
		if options.config != "" {
			return options, false, fmt.Errorf("--apalache-config is unsupported by Go TLC; use --tlc-config")
		}
		if options.randomTransitions {
			return options, false, fmt.Errorf("--random-transitions is unsupported by Go TLC")
		}
		if options.inductive != "" {
			return options, false, fmt.Errorf("--inductive-invariant is unsupported by Go TLC")
		}
		if options.temporal != "" {
			return options, false, fmt.Errorf("--temporal is not yet supported by the Go TLC adapter")
		}
		if options.outITF != "" {
			return options, false, fmt.Errorf("--out-itf is not yet supported by Go TLC; --out retains concrete TLC counterexample states")
		}
	} else if options.tlcConfig != "" {
		return options, false, fmt.Errorf("--tlc-config requires the Go TLC backend")
	}
	return options, false, nil
}

func verifyUsesTLC(options verifyOptions) bool {
	return options.backend == "tlc" || options.backend == "tlc-go"
}

// flag.FlagSet stops at the first positional argument; Quint accepts options on
// either side of its input. Normalize those arguments without changing values.
func verifyInterspersedArgs(args []string) ([]string, error) {
	flags, positional := []string{}, []string{}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			positional = append(positional, args[i+1:]...)
			break
		}
		if !strings.HasPrefix(arg, "-") || arg == "-" {
			positional = append(positional, arg)
			continue
		}
		name := strings.TrimLeft(strings.SplitN(arg, "=", 2)[0], "-")
		switch name {
		case "server-endpoint", "apalache-version":
			return nil, fmt.Errorf("--%s is unsupported by the in-process Go Apalache backend", name)
		case "invariants":
			if strings.Contains(arg, "=") {
				flags = append(flags, arg)
				continue
			}
			count := 0
			for i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				i++
				flags = append(flags, "--invariants="+args[i])
				count++
			}
			if count == 0 {
				return nil, fmt.Errorf("--invariants requires at least one invariant name")
			}
		case "no-random-transitions":
			flags = append(flags, "--random-transitions=false")
		case "random-transitions":
			if !strings.Contains(arg, "=") && i+1 < len(args) && (args[i+1] == "true" || args[i+1] == "false") {
				i++
				arg += "=" + args[i]
			}
			flags = append(flags, arg)
		case "h", "help":
			flags = append(flags, arg)
		default:
			flags = append(flags, arg)
			if !strings.Contains(arg, "=") {
				if i+1 >= len(args) {
					return nil, fmt.Errorf("flag needs an argument: %s", arg)
				}
				i++
				flags = append(flags, args[i])
			}
		}
	}
	return append(append(flags, "--"), positional...), nil
}

func verifyOutput(options verifyOptions, stage verifyStage, stdout, stderr io.Writer) int {
	code := 1
	if stage.Status == "ok" {
		code = 0
	}
	if options.out != "" {
		data, err := json.MarshalIndent(stage, "", "  ")
		if err == nil {
			err = os.WriteFile(options.out, append(data, '\n'), 0o644)
		}
		if err != nil {
			fmt.Fprintln(stderr, "write verification result:", err)
			return 1
		}
	}
	if options.outITF != "" && len(stage.Trace) > 0 {
		if !json.Valid(stage.Trace) {
			fmt.Fprintln(stderr, "write counterexample: invalid ITF JSON")
			return 1
		}
		if err := os.WriteFile(options.outITF, append(append([]byte(nil), stage.Trace...), '\n'), 0o644); err != nil {
			fmt.Fprintln(stderr, "write counterexample:", err)
			return 1
		}
	}
	if options.out != "" || options.outITF != "" || options.verbosity == 0 {
		return code
	}
	if code == 0 {
		var err error
		if verifyUsesTLC(options) {
			_, err = fmt.Fprintln(stdout, "[ok] "+stage.Explanation)
		} else if options.inductive != "" {
			_, err = fmt.Fprintln(stdout, "[ok] Inductive invariant verified.")
		} else {
			_, err = fmt.Fprintf(stdout, "[ok] No violation found up to %d steps.\n", options.maxSteps)
		}
		if err != nil {
			fmt.Fprintln(stderr, "write verification result:", err)
			return 1
		}
	} else {
		fmt.Fprintf(stderr, "[%s] %s\n", stage.Status, stage.Explanation)
		for _, diagnostic := range stage.Errors {
			fmt.Fprintln(stderr, diagnostic.Explanation)
		}
	}
	return code
}
