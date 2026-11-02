package quint

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"os"
	"testing"
)

func contextIdentityValue(t *testing.T, source string) RuntimeValue {
	t.Helper()
	expression, err := ParseExpression(source)
	if err != nil {
		t.Fatal(err)
	}
	value, err := NewRuntimeValue(expression)
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func contextIdentityDomain(t *testing.T, domain RuntimeDomain) RuntimeValue {
	t.Helper()
	value, err := RuntimeValueFromDomain(domain)
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func contextIdentityComposite(t *testing.T, op string, children ...RuntimeValue) RuntimeValue {
	t.Helper()
	value, err := NewRuntimeCompositeContext(context.Background(), op, children)
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func contextIdentityAssertEqualHash(t *testing.T, left, right RuntimeValue) {
	t.Helper()
	ctx := context.Background()
	for _, values := range [][2]RuntimeValue{{left, right}, {right, left}} {
		equal, err := values[0].EqualContext(ctx, values[1])
		if err != nil || !equal {
			t.Fatal("checked equality", equal, err)
		}
	}
	leftHash, err := left.HashContext(ctx)
	if err != nil {
		t.Fatal(err)
	}
	rightHash, err := right.HashContext(ctx)
	if err != nil || leftHash != rightHash {
		t.Fatal("equal values have different checked hashes", err)
	}
}

func TestRuntimeContextIdentityDomains(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	interval := NewRuntimeInterval(domainFixtureInteger(t, "1"), domainFixtureInteger(t, "3"))
	left := contextIdentityDomain(t, interval)
	right := contextIdentityValue(t, "Set(3,1,2)")
	contextIdentityAssertEqualHash(t, left, right)
	if left.Equal(right) || left.Hash() == right.Hash() {
		t.Fatal("context-free identity unexpectedly materialized a domain")
	}
	empty := contextIdentityDomain(t, NewRuntimeInterval(domainFixtureInteger(t, "5"), domainFixtureInteger(t, "3")))
	contextIdentityAssertEqualHash(t, empty, contextIdentityValue(t, "Set()"))
	product, err := NewRuntimeProduct(nil)
	if err != nil {
		t.Fatal(err)
	}
	// The reference's zero-factor cross product enumerates an empty Set even
	// though its cardinality and positional pick use a singleton empty tuple.
	contextIdentityAssertEqualHash(t, contextIdentityDomain(t, product), contextIdentityValue(t, "Set()"))
	for _, source := range []string{"1", "List(1,2,3)", "(1,2,3)", "true"} {
		if equal, err := left.EqualContext(ctx, contextIdentityValue(t, source)); err != nil || equal {
			t.Fatal("domain equated with a different constructor", source, equal, err)
		}
	}
	huge := NewRuntimeInterval(Integer{}, domainFixtureInteger(t, "1000000000000000000000000000000"))
	if equal, err := contextIdentityDomain(t, huge).EqualContext(ctx, right); err != nil || equal {
		t.Fatal("huge interval comparison should use cardinality", equal, err)
	}
	if equal, err := contextIdentityDomain(t, huge).EqualContext(ctx, contextIdentityDomain(t, huge)); err != nil || !equal {
		t.Fatal("huge interval equality should compare endpoints", equal, err)
	}
	for _, kind := range []string{"Int", "Nat"} {
		domain, _ := NewRuntimeInfiniteSet(kind)
		contextIdentityAssertEqualHash(t, contextIdentityDomain(t, domain), contextIdentityDomain(t, domain))
	}
	integers, _ := NewRuntimeInfiniteSet("Int")
	naturals, _ := NewRuntimeInfiniteSet("Nat")
	intValue, natValue := contextIdentityDomain(t, integers), contextIdentityDomain(t, naturals)
	if equal, err := intValue.EqualContext(ctx, natValue); err != nil || equal {
		t.Fatal("Int and Nat equated", equal, err)
	}
	intHash, _ := intValue.HashContext(ctx)
	natHash, _ := natValue.HashContext(ctx)
	if intHash == natHash {
		t.Fatal("infinite domain identity markers collided")
	}
	if equal, err := intValue.EqualContext(ctx, right); err != nil || equal {
		t.Fatal("Int equated with a finite set", equal, err)
	}
	booleans, err := RuntimeDomainFromValue(contextIdentityValue(t, "Set(false,true)"))
	if err != nil {
		t.Fatal(err)
	}
	powerset, _ := NewRuntimePowerset(interval)
	product, _ = NewRuntimeProduct([]RuntimeDomain{interval, booleans})
	mapset, _ := NewRuntimeMapSet(interval, booleans)
	for _, domain := range []RuntimeDomain{powerset, product, mapset} {
		materialized, err := domain.Materialize(ctx)
		if err != nil {
			t.Fatal(err)
		}
		contextIdentityAssertEqualHash(t, contextIdentityDomain(t, domain), materialized)
	}
}

func TestRuntimeContextIdentityRetainedContainers(t *testing.T) {
	t.Parallel()
	interval := contextIdentityDomain(t, NewRuntimeInterval(domainFixtureInteger(t, "1"), domainFixtureInteger(t, "2")))
	explicit := contextIdentityValue(t, "Set(2,1)")
	label := contextIdentityValue(t, `"D"`)
	key := contextIdentityValue(t, "0")
	for _, op := range []string{"List", "Tup", "variant", "Rec", "Map"} {
		t.Run(op, func(t *testing.T) {
			leftChildren, rightChildren := []RuntimeValue{interval}, []RuntimeValue{explicit}
			switch op {
			case "variant", "Rec":
				leftChildren = append([]RuntimeValue{label}, leftChildren...)
				rightChildren = append([]RuntimeValue{label}, rightChildren...)
			case "Map":
				leftChildren = []RuntimeValue{contextIdentityComposite(t, "Tup", key, interval)}
				rightChildren = []RuntimeValue{contextIdentityComposite(t, "Tup", key, explicit)}
			}
			contextIdentityAssertEqualHash(t,
				contextIdentityComposite(t, op, leftChildren...),
				contextIdentityComposite(t, op, rightChildren...))
		})
	}
	closure := newRuntimeClosure(0, func(context.Context, []RuntimeValue) (RuntimeValue, error) { return explicit, nil })
	left := contextIdentityComposite(t, "List", interval, closure)
	right := contextIdentityComposite(t, "List", explicit, closure)
	contextIdentityAssertEqualHash(t, left, right)
	otherClosure := newRuntimeClosure(0, closure.closure.apply)
	different := contextIdentityComposite(t, "List", explicit, otherClosure)
	if equal, err := left.EqualContext(context.Background(), different); err != nil || equal {
		t.Fatal("checked collection lost closure object identity", equal, err)
	}
	leftVariant := contextIdentityComposite(t, "variant", label, closure)
	rightVariant := contextIdentityComposite(t, "variant", label, closure)
	left = contextIdentityComposite(t, "List", interval, leftVariant)
	right = contextIdentityComposite(t, "List", explicit, rightVariant)
	if equal, err := left.EqualContext(context.Background(), right); err != nil || equal {
		t.Fatal("variant payload incorrectly used collection identity", equal, err)
	}
	// Reusing that exact variant object inside an immutable list does apply
	// the collection identity shortcut before Variant.equals is invoked.
	contextIdentityAssertEqualHash(t, left, contextIdentityComposite(t, "List", explicit, leftVariant))
	if equal, err := left.EqualContext(context.Background(), contextIdentityComposite(t, "List", explicit)); err != nil || equal {
		t.Fatal("different collection lengths equated", equal, err)
	}
	contextIdentityAssertEqualHash(t,
		contextIdentityComposite(t, "variant", label, contextIdentityComposite(t, "List", interval, closure)),
		contextIdentityComposite(t, "variant", label, contextIdentityComposite(t, "List", explicit, closure)))
}

func TestRuntimeContextIdentityConcreteAndCancellation(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	value := contextIdentityValue(t, `Map((1,List(2,3)),(4,variant("V",Set(6,5))))`)
	copy := contextIdentityValue(t, `Map((4,variant("V",Set(5,6))),(1,List(2,3)))`)
	if equal, err := value.EqualContext(ctx, copy); err != nil || equal != value.Equal(copy) {
		t.Fatal("concrete equality changed", equal, err)
	}
	if hash, err := value.HashContext(ctx); err != nil || hash != value.Hash() {
		t.Fatal("concrete hash changed", err)
	}
	compact := contextIdentityDomain(t, NewRuntimeInterval(Integer{}, domainFixtureInteger(t, "1000000000000000000000000000000")))
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	for _, input := range []RuntimeValue{value, compact} {
		if _, err := input.EqualContext(canceled, input); !errors.Is(err, context.Canceled) {
			t.Fatal("equality ignored cancellation", err)
		}
		if _, err := input.HashContext(canceled); !errors.Is(err, context.Canceled) {
			t.Fatal("hash ignored cancellation", err)
		}
		if _, err := runtimeReferenceCodeContext(canceled, input); !errors.Is(err, context.Canceled) {
			t.Fatal("reference hash ignored cancellation", err)
		}
	}
	if _, err := (RuntimeValue{}).EqualContext(ctx, value); err == nil {
		t.Fatal("invalid equality argument accepted")
	}
	if _, err := (RuntimeValue{}).HashContext(ctx); err == nil {
		t.Fatal("invalid hash argument accepted")
	}
	if _, err := runtimeReferenceCodeContext(ctx, RuntimeValue{}); err == nil {
		t.Fatal("invalid reference hash argument accepted")
	}
	integers, _ := NewRuntimeInfiniteSet("Int")
	powerset, _ := NewRuntimePowerset(integers)
	if _, err := contextIdentityDomain(t, powerset).HashContext(ctx); err == nil {
		t.Fatal("compound enumeration error lost")
	} else {
		var exception RuntimeDomainException
		if !errors.As(err, &exception) || exception.Name != "Error" || exception.Message != "Either state is Left" {
			t.Fatal("compound enumeration exception changed", err)
		}
	}
}

type contextIdentityCancelContext struct {
	context.Context
	checks int
}

func (ctx *contextIdentityCancelContext) Err() error {
	ctx.checks++
	if ctx.checks >= 25 {
		return context.Canceled
	}
	return nil
}

func TestRuntimeContextIdentityCancellationDuringTraversal(t *testing.T) {
	t.Parallel()
	compact := contextIdentityDomain(t, NewRuntimeInterval(Integer{}, domainFixtureInteger(t, "1000000000000000000000000000000")))
	ctx := &contextIdentityCancelContext{Context: context.Background()}
	if _, err := compact.HashContext(ctx); !errors.Is(err, context.Canceled) || ctx.checks < 25 {
		t.Fatal("finite-domain hashing did not cancel during enumeration", ctx.checks, err)
	}
	interval := contextIdentityDomain(t, NewRuntimeInterval(Integer{}, domainFixtureInteger(t, "1")))
	explicit := contextIdentityValue(t, "Set(0,1)")
	leftChildren, rightChildren := make([]RuntimeValue, 32), make([]RuntimeValue, 32)
	for i := range leftChildren {
		leftChildren[i], rightChildren[i] = interval, explicit
	}
	left := contextIdentityComposite(t, "List", leftChildren...)
	right := contextIdentityComposite(t, "List", rightChildren...)
	ctx = &contextIdentityCancelContext{Context: context.Background()}
	if _, err := left.EqualContext(ctx, right); !errors.Is(err, context.Canceled) || ctx.checks < 25 {
		t.Fatal("retained collection equality did not cancel", ctx.checks, err)
	}
	left.referenceCode = nil
	ctx = &contextIdentityCancelContext{Context: context.Background()}
	if _, err := runtimeReferenceCodeContext(ctx, left); !errors.Is(err, context.Canceled) || ctx.checks < 25 {
		t.Fatal("raw retained collection hashing did not cancel", ctx.checks, err)
	}
	integers, _ := NewRuntimeInfiniteSet("Int")
	factors := make([]RuntimeDomain, 32)
	for i := range factors {
		factors[i] = integers
	}
	product, _ := NewRuntimeProduct(factors)
	value := contextIdentityDomain(t, product)
	value.referenceCode = nil
	ctx = &contextIdentityCancelContext{Context: context.Background()}
	if _, err := runtimeReferenceCodeContext(ctx, value); !errors.Is(err, context.Canceled) || ctx.checks < 25 {
		t.Fatal("raw compound domain hashing did not cancel", ctx.checks, err)
	}
}

func TestRuntimeContextReferenceConcreteOracle(t *testing.T) {
	t.Parallel()
	var fixtures []struct {
		Name          string
		Input         Expr
		Hash          int64
		ImmutableHash int32
	}
	data, err := os.ReadFile("testdata/parser/runtime_reference_hash.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	for _, fixture := range fixtures {
		value, err := NewRuntimeValue(fixture.Input)
		if err != nil {
			t.Fatal(fixture.Name, err)
		}
		// Also exercise retained container traversal against the saved
		// upstream oracle; a conservative containsDomain flag is harmless.
		if value.payload != nil {
			value.containsDomain = true
		}
		code, err := runtimeReferenceCodeContext(context.Background(), value)
		if err != nil || code != float64(fixture.Hash) {
			t.Fatal(fixture.Name, "raw hash", code, fixture.Hash, err)
		}
		hash, err := runtimeReferenceHashContext(context.Background(), value)
		if err != nil || hash != fixture.ImmutableHash {
			t.Fatal(fixture.Name, "Immutable hash", hash, fixture.ImmutableHash, err)
		}
	}
}

func TestRuntimeContextReferenceCompactOracle(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	interval := contextIdentityDomain(t, NewRuntimeInterval(domainFixtureInteger(t, "1"), domainFixtureInteger(t, "3")))
	integers, _ := NewRuntimeInfiniteSet("Int")
	naturals, _ := NewRuntimeInfiniteSet("Nat")
	intValue, natValue := contextIdentityDomain(t, integers), contextIdentityDomain(t, naturals)
	label := contextIdentityValue(t, `"D"`)
	product, _ := NewRuntimeProduct([]RuntimeDomain{*interval.domain, integers})
	powerset, _ := NewRuntimePowerset(*interval.domain)
	mapset, _ := NewRuntimeMapSet(*interval.domain, integers)
	// Raw codes and Immutable hashes were obtained from the pinned TypeScript
	// rv constructors, without normalizing their compact retained children.
	tests := []struct {
		name  string
		value RuntimeValue
		code  float64
		hash  int32
	}{
		{"interval", interval, 4, 4},
		{"Int", intValue, 9007199254740991, -1},
		{"Nat", natValue, 9007199254740990, -2},
		{"product", contextIdentityDomain(t, product), 9007199254740996, 4},
		{"powerset", contextIdentityDomain(t, powerset), 4, 4},
		{"mapset", contextIdentityDomain(t, mapset), 9007199254740996, 4},
		{"list", contextIdentityComposite(t, "List", interval, intValue, natValue), -618596307, -618596307},
		{"tuple", contextIdentityComposite(t, "Tup", interval, intValue, natValue), -618596307, -618596307},
		{"record", contextIdentityComposite(t, "Rec", contextIdentityValue(t, `"a"`), interval, contextIdentityValue(t, `"i"`), intValue, contextIdentityValue(t, `"z"`), natValue), 140948319, 140948319},
		{"map", contextIdentityComposite(t, "Map", contextIdentityComposite(t, "Tup", contextIdentityValue(t, "1"), interval), contextIdentityComposite(t, "Tup", contextIdentityValue(t, "2"), intValue), contextIdentityComposite(t, "Tup", contextIdentityValue(t, "3"), natValue)), 653022269, 653022269},
		{"variant interval", contextIdentityComposite(t, "variant", label, interval), 8, 8},
		{"variant Int", contextIdentityComposite(t, "variant", label, intValue), 9007199254740990, -2},
		{"nested variant", contextIdentityComposite(t, "variant", label, contextIdentityComposite(t, "variant", label, intValue)), 9007199254740988, -4},
	}
	for _, test := range tests {
		test.value.referenceCode = nil
		code, err := runtimeReferenceCodeContext(ctx, test.value)
		if err != nil || code != test.code {
			t.Fatal(test.name, "raw code", code, test.code, err)
		}
		hash, err := runtimeReferenceHashContext(ctx, test.value)
		if err != nil || hash != test.hash {
			t.Fatal(test.name, "Immutable hash", hash, test.hash, err)
		}
		cached := test.value
		cached.referenceCode = &code
		if got, err := runtimeReferenceHashContext(ctx, cached); err != nil || got != hash {
			t.Fatal(test.name, "cached reference hash", got, hash, err)
		}
	}
	for _, test := range []struct {
		factors int
		code    float64
		hash    int32
	}{{1025, 9232379236109515000, -2048}, {2049, 18455751272964290000, -4096}} {
		factors := make([]RuntimeDomain, test.factors)
		for i := range factors {
			factors[i] = integers
		}
		product, _ := NewRuntimeProduct(factors)
		value := contextIdentityDomain(t, product)
		if code, err := runtimeReferenceCodeContext(ctx, value); err != nil || code != test.code {
			t.Fatal("Number sum above int64", test.factors, code, test.code, err)
		}
		if hash, err := runtimeReferenceHashContext(ctx, value); err != nil || hash != test.hash {
			t.Fatal("Number coercion above int64", test.factors, hash, test.hash, err)
		}
	}
	for _, code := range []float64{math.NaN(), math.Inf(1), math.Inf(-1)} {
		if runtimeReferenceNumberSMI(code) != 0 {
			t.Fatal("nonfinite Number did not coerce to zero")
		}
	}
	if runtimeReferenceNumberSMI(-4294967297) != -1 || runtimeReferenceNumberSMI(4294967297) != 1 {
		t.Fatal("Number coercion failed outside uint32")
	}
}

func TestRuntimeContextCompactClosureKeysInTransientMap(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	huge := NewRuntimeInterval(Integer{}, domainFixtureInteger(t, "1000000000000000000000000000000"))
	entries := make([]runtimeReferenceEntry, 12)
	domains := make([]RuntimeDomain, len(entries))
	var referenceHash int32
	for i := range entries {
		value := contextIdentityValue(t, "0")
		closure := newRuntimeClosure(0, func(context.Context, []RuntimeValue) (RuntimeValue, error) { return value, nil })
		set := contextIdentityComposite(t, "Set", closure)
		factor, err := RuntimeDomainFromValue(set)
		if err != nil {
			t.Fatal(err)
		}
		domains[i], err = NewRuntimeProduct([]RuntimeDomain{factor, huge})
		if err != nil {
			t.Fatal(err)
		}
		key := contextIdentityDomain(t, domains[i])
		hash, err := runtimeReferenceHashContext(ctx, key)
		if err != nil {
			t.Fatal(err)
		}
		if i == 0 {
			referenceHash = hash
		} else if hash != referenceHash || key.Hash() != entries[0].Key.Hash() {
			t.Fatal("fixture must collide in both reference and structural hashes")
		}
		entries[i] = runtimeReferenceEntry{Key: key, Value: value}
	}
	// Fresh display-identical closures must remain distinct keys as the raw
	// transient Set grows beyond its small-array representation. The compact
	// product's enormous interval is never enumerated for these comparisons.
	m, err := runtimeReferenceMapFromEntries(entries)
	if err != nil || m.Len() != len(entries) {
		t.Fatal("compact keys discarded distinct closure children", m.Len(), err)
	}
	for i, domain := range domains {
		alias := contextIdentityDomain(t, domain)
		value, found, err := m.Lookup(alias)
		if err != nil || !found || !sameRuntimeObject(value, entries[i].Value) {
			t.Fatal("compact collision lookup lost retained closure identity", i, found, err)
		}
	}
}
