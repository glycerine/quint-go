package quint

import (
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// SourceLookupPath separates filesystem identity from the name used in diagnostics.
type SourceLookupPath struct {
	NormalizedPath string
	SourceName     string
}

// SourceResolver supports file-backed and in-memory import loaders.
type SourceResolver interface {
	LookupPath(stem, importPath string) SourceLookupPath
	Load(SourceLookupPath) (string, error)
	StemPath(SourceLookupPath) string
}

// FileSourceResolver records the latest successfully loaded source text. Like
// upstream, Load rereads the file; import graph traversal handles duplicate loads.
// A resolver and its sourceCode map must not be mutated concurrently.
type FileSourceResolver struct {
	sourceCode map[string]string
	replace    func(string) string
}

func NewFileSourceResolver(sourceCode map[string]string, replace func(string) string) *FileSourceResolver {
	if sourceCode == nil {
		sourceCode = map[string]string{}
	}
	if replace == nil {
		replace = func(s string) string { return s }
	}
	return &FileSourceResolver{sourceCode: sourceCode, replace: replace}
}
func (r *FileSourceResolver) LookupPath(stem, importPath string) SourceLookupPath {
	return SourceLookupPath{NormalizedPath: filepath.Clean(filepath.Join(stem, importPath)), SourceName: r.replace(path.Join(filepath.ToSlash(stem), filepath.ToSlash(importPath)))}
}
func (r *FileSourceResolver) StemPath(p SourceLookupPath) string {
	return filepath.Dir(p.NormalizedPath)
}
func (r *FileSourceResolver) Load(p SourceLookupPath) (string, error) {
	data, err := os.ReadFile(p.NormalizedPath)
	if err != nil {
		return "", &SourceLoadError{Path: p.NormalizedPath, Cause: err}
	}
	text := strings.ReplaceAll(strings.ReplaceAll(decodeSourceUTF8(data), "\r\n", "\n"), "\r", "\n")
	r.sourceCode[p.NormalizedPath] = text
	return text, nil
}

// SourceLoadError retains the native cause for errors.Is while presenting the
// upstream missing-file message used by source-resolution diagnostics.
type SourceLoadError struct {
	Path  string
	Cause error
}

func (e *SourceLoadError) Error() string {
	if errors.Is(e.Cause, os.ErrNotExist) {
		return fmt.Sprintf("ENOENT: no such file or directory, open '%s'", e.Path)
	}
	return e.Cause.Error()
}
func (e *SourceLoadError) Unwrap() error { return e.Cause }
