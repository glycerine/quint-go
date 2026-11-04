package quint

// Clone forks the next ID without advancing or sharing the generator's counter.
func (g *IDGenerator) Clone() *IDGenerator {
	copy := &IDGenerator{}
	copy.next.Set(&g.next)
	return copy
}
