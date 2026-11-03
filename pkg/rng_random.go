package quint

import (
	"crypto/rand"
	"io"
	"math/big"
)

// NewRandomRNG initializes an independent generator with system entropy. Its
// initial state is in [0, 2^53-1), the range used by TypeScript's default RNG.
// Explicitly seeded callers should use NewRNG; its zero value stays deterministic.
// The entropy source differs from JavaScript's Math.random, so default runs are
// reproduced using the reported seed rather than a shared system RNG sequence.
func NewRandomRNG() (*RNG, error) {
	return newRandomRNG(rand.Reader)
}

func newRandomRNG(entropy io.Reader) (*RNG, error) {
	seed, err := rand.Int(entropy, big.NewInt(1<<53-1))
	if err != nil {
		return nil, err
	}
	return NewRNG(IntegerFromBig(seed)), nil
}
