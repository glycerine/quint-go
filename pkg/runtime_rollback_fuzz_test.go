package quint

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
)

// Every generated branch performs tentative writes and then returns false.
// Regardless of branch order or nesting, the enclosing failed action must
// preserve current and pending register values and their compiled cell identity.
func FuzzFailedActionRegisterRollback(f *testing.F) {
	for _, seed := range [][]byte{{}, {0}, {1}, {2, 255}, {3, 128, 0}, {4, 1, 2, 3}, {5, 9, 7}, {6, 255, 254, 253}, {7, 0, 128, 255}, {8, 3, 5, 7, 9}, {31, 32, 33}, {255, 254, 253, 252}} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 64 {
			t.Skip()
		}
		at := func(i int) byte {
			if len(data) == 0 {
				return 0
			}
			return data[i%len(data)]
		}
		integer := func(n int) Expr {
			value, err := ParseInteger(fmt.Sprint(n))
			if err != nil {
				t.Fatal(err)
			}
			return Expr{Kind: "int", Int: value}
		}
		storage := NewVariableStorage()
		count := 1 + int(at(0)%4)
		current, next := make([]*RuntimeRegister, count), make([]*RuntimeRegister, count)
		beforeCurrent, beforeNext := make([]RuntimeValue, count), make([]RuntimeValue, count)
		for i := 0; i < count; i++ {
			current[i], next[i] = storage.Register(fmt.Sprint(i), fmt.Sprintf("x%d", i))
			if at(i+1)&1 != 0 {
				current[i].Value, _ = NewRuntimeValue(integer(int(int8(at(i + 2)))))
			}
			if at(i+2)&1 != 0 {
				next[i].Value, _ = NewRuntimeValue(integer(int(int8(at(i + 3)))))
			}
			beforeCurrent[i], beforeNext[i] = current[i].Value, next[i].Value
		}
		var branch func(int, int) Expr
		branch = func(depth, offset int) Expr {
			variable := int(at(offset)) % count
			write := Expr{Kind: "app", Opcode: "assign", Args: []Expr{{Kind: "name", Name: fmt.Sprintf("x%d", variable)}, integer(int(int8(at(offset + 1))))}}
			failure := Expr{Kind: "bool", Bool: false}
			if depth > 0 {
				failure = branch(depth-1, offset+2)
			}
			all := Expr{Kind: "app", Opcode: "actionAll", Args: []Expr{write, failure}}
			if at(offset+1)&1 != 0 {
				return Expr{Kind: "app", Opcode: "actionAny", Args: []Expr{all, {Kind: "bool", Bool: false}}}
			}
			return all
		}
		expression := branch(int(at(1)%4), 2)
		before, err := json.Marshal(expression)
		if err != nil {
			t.Fatal(err)
		}
		seed, _ := ParseInteger(fmt.Sprint(at(3)))
		result, err := EvaluateStateExpressionWithRNG(context.Background(), expression, storage, NewRNG(seed))
		if err != nil || result.String() != "false" {
			t.Fatalf("generated failed action did not fail normally: %s, %v", result.String(), err)
		}
		for i := 0; i < count; i++ {
			c, n := storage.Register(fmt.Sprint(i), fmt.Sprintf("x%d", i))
			if c != current[i] || n != next[i] {
				t.Fatal("rollback replaced a compiled register cell")
			}
			for _, pair := range [][2]RuntimeValue{{c.Value, beforeCurrent[i]}, {n.Value, beforeNext[i]}} {
				if pair[0].Valid() != pair[1].Valid() || pair[0].Valid() && !pair[0].Equal(pair[1]) {
					t.Fatalf("failed speculative write leaked into register %d", i)
				}
			}
		}
		after, err := json.Marshal(expression)
		if err != nil || string(before) != string(after) {
			t.Fatal("evaluation changed caller-owned action IR", err)
		}
	})
}
