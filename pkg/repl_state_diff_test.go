package quint

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestREPLCommittedStateSnapshots(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	session := NewREPLSession()
	for _, input := range []string{"var x:int", "var y:int", "action init=all{x'=1,y'=2}", "action step=all{x'=x+1,y'=y+2}"} {
		if result, err := session.Evaluate(ctx, input); err != nil || result.OldState.Valid() || result.NewState.Valid() {
			t.Fatal(input, result, err)
		}
	}
	var saved REPLResult
	for _, step := range []struct {
		input, old, new, missing string
	}{
		{"true", "", "", ""},
		{"init", "Rec()", "Rec(\"x\", 1, \"y\", 2)", ""},
		{"x+y", "", "", ""},
		{"step", "Rec(\"x\", 1, \"y\", 2)", "Rec(\"x\", 2, \"y\", 4)", ""},
		{"all{x'=x,y'=y}", "Rec(\"x\", 2, \"y\", 4)", "Rec(\"x\", 2, \"y\", 4)", ""},
		{"x'=8", "Rec(\"x\", 2, \"y\", 4)", "Rec(\"x\", 8)", "y"},
		{"false", "", "", ""},
	} {
		result, err := session.Evaluate(ctx, step.input)
		if err != nil {
			t.Fatal(step.input, err)
		}
		if step.old == "" {
			if result.Shifted || result.OldState.Valid() || result.NewState.Valid() {
				t.Fatal("noncommit retained states", step.input, result)
			}
			continue
		}
		if !result.Shifted || result.OldState.String() != step.old || result.NewState.String() != step.new || strings.Join(result.Missing, ",") != step.missing {
			t.Fatalf("%s: old=%s new=%s shifted=%v missing=%v", step.input, result.OldState.String(), result.NewState.String(), result.Shifted, result.Missing)
		}
		if step.input == "init" {
			saved = result
		}
	}
	session.Reset()
	if saved.OldState.String() != "Rec()" || saved.NewState.String() != "Rec(\"x\", 1, \"y\", 2)" {
		t.Fatal("later commits/reset modified retained snapshots", saved)
	}
}

func TestREPLStateSnapshotsFollowInternalShiftAndReplay(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	session := NewREPLSession()
	for _, input := range []string{"var x:int", "action init=x'=1", "action step=x'=x+1"} {
		if _, err := session.Evaluate(ctx, input); err != nil {
			t.Fatal(err)
		}
	}
	result := replEvaluate(t, session, "init.then(step)", "true")
	if result.OldState.String() != "Rec(\"x\", 1)" || result.NewState.String() != "Rec(\"x\", 2)" {
		t.Fatal("snapshots must surround REPL commit after the internal then shift", result.OldState.String(), result.NewState.String())
	}
	path := filepath.Join(t.TempDir(), "session.qnt")
	if err := session.Save(ctx, path); err != nil {
		t.Fatal(err)
	}
	loaded := NewREPLSession()
	var replay []REPLReplayResult
	if err := loaded.LoadWithReplay(ctx, path, "", REPLReplayHooks{After: func(entry REPLReplayResult) error { replay = append(replay, entry); return nil }}); err != nil {
		t.Fatal(err)
	}
	if len(replay) != 1 || replay[0].Err != nil || !replay[0].Result.OldState.Equal(result.OldState) || !replay[0].Result.NewState.Equal(result.NewState) {
		t.Fatal("saved input lost its commit snapshots", replay)
	}
}

func TestREPLStateSnapshotsRetainImportedAndCompactValues(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "model.qnt")
	source := "module A { var x:Set[int] action init=x'=1.to(3) action step=x'=x.union(Set(4)) } module M { import A as I }"
	if err := os.WriteFile(path, []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	session := NewREPLSession()
	if err := session.Load(ctx, path, "M"); err != nil {
		t.Fatal(err)
	}
	init := replEvaluate(t, session, "I::init", "true")
	children, err := runtimeChildren(init.NewState)
	if err != nil || len(children) != 2 || children[0].expression.Str != "x" || children[1].domain == nil {
		t.Fatal("snapshot must retain the imported compact value without enumeration", init.NewState.String(), err)
	}
	step := replEvaluate(t, session, "I::step", "true")
	if !init.NewState.Equal(step.OldState) {
		t.Fatal("recompiling an imported action changed its old state", init.NewState.String(), step.OldState.String())
	}
	if _, err := session.Evaluate(ctx, "true"); err != nil {
		t.Fatal(err)
	}
	text, err := session.FormatValueContext(ctx, init.NewState, 80)
	if err != nil || text != "{ x: Set(1, 2, 3) }" {
		t.Fatal("later state changed compact snapshot", text, err)
	}
}
