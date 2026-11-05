package quint

// mustStates returns the built trace states, failing the test binary on error.
func mustStates(trace *RuntimeTrace) []RuntimeValue {
	states, err := trace.States()
	if err != nil {
		panic(err)
	}
	return states
}
