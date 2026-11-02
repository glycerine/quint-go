package quint

import (
	"fmt"
	"math/big"
)

// Integer is an immutable arbitrary-precision language integer. The zero value
// represents zero. Big returns a copy so callers cannot mutate shared IR values.
type Integer struct{ decimal string }

func ParseInteger(s string) (Integer, error) {
	if decimalJSON([]byte(s)) {
		if s == "0" || s == "-0" {
			return Integer{}, nil
		}
		return Integer{decimal: s}, nil
	}
	n, ok := new(big.Int).SetString(s, 10)
	if !ok {
		return Integer{}, fmt.Errorf("invalid integer %q", s)
	}
	if n.Sign() == 0 {
		return Integer{}, nil
	}
	return Integer{decimal: n.String()}, nil
}
func IntegerFromBig(n *big.Int) Integer {
	if n.Sign() == 0 {
		return Integer{}
	}
	return Integer{decimal: n.String()}
}
func (n Integer) String() string {
	if n.decimal == "" {
		return "0"
	}
	return n.decimal
}
func (n Integer) Big() *big.Int                { v, _ := new(big.Int).SetString(n.String(), 10); return v }
func (n Integer) MarshalJSON() ([]byte, error) { return []byte(n.String()), nil }
func (n *Integer) UnmarshalJSON(data []byte) error {
	if !decimalJSON(data) {
		return fmt.Errorf("expected integer JSON number, got %q", data)
	}
	v, err := ParseInteger(string(data))
	if err != nil {
		return err
	}
	*n = v
	return nil
}
func decimalJSON(data []byte) bool {
	if len(data) == 0 {
		return false
	}
	if data[0] == '-' {
		data = data[1:]
	}
	if len(data) == 0 || (len(data) > 1 && data[0] == '0') {
		return false
	}
	for _, c := range data {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// ID retains arbitrarily large upstream IDs without machine-word truncation.
// It is comparable and can be used as a lookup-table key.
type ID struct{ value Integer }

func ParseID(s string) (ID, error) {
	n, err := ParseInteger(s)
	if err != nil {
		return ID{}, err
	}
	if n.String()[0] == '-' {
		return ID{}, fmt.Errorf("negative IR ID %s", s)
	}
	if n.String() == "0" {
		return ID{}, nil
	}
	return ID{value: n}, nil
}
func (id ID) String() string               { return id.value.String() }
func (id ID) MarshalJSON() ([]byte, error) { return id.value.MarshalJSON() }
func (id *ID) UnmarshalJSON(data []byte) error {
	var n Integer
	if err := n.UnmarshalJSON(data); err != nil {
		return err
	}
	if n.String()[0] == '-' {
		return fmt.Errorf("negative IR ID %s", n.String())
	}
	*id = ID{value: n}
	return nil
}
func (id ID) MarshalText() ([]byte, error) { return []byte(id.String()), nil }
func (id *ID) UnmarshalText(data []byte) error {
	v, err := ParseID(string(data))
	if err != nil {
		return err
	}
	*id = v
	return nil
}

// IDGenerator is scoped to one compilation and is not shared across goroutines.
type IDGenerator struct{ next big.Int }

func NewIDGenerator() *IDGenerator { g := &IDGenerator{}; g.next.SetInt64(1); return g }
func (g *IDGenerator) Next() ID {
	id, _ := ParseID(g.next.String())
	g.next.Add(&g.next, big.NewInt(1))
	return id
}
