package quint

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFileSourceResolverPaths(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "model.qnt")
	if err := os.WriteFile(file, []byte("module M {}\r\n// end\r"), 0600); err != nil {
		t.Fatal(err)
	}
	loaded := map[string]string{}
	r := NewFileSourceResolver(loaded, func(s string) string { return strings.ReplaceAll(s, filepath.ToSlash(root), "fixture") })
	for _, importPath := range []string{"model.qnt", "./model.qnt", "sub/../model.qnt"} {
		p := r.LookupPath(root, importPath)
		if p.NormalizedPath != file || p.SourceName != "fixture/model.qnt" || r.StemPath(p) != root {
			t.Fatalf("path %+v", p)
		}
		text, err := r.Load(p)
		if err != nil || text != "module M {}\n// end\n" {
			t.Fatalf("load %q %v", text, err)
		}
	}
	if len(loaded) != 1 || loaded[file] != "module M {}\n// end\n" {
		t.Fatalf("source storage %v", loaded)
	}
	p := r.LookupPath(filepath.Join(root, "sub"), "../model.qnt")
	if p.NormalizedPath != file {
		t.Fatal("parent resolution")
	}
	if err := os.WriteFile(file, []byte("module Changed {}"), 0600); err != nil {
		t.Fatal(err)
	}
	text, err := r.Load(p)
	if err != nil || text != "module Changed {}" || loaded[file] != text {
		t.Fatal("load must refresh stored source")
	}
}

func TestFileSourceResolverFailures(t *testing.T) {
	root := t.TempDir()
	loaded := map[string]string{}
	r := NewFileSourceResolver(loaded, nil)
	p := r.LookupPath(root, "missing.qnt")
	_, err := r.Load(p)
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("missing cause %v", err)
	}
	want := "ENOENT: no such file or directory, open '" + p.NormalizedPath + "'"
	if err.Error() != want || len(loaded) != 0 {
		t.Fatalf("failed read: %v %v", err, loaded)
	}
	if _, err = r.Load(r.LookupPath(root, ".")); err == nil {
		t.Fatal("accepted directory")
	}
	var resolver SourceResolver = NewFileSourceResolver(nil, nil)
	if resolver.LookupPath("a", "b/../c").NormalizedPath != filepath.Join("a", "c") {
		t.Fatal("relative path normalization")
	}
}
