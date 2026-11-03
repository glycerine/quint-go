package cli

import (
	"encoding/json"
	"reflect"
	"testing"

	quint "github.com/glycerine/quint-go/pkg"
)

func runSelectorFrontend(t *testing.T, source string) quint.FrontendResult {
	t.Helper()
	modules, sources, err := quint.ParseModulesWithSource("/selectors/M.qnt", source)
	if err != nil {
		t.Fatal(err)
	}
	resolution, err := quint.ResolveLinkedModules(modules)
	if err != nil || len(resolution.Errors) > 0 {
		t.Fatal(err, resolution.Errors)
	}
	return quint.FrontendResult{Modules: modules, Sources: sources, Table: resolution.Table, Scopes: resolution.Modules, IDs: quint.NewIDGenerator()}
}

func runSelectorSnapshot(t *testing.T, front quint.FrontendResult) []byte {
	t.Helper()
	entries := []any{front.Modules, front.Sources}
	for id, definition := range front.Table.All() {
		entries = append(entries, []any{id, definition})
	}
	for name, scope := range front.Scopes.All() {
		entries = append(entries, []any{name, scope.Module, scope.Errors})
		for key, definitions := range scope.Definitions.All() {
			entries = append(entries, []any{key, definitions})
		}
	}
	data, err := json.Marshal(entries)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestRunSelectorValidationPreservesFrontend(t *testing.T) {
	t.Parallel()
	front := runSelectorFrontend(t, `module A { action init=true action step=true } module M { import A as I pure val local=true }`)
	before := runSelectorSnapshot(t, front)
	for range 2 {
		problems, err := runValidateSelectors(front, "M", "I::init", "I::step", "List(1).map(value => value).size() > 0", []string{"{ val scoped = true; scoped }"})
		if err != nil || len(problems) != 0 {
			t.Fatal(err, problems)
		}
		problems, err = runValidateSelectors(front, "M", "{ val scoped = true; scoped }", "I::step", "true", nil)
		if err != nil || len(problems) != 0 {
			t.Fatal(err, problems)
		}
		problems, err = runValidateSelectors(front, "M", "missing", "I::step", "true", []string{"{ val scoped = true; scoped }"})
		if err != nil || len(problems) != 4 {
			t.Fatal(err, problems)
		}
		for _, problem := range problems {
			if problem.Explanation != "[QNT404] Name 'missing' not found" || len(problem.Locations) != 0 {
				t.Fatal(problem)
			}
		}
	}
	if after := runSelectorSnapshot(t, front); string(before) != string(after) {
		t.Fatal("selector validation changed source modules, scopes, table, or sources")
	}
	if got := front.IDs.Next().String(); got != "1" {
		t.Fatal("selector validation consumed frontend IDs", got)
	}
}

func TestRunSelectorValidationErrorKindsAndOrdering(t *testing.T) {
	t.Parallel()
	front := runSelectorFrontend(t, `module M { var state:bool pure val init=true action step=true }`)
	problems, err := runValidateSelectors(front, "M", "init", "state", "missing", []string{"val declaration=1", "true"})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"[QNT501] default --init action name 'init' is used by a pureval declaration",
		"[QNT501] default --step action name 'state' is used by a var declaration",
		"[QNT404] Name 'missing' not found",
		"[QNT501] Expected val declaration=1 to be a valid expression",
		"[QNT404] Name 'missing' not found",
	}
	got := make([]string, len(problems))
	for i, problem := range problems {
		got[i] = problem.Explanation
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatal(got, want)
	}
	if _, err = runValidateSelectors(quint.FrontendResult{}, "M", "init", "step", "true", nil); err == nil {
		t.Fatal("accepted missing linked scope data")
	}
}
