package quint

import (
	"context"
	"io"
	"testing"
)

func TestREPLFormatValueSharesSerializationHistory(t *testing.T) {
	t.Parallel()
	session := NewREPLSession()
	ctx := WithRuntimeHooks(context.Background(), RuntimeHooks{DebugWriter: io.Discard})
	result, err := session.Evaluate(ctx, `q::debug("set",Set(0,1,2,3,4,5,6,7,8))`)
	if err != nil {
		t.Fatal(err)
	}
	if got := session.runtime.ObjectHashState().Counter(); got != 18 {
		t.Fatalf("debug serialization counter: got %d, want 18", got)
	}
	text, err := session.FormatValue(result.Value, 0)
	if err != nil || text != "Set(0, 1, 2, 3, 4, 5, 6, 7, 8)" {
		t.Fatal(text, err)
	}
	if got := session.runtime.ObjectHashState().Counter(); got != 36 {
		t.Fatalf("result serialization counter: got %d, want 36", got)
	}
	// Formatting a retained result again allocates fresh serialization objects,
	// while a separate session starts with an independent compatibility history.
	if _, err = session.FormatValue(result.Value, 80); err != nil {
		t.Fatal(err)
	}
	if got := session.runtime.ObjectHashState().Counter(); got != 54 {
		t.Fatalf("second serialization counter: got %d, want 54", got)
	}
	other := NewREPLSession()
	if _, err = other.FormatValue(result.Value, 80); err != nil || other.runtime.ObjectHashState().Counter() != 18 {
		t.Fatal("independent session history", other.runtime.ObjectHashState().Counter(), err)
	}
}

func TestREPLFormatValueRejectsInvalidOwnersAndValues(t *testing.T) {
	t.Parallel()
	for _, session := range []*REPLSession{nil, {}, NewREPLSession()} {
		if text, err := session.FormatValue(RuntimeValue{}, 80); err == nil || text != "" {
			t.Fatalf("invalid result: text=%q err=%v", text, err)
		}
	}
}
