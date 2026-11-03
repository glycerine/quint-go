// Package tlc adapts compiled Quint to the native Go TLC model checker. It does
// not launch Java, Scala, subprocesses, or network services.
package tlc

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	quint "github.com/glycerine/quint-go/pkg"
	"github.com/glycerine/quint-go/pkg/backend/apalache"
	"github.com/glycerine/tlago"
	native "github.com/glycerine/tlago/tlc"
)

// Options selects exhaustive finite-state checking. Zero Workers uses
// GOMAXPROCS workers, matching Quint's automatic worker selection. Deadlock
// checking is opt-in, matching Quint's TLC command. Unlike
// bounded Apalache checking, TLC has no maximum-transition bound. Temporal
// names select TLA operators, including the compiled q_temporalProps wrapper.
// INIT/NEXT permits implicit infinite stuttering; fairness must be explicit in
// the requested properties when progress requires it.
type Options struct {
	Init, Next           string
	Invariants, Temporal []string
	Workers              int
	CheckDeadlock        bool
	// ITF exports counterexamples using checked Quint types. Available through
	// Check; untyped CheckTLA cannot distinguish tuples, lists, maps or variants.
	// Conversion failures populate Result.ITFError without changing the verdict.
	ITF bool
	// MaxDuration starts a cooperative cancellation deadline, including TLA
	// generation. Native parsing, tool construction (which may expand constant
	// next-state quantifiers), liveness graph analysis, and individual builtins
	// cannot be preempted.
	// They may exceed this duration; expiry always returns incomplete + error.
	MaxDuration time.Duration
}

type Outcome string

const (
	Safe      Outcome = "safe"
	Violation Outcome = "violation"
	Deadlock  Outcome = "deadlock"
)

// Result reports a definitive check outcome only when Complete is true. Safe
// means exhaustive exploration; violations may stop the search early. Log retains
// the native TLC diagnostic text; Messages retains stable machine-readable codes.
type Result struct {
	Outcome         Outcome   `json:"outcome,omitempty"`
	Complete        bool      `json:"complete"`
	ExitStatus      int       `json:"exitStatus"`
	ErrorCode       int       `json:"errorCode"`
	StatesGenerated int64     `json:"statesGenerated"`
	DistinctStates  uint64    `json:"distinctStates"`
	InitialStates   int64     `json:"initialStates"`
	QueueSize       int64     `json:"queueSize"`
	SearchDepth     int64     `json:"searchDepth"`
	Messages        []Message `json:"messages"`
	Trace           []State   `json:"trace,omitempty"`
	// Loop is the zero-based target of the final transition in a temporal
	// counterexample. A last-state target represents infinite stuttering.
	Loop *int   `json:"loop,omitempty"`
	Log  string `json:"log,omitempty"`
	// ITF is an immutable typed counterexample snapshot when requested. The CLI
	// renders it in its standard trace field or writes it through --out-itf.
	ITF json.RawMessage `json:"-"`
	// ITFError is an export failure independent of a definitive checker verdict.
	// Cancellation still returns an incomplete result and a context error.
	ITFError string `json:"itfError,omitempty"`
}

// State retains the native counterexample's concrete TLA values. Values are
// strings in TLA syntax; this deliberately does not mislabel them as ITF JSON.
type State struct {
	Index  int          `json:"index"`
	Values []Assignment `json:"values"`
}

type Assignment struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type Message struct {
	Code       int      `json:"code"`
	Parameters []string `json:"parameters,omitempty"`
}

// TLC owns process-wide intern tables, output streams, and checker state. Calls
// through this adapter are serialized, including parsing and tool construction.
// Runner configuration is restored after each call. Intern tables and native
// class caches remain shared: callers must not concurrently invoke native TLC
// or resume tools/states constructed before an adapter call.
var ownership = make(chan struct{}, 1)

// Variants is Apalache's standard module, also used by its Go printer. The
// upstream tlago library currently embeds the other generated dependencies.
// Source: github.com/glycerine/apalache-go/stdlib/Variants.tla (Apache-2.0).
//
//go:embed stdlib/Variants.tla
var variantsModule []byte

