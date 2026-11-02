package quint

import (
	"fmt"
	"math/big"
)

// RNG reproduces the TypeScript runtime's counter-based Squares generator.
// Each instance belongs to one evaluation; it is not safe for concurrent use.
// Its zero value starts at counter zero. Explicit seeds are retained exactly,
// whereas SetState applies upstream's absolute-value/modulo normalization.
type RNG struct{ state Integer }

func NewRNG(initial Integer) *RNG { return &RNG{state: initial} }
func (r *RNG) State() Integer     { return r.state }
func (r *RNG) SetState(state Integer) {
	n := state.Big()
	n.Abs(n)
	n.Mod(n, new(big.Int).Lsh(big.NewInt(1), 64))
	r.state = IntegerFromBig(n)
}

// Next preserves upstream's chunk-based bounded sampling, including its modulo
// bias. Invalid bounds return an error without advancing the counter.
func (r *RNG) Next(bound Integer) (Integer, error) {
	input := bound.Big()
	if input.Sign() <= 0 {
		return Integer{}, fmt.Errorf("RNG bound must be positive: %s", bound.String())
	}
	u32 := new(big.Int).Lsh(big.NewInt(1), 32)
	u64 := new(big.Int).Lsh(big.NewInt(1), 64)
	state := r.state.Big()
	draw := func() *big.Int {
		value := squaresCounter(state)
		state.Add(state, big.NewInt(1))
		state.Rem(state, u64)
		return value
	}
	output, base := new(big.Int), big.NewInt(1)
	for input.Cmp(u32) >= 0 {
		output.Mul(output, u32)
		output.Add(output, draw())
		input.Quo(input, u32)
		base.Mul(base, u32)
	}
	high := draw()
	high.Rem(high, input)
	high.Mul(high, base)
	output.Add(high, output)
	r.state = IntegerFromBig(state)
	return IntegerFromBig(output), nil
}

func squaresCounter(counter *big.Int) *big.Int {
	// uint64 overflow implements the four modular rounds for ordinary states.
	if counter.Sign() >= 0 {
		const key uint64 = 0xfb9e125878fa6cb3
		x := counter.Uint64() * key
		y, z := x, x+key
		for _, add := range []uint64{y, z, y} {
			x = x*x + add
			x = x>>32 | x<<32
		}
		return new(big.Int).SetUint64((x*x + z) >> 32)
	}
	// Explicit negative initial states are accepted by upstream's constructor.
	// BigInt remainder/division are signed; SetState takes a different path.
	u32 := new(big.Int).Lsh(big.NewInt(1), 32)
	u64 := new(big.Int).Lsh(big.NewInt(1), 64)
	key, _ := new(big.Int).SetString("fb9e125878fa6cb3", 16)
	x := new(big.Int).Mul(counter, key)
	x.Rem(x, u64)
	y := new(big.Int).Set(x)
	z := new(big.Int).Add(y, key)
	z.Rem(z, u64)
	for _, add := range []*big.Int{y, z, y} {
		x.Mul(x, x)
		x.Rem(x, u64)
		x.Add(x, add)
		x.Rem(x, u64)
		low := new(big.Int).Mul(x, u32)
		low.Rem(low, u64)
		x.Quo(x, u32)
		x.Add(x, low)
		x.Rem(x, u64)
	}
	x.Mul(x, x)
	x.Rem(x, u64)
	x.Add(x, z)
	x.Rem(x, u64)
	x.Quo(x, u32)
	return x
}
