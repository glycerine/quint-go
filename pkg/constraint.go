package quint

import "fmt"

// Constraint represents type equalities, conjunctions, and defined-type checks.
// SourceID identifies the expression responsible for a constraint.
type Constraint struct {
	Kind        string
	SourceID    ID
	Types       [2]Type
	Type        Type
	Constraints []Constraint
}

func ConstraintRank(kind string) (int, error) {
	switch kind {
	case "empty":
		return 0, nil
	case "eq":
		return 1, nil
	case "conjunction":
		return 2, nil
	case "isDefined":
		return 3, nil
	}
	return 0, fmt.Errorf("unknown constraint kind %q", kind)
}
func ApplyConstraintSubstitution(subs []Substitution, c Constraint) (Constraint, error) {
	out := c
	switch c.Kind {
	case "empty":
		return out, nil
	case "eq":
		for i, t := range c.Types {
			v, err := ApplySubstitution(subs, t)
			if err != nil {
				return Constraint{}, err
			}
			out.Types[i] = v
		}
	case "isDefined":
		v, err := ApplySubstitution(subs, c.Type)
		if err != nil {
			return Constraint{}, err
		}
		out.Type = v
	case "conjunction":
		if c.Constraints != nil {
			out.Constraints = make([]Constraint, len(c.Constraints))
		}
		for i, child := range c.Constraints {
			v, err := ApplyConstraintSubstitution(subs, child)
			if err != nil {
				return Constraint{}, err
			}
			out.Constraints[i] = v
		}
	default:
		return Constraint{}, fmt.Errorf("unknown constraint kind %q", c.Kind)
	}
	return out, nil
}
