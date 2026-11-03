package quint

import (
	"context"
	"encoding/json"
	"os"
	"reflect"
	"strconv"
	"testing"
)

type sourceSetAsymmetryObservation struct {
	Maps, Variants, Sets []bool
}

func TestUpstreamSourceSetMembershipEquality(t *testing.T) {
	t.Parallel()
	var fixture struct {
		Before, After sourceSetAsymmetryObservation
		Hashes        []float64
	}
	data, err := os.ReadFile("testdata/parser/runtime_source_set_asymmetry.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	zero := contextIdentityValue(t, "0")
	label := contextIdentityValue(t, `"X"`)
	domain := NewRuntimeInterval(domainFixtureInteger(t, "-10"), domainFixtureInteger(t, "8"))
	explicit, err := domain.Materialize(ctx)
	if err != nil {
		t.Fatal(err)
	}
	intervalKey := contextIdentityComposite(t, "variant", label, contextIdentityDomain(t, domain))
	explicitKey := contextIdentityComposite(t, "variant", label, explicit)
	singleton := func(n int) RuntimeValue {
		return contextIdentityComposite(t, "variant", label, contextIdentityComposite(t, "Set", contextIdentityValue(t, strconv.Itoa(n))))
	}
	prefix := contextIdentityComposite(t, "variant", label, contextIdentityDomain(t, NewRuntimeInterval(domainFixtureInteger(t, "7"), domainFixtureInteger(t, "7"))))
	fillers := make([]RuntimeValue, 7)
	for i := range fillers {
		fillers[i] = singleton(i)
	}
	makeMap := func(keys []RuntimeValue) RuntimeValue {
		pairs := make([]RuntimeValue, len(keys))
		for i, key := range keys {
			pairs[i] = contextIdentityComposite(t, "Tup", key, zero)
		}
		return contextIdentityComposite(t, "Map", pairs...)
	}
	initial := makeMap(append([]RuntimeValue{intervalKey, prefix}, fillers...))
	raw, err := runtimeMap(initial)
	if err != nil {
		t.Fatal(err)
	}
	raw, err = raw.SetContext(ctx, explicitKey, zero)
	if err != nil {
		t.Fatal(err)
	}
	raw, err = raw.DeleteContext(ctx, prefix)
	if err != nil {
		t.Fatal(err)
	}
	duplicate, err := runtimeWithEntriesContext(ctx, "Map", raw)
	if err != nil {
		t.Fatal(err)
	}
	comparison := makeMap(append(append([]RuntimeValue{explicitKey}, fillers...), singleton(7)))
	outerLabel := contextIdentityValue(t, `"M"`)
	variantDuplicate := contextIdentityComposite(t, "variant", outerLabel, duplicate)
	variantComparison := contextIdentityComposite(t, "variant", outerLabel, comparison)
	outerDuplicate := contextIdentityComposite(t, "Set", variantDuplicate)
	outerComparison := contextIdentityComposite(t, "Set", variantComparison)
	observe := func() sourceSetAsymmetryObservation {
		compare := func(left, right RuntimeValue) bool {
			value, err := runtimeSourceEqualContext(ctx, left, right)
			if err != nil {
				t.Fatal(err)
			}
			return value
		}
		return sourceSetAsymmetryObservation{
			Maps:     []bool{compare(comparison, duplicate), compare(duplicate, comparison)},
			Variants: []bool{compare(variantComparison, variantDuplicate), compare(variantDuplicate, variantComparison)},
			Sets:     []bool{compare(outerDuplicate, outerComparison), compare(outerComparison, outerDuplicate)},
		}
	}
	if got := observe(); !reflect.DeepEqual(got, fixture.Before) {
		t.Fatal("source Set equality must only query membership", got, fixture.Before)
	}
	for _, pair := range [][2]RuntimeValue{{outerDuplicate, outerComparison}, {outerComparison, outerDuplicate}} {
		if equal, err := pair[0].EqualContext(ctx, pair[1]); err != nil || equal {
			t.Fatal("public equality must remain symmetric", equal, err)
		}
	}
	for i, value := range []RuntimeValue{outerDuplicate, outerComparison} {
		code, err := runtimeReferenceCodeContext(ctx, value)
		if err != nil || code != fixture.Hashes[i] {
			t.Fatal("outer Set hash", code, fixture.Hashes[i], err)
		}
		if err := runtimeMarkReferenceHashContext(ctx, value); err != nil {
			t.Fatal(err)
		}
	}
	if got := observe(); !reflect.DeepEqual(got, fixture.After) {
		t.Fatal("source Set hash memo history", got, fixture.After)
	}
}

func TestUpstreamSourceConcreteStaleValueNodeHash(t *testing.T) {
	t.Parallel()
	var fixture struct {
		ConcreteHistory struct {
			KeyHashes      []int32
			Sizes          []int
			Lookup         []bool
			SourceEquality []bool
		}
	}
	data, err := os.ReadFile("testdata/parser/runtime_source_set_asymmetry.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	wrap := func(value RuntimeValue) RuntimeValue {
		for i := 0; i < 5; i++ {
			value = contextIdentityComposite(t, "variant", contextIdentityValue(t, `"K"`), value)
		}
		return value
	}
	oldKey := wrap(contextIdentityComposite(t, "List"))
	incomingKey := wrap(contextIdentityComposite(t, "Map"))
	zero, one := contextIdentityValue(t, "0"), contextIdentityValue(t, "1")
	extra := contextIdentityValue(t, "320")
	fillers := make([]RuntimeValue, 8)
	for i := range fillers {
		fillers[i] = contextIdentityComposite(t, "Tup", contextIdentityValue(t, strconv.Itoa(i+1)), zero)
	}
	initial := contextIdentityComposite(t, "Map", append([]RuntimeValue{contextIdentityComposite(t, "Tup", oldKey, zero)}, fillers...)...)
	raw, err := runtimeMap(initial)
	if err != nil {
		t.Fatal(err)
	}
	raw, err = raw.SetContext(ctx, incomingKey, one)
	if err != nil {
		t.Fatal(err)
	}
	raw, err = raw.SetContext(ctx, extra, zero)
	if err != nil {
		t.Fatal(err)
	}
	changed, err := runtimeWithEntriesContext(ctx, "Map", raw)
	if err != nil {
		t.Fatal(err)
	}
	freshPairs := append([]RuntimeValue{contextIdentityComposite(t, "Tup", incomingKey, one)}, fillers...)
	freshPairs = append(freshPairs, contextIdentityComposite(t, "Tup", extra, zero))
	fresh := contextIdentityComposite(t, "Map", freshPairs...)
	if changed.containsDomain || fresh.containsDomain || !changed.Equal(fresh) || changed.Hash() != fresh.Hash() {
		t.Fatal("fixture must retain identical concrete structural values")
	}
	for i, key := range []RuntimeValue{oldKey, incomingKey} {
		code, err := runtimeReferenceHashContext(ctx, key)
		if err != nil || code != fixture.ConcreteHistory.KeyHashes[i] {
			t.Fatal("raw key hash", i, code, fixture.ConcreteHistory.KeyHashes[i], err)
		}
	}
	for i, value := range []RuntimeValue{changed, fresh} {
		m, err := runtimeMap(value)
		if err != nil || m.Len() != fixture.ConcreteHistory.Sizes[i] {
			t.Fatal("map size", i, m.Len(), err)
		}
		_, found, err := m.LookupContext(ctx, incomingKey)
		if err != nil || found != fixture.ConcreteHistory.Lookup[i] {
			t.Fatal("stale routing hash lookup", i, found, fixture.ConcreteHistory.Lookup[i], err)
		}
	}
	for i, pair := range [][2]RuntimeValue{{changed, fresh}, {fresh, changed}} {
		equal, err := runtimeSourceEqualContext(ctx, pair[0], pair[1])
		if err != nil || equal != fixture.ConcreteHistory.SourceEquality[i] {
			t.Fatal("source equality must traverse concrete history", i, equal, fixture.ConcreteHistory.SourceEquality[i], err)
		}
		contextIdentityAssertEqualHash(t, pair[0], pair[1])
	}
}
