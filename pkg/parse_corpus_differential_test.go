package quint

import (
	"bytes"
	"encoding/json"
	"os"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"testing"
)

func TestUpstreamParserCorpus(t *testing.T) {
	if os.Getenv("QUINT_CORPUS_TESTS") == "" {
		t.Skip("set QUINT_CORPUS_TESTS=1 for full upstream parser corpus")
	}
	data, err := os.ReadFile("testdata/parser/corpus.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixtures []struct {
		Name, Source string
		Modules      json.RawMessage
		Errors       []Diagnostic
	}
	if err = json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	decode := func(data []byte) any {
		var v any
		d := json.NewDecoder(bytes.NewReader(data))
		d.UseNumber()
		if err := d.Decode(&v); err != nil {
			t.Fatal(err)
		}
		normalizeGeneratedParserNames(v)
		return withoutParserIDs(v)
	}
	for _, f := range fixtures {
		t.Run(f.Name, func(t *testing.T) {
			modules, err := ParseModules(f.Source)
			if len(f.Errors) > 0 {
				if err == nil {
					t.Fatalf("accepted upstream-invalid source: %+v", f.Errors)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			actual, err := json.Marshal(modules)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(decode(actual), decode(f.Modules)) {
				t.Fatal("IR differs from upstream after documented ID/name normalization")
			}
		})
	}
}

// Normalize only ID-suffixed compiler temporaries in name fields, preserving
// binding/reference equality. Sorted object keys make traversal deterministic.
func normalizeGeneratedParserNames(root any) {
	pattern := regexp.MustCompile(`^(quintDestructTemp|quintTupledLambdaParam)[0-9]+$`)
	names := map[string]string{}
	var visit func(any)
	visit = func(v any) {
		switch x := v.(type) {
		case map[string]any:
			keys := make([]string, 0, len(x))
			for k := range x {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				if name, ok := x[k].(string); k == "name" && ok && pattern.MatchString(name) {
					replacement, ok := names[name]
					if !ok {
						replacement = pattern.FindStringSubmatch(name)[1] + "#" + strconv.Itoa(len(names))
						names[name] = replacement
					}
					x[k] = replacement
				} else {
					visit(x[k])
				}
			}
		case []any:
			for _, child := range x {
				visit(child)
			}
		}
	}
	visit(root)
}
