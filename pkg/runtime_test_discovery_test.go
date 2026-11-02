package quint

import (
	"encoding/json"
	"os"
	"regexp"
	"slices"
	"testing"
)

func TestUpstreamTestDiscovery(t *testing.T) {
	t.Parallel()
	var fixtures []struct {
		Name     string
		Source   string
		Main     string
		Match    *string
		Selected []struct {
			ID         ID
			Name       string
			Namespaces []string
			Hidden     bool
			Qualifier  Qualifier
		}
		NoTestsMessage string
	}
	data, err := os.ReadFile("testdata/parser/runtime_test_discovery.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	resolutions := map[string]LinkedNameResolution{}
	for _, fixture := range fixtures {
		t.Run(fixture.Name+"/"+func() string {
			if fixture.Match == nil {
				return "default"
			}
			return *fixture.Match
		}(), func(t *testing.T) {
			resolution, ok := resolutions[fixture.Source]
			if !ok {
				modules, err := ParseModules(fixture.Source)
				if err != nil {
					t.Fatal(err)
				}
				resolution, err = ResolveLinkedModules(modules)
				if err != nil || len(resolution.Errors) > 0 {
					t.Fatal(err, resolution.Errors)
				}
				resolutions[fixture.Source] = resolution
			}
			var match func(string) bool
			pattern := ""
			if fixture.Match != nil {
				pattern = *fixture.Match
			}
			if pattern != "" {
				// Corpus patterns use the shared JS/RE2 subset. This helper accepts a
				// predicate, so JS-only regex syntax remains CLI-adapter work.
				compiled, err := regexp.Compile(pattern)
				if err != nil {
					t.Fatal(err)
				}
				match = compiled.MatchString
			}
			got, err := DiscoverTests(resolution, fixture.Main, match)
			if err != nil {
				t.Fatal(err)
			}
			if len(got) != len(fixture.Selected) {
				t.Fatal("selected count", len(got), len(fixture.Selected))
			}
			for i, want := range fixture.Selected {
				if got[i].ID != want.ID || got[i].Name != want.Name || !slices.Equal(got[i].Namespaces, want.Namespaces) || got[i].Hidden != want.Hidden || got[i].Declaration.Qualifier != want.Qualifier {
					t.Fatalf("selection %d: got %+v want %+v", i, got[i], want)
				}
			}
			if NoTestsMatchedMessage(pattern) != fixture.NoTestsMessage {
				t.Fatal(NoTestsMatchedMessage(pattern), fixture.NoTestsMessage)
			}
		})
	}
}

func TestTestDiscoveryValidationAndOwnedSelection(t *testing.T) {
	if _, err := DiscoverTests(LinkedNameResolution{}, "M", nil); err == nil {
		t.Fatal("missing scopes accepted")
	}
	modules, err := ParseModules("module A { const C:int val firstTest=true val helper=1 } module M { import A as I val localTest=true val after=0 }")
	if err != nil {
		t.Fatal(err)
	}
	resolution, err := ResolveLinkedModules(modules)
	if err != nil || len(resolution.Errors) > 0 {
		t.Fatal(err, resolution.Errors)
	}
	if _, err = DiscoverTests(resolution, "Missing", nil); err == nil {
		t.Fatal("missing module accepted")
	}
	called := []string{}
	got, err := DiscoverTests(resolution, "M", func(name string) bool { called = append(called, name); return true })
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(called, []string{"firstTest", "helper", "localTest", "after"}) {
		t.Fatal("predicate calls", called)
	}
	if len(got) != 4 || !got[0].Hidden || len(got[0].Namespaces) != 1 {
		t.Fatal(got)
	}
	got[0].Name = "changed"
	got[0].Namespaces[0] = "changed"
	again, err := DiscoverTests(resolution, "M", nil)
	if err != nil || len(again) != 2 || again[0].Name != "firstTest" || again[0].Namespaces[0] != "I" {
		t.Fatal("selection aliases scope", again, err)
	}
	none, err := DiscoverTests(resolution, "M", func(string) bool { return false })
	if err != nil || none == nil || len(none) != 0 {
		t.Fatal("empty selection", none, err)
	}
}
