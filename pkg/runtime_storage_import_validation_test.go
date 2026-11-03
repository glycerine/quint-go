package quint

import (
	"bytes"
	"encoding/json"
	"errors"
	"reflect"
	"strconv"
	"testing"
)

// These are explicit Go API policies over the eight malformed direct-import
// reference cases. They are not claims that a Go diagnostic equals a thrown
// TypeScript exception. In particular, the outer shape check intentionally
// preserves state where TypeScript resets it before throwing.
func TestStorageImportValidationPolicy(t *testing.T) {
	t.Parallel()
	policies := []struct {
		name, message, referenceMessage string
	}{
		{"from_outer_nonrecord_resets_before_throw", "Expected a record value", "Expected a record value but got 42"},
		{"from_action_nonstring_preserves_earlier_sorted_variable", "Expected a string action name", "Expected a string value"},
		{"from_pick_container_nonrecord_preserves_action", "Expected a record of nondeterministic picks", "Expected a record value but got 42"},
		{"from_pick_nonvariant_preserves_prior_sorted_pick", "Expected a variant pick value", "Expected a variant value"},
	}
	covered := 0
	for _, fixture := range loadMBTMetadataFixtures(t).DirectCases {
		if fixture.Classification != "direct API invalid metadata value" {
			continue
		}
		covered++
		message, referenceMessage := "", ""
		for _, policy := range policies {
			if fixture.Name == policy.name {
				message, referenceMessage = policy.message, policy.referenceMessage
				break
			}
		}
		if message == "" {
			t.Fatal("unclassified invalid import", fixture.Name)
		}
		t.Run(fixture.Name+"/metadata_"+strconv.FormatBool(fixture.Metadata), func(t *testing.T) {
			t.Parallel()
			outerShape := fixture.Input.Kind != "app" || fixture.Input.Opcode != "Rec"
			wantError := outerShape || fixture.Metadata
			// Retain the actual reference exception and effect-state evidence.
			// Disabled metadata ignores the three malformed record fields.
			if wantError {
				if fixture.Thrown == nil || fixture.Thrown.Name != "Error" || fixture.Thrown.Message != referenceMessage {
					t.Fatalf("reference evidence got %+v, want Error: %s", fixture.Thrown, referenceMessage)
				}
			} else if fixture.Thrown != nil {
				t.Fatal("disabled metadata reference unexpectedly threw", fixture.Thrown)
			}
			if outerShape {
				for _, register := range append(append([]mbtMetadataNamedValue{}, fixture.After.Current...), fixture.After.Next...) {
					if register.Value != nil || register.Error == nil || register.Error.Code != "QNT502" || register.Error.Message != "Variable "+register.Name+" not set" {
						t.Fatal("reference outer-shape rejection did not reset first", register)
					}
				}
				if !reflect.DeepEqual(fixture.After.Caches, fixture.Before.Caches) {
					t.Fatal("reference outer-shape failure unexpectedly cleared caches")
				}
				if fixture.Metadata {
					if fixture.After.Action != "" || len(fixture.After.Picks) != len(fixture.Before.Picks) {
						t.Fatal("reference reset did not clear enabled metadata")
					}
					for i, pick := range fixture.After.Picks {
						if pick.Name != fixture.Before.Picks[i].Name || pick.Value != nil {
							t.Fatal("reference reset changed registered names or retained a pick", pick)
						}
					}
				} else if fixture.After.Action != fixture.Before.Action || !reflect.DeepEqual(fixture.After.Picks, fixture.Before.Picks) {
					t.Fatal("reference reset changed disabled metadata")
				}
			}

			storage := NewVariableStorageWithMetadata(fixture.Metadata)
			makeValue := func(expression *Expr) RuntimeValue {
				t.Helper()
				if expression == nil {
					return RuntimeValue{}
				}
				value, err := NewRuntimeValue(*expression)
				if err != nil {
					t.Fatal(err)
				}
				return value
			}
			for _, register := range fixture.Before.Current {
				current, next := storage.Register(register.Key, register.Name)
				current.Value = makeValue(register.Value)
				for _, pending := range fixture.Before.Next {
					if pending.Key == register.Key {
						next.Value = makeValue(pending.Value)
					}
				}
			}
			storage.ActionTaken = fixture.Before.Action
			for _, pick := range fixture.Before.Picks {
				storage.NondetPicks.Set(pick.Name, makeValue(pick.Value))
			}
			for _, cache := range fixture.Before.Caches {
				storage.TrackCache(&RuntimeCache{Value: makeValue(cache.Value)})
			}
			assertMBTMetadataState(t, storage, fixture.Before)
			input := makeValue(&fixture.Input)
			inputBefore := input.Expression()
			sourceBefore := copyRuntimeExpression(input.source)
			fixtureBefore, err := json.Marshal(fixture.Input)
			if err != nil {
				t.Fatal(err)
			}
			generation := storage.generation
			err = storage.FromRecord(input)
			if wantError {
				var diagnostic Diagnostic
				if !errors.As(err, &diagnostic) || !reflect.DeepEqual(diagnostic, Diagnostic{Code: "QNT501", Message: message}) {
					t.Fatalf("owned Go API diagnostic got %v, want QNT501: %s", err, message)
				}
				var exception RuntimeDomainException
				if errors.As(err, &exception) {
					t.Fatal("Go API leaked a reference exception", err)
				}
			} else if err != nil {
				t.Fatal("disabled metadata fields were not ignored", err)
			}

			want := fixture.After
			if outerShape {
				want = fixture.Before // Go rejects an invalid outer shape before Reset.
			}
			assertMBTMetadataState(t, storage, want)
			wantCaches := fixture.After.Caches
			if wantError {
				wantCaches = fixture.Before.Caches
			}
			if len(storage.caches) != len(wantCaches) {
				t.Fatal("tracked cache count changed")
			}
			for i, expected := range wantCaches {
				assertMBTMetadataValue(t, storage.caches[i].Value, expected.Value)
				assertMBTMetadataDiagnostic(t, storage.caches[i].Err, expected.Error)
			}
			wantGeneration := generation
			if !wantError {
				wantGeneration++
			}
			if storage.generation != wantGeneration {
				t.Fatalf("cache generation got %d, want %d", storage.generation, wantGeneration)
			}
			fixtureAfter, err := json.Marshal(fixture.Input)
			if err != nil || !bytes.Equal(fixtureBefore, fixtureAfter) || !reflect.DeepEqual(input.Expression(), inputBefore) || !reflect.DeepEqual(input.source, sourceBefore) {
				t.Fatal("record import changed its immutable input", err)
			}
		})
	}
	if covered != 8 {
		t.Fatalf("Go validation policy covered %d malformed direct imports, want 8", covered)
	}
}
