package quint

import "fmt"

// An effect-variable cycle containing an arrow grows the tree on every turn.
// Alias-only cycles are handled by the substitution pass history instead.
func checkEffectArrowCycles(subs []EffectSubstitution, input Effect) error {
	active := map[string]int{}
	var visit func(Effect, int) error
	visit = func(e Effect, depth int) error {
		if e.Kind == "variable" {
			if previous, ok := active[e.Name]; ok {
				if depth > previous {
					return fmt.Errorf("cyclic effect substitution for %s", e.Name)
				}
				return nil
			}
			for _, s := range subs {
				if s.Name == e.Name {
					if s.Kind == "effect" {
						active[e.Name] = depth
						err := visit(s.Effect, depth)
						delete(active, e.Name)
						return err
					}
					break
				}
			}
		}
		for _, p := range e.Params {
			if err := visit(p, depth+1); err != nil {
				return err
			}
		}
		if e.Result != nil {
			return visit(*e.Result, depth+1)
		}
		return nil
	}
	return visit(input, 0)
}

// Read and temporal unions have finitely many members after deduplication.
// Updates keep multiplicities: a reachable cycle expands forever if it emits
// more than one cyclic member, or emits any additional non-vanishing member.
// A cycle may still stabilize when its other branches eventually become empty.
func checkEffectSubstitutionGrowth(subs []EffectSubstitution, input Effect) error {
	var entity func(Entity) error
	entity = func(input Entity) error {
		type node struct {
			name       string
			edges      []int
			productive bool
		}
		nodes := []node{}
		indices := map[string]int{}
		var add func(string) int
		add = func(name string) int {
			if i, ok := indices[name]; ok {
				return i
			}
			i := len(nodes)
			indices[name] = i
			nodes = append(nodes, node{name: name})
			var value *Entity
			for _, s := range subs {
				if s.Name == name {
					if s.Kind == "entity" {
						v := s.Entity
						value = &v
					}
					break
				}
			}
			if value == nil {
				nodes[i].productive = true
				return i
			}
			var visit func(Entity)
			visit = func(e Entity) {
				switch e.Kind {
				case "variable":
					edge := add(e.Name)
					nodes[i].edges = append(nodes[i].edges, edge)
				case "concrete":
					nodes[i].productive = nodes[i].productive || len(e.StateVariables) > 0
				case "union":
					for _, child := range e.Entities {
						visit(child)
					}
				}
			}
			visit(*value)
			return i
		}
		for _, name := range EntityNames(input) {
			add(name)
		}
		// Strongly connected components distinguish branching cycles from
		// vanishing branches without imposing an arbitrary iteration limit.
		next := 0
		order, low := make([]int, len(nodes)), make([]int, len(nodes))
		stack, onStack := []int{}, make([]bool, len(nodes))
		components, component := [][]int{}, make([]int, len(nodes))
		var connect func(int)
		connect = func(i int) {
			next++
			order[i], low[i] = next, next
			stack, onStack[i] = append(stack, i), true
			for _, j := range nodes[i].edges {
				if order[j] == 0 {
					connect(j)
					low[i] = min(low[i], low[j])
				} else if onStack[j] {
					low[i] = min(low[i], order[j])
				}
			}
			if low[i] == order[i] {
				members := []int{}
				for {
					j := stack[len(stack)-1]
					stack, onStack[j] = stack[:len(stack)-1], false
					component[j] = len(components)
					members = append(members, j)
					if j == i {
						break
					}
				}
				components = append(components, members)
			}
		}
		for i := range nodes {
			if order[i] == 0 {
				connect(i)
			}
		}
		var survives func(int, map[int]bool) bool
		survives = func(i int, active map[int]bool) bool {
			if active[i] || nodes[i].productive {
				return true
			}
			active[i] = true
			defer delete(active, i)
			for _, j := range nodes[i].edges {
				if survives(j, active) {
					return true
				}
			}
			return false
		}
		for c, members := range components {
			for _, i := range members {
				internal, outgoing := 0, false
				for _, j := range nodes[i].edges {
					if component[j] == c {
						internal++
					} else if survives(j, map[int]bool{}) {
						outgoing = true
					}
				}
				if internal > 1 || internal > 0 && (outgoing || nodes[i].productive) {
					return fmt.Errorf("cyclic entity substitution for %s", nodes[i].name)
				}
			}
		}
		return nil
	}
	var visit func(Effect) error
	visit = func(e Effect) error {
		for _, c := range e.Components {
			if c.Kind == "update" {
				if err := entity(c.Entity); err != nil {
					return err
				}
			}
		}
		for _, p := range e.Params {
			if err := visit(p); err != nil {
				return err
			}
		}
		if e.Result != nil {
			return visit(*e.Result)
		}
		return nil
	}
	return visit(input)
}
