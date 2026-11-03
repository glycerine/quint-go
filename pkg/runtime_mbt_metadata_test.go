package quint

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"strconv"
	"testing"
)

type mbtMetadataNamedValue struct {
	Key, Name string
	Value     *Expr
	Error     *Diagnostic
}

type mbtMetadataState struct {
	Record        Expr
	Action        string
	Picks         []mbtMetadataNamedValue
	Current, Next []mbtMetadataNamedValue
	Caches        []mbtMetadataNamedValue
}

type mbtMetadataFixtures struct {
	SourceCases []struct {
		Name, Source, Classification, Seed string
		Module                             Module
		Metadata                           bool
		AnalysisErrors                     []Diagnostic
		Operations                         []struct {
			mbtMetadataState
			Operation, RNG       string
			Output               *Expr
			Error                *Diagnostic
			Thrown               *symbolicNondetException
			TraceLength          int
			BuilderPicks         []mbtMetadataNamedValue
			BuilderPicksAttached bool
		}
	}
	DirectCases []struct {
		Name, Classification string
		Metadata             bool
		Input                Expr
		Before, After        mbtMetadataState
		Thrown               *symbolicNondetException
	}
}

func loadMBTMetadataFixtures(t *testing.T) mbtMetadataFixtures {
	t.Helper()
	data, err := os.ReadFile("testdata/parser/runtime_mbt_metadata.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixtures mbtMetadataFixtures
	if err = json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	if len(fixtures.SourceCases) != 12 || len(fixtures.DirectCases) != 14 {
		t.Fatal("metadata oracle case count", len(fixtures.SourceCases), len(fixtures.DirectCases))
	}
	return fixtures
}

func assertMBTMetadataValue(t *testing.T, value RuntimeValue, want *Expr) {
	t.Helper()
	if want == nil {
		if value.Valid() {
			t.Fatal("expected unset value, got", value.String())
		}
		return
	}
	got, err := SerializeRuntimeValueContext(context.Background(), value, nil)
	if err != nil || !reflect.DeepEqual(got, *want) {
		t.Fatalf("value got %s, want %s (%v)", ExpressionString(got), ExpressionString(*want), err)
	}
}

func assertMBTMetadataDiagnostic(t *testing.T, err error, want *Diagnostic) {
	t.Helper()
	if want == nil {
		if err != nil {
			t.Fatal(err)
		}
		return
	}
	var diagnostic Diagnostic
	if !errors.As(err, &diagnostic) || diagnostic.Code != want.Code || diagnostic.Message != want.Message || !reflect.DeepEqual(diagnostic.Reference, want.Reference) {
		t.Fatalf("diagnostic got %v, want %+v", err, *want)
	}
}

func assertMBTMetadataState(t *testing.T, storage *VariableStorage, want mbtMetadataState) {
	t.Helper()
	record, err := storage.Record()
	if err != nil {
		t.Fatal(err)
	}
	assertMBTMetadataValue(t, record, &want.Record)
	if storage.ActionTaken != want.Action {
		t.Fatal("action", storage.ActionTaken, "want", want.Action)
	}
	if storage.NondetPicks.Len() != len(want.Picks) {
		t.Fatal("pick count", storage.NondetPicks.Len(), "want", len(want.Picks))
	}
	index := 0
	for name, value := range storage.NondetPicks.All() {
		expected := want.Picks[index]
		if name != expected.Name {
			t.Fatal("pick insertion order", index, name, "want", expected.Name)
		}
		assertMBTMetadataValue(t, value, expected.Value)
		index++
	}
	if storage.current.Len() != len(want.Current) || storage.next.Len() != len(want.Next) {
		t.Fatal("register counts", storage.current.Len(), storage.next.Len(), "want", len(want.Current), len(want.Next))
	}
	index = 0
	for key, register := range storage.current.All() {
		expected := want.Current[index]
		if key != expected.Key || register.Name != expected.Name {
			t.Fatal("current register order", key, register.Name, "want", expected.Key, expected.Name)
		}
		value, err := register.Read()
		assertMBTMetadataDiagnostic(t, err, expected.Error)
		if err == nil {
			assertMBTMetadataValue(t, value, expected.Value)
		}
		index++
	}
	index = 0
	for key, register := range storage.next.All() {
		expected := want.Next[index]
		if key != expected.Key || register.Name != expected.Name {
			t.Fatal("next register order", key, register.Name, "want", expected.Key, expected.Name)
		}
		value, err := register.Read()
		assertMBTMetadataDiagnostic(t, err, expected.Error)
		if err == nil {
			assertMBTMetadataValue(t, value, expected.Value)
		}
		index++
	}
}

func TestUpstreamMBTSourceMetadataGeneration(t *testing.T) {
	t.Parallel()
	for _, fixture := range loadMBTMetadataFixtures(t).SourceCases {
		t.Run(fixture.Name+"/metadata_"+strconv.FormatBool(fixture.Metadata), func(t *testing.T) {
			t.Parallel()
			if fixture.Classification != "typed source module" || fixture.Source == "" || len(fixture.AnalysisErrors) != 0 {
				t.Fatal("source oracle classification", fixture.Classification, fixture.AnalysisErrors)
			}
			seed, err := ParseInteger(fixture.Seed)
			if err != nil {
				t.Fatal(err)
			}
			evaluator, err := NewModuleEvaluator(fixture.Module, NewRNG(seed), fixture.Metadata)
			if err != nil {
				t.Fatal(err)
			}
			ctx := WithRuntimeNondetOptions(context.Background(), RuntimeNondetOptions{RetrySmallerThan: Integer{decimal: "100"}})
			for index, operation := range fixture.Operations {
				t.Run(strconv.Itoa(index)+"_"+operation.Operation, func(t *testing.T) {
					var got RuntimeValue
					var err error
					switch operation.Operation {
					case "clear":
						evaluator.Storage.ClearMetadata()
					case "shift":
						err = evaluator.Shift()
					default:
						got, err = evaluator.Evaluate(ctx, operation.Operation, nil)
					}
					if operation.Thrown != nil {
						t.Fatal("unexpected thrown source fixture", operation.Thrown)
					}
					assertMBTMetadataDiagnostic(t, err, operation.Error)
					if operation.Output != nil {
						assertMBTMetadataValue(t, got, operation.Output)
					}
					assertMBTMetadataState(t, evaluator.Storage, operation.mbtMetadataState)
					if evaluator.RNG.State().String() != operation.RNG || len(evaluator.Trace.States()) != operation.TraceLength {
						t.Fatal("RNG/trace", evaluator.RNG.State(), len(evaluator.Trace.States()), "want", operation.RNG, operation.TraceLength)
					}
				})
			}
		})
	}
}

func TestUpstreamMBTSupportedRecordImports(t *testing.T) {
	t.Parallel()
	supported := 0
	for _, fixture := range loadMBTMetadataFixtures(t).DirectCases {
		// Malformed direct imports remain explicit pending evidence. Their
		// thrown exception and partial reset contract is not asserted here.
		if fixture.Classification != "direct API record import" || fixture.Thrown != nil {
			continue
		}
		supported++
		t.Run(fixture.Name+"/metadata_"+strconv.FormatBool(fixture.Metadata), func(t *testing.T) {
			t.Parallel()
			storage := NewVariableStorageWithMetadata(fixture.Metadata)
			for _, register := range fixture.Before.Current {
				current, next := storage.Register(register.Key, register.Name)
				if register.Value != nil {
					current.Value, _ = NewRuntimeValue(*register.Value)
				}
				for _, pending := range fixture.Before.Next {
					if pending.Key == register.Key && pending.Value != nil {
						next.Value, _ = NewRuntimeValue(*pending.Value)
					}
				}
			}
			storage.ActionTaken = fixture.Before.Action
			for _, pick := range fixture.Before.Picks {
				value := RuntimeValue{}
				if pick.Value != nil {
					value, _ = NewRuntimeValue(*pick.Value)
				}
				storage.NondetPicks.Set(pick.Name, value)
			}
			for _, expected := range fixture.Before.Caches {
				cache := &RuntimeCache{}
				if expected.Value != nil {
					cache.Value, _ = NewRuntimeValue(*expected.Value)
				}
				storage.TrackCache(cache)
			}
			assertMBTMetadataState(t, storage, fixture.Before)
			input, err := NewRuntimeValue(fixture.Input)
			if err != nil {
				t.Fatal(err)
			}
			if err = storage.FromRecord(input); err != nil {
				t.Fatal(err)
			}
			assertMBTMetadataState(t, storage, fixture.After)
			if len(storage.caches) != len(fixture.After.Caches) {
				t.Fatal("cache count")
			}
			for i, cache := range storage.caches {
				assertMBTMetadataValue(t, cache.Value, fixture.After.Caches[i].Value)
				assertMBTMetadataDiagnostic(t, cache.Err, fixture.After.Caches[i].Error)
			}
		})
	}
	if supported != 6 {
		t.Fatal("supported import oracle count", supported, "want 6")
	}
}
