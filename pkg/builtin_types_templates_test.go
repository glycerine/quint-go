package quint

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"regexp"
	"sort"
	"testing"
)

func builtinTypeSourceDigest(signatures map[string]string) string {
	names := make([]string, 0, len(signatures))
	for name := range signatures {
		names = append(names, name)
	}
	sort.Strings(names)
	hash := sha256.New()
	for _, name := range names {
		fmt.Fprintf(hash, "%s\x00%s\n", name, signatures[name])
	}
	return hex.EncodeToString(hash.Sum(nil))
}

func TestFixedBuiltinTypeTemplateSources(t *testing.T) {
	t.Parallel()
	if actual := builtinTypeSourceDigest(fixedBuiltinTypes); actual != fixedBuiltinTypeSourceSHA256 {
		t.Fatalf("Go builtin signature source changed; run go generate ./pkg: %s != %s", actual, fixedBuiltinTypeSourceSHA256)
	}
	source, err := os.ReadFile("../quint/src/types/builtinSignatures.ts")
	if err != nil {
		t.Fatal(err)
	}
	upstream := map[string]string{}
	for _, entry := range regexp.MustCompile(`\{ name: '([^']+)', type: '([^']+)' \}`).FindAllStringSubmatch(string(source), -1) {
		if _, exists := upstream[entry[1]]; exists {
			t.Fatalf("duplicate upstream signature %q", entry[1])
		}
		upstream[entry[1]] = entry[2]
	}
	if actual := builtinTypeSourceDigest(upstream); actual != fixedBuiltinTypeSourceSHA256 {
		t.Fatalf("upstream builtin signature source changed; run go generate ./pkg: %s != %s", actual, fixedBuiltinTypeSourceSHA256)
	}
}

type builtinTypeTemplateFixture struct {
	Name      string     `json:"name"`
	Signature string     `json:"signature"`
	Scheme    TypeScheme `json:"scheme"`
}

func readBuiltinTypeTemplateFixtures(t *testing.T) []builtinTypeTemplateFixture {
	return readBuiltinTypeTemplateFixtureFile(t, "builtin_types_fixed.json")
}

func readBuiltinTypeTemplateFixtureFile(t *testing.T, name string) []builtinTypeTemplateFixture {
	t.Helper()
	data, err := os.ReadFile("testdata/parser/" + name)
	if err != nil {
		t.Fatal(err)
	}
	var fixtures []builtinTypeTemplateFixture
	if err := json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	return fixtures
}

func assertBuiltinTypeTemplateNoIDs(t *testing.T, typ Type) {
	t.Helper()
	if typ.ID != nil {
		t.Fatalf("builtin type template retained source ID %s", *typ.ID)
	}
	for _, child := range []*Type{typ.Elem, typ.Arg, typ.Res, typ.Ctor} {
		if child != nil {
			assertBuiltinTypeTemplateNoIDs(t, *child)
		}
	}
	for _, child := range typ.Args {
		assertBuiltinTypeTemplateNoIDs(t, child)
	}
	for row := typ.Fields; row != nil; row = row.Other {
		for _, field := range row.Fields {
			assertBuiltinTypeTemplateNoIDs(t, field.Type)
		}
	}
}

