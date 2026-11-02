package quint

import "math/big"

// FreshNames maintains independent counters per prefix. Its zero value is ready
// to use. Like compilation IDs, it belongs to one compilation goroutine.
type FreshNames struct{ counters map[string]*big.Int }

func (g *FreshNames) Next(prefix string) string {
	if g.counters == nil {
		g.counters = map[string]*big.Int{}
	}
	counter := g.counters[prefix]
	if counter == nil {
		counter = new(big.Int)
		g.counters[prefix] = counter
	}
	name := prefix + counter.String()
	counter.Add(counter, big.NewInt(1))
	return name
}
