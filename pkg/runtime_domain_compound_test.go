package quint

import (
	"context"
	"errors"
	"testing"
)

func compoundTestSet(t *testing.T, source string) RuntimeDomain {
	t.Helper()
	expression, err := ParseExpression(source)
	if err != nil {
		t.Fatal(err)
	}
	value, err := NewRuntimeValue(expression)
	if err != nil {
		t.Fatal(err)
	}
	domain, err := RuntimeDomainFromValue(value)
	if err != nil {
		t.Fatal(err)
	}
	return domain
}

func TestCompoundDomainConstructorAndChildOwnership(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	base := compoundTestSet(t, "Set(Map((2,20),(1,10)))")
	entries, _ := runtimeEntries(base.explicit)
	child := entries[0].Value
	factors := []RuntimeDomain{base}
	product, err := NewRuntimeProduct(factors)
	if err != nil {
		t.Fatal(err)
	}
	powerset, err := NewRuntimePowerset(base)
	if err != nil {
		t.Fatal(err)
	}
	keys := compoundTestSet(t, "Set(3,1)")
	mapset, err := NewRuntimeMapSet(keys, base)
	if err != nil {
		t.Fatal(err)
	}
	// Constructors keep immutable child copies and independently own factor
	// slices. Replacing caller variables cannot alter constructed domains.
	factors[0] = RuntimeDomain{}
	base = RuntimeDomain{}
	keys = RuntimeDomain{}
	count := 0
	if err := product.ForEach(ctx, func(value RuntimeValue) (bool, error) {
		children, err := runtimeChildren(value)
		if err != nil || len(children) != 1 || !sameRuntimeObject(children[0], child) {
			t.Fatal("product rebuilt stored factor child", err)
		}
		count++
		return true, nil
	}); err != nil || count != 1 {
		t.Fatal("product iteration", err, count)
	}
	picked, consumed, err := product.PickWithPositions(ctx, []Integer{{}})
	children, childErr := runtimeChildren(picked)
	if err != nil || childErr != nil || consumed != 1 || !sameRuntimeObject(children[0], child) {
		t.Fatal("product pick reconstructed element", err, childErr, consumed)
	}
	selected, err := powerset.Pick(ctx, []Integer{domainFixtureInteger(t, "1")})
	selectedEntries, selectedErr := runtimeEntries(selected)
	if err != nil || selectedErr != nil || len(selectedEntries) != 1 || !selectedEntries[0].Value.Equal(child) || sameRuntimeObject(selectedEntries[0].Value, child) {
		t.Fatal("powerset must normalize chosen map", err, selectedErr)
	}
	positions := []Integer{{}, {}}
	picked, consumed, err = mapset.PickWithPositions(ctx, positions)
	pairs, pairErr := runtimeEntries(picked)
	if err != nil || pairErr != nil || consumed != 2 || len(pairs) != 2 || positions[0] != (Integer{}) || positions[1] != (Integer{}) {
		t.Fatal("map pick", err, pairErr, consumed)
	}
	for _, pair := range pairs {
		if !sameRuntimeObject(pair.Value, child) {
			t.Fatal("mapset copied stored range value")
		}
	}
	nested := compoundTestSet(t, "Set(Set(1,2))")
	interval := NewRuntimeInterval(domainFixtureInteger(t, "1"), domainFixtureInteger(t, "2"))
	if found, err := nested.ContainsDomain(ctx, interval); err != nil || !found {
		t.Fatal("explicit membership did not normalize interval", err)
	}
	if found, err := interval.ContainsDomain(ctx, nested); err != nil || found {
		t.Fatal("integer interval accepted a domain element", err)
	}
}

