package quint

import (
	"encoding/json"
	"hash/fnv"
	"os"
	"reflect"
	"testing"

	"github.com/glycerine/insdict"
)

func TestUpstreamTypeConstraintSolverSuite(t *testing.T) {
	t.Parallel()
	type errorEntry struct {
		ID    string
		Error ErrorTree
	}
	var cases []struct {
		Name, Op string
		Table    []struct {
			ID         ID
			Definition Declaration
		}
		Input         Constraint
		Types         [2]Type
		Rows          [2]Row
		Substitutions []typeCompositionSubstitution
		Error         *ErrorTree
		Errors        []errorEntry
	}
	data, err := os.ReadFile("testdata/parser/type_solver_suite.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	if len(cases) != 28 {
		t.Fatal("missing upstream solver tests")
	}
	for _, f := range cases {
		t.Run(f.Name, func(t *testing.T) {
			t.Parallel()
			table := insdict.NewDictFunc[ID, LookupDefinition](func(id ID) uint64 {
				hash := fnv.New64a()
				_, _ = hash.Write([]byte(id.String()))
				return hash.Sum64()
			})
			for _, entry := range f.Table {
				table.Set(entry.ID, declarationLookup(entry.Definition))
			}
			unifier := TypeUnifier{Table: table}
			var substitutions []Substitution
			var failure *ErrorTree
			switch f.Op {
			case "solveConstraint":
				solution, err := unifier.SolveConstraint(f.Input)
				if err != nil {
					t.Fatal(err)
				}
				errors := []errorEntry{}
				for id, value := range solution.Errors.All() {
					errors = append(errors, errorEntry{id, value})
				}
				// A successful Either has no errors payload. Compare an empty
				// list in that case while preserving actual error order otherwise.
				want := f.Errors
				if want == nil {
					want = []errorEntry{}
				}
				if !reflect.DeepEqual(errors, want) {
					t.Fatal("solver errors", errors, "TypeScript", want)
				}
				substitutions = solution.Substitutions
			case "unify":
				substitutions, failure = unifier.Unify(f.Types[0], f.Types[1])
			case "unifyRows":
				substitutions, failure = unifier.UnifyRows(f.Rows[0], f.Rows[1])
			default:
				t.Fatal("unknown upstream operation", f.Op)
			}
			if !reflect.DeepEqual(failure, f.Error) {
				t.Fatal("Go unification error", failure, "TypeScript", f.Error)
			}
			want := decodeTypeCompositionSubstitutions(t, f.Substitutions)
			if !reflect.DeepEqual(substitutions, want) {
				t.Fatalf("Go substitutions %+v\nTypeScript %+v", substitutions, want)
			}
		})
	}
}
