package quint

import (
	"context"
	"errors"
	"sync"
	"testing"
)

func TestReferenceMemoMarksOnlyActualSourceHashCalls(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	interval := contextAccessInterval(t, "0", "1")
	label := contextAccessValue(t, `"K"`)
	key := contextAccessComposite(t, "variant", label, interval)
	a := contextAccessComposite(t, "Set", key)
	b := contextAccessComposite(t, "Set", contextAccessComposite(t, "variant", label, contextAccessValue(t, "Set(0,1)")))
	if runtimeReferenceHashCached(a) || runtimeReferenceHashCached(b) {
		t.Fatal("numeric precomputation changed source history")
	}
	var small runtimeReferenceMap
	for _, v := range []RuntimeValue{a, b} {
		var err error
		small, err = small.SetContext(ctx, v, contextAccessValue(t, "0"))
		if err != nil {
			t.Fatal(err)
		}
	}
	if small.Len() != 1 || runtimeReferenceHashCached(a) || runtimeReferenceHashCached(b) {
		t.Fatal("small array comparison eagerly hashed keys")
	}
	if _, _, err := small.LookupContext(ctx, a); err != nil {
		t.Fatal(err)
	}
	if runtimeReferenceHashCached(a) {
		t.Fatal("array lookup eagerly hashed key")
	}
	publicBefore, err := a.EqualContext(ctx, b)
	if err != nil || !publicBefore {
		t.Fatalf("public equality %v %v", publicBefore, err)
	}
	sourceBefore, err := runtimeSourceEqualContext(ctx, a, b)
	if err != nil || !sourceBefore {
		t.Fatalf("source equality %v %v", sourceBefore, err)
	}
	if err := runtimeMarkReferenceHashContext(ctx, a); err != nil {
		t.Fatal(err)
	}
	if err := runtimeMarkReferenceHashContext(ctx, b); err != nil {
		t.Fatal(err)
	}
	if !runtimeReferenceHashCached(a) || !runtimeReferenceHashCached(b) {
		t.Fatal("source hash did not mark underlying collections")
	}
	sourceAfter, err := runtimeSourceEqualContext(ctx, a, b)
	if err != nil || sourceAfter {
		t.Fatalf("source cached hash precheck %v %v", sourceAfter, err)
	}
	publicAfter, err := a.EqualContext(ctx, b)
	if err != nil || !publicAfter {
		t.Fatalf("public equality depends on source history %v %v", publicAfter, err)
	}
	ha, err := a.HashContext(ctx)
	if err != nil {
		t.Fatal(err)
	}
	hb, err := b.HashContext(ctx)
	if err != nil || ha != hb {
		t.Fatalf("public hashes differ %v", err)
	}
}

func TestReferenceMemoUnderlyingCollectionOwnership(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	domain := contextAccessInterval(t, "0", "1")
	label := contextAccessValue(t, `"K"`)
	variant := contextAccessComposite(t, "variant", label, domain)
	set := contextAccessComposite(t, "Set", variant)
	record := contextAccessComposite(t, "Rec", label, variant)
	list := contextAccessComposite(t, "List", variant)
	pair := contextAccessComposite(t, "Tup", label, variant)
	m := contextAccessComposite(t, "Map", pair)
	for _, v := range []RuntimeValue{set, record, list, m} {
		if err := runtimeMarkReferenceHashContext(ctx, v); err != nil {
			t.Fatal(err)
		}
		normalized, err := runtimeNormalFormContext(ctx, v)
		if err != nil {
			t.Fatal(err)
		}
		share := v.expression.Opcode == "Set" || v.expression.Opcode == "Rec"
		if runtimeReferenceHashCached(normalized) != share {
			t.Fatalf("%s normalForm memo sharing %v", v.expression.Opcode, runtimeReferenceHashCached(normalized))
		}
		if share && runtimeReferenceMemoOf(normalized) != runtimeReferenceMemoOf(v) {
			t.Fatalf("%s changed underlying memo", v.expression.Opcode)
		}
		if sameRuntimeObject(v, normalized) {
			t.Fatalf("%s normalForm reused wrapper identity", v.expression.Opcode)
		}
	}
	noOp, err := EvalMapBuiltinContext(ctx, "put", []RuntimeValue{m, label, variant})
	if err != nil || runtimeReferenceMemoOf(noOp) != runtimeReferenceMemoOf(m) {
		t.Fatalf("no-op update memo %v", err)
	}
	changed, err := EvalMapBuiltinContext(ctx, "put", []RuntimeValue{m, label, domain})
	if err != nil || runtimeReferenceHashCached(changed) {
		t.Fatalf("changed update kept old memo %v", err)
	}
	concrete := contextAccessValue(t, `{x:variant("K",Set(0,1))}`)
	if err := runtimeMarkReferenceHashContext(ctx, concrete); err != nil {
		t.Fatal(err)
	}
	normalized, err := runtimeNormalFormContext(ctx, concrete)
	if err != nil || runtimeReferenceMemoOf(normalized) != runtimeReferenceMemoOf(concrete) {
		t.Fatalf("concrete record no-op map memo %v", err)
	}
}

func TestReferenceMemoCancellationAndConcurrentMarks(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	value := contextAccessComposite(t, "List", contextAccessInterval(t, "0", "1"))
	if err := runtimeMarkReferenceHashContext(ctx, value); !errors.Is(err, context.Canceled) || runtimeReferenceHashCached(value) {
		t.Fatalf("canceled hash changed history %v", err)
	}
	ctx = context.Background()
	var group sync.WaitGroup
	for i := 0; i < 8; i++ {
		group.Add(1)
		go func() {
			defer group.Done()
			if err := runtimeMarkReferenceHashContext(ctx, value); err != nil {
				t.Error(err)
			}
		}()
	}
	group.Wait()
	if !runtimeReferenceHashCached(value) {
		t.Fatal("concurrent hash marks lost")
	}
	for _, op := range []string{"tuples", "powerset", "setOfMaps"} {
		args := []RuntimeValue{contextAccessComposite(t, "Set", contextAccessValue(t, "1"))}
		if op == "setOfMaps" {
			args = append(args, args[0])
		}
		v, err := EvalSymbolicBuiltin(ctx, op, args)
		if err != nil {
			t.Fatal(err)
		}
		if err := runtimeMarkReferenceHashContext(ctx, v); err != nil {
			t.Fatal(err)
		}
		if !runtimeReferenceHashCached(args[0]) {
			t.Fatalf("%s class hash failed to hash underlying factor", op)
		}
	}
}
