package quint

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

type countingResolver struct {
	SourceResolver
	loads map[string]int
}

func (r *countingResolver) Load(p SourceLookupPath) (string, error) {
	r.loads[p.NormalizedPath]++
	return r.SourceResolver.Load(p)
}
func TestSourceImportTraversal(t *testing.T) {
	root := t.TempDir()
	for name, source := range map[string]string{
		"main.qnt":   `module Main { import A.* from "a" import A.* from "./a" import B(N=1) as C from "b" }`,
		"a.qnt":      `module A { import Shared.* from "shared" } module Extra {}`,
		"b.qnt":      `module B { import Shared.* from "shared" }`,
		"shared.qnt": `module Shared { import Main from "main" }`,
	} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(source), 0600); err != nil {
			t.Fatal(err)
		}
	}
	r := &countingResolver{NewFileSourceResolver(nil, nil), map[string]int{}}
	modules, err := LoadSourceModules(r, r.LookupPath(root, "main.qnt"))
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"Main", "Extra", "A", "B", "Shared"}
	if len(modules) != len(want) {
		t.Fatalf("modules %v", modules)
	}
	seen := map[ID]bool{}
	for i, m := range modules {
		if m.Name != want[i] {
			t.Fatalf("order %d: %s", i, m.Name)
		}
		if seen[m.ID] {
			t.Fatal("duplicate module ID")
		}
		seen[m.ID] = true
		for _, d := range m.Declarations {
			if seen[d.ID] {
				t.Fatal("duplicate declaration ID")
			}
			seen[d.ID] = true
		}
	}
	for path, count := range r.loads {
		if count != 1 {
			t.Fatalf("%s loaded %d times", path, count)
		}
	}
	if len(r.loads) != 4 {
		t.Fatal("missing transitive loads")
	}
}
func TestSourceImportFailures(t *testing.T) {
	for _, tc := range []struct{ source, code string }{
		{`module M { import A from "missing" }`, QNT013},
		{`module M { import A from "a" import A from "A" }`, QNT408},
	} {
		root := t.TempDir()
		for name, source := range map[string]string{"main.qnt": tc.source, "a.qnt": "module A {}"} {
			if err := os.WriteFile(filepath.Join(root, name), []byte(source), 0600); err != nil {
				t.Fatal(err)
			}
		}
		r := NewFileSourceResolver(nil, nil)
		_, err := LoadSourceModules(r, r.LookupPath(root, "main.qnt"))
		var d Diagnostic
		if !errors.As(err, &d) || d.Code != tc.code || d.Reference == nil {
			t.Fatalf("diagnostic %v", err)
		}
	}
}