// Check generates TLA with the existing Go Apalache printer and checks it with
// Go TLC. Nil invariants select the compiled q::inv wrapper when present.
func Check(ctx context.Context, compiled quint.CompilationResult, options Options) (Result, error) {
	if err := validateOptions(options); err != nil {
		return Result{}, err
	}
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	if options.MaxDuration > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, options.MaxDuration)
		defer cancel()
		options.MaxDuration = 0
	}
	if options.Invariants == nil {
		options.Invariants = []string{}
		for _, d := range compiled.MainModule.Declarations {
			if d.Kind == "def" && d.Name == "q::inv" {
				options.Invariants = append(options.Invariants, "q_inv")
			}
		}
	}
	tla, err := apalache.New().TLA(ctx, compiled)
	if err != nil {
		return Result{}, fmt.Errorf("generate TLC input: %w", err)
	}
	tla, err = nativeGeneratedTLA(tla, compiled)
	if err != nil {
		return Result{}, err
	}
	var variables []itfVariable
	var itfErr error
	if options.ITF {
		variables, itfErr = nativeITFVariables(compiled)
		if itfErr != nil {
			options.ITF = false
		}
	}
	result, err := checkTLA(ctx, compiled.MainModule.Name+".tla", tla, options, variables)
	if err == nil && itfErr != nil && result.Outcome != Safe {
		result.ITFError = itfErr.Error()
	}
	return result, err
}

// CheckTLA checks TLA generated by Quint, using compiled wrapper names by
// default. Every result with an error is incomplete and has an empty Outcome.
func CheckTLA(ctx context.Context, filename, source string, options Options) (result Result, err error) {
	if options.ITF {
		return result, fmt.Errorf("TLC ITF export requires checked Quint types; use Check")
	}
	return checkTLA(ctx, filename, source, options, nil)
}

