package quint

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path"
	"reflect"
	"testing"
)

type fixtureSourceResolver struct {
	sources map[string]string
	reads   []string
}

func (r *fixtureSourceResolver) LookupPath(stem, file string) SourceLookupPath {
	p := path.Join(stem, file)
	return SourceLookupPath{NormalizedPath: p, SourceName: p}
}
func (r *fixtureSourceResolver) StemPath(p SourceLookupPath) string {
	return path.Dir(p.NormalizedPath)
}
func (r *fixtureSourceResolver) Load(p SourceLookupPath) (string, error) {
	r.reads = append(r.reads, p.NormalizedPath)
	s, ok := r.sources[p.NormalizedPath]
	if !ok {
		return "", fmt.Errorf("missing %s", p.NormalizedPath)
	}
	return s, nil
}

func TestUpstreamSourceResolutionFixtures(t *testing.T) {
	data, err := os.ReadFile("testdata/parser/imports.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixtures []struct {
		Name           string
		Sources        map[string]string
		Reads, Modules []string
		Errors         []Diagnostic
	}
	if err = json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	for _, f := range fixtures {
		t.Run(f.Name, func(t *testing.T) {
			r := &fixtureSourceResolver{sources: f.Sources}
			ms, err := LoadAndSortModules(r, r.LookupPath(".", "main.qnt"))
			if len(f.Errors) > 0 {
				var d Diagnostic
				if !errors.As(err, &d) || d.Code != f.Errors[0].Code || d.Message != f.Errors[0].Message {
					t.Fatalf("Go %v; upstream %+v", err, f.Errors)
				}
			} else {
				if err != nil {
					t.Fatal(err)
				}
				names := []string{}
				for _, m := range ms {
					names = append(names, m.Name)
				}
				if !reflect.DeepEqual(names, f.Modules) {
					t.Fatalf("Go %v; upstream %v", names, f.Modules)
				}
			}
			// Upstream phase two receives the parsed main file, whereas this Go entry
			// point also loads main.qnt. Compare the subsequent import reads exactly.
			if !reflect.DeepEqual(r.reads[1:], f.Reads) {
				t.Fatalf("Go reads %v; upstream %v", r.reads[1:], f.Reads)
			}
		})
	}
}
