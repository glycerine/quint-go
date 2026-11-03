package quint

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestFileSourceResolverPreservesMainLineEnds(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	path := filepath.Join(directory, "source.qnt")
	raw := "module M {\r\n val text=\"😀\"\r}\n"
	if err := os.WriteFile(path, []byte(raw), 0600); err != nil {
		t.Fatal(err)
	}
	code := map[string]string{}
	resolver := NewFileSourceResolver(code, nil)
	lookup := resolver.LookupPath(directory, "source.qnt")
	text, err := resolver.LoadPreservingLineEnds(lookup)
	if err != nil || text != raw || code[path] != raw {
		t.Fatal(text, code, err)
	}
	text, err = resolver.Load(lookup)
	if err != nil || text != "module M {\n val text=\"😀\"\n}\n" || code[path] != text {
		t.Fatal("ordinary imported source normalization changed", text, code, err)
	}
	if err := os.WriteFile(path, []byte{'x', 0xff, 0xc0, 0x80}, 0600); err != nil {
		t.Fatal(err)
	}
	text, err = resolver.LoadPreservingLineEnds(lookup)
	if err != nil || text != "x\ufffd\ufffd\ufffd" || code[path] != text {
		t.Fatal("raw loads lost the Node UTF-8 decoding contract", text, code, err)
	}
	missing := resolver.LookupPath(directory, "missing.qnt")
	_, err = resolver.LoadPreservingLineEnds(missing)
	var loadError *SourceLoadError
	if !errors.Is(err, os.ErrNotExist) || !errors.As(err, &loadError) || loadError.Path != missing.NormalizedPath || code[path] != text {
		t.Fatal("failed raw load changed retained source or lost native error", code, err)
	}
	if _, exists := code[missing.NormalizedPath]; exists {
		t.Fatal("failed source load was recorded")
	}
}
