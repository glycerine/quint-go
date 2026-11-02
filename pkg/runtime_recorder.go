package quint

import (
	"context"
	"sort"
)

// ExecutionListener receives operator, action-choice, state, and run events.
// Calls and returns must be balanced by the evaluator. Arguments and values are
// immutable runtime objects; listeners that retain expression slices must copy
// them. A nil listener disables recording.
type ExecutionListener interface {
	OnUserOperatorCall(app Expr)
	OnUserOperatorReturn(app Expr, args []RuntimeValue, result RuntimeValue, err error)
	OnAnyOptionCall(app Expr, position int)
	OnAnyOptionReturn(app Expr, position int)
	OnAnyReturn(noptions, choice int)
	OnNextState(oldState, newState RuntimeValue)
	OnRunCall()
	OnRunReturn(result RuntimeValue, err error, trace []RuntimeValue)
}

type executionListenerKey struct{}

// WithExecutionListener enables execution events for evaluations using ctx.
// Listeners belong to one sequential execution, like the evaluator's registers.
func WithExecutionListener(ctx context.Context, listener ExecutionListener) context.Context {
	return context.WithValue(ctx, executionListenerKey{}, listener)
}

// ExecutionListenerFromContext returns the optional listener installed on ctx.
func ExecutionListenerFromContext(ctx context.Context) ExecutionListener {
	listener, _ := ctx.Value(executionListenerKey{}).(ExecutionListener)
	return listener
}

// RecordedTrace stores a captured execution and the RNG state before its run.
type RecordedTrace struct {
	Frame ExecutionFrame
	Seed  Integer
}

type recordedExecutionFrame struct {
	app       Expr
	args      []RuntimeValue
	result    RuntimeValue
	err       error
	subframes []*recordedExecutionFrame
}

type recordedExecutionTrace struct {
	frame *recordedExecutionFrame
	seed  Integer
}

// TraceRecorder captures call trees and retains the best completed runs. Error
// and false runs rank before successful runs; shorter failing runs and longer
// successful runs rank first. Equal-quality runs retain encounter order.
// VerbosityLevel and RNG may be changed between events. Operator and action
// tracking are enabled at verbosity 3 and above; run/state ranking is retained
// at every verbosity. The recorder is owned by one sequential execution.
type TraceRecorder struct {
	VerbosityLevel int
	RNG            *RNG
	currentFrame   *recordedExecutionFrame
	frameStack     []*recordedExecutionFrame
	bestTraces     []recordedExecutionTrace
	tracesToStore  int
	runSeed        Integer
}

// NewTraceRecorder starts with an empty successful bottom frame. The optional
// capacity defaults to one. Nonpositive capacities retain no completed runs.
// Passing nil RNG creates a counter-zero generator owned by the recorder.
func NewTraceRecorder(verbosityLevel int, rng *RNG, capacity ...int) *TraceRecorder {
	if rng == nil {
		rng = NewRNG(Integer{})
	}
	limit := 1
	if len(capacity) > 0 {
		limit = max(0, capacity[0])
	}
	bottom := newRecordedBottomFrame()
	return &TraceRecorder{VerbosityLevel: verbosityLevel, RNG: rng, currentFrame: bottom, frameStack: []*recordedExecutionFrame{bottom}, tracesToStore: limit, runSeed: rng.State()}
}

func newRecordedBottomFrame() *recordedExecutionFrame {
	truth, _ := NewRuntimeValue(Expr{Kind: "bool", Bool: true})
	return &recordedExecutionFrame{app: Expr{Kind: "app", Opcode: "Rec", Args: []Expr{}}, args: []RuntimeValue{}, result: truth, subframes: []*recordedExecutionFrame{}}
}

func newRecordedCallFrame(app Expr) *recordedExecutionFrame {
	return &recordedExecutionFrame{app: copyRuntimeExpression(app), args: []RuntimeValue{}, err: Diagnostic{Code: "QNT501", Message: "empty frame"}, subframes: []*recordedExecutionFrame{}}
}

func snapshotRecordedFrame(frame *recordedExecutionFrame) ExecutionFrame {
	if frame == nil {
		return ExecutionFrame{}
	}
	result := ExecutionFrame{App: copyRuntimeExpression(frame.app), Args: append([]RuntimeValue{}, frame.args...), Result: frame.result, Err: frame.err, Subframes: make([]ExecutionFrame, len(frame.subframes))}
	for i, child := range frame.subframes {
		result.Subframes[i] = snapshotRecordedFrame(child)
	}
	return result
}

// CurrentFrame returns an owned snapshot of the frame established by creation
// or Clear. This matches upstream's REPL frame: OnRunCall replaces the active
// run root without replacing this frame. Use ActiveFrame during simulation.
func (r *TraceRecorder) CurrentFrame() ExecutionFrame {
	if r == nil {
		return ExecutionFrame{}
	}
	return snapshotRecordedFrame(r.currentFrame)
}

// ActiveFrame returns an owned snapshot of the current run's root frame.
func (r *TraceRecorder) ActiveFrame() ExecutionFrame {
	if r == nil || len(r.frameStack) == 0 {
		return ExecutionFrame{}
	}
	return snapshotRecordedFrame(r.frameStack[0])
}

