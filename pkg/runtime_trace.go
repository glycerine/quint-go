package quint

// RuntimeTrace retains one state per shift for one sequential execution.
// Shift keeps a state as its capture until it is read, so a run that is
// discarded never builds its records. A state whose values need a symbolic
// domain is built at once, because building it needs a context.
// Returned slices and reset inputs are copied; state values own their IR.
type RuntimeTrace struct{ states []traceEntry }

// traceEntry holds a built state, or the capture of a state not yet built.
type traceEntry struct {
	state   RuntimeValue
	pending *stateCapture
}

// Len reports the number of states without building pending ones.
func (t *RuntimeTrace) Len() int { return len(t.states) }

// States builds any pending states and returns a copy of the trace. It returns
// an error only if a pending state cannot be built from its captured values.
func (t *RuntimeTrace) States() ([]RuntimeValue, error) {
	var states []RuntimeValue
	for i := range t.states {
		entry := &t.states[i]
		if entry.pending != nil {
			state, err := entry.pending.materialize()
			if err != nil {
				return nil, err
			}
			entry.state, entry.pending = state, nil
		}
		states = append(states, entry.state)
	}
	return states, nil
}

func (t *RuntimeTrace) Reset(states ...RuntimeValue) {
	t.states = nil
	for _, state := range states {
		t.Extend(state)
	}
}
func (t *RuntimeTrace) Extend(state RuntimeValue) {
	t.states = append(t.states, traceEntry{state: state})
}
func (t *RuntimeTrace) DropLast() {
	if len(t.states) > 0 {
		t.states = t.states[:len(t.states)-1]
	}
}

// Shift commits next registers, records the resulting state including metadata,
// then clears metadata for the following action, matching the reference context.
// The record is built now only if it needs a symbolic domain; otherwise it is
// built when the trace is read.
func (t *RuntimeTrace) Shift(storage *VariableStorage) error {
	storage.Shift()
	capture := storage.capture()
	if capture.hasDomain() {
		state, err := capture.materialize()
		if err != nil {
			return err
		}
		t.Extend(state)
	} else {
		t.states = append(t.states, traceEntry{pending: &capture})
	}
	storage.ClearMetadata()
	return nil
}
