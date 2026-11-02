package quint

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

type replSaveFixture struct {
	Name, Input, Load, Main, UnsafeUpstream string
	Files                                   []struct{ Name, Source string }
	Evaluations                             []string
	Saved                                   *string
}

func replSaveFixtures(t *testing.T) []replSaveFixture {
	t.Helper()
	encoded, err := os.ReadFile("testdata/parser/repl_save.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixtures struct{ Cases []replSaveFixture }
	if err = json.Unmarshal(encoded, &fixtures); err != nil {
		t.Fatal(err)
	}
	return fixtures.Cases
}

func TestUpstreamREPLSavedSourceAndStableReplay(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	for _, fixture := range replSaveFixtures(t) {
		t.Run(fixture.Name, func(t *testing.T) {
			t.Parallel()
			if fixture.Saved == nil {
				return // Missing filename is exercised through the CLI.
			}
			directory := t.TempDir()
			for _, file := range fixture.Files {
				if err := os.WriteFile(filepath.Join(directory, file.Name), []byte(file.Source), 0600); err != nil {
					t.Fatal(err)
				}
			}
			session := NewREPLSession()
			if fixture.Load != "" {
				if err := session.Load(ctx, filepath.Join(directory, fixture.Load), fixture.Main); err != nil {
					t.Fatal(err)
				}
			}
			for _, input := range fixture.Evaluations {
				if fixture.Name == "crlf_annotation_not_recognized" && strings.HasPrefix(input, "import M.*") {
					continue // Go honors the existing CRLF annotation.
				}
				_, _ = session.Evaluate(ctx, input)
			}
			if strings.Contains(fixture.Input, ".clear") {
				session.Reset()
			}
			got, err := session.SavedText()
			if fixture.Name == "expression_comment_delimiter" {
				if err == nil || !strings.Contains(err.Error(), "*/") {
					t.Fatal("unsafe comment serialization accepted", got, err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			want := *fixture.Saved
			switch fixture.Name {
			case "loaded_main_with_final_newline":
				want = "// @mainModule M\n// original comment\nmodule M { pure val base=7 pure val extra = base+1\n\n}\n/*! extra !*/\n"
			case "loaded_main_not_final_module":
				want = "// @mainModule M\nmodule M { pure val base=7 pure val extra = base+1\n\n}\nmodule Other { pure val other=0 }/*! extra !*/\n"
			case "main_annotation_explicit_override_preserves_old_marker":
				want = strings.Replace(want, "// @mainModule M", "// @mainModule N", 1)
			case "crlf_annotation_not_recognized":
				want = "// @mainModule M\r\nmodule M { pure val base=7 }\r\n/*! base !*/\n"
			case "clear_preserves_last_module_annotation":
				want = "// @mainModule __repl__\n"
			}
			if got != want {
				t.Fatalf("Go %q\nwant %q\nraw upstream %q", got, want, *fixture.Saved)
			}
			path := filepath.Join(directory, "session.qnt")
			if err = session.Save(ctx, path); err != nil {
				t.Fatal(err)
			}
			restored := NewREPLSession()
			var replay []REPLReplayResult
			if err = restored.LoadWithReplay(ctx, path, "", REPLReplayHooks{After: func(entry REPLReplayResult) error { replay = append(replay, entry); return nil }}); err != nil {
				t.Fatal("saved session failed to load", err, got)
			}
			if len(replay) != len(session.saved.expressions) {
				t.Fatal("saved expressions were not replayed exactly once", replay, session.saved.expressions)
			}
			again, err := restored.SavedText()
			if err != nil {
				t.Fatal(err)
			}
			if session.saved.text == "" {
				// An empty saved session acquires its initialized empty module
				// on load; subsequent serialization is stable.
				if !strings.Contains(again, replEmptyModuleText) {
					t.Fatal(again)
				}
			} else if again != got {
				t.Fatalf("load/save duplicated replay comments or changed source\nfirst %q\nagain %q", got, again)
			}
		})
	}
}

func TestREPLSavedReplayStateFailuresAndCallbacks(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "state.qnt")
	text := "// @mainModule M\nmodule M { var x:int }/*! x'=1 !*/\n\n/*! x'=x+1 !*/\n\n/*! 1/0 !*/\n\n/*! missing !*/\n\n/*! x !*/\n"
	if err := os.WriteFile(path, []byte(text), 0600); err != nil {
		t.Fatal(err)
	}
	session := NewREPLSession()
	before := []string{}
	entries := []REPLReplayResult{}
	hooks := REPLReplayHooks{Before: func(file, input string) error {
		if file != path {
			t.Fatal(file)
		}
		before = append(before, input)
		return nil
	}, After: func(entry REPLReplayResult) error { entries = append(entries, entry); return nil }}
	if err := session.LoadWithReplay(ctx, path, "", hooks); err != nil || len(before) != 5 || len(entries) != 5 || entries[2].Err == nil || entries[3].Err == nil || entries[4].Result.Value.String() != "2" {
		t.Fatal(err, before, entries)
	}
	if len(session.History()) != 4 {
		t.Fatal("statically rejected replay entered history", session.History())
	}
	replEvaluate(t, session, "x'=x+10", "true")
	if err := session.ReloadWithReplay(ctx, REPLReplayHooks{}); err != nil {
		t.Fatal(err)
	}
	replEvaluate(t, session, "x", "2")
	stop := errors.New("output stopped")
	for _, after := range []bool{false, true} {
		session := NewREPLSession()
		hooks := REPLReplayHooks{}
		if after {
			hooks.After = func(REPLReplayResult) error { return stop }
		} else {
			hooks.Before = func(string, string) error { return stop }
		}
		if err := session.LoadWithReplay(ctx, path, "", hooks); !errors.Is(err, stop) || len(session.History()) != map[bool]int{false: 0, true: 1}[after] {
			t.Fatal(err, session.History())
		}
	}
	canceled, cancel := context.WithCancel(ctx)
	err := session.LoadWithReplay(canceled, path, "", REPLReplayHooks{After: func(REPLReplayResult) error { cancel(); return nil }})
	if !errors.Is(err, context.Canceled) || len(session.History()) != 1 {
		t.Fatal(err, session.History())
	}
	if err := NewREPLSession().ReloadWithReplay(ctx, REPLReplayHooks{}); err == nil {
		t.Fatal("reloaded without a source")
	}
}

func TestREPLSaveOwnershipAndErrors(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	session := NewREPLSession()
	for _, input := range []string{"var x:int", "action init=x'=4"} {
		if _, err := session.Evaluate(ctx, input); err != nil {
			t.Fatal(err)
		}
	}
	replEvaluate(t, session, "init", "true")
	closure := replEvaluate(t, session, "n=>x+n", "((_a0) => \"lambda_1_params\")").Value
	before := session.History()
	state, _ := session.State()
	seed := session.Seed().String()
	path := filepath.Join(t.TempDir(), "saved.qnt")
	if err := session.Save(ctx, path); err != nil {
		t.Fatal(err)
	}
	current, _ := session.State()
	if !reflect.DeepEqual(before, session.History()) || current.String() != state.String() || session.Seed().String() != seed || session.loaded != "" {
		t.Fatal("saving changed history, state, randomness, or reload target")
	}
	oneInteger, _ := ParseInteger("1")
	one, _ := NewRuntimeValue(Expr{Kind: "int", Int: oneInteger})
	if value, err := closure.Apply(ctx, []RuntimeValue{one}); err != nil || value.String() != "5" {
		t.Fatal("saving changed closure ownership", value.String(), err)
	}
	for _, invalid := range []string{filepath.Dir(path), filepath.Join(t.TempDir(), "missing", "session.qnt")} {
		if err := session.Save(ctx, invalid); err == nil {
			t.Fatal("save accepted invalid path", invalid)
		}
	}
	if err := os.WriteFile(path, []byte("unchanged"), 0600); err != nil {
		t.Fatal(err)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if err := session.Save(canceled, path); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if _, err := session.Evaluate(ctx, `"a */ b"`); err != nil {
		t.Fatal(err)
	}
	if err := session.Save(ctx, path); err == nil {
		t.Fatal("unsafe expression overwrote destination")
	}
	encoded, _ := os.ReadFile(path)
	if string(encoded) != "unchanged" {
		t.Fatal(string(encoded))
	}
	if _, err := (*REPLSession)(nil).SavedText(); err == nil {
		t.Fatal("nil session serialized")
	}
	if _, err := (&REPLSession{}).SavedText(); err == nil {
		t.Fatal("uninitialized session serialized")
	}
}

func TestREPLReplayMarkersAreLexicalAndOwned(t *testing.T) {
	t.Parallel()
	text := "module M { pure val quoted=\"/*! 99 !*/\" }\n// /*! 88 !*/\n/* unrelated /*! 77 !*/\n/*! List(\r\n1,\r\n2\r\n) !*/\r\n\n/*! 3 !*/\n"
	source, expressions := replExtractExpressions(text)
	if !reflect.DeepEqual(expressions, []string{"List(\r\n1,\r\n2\r\n)", "3"}) || !strings.Contains(source, `"/*! 99 !*/"`) || !strings.Contains(source, "// /*! 88 !*/") || !strings.Contains(source, "/* unrelated /*! 77 !*/") {
		t.Fatal(source, expressions)
	}
	for _, incomplete := range []string{`"unterminated`, "/* unterminated", "// no newline"} {
		got, expressions := replExtractExpressions(incomplete)
		if got != incomplete || len(expressions) != 0 {
			t.Fatal(got, expressions)
		}
	}
	for _, source := range []string{"module M {", "module Other {}"} {
		if _, err := newREPLArchive("test", source, "M", false); err == nil {
			t.Fatal("unverified module closing brace accepted", source)
		}
	}
	for _, text := range []string{"module M { pure/*! 2 !*/val x=1 }", "module M { pure/*! 2 !*/\nval x=1 }"} {
		cleaned, expressions := replExtractExpressions(text)
		if cleaned != "module M { pure val x=1 }" || !reflect.DeepEqual(expressions, []string{"2"}) {
			t.Fatal("removed comment changed token boundaries", cleaned, expressions)
		}
		if _, err := newREPLArchive("test", text, "M", false); err != nil {
			t.Fatal("valid inline replay comment rejected", err)
		}
	}
	cleaned, expressions := replExtractExpressions("// ordinary\r/*! 3 !*/\rmodule M{}")
	if cleaned != "// ordinary\r\rmodule M{}" || !reflect.DeepEqual(expressions, []string{"3"}) {
		t.Fatal("bare CR line comment swallowed replay", cleaned, expressions)
	}
	hashbang := "#!/usr/bin/env /*! 99 !*/\nmodule M{}"
	if cleaned, expressions := replExtractExpressions(hashbang); cleaned != hashbang || len(expressions) != 0 {
		t.Fatal("hashbang text was treated as executable history", cleaned, expressions)
	}
}

func TestREPLSaveOwnsLoadedSourceAndRelativeImports(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	directory := t.TempDir()
	mainPath := filepath.Join(directory, "main.qnt")
	mainText := "// original source 🙂\r\nmodule M{ import Base.* from \"base\" pure val y=x+1}\r\nmodule Other{}\r\n"
	for name, text := range map[string]string{"main.qnt": mainText, "base.qnt": "module Base { pure val x=7 }"} {
		if err := os.WriteFile(filepath.Join(directory, name), []byte(text), 0600); err != nil {
			t.Fatal(err)
		}
	}
	session := NewREPLSession()
	if err := session.Load(ctx, mainPath, "M"); err != nil {
		t.Fatal(err)
	}
	if _, err := session.Evaluate(ctx, "pure val extra=y+1"); err != nil {
		t.Fatal(err)
	}
	replEvaluate(t, session, "extra", "9")
	if err := os.WriteFile(mainPath, []byte("module M { pure val y=100 }"), 0600); err != nil {
		t.Fatal(err)
	}
	savedPath := filepath.Join(directory, "saved.qnt")
	if err := session.Save(ctx, savedPath); err != nil {
		t.Fatal(err)
	}
	encoded, err := os.ReadFile(savedPath)
	if err != nil {
		t.Fatal(err)
	}
	want := "// @mainModule M\n" + strings.Replace(mainText, "y=x+1}", "y=x+1\npure val extra=y+1\n}", 1) + "/*! extra !*/\n"
	if string(encoded) != want {
		t.Fatalf("save reread the changed file or lost its original text\ngot  %q\nwant %q", encoded, want)
	}
	restored := NewREPLSession()
	if err := restored.Load(ctx, savedPath, ""); err != nil {
		t.Fatal("saved relative import failed", err)
	}
	replEvaluate(t, restored, "extra", "9")
	if err := session.Reload(ctx); err != nil {
		t.Fatal(err)
	}
	replEvaluate(t, session, "y", "100")
}

func TestREPLSavePreservesBareCarriageReturns(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	for _, text := range []string{
		"// @mainModule M\rmodule M { pure val x=1 }\r",
		"// @mainModule M\rmodule M { // ordinary\rpure val x=1 }\r/*! 2 !*/\r",
	} {
		directory := t.TempDir()
		path := filepath.Join(directory, "main.qnt")
		if err := os.WriteFile(path, []byte(text), 0600); err != nil {
			t.Fatal(err)
		}
		session := NewREPLSession()
		if err := session.Load(ctx, path, ""); err != nil {
			t.Fatal("raw CR source rejected", err)
		}
		if _, err := session.Evaluate(ctx, "pure val extra=x+1"); err != nil {
			t.Fatal(err)
		}
		originalPrefix := strings.Split(text, "pure val x=1")[0] + "pure val x=1 "
		saved, err := session.SavedText()
		if err != nil || !strings.HasPrefix(saved, originalPrefix) {
			t.Fatal("saving changed original CR line ends", saved, err)
		}
		if err := session.Save(ctx, path); err != nil {
			t.Fatal(err)
		}
		restored := NewREPLSession()
		if err := restored.Load(ctx, path, ""); err != nil {
			t.Fatal("saved CR source rejected", err)
		}
		again, err := restored.SavedText()
		if err != nil || again != saved {
			t.Fatal("CR session serialization was not stable", saved, again, err)
		}
		replEvaluate(t, restored, "extra", "2")
	}
}
