package quint

import (
	"bytes"
	"encoding/json"
	"sort"
	"strconv"

	"github.com/glycerine/insdict"
)

// UpstreamSourceMapEntry records one ordered source-map insertion. Positions
// retain upstream code-point/inclusive-end coordinates, without UTF-16 conversion.
type UpstreamSourceMapEntry struct {
	ID       ID
	Location UpstreamLocation
}

// CompactedLocation is the source index, start, and optional end triple used by
// Quint's compact source-map transport. A missing end serializes as {}, not null.
type CompactedLocation struct {
	SourceIndex int
	Start       UpstreamPosition
	End         *UpstreamPosition
}

func (location CompactedLocation) MarshalJSON() ([]byte, error) {
	var end any = struct{}{}
	if location.End != nil {
		end = *location.End
	}
	return compactSourceJSON([]any{location.SourceIndex, location.Start, end})
}

// CompactedSourceMap owns its dictionaries and copied positions. Dictionary
// iteration retains insertion order; JSON object keys follow JavaScript's
// integer-index ordering, including the distinction at ID 4294967295.
type CompactedSourceMap struct {
	SourceIndex *insdict.Dict[string, string]            `json:"sourceIndex"`
	Map         *insdict.Dict[string, CompactedLocation] `json:"map"`
}

func (sources CompactedSourceMap) MarshalJSON() ([]byte, error) {
	index, err := compactSourceObjectJSON(sources.SourceIndex)
	if err != nil {
		return nil, err
	}
	locations, err := compactSourceObjectJSON(sources.Map)
	if err != nil {
		return nil, err
	}
	var output bytes.Buffer
	output.WriteString(`{"sourceIndex":`)
	output.Write(index)
	output.WriteString(`,"map":`)
	output.Write(locations)
	output.WriteByte('}')
	return output.Bytes(), nil
}

// CompactSourceMap preserves the supplied JavaScript Map insertion order.
// Repeated IDs replace their location without changing their original slot.
// Source indexes are first-location offsets, so repeated files leave gaps.
// Input locations are borrowed only for this call; returned end positions and
// dictionaries may be changed independently. Use CompactSourceMapByID for an
// unordered UpstreamSourceMap when ascending ID order is the desired convention.
func CompactSourceMap(entries []UpstreamSourceMapEntry) CompactedSourceMap {
	ordered := insdict.NewDict[string, UpstreamLocation]()
	for _, entry := range entries {
		ordered.Set(entry.ID.String(), entry.Location)
	}
	result := CompactedSourceMap{
		SourceIndex: insdict.NewDict[string, string](),
		Map:         insdict.NewDict[string, CompactedLocation](),
	}
	indexes := insdict.NewDict[string, int]()
	position := 0
	for id, location := range ordered.All() {
		index, present := indexes.Get2(location.Source)
		if !present {
			index = position
			indexes.Set(location.Source, index)
			result.SourceIndex.Set(strconv.Itoa(index), location.Source)
		}
		compact := CompactedLocation{SourceIndex: index, Start: location.Start}
		if location.End != nil {
			end := *location.End
			compact.End = &end
		}
		result.Map.Set(id, compact)
		position++
	}
	return result
}

// CompactSourceMapByID compacts an unordered Go map in ascending numeric ID
// order. This deterministic convention matches the Go CLI; it cannot recover
// a JavaScript Map's original insertion order. Coordinates are not converted.
func CompactSourceMapByID(sources UpstreamSourceMap) CompactedSourceMap {
	entries := make([]UpstreamSourceMapEntry, 0, len(sources))
	for id, location := range sources {
		entries = append(entries, UpstreamSourceMapEntry{ID: id, Location: location})
	}
	sort.Slice(entries, func(i, j int) bool {
		a, b := entries[i].ID.String(), entries[j].ID.String()
		return len(a) < len(b) || len(a) == len(b) && a < b
	})
	return CompactSourceMap(entries)
}

func compactSourceJSON(value any) ([]byte, error) {
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		return nil, err
	}
	return bytes.TrimSuffix(buffer.Bytes(), []byte{'\n'}), nil
}

func compactSourceObjectJSON[V any](values *insdict.Dict[string, V]) ([]byte, error) {
	var integerKeys, remaining []string
	if values != nil {
		for key := range values.All() {
			if number, err := strconv.ParseUint(key, 10, 32); err == nil && number < 1<<32-1 && strconv.FormatUint(number, 10) == key {
				integerKeys = append(integerKeys, key)
			} else {
				remaining = append(remaining, key)
			}
		}
	}
	sort.Slice(integerKeys, func(i, j int) bool {
		a, b := integerKeys[i], integerKeys[j]
		return len(a) < len(b) || len(a) == len(b) && a < b
	})
	var output bytes.Buffer
	output.WriteByte('{')
	for i, key := range append(integerKeys, remaining...) {
		if i > 0 {
			output.WriteByte(',')
		}
		encodedKey, err := compactSourceJSON(key)
		if err != nil {
			return nil, err
		}
		encodedValue, err := compactSourceJSON(values.Get(key))
		if err != nil {
			return nil, err
		}
		output.Write(encodedKey)
		output.WriteByte(':')
		output.Write(encodedValue)
	}
	output.WriteByte('}')
	return output.Bytes(), nil
}
