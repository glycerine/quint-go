package quint

// RuntimeTrace retains immutable state values for one sequential execution.
// Returned slices and reset inputs are copied; state values own their IR.
type RuntimeTrace struct{ states []RuntimeValue }

func (t *RuntimeTrace) States() []RuntimeValue { return append([]RuntimeValue(nil), t.states...) }
func (t *RuntimeTrace) Reset(states ...RuntimeValue) {
	t.states = append([]RuntimeValue(nil), states...)
}
func (t *RuntimeTrace) Extend(state RuntimeValue) { t.states = append(t.states, state) }
func (t *RuntimeTrace) DropLast() {
	if len(t.states) > 0 {
		t.states = t.states[:len(t.states)-1]
	}
}

// Shift commits next registers, records the resulting state including metadata,
// then clears metadata for the following action, matching the reference context.
func (t *RuntimeTrace) Shift(storage *VariableStorage) error {
	storage.Shift()
	state, err := storage.Record()
	if err != nil {
		return err
	}
	t.Extend(state)
	storage.ClearMetadata()
	return nil
}
