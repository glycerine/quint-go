package quint

import (
	"github.com/glycerine/insdict"
	"sort"
)

// ConstraintSolution contains either substitutions or source-ID-keyed errors.
// Diagnostic keys are decimal IDs and retain insertion order.
type ConstraintSolution struct {
	Substitutions []Substitution
	Errors        *insdict.Dict[string, ErrorTree]
}

// SolveConstraint solves validated constraints. Conjunctions collect subsequent
// errors after a failure, without applying earlier substitutions to later items.
// Structural substitution failures are returned separately from type errors.
func (u TypeUnifier) SolveConstraint(c Constraint) (ConstraintSolution, error) {
	result := ConstraintSolution{Substitutions: []Substitution{}, Errors: insdict.NewDict[string, ErrorTree]()}
	if _, err := ConstraintRank(c.Kind); err != nil {
		return result, err
	}
	failed := false
	record := func(id ID, e *ErrorTree) {
		if e != nil {
			result.Errors.Set(id.String(), *e)
			result.Substitutions = nil
			failed = true
		}
	}
	switch c.Kind {
	case "empty":
	case "eq":
		s, e := u.Unify(c.Types[0], c.Types[1])
		result.Substitutions = s
		record(c.SourceID, e)
	case "isDefined":
		if u.Table != nil {
			for _, d := range u.Table.All() {
				if d.Declaration != nil && d.Declaration.Kind == "typedef" && d.Declaration.Type != nil {
					if s, e := u.Unify(*d.Declaration.Type, c.Type); e == nil {
						result.Substitutions = s
						return result, nil
					}
				}
			}
		}
		e := ErrorLeaf("Looking for defined type unifying with "+TypeString(c.Type), "Expected type is not defined")
		record(c.SourceID, &e)
	case "conjunction":
		children := append([]Constraint{}, c.Constraints...)
		for _, child := range children {
			if _, err := ConstraintRank(child.Kind); err != nil {
				return result, err
			}
		}
		sort.SliceStable(children, func(i, j int) bool {
			a, _ := ConstraintRank(children[i].Kind)
			b, _ := ConstraintRank(children[j].Kind)
			return a < b
		})
		for _, child := range children {
			next := child
			if !failed {
				var err error
				next, err = ApplyConstraintSubstitution(result.Substitutions, child)
				if err != nil {
					return result, err
				}
			}
			solved, err := u.SolveConstraint(next)
			if err != nil {
				return result, err
			}
			childFailed := false
			for id, e := range solved.Errors.All() {
				result.Errors.Set(id, e)
				childFailed = true
			}
			if childFailed {
				failed = true
				result.Substitutions = nil
			}
			if !failed {
				s, e := u.Compose(solved.Substitutions, result.Substitutions)
				result.Substitutions = s
				record(child.SourceID, e)
			}
		}
	}
	return result, nil
}
