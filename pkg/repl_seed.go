package quint

// NewREPLSessionWithSeed starts an independent session at the exact initial
// counter, matching NewRNG's constructor semantics. Unlike SetSeed, it does not
// normalize negative or arbitrarily large seeds. NewREPLSession starts at zero.
func NewREPLSessionWithSeed(seed Integer) *REPLSession {
	session := NewREPLSession()
	session.runtime.RNG.state = seed
	return session
}