// BestTraces returns owned snapshots sorted from best to worst quality.
func (r *TraceRecorder) BestTraces() []RecordedTrace {
	if r == nil {
		return nil
	}
	traces := make([]RecordedTrace, len(r.bestTraces))
	for i, trace := range r.bestTraces {
		traces[i] = RecordedTrace{Frame: snapshotRecordedFrame(trace.frame), Seed: trace.seed}
	}
	return traces
}

// Clear discards captured trees without advancing or rewinding the RNG.
func (r *TraceRecorder) Clear() {
	if r == nil {
		return
	}
	bottom := newRecordedBottomFrame()
	r.bestTraces = nil
	r.currentFrame = bottom
	r.frameStack = []*recordedExecutionFrame{bottom}
}

func (r *TraceRecorder) OnUserOperatorCall(app Expr) {
	if r == nil || r.VerbosityLevel < 3 {
		return
	}
	frame := newRecordedCallFrame(app)
	if len(r.frameStack) == 0 {
		r.frameStack = []*recordedExecutionFrame{frame}
	} else if len(r.frameStack) == 2 && r.frameStack[1].app.Opcode == "_" {
		// The new state's placeholder already belongs to the root. Its first
		// operator supplies the real label and result in that same frame.
		*r.frameStack[1] = *frame
	} else {
		top := r.frameStack[len(r.frameStack)-1]
		top.subframes = append(top.subframes, frame)
		r.frameStack = append(r.frameStack, frame)
	}
}

func (r *TraceRecorder) OnUserOperatorReturn(_ Expr, args []RuntimeValue, result RuntimeValue, err error) {
	if r == nil || r.VerbosityLevel < 3 || len(r.frameStack) == 0 {
		return
	}
	top := r.frameStack[len(r.frameStack)-1]
	r.frameStack = r.frameStack[:len(r.frameStack)-1]
	top.args, top.result, top.err = append([]RuntimeValue{}, args...), result, err
}

func (r *TraceRecorder) OnAnyOptionCall(app Expr, _ int) {
	if r == nil || r.VerbosityLevel < 3 {
		return
	}
	frame := newRecordedCallFrame(app)
	if len(r.frameStack) > 0 {
		top := r.frameStack[len(r.frameStack)-1]
		top.subframes = append(top.subframes, frame)
	}
	r.frameStack = append(r.frameStack, frame)
}

func (r *TraceRecorder) OnAnyOptionReturn(_ Expr, _ int) {
	if r != nil && r.VerbosityLevel >= 3 && len(r.frameStack) > 0 {
		r.frameStack = r.frameStack[:len(r.frameStack)-1]
	}
}

func (r *TraceRecorder) OnAnyReturn(noptions, choice int) {
	if r == nil || r.VerbosityLevel < 3 || len(r.frameStack) == 0 {
		return
	}
	top := r.frameStack[len(r.frameStack)-1]
	start := len(top.subframes) - noptions
	kept := make([]*recordedExecutionFrame, 0, len(top.subframes))
	for i, frame := range top.subframes {
		if i < start || i == start+choice {
			kept = append(kept, frame)
		}
	}
	if choice >= 0 && len(kept) > 0 {
		option := kept[len(kept)-1]
		kept = append(kept[:len(kept)-1], option.subframes...)
	}
	top.subframes = kept
}

func (r *TraceRecorder) OnNextState(_, _ RuntimeValue) {
	if r == nil || r.VerbosityLevel < 3 || len(r.frameStack) == 0 {
		return
	}
	frame := newRecordedCallFrame(Expr{Kind: "app", Opcode: "_", Args: []Expr{}})
	bottom := r.frameStack[0]
	r.frameStack = []*recordedExecutionFrame{bottom, frame}
	bottom.subframes = append(bottom.subframes, frame)
}

func (r *TraceRecorder) OnRunCall() {
	if r == nil {
		return
	}
	r.frameStack = []*recordedExecutionFrame{newRecordedBottomFrame()}
	if r.RNG != nil {
		r.runSeed = r.RNG.State()
	}
}

func (r *TraceRecorder) OnRunReturn(result RuntimeValue, err error, trace []RuntimeValue) {
	if r == nil || len(r.frameStack) == 0 {
		return
	}
	root := r.frameStack[0]
	root.args, root.result, root.err = append([]RuntimeValue{}, trace...), result, err
	candidate := recordedExecutionTrace{frame: root, seed: r.runSeed}
	index := sort.Search(len(r.bestTraces), func(i int) bool { return compareRecordedTraces(r.bestTraces[i], candidate) > 0 })
	r.bestTraces = append(r.bestTraces, recordedExecutionTrace{})
	copy(r.bestTraces[index+1:], r.bestTraces[index:])
	r.bestTraces[index] = candidate
	if len(r.bestTraces) > r.tracesToStore {
		r.bestTraces = r.bestTraces[:len(r.bestTraces)-1]
	}
}

func compareRecordedTraces(a, b recordedExecutionTrace) int {
	failed := func(frame *recordedExecutionFrame) bool {
		return frame.err != nil || (frame.result.expression.Kind == "bool" && !frame.result.expression.Bool)
	}
	aFailed, bFailed := failed(a.frame), failed(b.frame)
	if aFailed {
		if bFailed {
			return len(a.frame.args) - len(b.frame.args)
		}
		return -1
	}
	if bFailed {
		return 1
	}
	return len(b.frame.args) - len(a.frame.args)
}
