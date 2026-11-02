package quint

import (
	"context"
	"testing"
)

func TestREPLExecutionRecorderSharesHistoryAndRNG(t *testing.T) {
	t.Parallel()
	for _, session := range []*REPLSession{nil, {}} {
		if session.ObjectHashState() != nil {
			t.Fatal("uninitialized session returned object history")
		}
		if _, err := session.NewTraceRecorder(3); err == nil {
			t.Fatal("uninitialized session accepted a recorder")
		}
	}
	session := NewREPLSession()
	recorder, err := session.NewTraceRecorder(3)
	if err != nil || recorder.RNG != session.runtime.RNG || session.ObjectHashState() != session.runtime.ObjectHashState() {
		t.Fatal("recorder or formatting history detached from execution", err)
	}
	seed, _ := ParseInteger("17")
	session.SetSeed(seed)
	if recorder.RNG.State() != session.Seed() {
		t.Fatal("recorder did not follow session RNG updates")
	}
	ctx := WithExecutionListener(context.Background(), recorder)
	if _, err := session.Evaluate(ctx, "pure def inc(x)=x+1"); err != nil {
		t.Fatal(err)
	}
	if _, err := session.Evaluate(ctx, "inc(1)"); err != nil {
		t.Fatal(err)
	}
	if len(recorder.CurrentFrame().Subframes) == 0 {
		t.Fatal("session recorder captured no execution")
	}
}