func TestFixedBuiltinTypeTemplatesMatchReference(t *testing.T) {
	t.Parallel()
	fixtures := readBuiltinTypeTemplateFixtures(t)
	if len(fixtures) != len(fixedBuiltinTypes) || len(fixtures) != len(fixedBuiltinTypeCache) {
		t.Fatalf("fixture inventory %d differs from source %d or templates %d", len(fixtures), len(fixedBuiltinTypes), len(fixedBuiltinTypeCache))
	}
	seen := map[string]bool{}
	for _, fixture := range fixtures {
		if seen[fixture.Name] {
			t.Fatalf("duplicate fixture %q", fixture.Name)
		}
		seen[fixture.Name] = true
		if source, exists := fixedBuiltinTypes[fixture.Name]; !exists || source != fixture.Signature {
			t.Fatalf("fixture source for %s differs from Go source", fixture.Name)
		}
		t.Run(fixture.Name, func(t *testing.T) {
			t.Parallel()
			for _, arity := range []int{0, 999} {
				got, err := BuiltinTypeSignature(fixture.Name, arity)
				if err != nil {
					t.Fatal(err)
				}
				// The public signatures now retain upstream provenance IDs. Keep
				// the original no-ID fixture as an independent structural oracle.
				if err := WalkType(&got.Type, func(typ *Type) (bool, error) { typ.ID = nil; return true, nil }); err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(got, fixture.Scheme) {
					t.Fatalf("%s/%d differs from reference:\n got %#v\nwant %#v", fixture.Name, arity, got, fixture.Scheme)
				}
				if err := got.Type.Validate(); err != nil {
					t.Fatal(err)
				}
				assertBuiltinTypeTemplateNoIDs(t, got.Type)
			}
		})
	}
}

func TestFixedBuiltinTypeTemplatesMatchRawReference(t *testing.T) {
	t.Parallel()
	fixtures := readBuiltinTypeTemplateFixtureFile(t, "builtin_types_fixed_ids.json")
	if len(fixtures) != len(fixedBuiltinTypes) {
		t.Fatalf("raw fixture inventory %d differs from source %d", len(fixtures), len(fixedBuiltinTypes))
	}
	seen := map[string]bool{}
	for _, fixture := range fixtures {
		if seen[fixture.Name] || fixedBuiltinTypes[fixture.Name] != fixture.Signature {
			t.Fatalf("raw fixture inventory/source drift for %s", fixture.Name)
		}
		seen[fixture.Name] = true
		t.Run(fixture.Name, func(t *testing.T) {
			t.Parallel()
			for _, arity := range []int{0, 999} {
				got, err := BuiltinTypeSignature(fixture.Name, arity)
				if err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(got, fixture.Scheme) {
					actual, _ := json.Marshal(got)
					expected, _ := json.Marshal(fixture.Scheme)
					t.Fatalf("%s/%d differs from raw reference:\n got %s\nwant %s", fixture.Name, arity, actual, expected)
				}
				if err := WalkType(&got.Type, func(typ *Type) (bool, error) {
					if typ.ID == nil {
						t.Errorf("%s node %s has no parser provenance", fixture.Name, typ.Kind)
					}
					return true, nil
				}); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}

func TestFixedBuiltinTypeTemplatesGoParserParity(t *testing.T) {
	if os.Getenv("QUINT_CORPUS_TESTS") == "" {
		t.Skip("set QUINT_CORPUS_TESTS=1 to compare every builtin with the Go parser")
	}
	t.Parallel()
	for _, fixture := range readBuiltinTypeTemplateFixtures(t) {
		t.Run(fixture.Name, func(t *testing.T) {
			t.Parallel()
			parsed, err := parseBuiltinTypeScheme(fixture.Signature)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(parsed, fixture.Scheme) {
				t.Fatalf("Go parser differs from reference for %s:\n got %#v\nwant %#v", fixture.Name, parsed, fixture.Scheme)
			}
		})
	}
}

func BenchmarkBuiltinTypeColdTemplate(b *testing.B) {
	for _, name := range []string{"iadd", "foldl", "setToMap"} {
		for _, implementation := range []string{"parsed", "lookup"} {
			b.Run(name+"/"+implementation, func(b *testing.B) {
				b.ReportAllocs()
				for b.Loop() {
					var err error
					if implementation == "parsed" {
						var scheme TypeScheme
						scheme, err = parseBuiltinTypeScheme(fixedBuiltinTypes[name])
						if err == nil {
							_, err = cloneBuiltinTypeScheme(scheme)
						}
					} else {
						_, err = BuiltinTypeSignature(name, 2)
					}
					if err != nil {
						b.Fatal(err)
					}
				}
			})
		}
	}
}
