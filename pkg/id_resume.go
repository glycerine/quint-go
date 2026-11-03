package quint

// NewIDGeneratorAfter continues after the last allocated ID. The first call to
// Next returns last+1, preserving arbitrarily large IDs. The generator owns its
// counter and, like NewIDGenerator, is for one sequential compilation/session.
func NewIDGeneratorAfter(last ID) *IDGenerator {
	generator := NewIDGenerator()
	generator.next.Add(&generator.next, last.value.Big())
	return generator
}