func checkTLA(ctx context.Context, filename, source string, options Options, variables []itfVariable) (result Result, err error) {
	if err = validateOptions(options); err != nil {
		return result, err
	}
	if options.MaxDuration > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, options.MaxDuration)
		defer cancel()
	}
	select {
	case ownership <- struct{}{}:
		defer func() { <-ownership }()
	case <-ctx.Done():
		return result, ctx.Err()
	}
	if err = ctx.Err(); err != nil {
		return result, err
	}
	restoreGlobals := isolateGlobals()
	defer restoreGlobals()
	// Capture runtime panics as failures. Native worker failures are returned by
	// ModelCheck; this boundary also covers parser and tool-construction panics.
	defer func() {
		if p := recover(); p != nil {
			result.Outcome = ""
			result.Complete = false
			err = fmt.Errorf("Go TLC failure: %v", p)
		}
	}()
	var log lockedBuffer
	restore := native.ToolIOSetSystemStreams(&log, &log)
	defer restore()
	defer func() { result.Log = log.String() }()
	native.InitializeToolProperties()
	if native.IsProbabilistic() {
		return result, fmt.Errorf("Go TLC probabilistic state sampling is unsupported for exhaustive verification")
	}
	// Validate temporary storage before native resolvers attempt their own
	// allocations, whose failure reporting writes directly to process stderr.
	metadata, err := os.MkdirTemp("", "quint-go-tlc-")
	if err != nil {
		return result, err
	}
	defer os.RemoveAll(metadata)
	config, err := modelConfig(options)
	if err != nil {
		return result, err
	}
	native.SetModelValues()
	if err = os.WriteFile(filepath.Join(metadata, "Variants.tla"), variantsModule, 0600); err != nil {
		return result, err
	}
	spec, diagnostics := tlago.CheckSanySourceWithOptions(filename, source, tlago.LoadOptions{LibraryPaths: []string{metadata}})
	if diagnostics.HasErrors() {
		return result, fmt.Errorf("TLC parsing: %v", diagnostics)
	}
	tool, diagnostics := tlago.BuildTLCTool(spec, config, native.RuntimeParameters{})
	if diagnostics.HasErrors() {
		return result, fmt.Errorf("TLC tool construction: %v", diagnostics)
	}
	installCancellation(ctx, tool)
	if err = ctx.Err(); err != nil {
		return result, err
	}
	native.SetMainChecker(nil)
	defer native.SetMainChecker(nil)
	stop := watchCancellation(ctx)
	defer stop()
	workers := options.Workers
	if workers == 0 {
		workers = runtime.GOMAXPROCS(0)
	}
	checked, runErr := native.ModelCheck(ctx, native.Options{Tool: tool, FPSet: native.NewMemFPSet(), SpecFile: filename, MetaDir: metadata, Workers: workers, Deadlock: options.CheckDeadlock, NoDeadlock: !options.CheckDeadlock, Cleanup: true, DFIDModeSet: true, DFIDMode: false, StopAfter: -1, ToolModeSet: true, ToolMode: false})
	stop()
	if checked != nil {
		result.ExitStatus = checked.ExitStatus
		result.ErrorCode = checked.ErrorCode
		result.StatesGenerated = checked.StatesGenerated
		result.DistinctStates = checked.DistinctStates
		result.InitialStates = checked.InitialStates
		result.QueueSize = checked.QueueSize
		result.SearchDepth = checked.SearchDepth
		for _, m := range checked.Messages {
			result.Messages = append(result.Messages, Message{Code: m.Code, Parameters: append([]string(nil), m.Params...)})
		}
		var vars []string
		for _, d := range spec.Root.Declarations {
			if d.Kind == tlago.VariableDecl {
				vars = append(vars, d.Names...)
			}
		}
		result.Trace = counterexample(checked.Messages, vars)
		result.Loop, err = temporalLoop(checked.Messages, len(result.Trace))
		if err != nil {
			return result, fmt.Errorf("TLC counterexample: %w", err)
		}
		// TLC reports an initial invariant/property violation as an error message rather
		// than a state-print event, but retains the actual state on its checker.
		if initialViolation(checked.ErrorCode) && len(result.Trace) == 0 {
			if checker, ok := native.MainChecker().(*native.ModelChecker); ok && checker.ErrState != nil {
				result.Trace = counterexample([]native.Message{{Code: native.ECTLCStatePrint1, State: checker.ErrState}}, vars)
			}
		}
	}
	if err = ctx.Err(); err != nil {
		return result, err
	}
	if runErr != nil {
		return result, fmt.Errorf("TLC checking: %w", runErr)
	}
	if checked == nil {
		return result, fmt.Errorf("TLC returned no result")
	}
	// Some native modes continue after a violation and return zero. Never turn
	// an emitted error into a successful verification, even if native defaults
	// change in a future version.
	if checked.ExitStatus == native.ExitStatusSuccess {
		for _, message := range checked.Messages {
			if message.Severity == native.SeverityError && !message.FormattingOnly {
				result.ErrorCode = message.Code
				result.ExitStatus = native.ExitStatusForErrorCode(message.Code)
				checked.ExitStatus = result.ExitStatus
				break
			}
		}
	}
	// Native exit statuses group several evaluation errors with genuine
	// assumption/action-property violations. A failed predicate evaluation does
	// not establish a counterexample, even when native TLC returns status 10–14.
	if evaluationFailure(result.ErrorCode) {
		return result, fmt.Errorf("TLC evaluation failed (error %d)", result.ErrorCode)
	}
	switch checked.ExitStatus {
	case native.ExitStatusSuccess:
		if result.QueueSize != 0 {
			return result, fmt.Errorf("TLC stopped with %d states still queued", result.QueueSize)
		}
		result.Outcome = Safe
	case native.ExitStatusViolationDeadlock:
		result.Outcome = Deadlock
	case native.ExitStatusViolationSafety, native.ExitStatusViolationAssumption, native.ExitStatusViolationAssert:
		result.Outcome = Violation
	case native.ExitStatusViolationLiveness:
		// State and action PROPERTY predicates can fail on a finite prefix.
		// Only a genuine temporal counterexample needs a closing lasso marker.
		finiteProperty := result.ErrorCode == native.ECTLCPropertyViolatedInitial || result.ErrorCode == native.ECTLCActionPropertyViolatedBehavior
		if !finiteProperty && (result.Loop == nil || len(result.Trace) == 0) {
			return result, fmt.Errorf("TLC temporal counterexample is missing loop metadata")
		}
		result.Outcome = Violation
	default:
		return result, fmt.Errorf("TLC failed with exit status %d (error %d)", checked.ExitStatus, result.ErrorCode)
	}
	if options.ITF && result.Outcome != Safe {
		states := nativeITFStates(checked.Messages)
		if initialViolation(checked.ErrorCode) && len(states) == 0 {
			if checker, ok := native.MainChecker().(*native.ModelChecker); ok && checker.ErrState != nil {
				states = append(states, checker.ErrState)
			}
		}
		values := make([][]native.Value, 0, len(states))
		for _, state := range states {
			entry := make([]native.Value, len(variables))
			for i, variable := range variables {
				entry[i] = state.LookupName(variable.tlaName)
			}
			values = append(values, entry)
		}
		result.ITF, err = nativeITFTrace(ctx, variables, values, result.Loop)
		if err != nil {
			if ctxErr := ctx.Err(); ctxErr != nil {
				result.Outcome = ""
				return result, ctxErr
			}
			result.ITFError = err.Error()
			result.ITF = nil
			err = nil
		}
	}
	result.Complete = true
	return result, nil
}

func counterexample(messages []native.Message, variables []string) []State {
	var trace []State
	for _, m := range messages {
		if m.Code != native.ECTLCStatePrint1 && m.Code != native.ECTLCStatePrint2 {
			continue
		}
		state := m.State
		if m.StateInfo != nil {
			state = m.StateInfo.GetOriginalState()
		}
		if state == nil {
			continue
		}
		entry := State{Index: len(trace), Values: []Assignment{}}
		for _, name := range variables {
			if value := state.LookupName(name); value != nil {
				entry.Values = append(entry.Values, Assignment{name, value.String()})
			}
		}
		trace = append(trace, entry)
	}
	return trace
}

