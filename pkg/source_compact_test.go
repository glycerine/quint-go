package quint

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

type apiSourceCompactFixture struct {
	SourceMaps []struct {
		Name    string
		Entries []struct {
			ID       string
			Location UpstreamLocation
		}
		Output, ByIDOutput string
	}
	IDs []struct {
		Initial string
		Next    []string
	}
}

func readAPISourceCompactFixture(t *testing.T) apiSourceCompactFixture {
	t.Helper()
	data, err := os.ReadFile("testdata/parser/api_source_compact.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture apiSourceCompactFixture
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	return fixture
}

func TestUpstreamCompactSourceMapAPI(t *testing.T) {
	t.Parallel()
	for _, fixture := range readAPISourceCompactFixture(t).SourceMaps {
		t.Run(fixture.Name, func(t *testing.T) {
			t.Parallel()
			entries := make([]UpstreamSourceMapEntry, len(fixture.Entries))
			unordered := UpstreamSourceMap{}
			for i, entry := range fixture.Entries {
				id, err := ParseID(entry.ID)
				if err != nil {
					t.Fatal(err)
				}
				entries[i] = UpstreamSourceMapEntry{ID: id, Location: entry.Location}
				unordered[id] = entry.Location
			}
			for _, tc := range []struct {
				result CompactedSourceMap
				output string
			}{{CompactSourceMap(entries), fixture.Output}, {CompactSourceMapByID(unordered), fixture.ByIDOutput}} {
				got, err := tc.result.MarshalJSON()
				if err != nil || string(got) != tc.output {
					t.Fatalf("compact map=%s, %v; want %s", got, err, tc.output)
				}
				encoded, err := json.Marshal(tc.result)
				if err != nil {
					t.Fatal(err)
				}
				var decoded, expected any
				if err := json.Unmarshal(encoded, &decoded); err != nil {
					t.Fatal(err)
				}
				if err := json.Unmarshal([]byte(tc.output), &expected); err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(decoded, expected) {
					t.Fatalf("standard JSON marshaling changed compact data: %s", encoded)
				}
			}
		})
	}
}

func TestCompactSourceMapOwnership(t *testing.T) {
	t.Parallel()
	first, _ := ParseID("18446744073709551616")
	second, _ := ParseID("1")
	end := UpstreamPosition{Line: 2, Col: 7, Index: 19}
	location := UpstreamLocation{Source: "source.qnt", Start: UpstreamPosition{Line: 1, Col: 3, Index: 10}, End: &end}
	entries := []UpstreamSourceMapEntry{{ID: first, Location: location}, {ID: second, Location: location}}
	left, right := CompactSourceMap(entries), CompactSourceMap(entries)
	end.Index = 1000
	entries[0].Location.Start.Index = 2000
	if got := left.Map.Get(first.String()); got.Start.Index != 10 || got.End.Index != 19 {
		t.Fatal("compaction retained input positions", got)
	}
	left.Map.Get(first.String()).End.Index = 3000
	left.SourceIndex.Set("0", "changed.qnt")
	left.Map.Set("99", CompactedLocation{})
	_, unexpected := right.Map.Get2("99")
	if end.Index != 1000 || left.Map.Get(second.String()).End.Index != 19 || right.Map.Get(first.String()).End.Index != 19 || right.SourceIndex.Get("0") != "source.qnt" || unexpected {
		t.Fatal("compacted positions or dictionaries share mutable ownership")
	}
}

func TestCompactSourceMapEmptyTransport(t *testing.T) {
	t.Parallel()
	for _, result := range []CompactedSourceMap{{}, CompactSourceMap(nil), CompactSourceMapByID(nil)} {
		got, err := json.Marshal(result)
		if err != nil || string(got) != `{"sourceIndex":{},"map":{}}` {
			t.Fatalf("empty compact map=%s, %v", got, err)
		}
	}
}

func TestCompactSourceMapConcurrentOwnership(t *testing.T) {
	t.Parallel()
	id, _ := ParseID("999999999999999999999999999999999999")
	end := UpstreamPosition{Line: 1, Col: 7, Index: 19}
	entries := []UpstreamSourceMapEntry{{ID: id, Location: UpstreamLocation{Source: "shared.qnt", End: &end}}}
	t.Run("independent outputs", func(t *testing.T) {
		for i := 0; i < 8; i++ {
			t.Run(string(rune('a'+i)), func(t *testing.T) {
				t.Parallel()
				result := CompactSourceMap(entries)
				result.Map.Get(id.String()).End.Index = i
				result.SourceIndex.Set("0", "private.qnt")
				if _, err := json.Marshal(result); err != nil {
					t.Fatal(err)
				}
			})
		}
	})
	if end.Index != 19 || entries[0].Location.Source != "shared.qnt" {
		t.Fatal("concurrent compaction changed shared input")
	}
}
