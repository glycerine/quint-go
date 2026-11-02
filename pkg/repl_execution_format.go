package quint

import "fmt"

// ObjectHashState shares the REPL's display-order history with checked value
// and execution-frame formatting. A session owns its state sequentially.
// Uninitialized sessions have no history and return nil.
func (s *REPLSession) ObjectHashState() *RuntimeObjectHashState {
	if s == nil || s.runtime == nil {
		return nil
	}
	return s.runtime.ObjectHashState()
}

// NewTraceRecorder creates a recorder using this session's advancing RNG.
// Install it with WithExecutionListener for the desired evaluations. Creating
// a recorder does not enable recording or modify the session's random state.
func (s *REPLSession) NewTraceRecorder(verbosity int, capacity ...int) (*TraceRecorder, error) {
	if s == nil || s.runtime == nil {
		return nil, fmt.Errorf("REPL session is not initialized")
	}
	return NewTraceRecorder(verbosity, s.runtime.RNG, capacity...), nil
}