func validateOptions(options Options) error {
	if options.Workers < 0 {
		return fmt.Errorf("TLC workers cannot be negative")
	}
	if options.MaxDuration < 0 {
		return fmt.Errorf("TLC maximum duration cannot be negative")
	}
	names := append([]string{options.Init, options.Next}, options.Invariants...)
	names = append(names, options.Temporal...)
	for _, name := range names {
		if name != "" && !validName(name) {
			return fmt.Errorf("invalid TLC operator name %q", name)
		}
	}
	for _, name := range options.Invariants {
		if name == "" {
			return fmt.Errorf("TLC invariant name cannot be empty")
		}
	}
	for _, name := range options.Temporal {
		if name == "" {
			return fmt.Errorf("TLC temporal property name cannot be empty")
		}
	}
	return nil
}

func validName(name string) bool {
	for i, c := range name {
		if !(c == '_' || c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || i > 0 && c >= '0' && c <= '9') {
			return false
		}
	}
	return name != ""
}

func modelConfig(options Options) (*native.ModelConfig, error) {
	init, next := options.Init, options.Next
	if init == "" {
		init = "q_init"
	}
	if next == "" {
		next = "q_step"
	}
	var cfg strings.Builder
	fmt.Fprintf(&cfg, "INIT %s\nNEXT %s\n", init, next)
	for _, name := range options.Invariants {
		fmt.Fprintf(&cfg, "INVARIANT %s\n", name)
	}
	for _, name := range options.Temporal {
		fmt.Fprintf(&cfg, "PROPERTY %s\n", name)
	}
	return native.ParseModelConfigSource("quint.cfg", cfg.String())
}

func watchCancellation(ctx context.Context) func() {
	done, joined := make(chan struct{}), make(chan struct{})
	go func() {
		defer close(joined)
		select {
		case <-done:
			return
		case <-ctx.Done():
		}
		// Cancellation may arrive between tool creation and checker publication.
		ticker := time.NewTicker(time.Millisecond)
		defer ticker.Stop()
		for {
			if checker := native.MainChecker(); checker != nil {
				checker.Stop()
				return
			}
			select {
			case <-done:
				return
			case <-ticker.C:
			}
		}
	}()
	var once sync.Once
	return func() { once.Do(func() { close(done); <-joined }) }
}

type lockedBuffer struct {
	mu sync.Mutex
	bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.Buffer.Write(p)
}
func (b *lockedBuffer) String() string { b.mu.Lock(); defer b.mu.Unlock(); return b.Buffer.String() }

// ParseConfig accepts native runtime settings and rejects legacy JVM settings.
// It returns an error for unknown keys, preventing silently ignored options.
func ParseConfig(data json.RawMessage) (Options, error) {
	var cfg struct {
		Workers       json.RawMessage `json:"workers"`
		CheckDeadlock json.RawMessage `json:"checkDeadlock"`
		TimeoutMillis json.RawMessage `json:"timeoutMillis"`
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if len(bytes.TrimSpace(data)) == 0 || bytes.TrimSpace(data)[0] != '{' {
		return Options{}, fmt.Errorf("TLC configuration must be a JSON object")
	}
	if err := decoder.Decode(&cfg); err != nil {
		return Options{}, fmt.Errorf("TLC configuration: %w", err)
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return Options{}, fmt.Errorf("TLC configuration must contain one JSON object")
	}
	options := Options{}
	if len(cfg.CheckDeadlock) > 0 {
		if bytes.Equal(bytes.TrimSpace(cfg.CheckDeadlock), []byte("null")) {
			return Options{}, fmt.Errorf("TLC checkDeadlock must be a boolean")
		}
		if err := json.Unmarshal(cfg.CheckDeadlock, &options.CheckDeadlock); err != nil {
			return Options{}, fmt.Errorf("TLC checkDeadlock must be a boolean")
		}
	}
	if len(cfg.Workers) > 0 && string(cfg.Workers) != `"auto"` {
		if err := json.Unmarshal(cfg.Workers, &options.Workers); err != nil || options.Workers <= 0 {
			return Options{}, fmt.Errorf("TLC workers must be a positive integer or auto")
		}
	}
	var timeoutMillis int64
	if len(cfg.TimeoutMillis) > 0 {
		if bytes.Equal(bytes.TrimSpace(cfg.TimeoutMillis), []byte("null")) {
			return Options{}, fmt.Errorf("TLC timeoutMillis must be an integer")
		}
		if err := json.Unmarshal(cfg.TimeoutMillis, &timeoutMillis); err != nil {
			return Options{}, fmt.Errorf("TLC timeoutMillis must be an integer")
		}
	}
	if timeoutMillis < 0 || timeoutMillis > int64((1<<63-1)/time.Millisecond) {
		return Options{}, fmt.Errorf("invalid TLC timeoutMillis")
	}
	options.MaxDuration = time.Duration(timeoutMillis) * time.Millisecond
	return options, validateOptions(options)
}