func TestCompoundDomainValidationCancellationAndStop(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	base := compoundTestSet(t, "Set(3,1)")
	product, _ := NewRuntimeProduct([]RuntimeDomain{base, base})
	powerset, _ := NewRuntimePowerset(base)
	mapset, _ := NewRuntimeMapSet(base, base)
	if _, err := NewRuntimeProduct([]RuntimeDomain{{}}); err == nil {
		t.Fatal("invalid product factor accepted")
	}
	if _, err := NewRuntimePowerset(RuntimeDomain{}); err == nil {
		t.Fatal("invalid powerset base accepted")
	}
	for _, pair := range [][2]RuntimeDomain{{{}, base}, {base, {}}} {
		if _, err := NewRuntimeMapSet(pair[0], pair[1]); err == nil {
			t.Fatal("invalid mapset child accepted")
		}
	}
	failure := errors.New("iteration callback failure")
	value, _ := NewRuntimeValue(Expr{Kind: "bool", Bool: true})
	for _, domain := range []RuntimeDomain{product, powerset, mapset} {
		count := 0
		if err := domain.ForEach(ctx, func(RuntimeValue) (bool, error) { count++; return false, nil }); err != nil || count != 1 {
			t.Fatal("early stop", err, count)
		}
		if err := domain.ForEach(ctx, func(RuntimeValue) (bool, error) { return true, failure }); !errors.Is(err, failure) {
			t.Fatal("callback failure lost", err)
		}
		iterctx, itercancel := context.WithCancel(ctx)
		count = 0
		if err := domain.ForEach(iterctx, func(RuntimeValue) (bool, error) { count++; itercancel(); return true, nil }); !errors.Is(err, context.Canceled) || count != 1 {
			t.Fatal("iteration cancellation", err, count)
		}
		canceled, cancel := context.WithCancel(ctx)
		cancel()
		if _, err := domain.Cardinality(canceled); !errors.Is(err, context.Canceled) {
			t.Fatal("cardinality cancellation", err)
		}
		if _, err := domain.Bounds(canceled); !errors.Is(err, context.Canceled) {
			t.Fatal("bounds cancellation", err)
		}
		if _, err := domain.Contains(canceled, value); !errors.Is(err, context.Canceled) {
			t.Fatal("membership cancellation", err)
		}
		if _, err := domain.ContainsDomain(canceled, base); !errors.Is(err, context.Canceled) {
			t.Fatal("domain membership cancellation", err)
		}
		if _, err := domain.Subset(canceled, domain); !errors.Is(err, context.Canceled) {
			t.Fatal("subset cancellation", err)
		}
		if _, err := domain.Equal(canceled, domain); !errors.Is(err, context.Canceled) {
			t.Fatal("equality cancellation", err)
		}
		if _, consumed, err := domain.PickWithPositions(canceled, nil); !errors.Is(err, context.Canceled) || consumed != 0 {
			t.Fatal("pick cancellation consumed positions", err, consumed)
		}
		rng := NewRNG(domainFixtureInteger(t, "17"))
		if _, err := domain.Sample(canceled, rng); !errors.Is(err, context.Canceled) || rng.State().String() != "17" {
			t.Fatal("sample cancellation consumed RNG", err, rng.State())
		}
		if _, err := domain.Materialize(canceled); !errors.Is(err, context.Canceled) {
			t.Fatal("materialization cancellation", err)
		}
		if _, err := domain.Serialize(canceled, &RuntimeObjectHashState{}); !errors.Is(err, context.Canceled) {
			t.Fatal("serialization cancellation", err)
		}
	}
	if _, err := powerset.ContainsDomain(ctx, RuntimeDomain{}); err == nil {
		t.Fatal("invalid member domain accepted")
	}
	if _, err := (RuntimeDomain{}).ContainsDomain(ctx, base); err == nil {
		t.Fatal("invalid containing domain accepted")
	}
	if exception := (RuntimeDomainException{Name: "Error", Message: "failure"}); exception.Error() != "failure" {
		t.Fatal("exception message")
	}
}

func TestCompoundDomainMixedEqualityAndZeroBounds(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	empty := compoundTestSet(t, "Set()")
	zeroProduct, _ := NewRuntimeProduct(nil)
	if equal, err := zeroProduct.Equal(ctx, empty); err != nil || !equal {
		t.Fatal("zero-factor product equals enumerated empty set", err)
	}
	if equal, err := empty.Equal(ctx, zeroProduct); err != nil || !equal {
		t.Fatal("empty set equals zero-factor product", err)
	}
	integer, _ := NewRuntimeInfiniteSet("Int")
	emptyMaps, _ := NewRuntimeMapSet(empty, integer)
	for _, domain := range []RuntimeDomain{zeroProduct, emptyMaps} {
		if bounds, err := domain.Bounds(ctx); err != nil || len(bounds) != 0 {
			t.Fatal("zero bounds", bounds, err)
		}
		rng := NewRNG(domainFixtureInteger(t, "17"))
		value, err := domain.Sample(ctx, rng)
		if err != nil || rng.State().String() != "17" {
			t.Fatal("zero bounds consumed RNG", err, rng.State())
		}
		withoutRNG, err := domain.Sample(ctx, nil)
		if err != nil || !value.Equal(withoutRNG) {
			t.Fatal("zero-bound sample required RNG", err)
		}
	}
	powerset, _ := NewRuntimePowerset(integer)
	if _, err := powerset.Equal(ctx, empty); err == nil {
		t.Fatal("infinite-base mixed equality did not report enumeration error")
	}
	base := compoundTestSet(t, "Set(1)")
	product, _ := NewRuntimeProduct([]RuntimeDomain{base, base})
	explicit := compoundTestSet(t, "Set((1,1))")
	if equal, err := product.Equal(ctx, explicit); err != nil || !equal {
		t.Fatal("product explicit equality", err)
	}
	if subset, err := product.Subset(ctx, explicit); err != nil || !subset {
		t.Fatal("product explicit subset", err)
	}
	if found, err := product.ContainsDomain(ctx, base); err != nil || found {
		t.Fatal("product accepted set-valued element", err)
	}
}

func TestCompoundDomainNumberHashOverflow(t *testing.T) {
	t.Parallel()
	integer, _ := NewRuntimeInfiniteSet("Int")
	// Obtained directly from Node22.22.2/V8 12.4 RuntimeValueCrossProd.hashCode
	// and Immutable.hash. These exceed int64 and retain Number rounding.
	for _, fixture := range []struct {
		count int
		code  float64
		hash  int32
	}{{1025, 9232379236109515000, -2048}, {2049, 18455751272964290000, -4096}} {
		factors := make([]RuntimeDomain, fixture.count)
		for i := range factors {
			factors[i] = integer
		}
		product, err := NewRuntimeProduct(factors)
		if err != nil {
			t.Fatal(err)
		}
		code, err := product.referenceHashCode()
		if err != nil || code != fixture.code {
			t.Fatal("Number hashCode", code, fixture.code, err)
		}
		hash, err := product.referenceHash()
		if err != nil || hash != fixture.hash {
			t.Fatal("Immutable hash", hash, fixture.hash, err)
		}
	}
}
