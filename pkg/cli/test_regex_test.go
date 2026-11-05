package cli

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestJavaScriptRegexReference(t *testing.T) {
	t.Parallel()
	data, err := os.ReadFile("testdata/test_regex.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixtures []struct {
		Name, Pattern string
		Invalid       bool
		Cases         []struct {
			Input string
			Match bool
		}
	}
	if err := json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	for _, fixture := range fixtures {
		t.Run(fixture.Name, func(t *testing.T) {
			if fixture.Pattern == "" {
				for _, tc := range fixture.Cases {
					if got := strings.HasSuffix(tc.Input, "Test"); got != tc.Match {
						t.Fatalf("default match %q=%v; want %v", tc.Input, got, tc.Match)
					}
				}
				return
			}
			compiled, err := testCompileJavaScriptRegex(fixture.Pattern)
			if fixture.Invalid {
				if err == nil {
					t.Fatalf("accepted invalid JavaScript expression %q", fixture.Pattern)
				}
				return
			}
			if err != nil {
				t.Fatal(fixture.Pattern, err)
			}
			for _, tc := range fixture.Cases {
				got, err := compiled.MatchString(context.Background(), tc.Input)
				if err != nil || got != tc.Match {
					t.Fatalf("JavaScript /%s/ on %q=%v,%v; want %v", fixture.Pattern, tc.Input, got, err, tc.Match)
				}
			}
		})
	}
}

func TestJavaScriptRegexConsumption(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		body     string
		consumes bool
	}{
		{"a", true}, {"a|b", true}, {"a|", false}, {"|b", false},
		{"a?", false}, {"a*", false}, {"a+?", true}, {"a?b", true},
		{"a{0}", false}, {"a{00,2}?", false}, {"a{2,}", true}, {"a{2,3}", true},
		{"a{,2}", true}, {"a{x}", true}, {"a{2,x}", true},
		{"(?:a|b)+", true}, {"(?:a?|b)+", false}, {"(?<π>a)", true},
		{"(?=a)", false}, {"(?!a)", false}, {"(?<=a)", false}, {"(?<!a)b", true},
		{`\b\B^$`, false}, {`\1`, false}, {`\12?`, false}, {`\k<π>?`, true},
		{`(?<π>)\k<π>?`, false}, {`a\k<|(?=>)>{0}`, false}, {`a\k<|(?=>)>?`, false},
		{`\01?`, false}, {`\u0061?`, false}, {`\x61+`, true}, {`\cA`, true},
		{`\c_`, true}, {`\π`, true}, {"π", true}, {"😀?", false},
		{"[a-z]", true}, {`[\c_]?`, false}, {"[]", true},
	} {
		if got := testRegexMustConsume(tc.body, testRegexHasNamedCapture(tc.body)); got != tc.consumes {
			t.Errorf("must consume /%s/ = %v; want %v", tc.body, got, tc.consumes)
		}
	}
}

func TestJavaScriptRegexCancellationAndBudget(t *testing.T) {
	t.Parallel()
	compiled, err := testCompileJavaScriptRegex(`^(a+)+$`)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := compiled.MatchString(ctx, "a"); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if _, err := compiled.MatchString(context.Background(), "πTest"); err == nil {
		t.Fatal("accepted a name outside the ASCII Quint identifier domain")
	}
	for _, deadline := range []bool{false, true} {
		t.Run(map[bool]string{false: "budget", true: "context"}[deadline], func(t *testing.T) {
			t.Parallel()
			input := testCommandFile(t, "module M { val "+strings.Repeat("a", 32)+"Test=assert(false) }")
			ctx := context.Background()
			want := "execution budget"
			if deadline {
				var cancel context.CancelFunc
				ctx, cancel = context.WithTimeout(ctx, 10*time.Millisecond)
				defer cancel()
				want = "deadline exceeded"
			}
			start := time.Now()
			status, stdout, stderr := testCommandCall(t, ctx, input, `--match=^(a+)+b$`, "--seed=0")
			if status != 1 || stdout != "" || !strings.Contains(stderr, want) {
				t.Fatal("regex failure must abort discovery before tests run", status, stdout, stderr)
			}
			if elapsed := time.Since(start); elapsed > 5*time.Second {
				t.Fatalf("backtracking was not interrupted: %s", elapsed)
			}
		})
	}
}

func TestTestCommandJavaScriptRegexSelection(t *testing.T) {
	t.Parallel()
	data, err := os.ReadFile("testdata/test_regex.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixtures []struct {
		Name, Pattern string
		Cases         []struct {
			Input string
			Match bool
		}
	}
	if err := json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	names := []string{"a", "aa", "b", "aba", "abb", "abc", "Afoo", "foo", "Test", "fooTest"}
	var source strings.Builder
	source.WriteString("module M {")
	for _, name := range names {
		source.WriteString(" val " + name + "=true")
	}
	source.WriteByte('}')
	for _, fixture := range fixtures {
		if !slices.Contains([]string{"default_suffix", "pattern_001", "pattern_002", "pattern_009", "pattern_014", "pattern_020", "pattern_055", "pattern_058", "pattern_079", "pattern_080", "pattern_081", "pattern_083", "pattern_085", "pattern_086", "pattern_109", "pattern_165", "pattern_204", "pattern_210", "pattern_211", "pattern_265"}, fixture.Name) {
			continue
		}
		t.Run(fixture.Name, func(t *testing.T) {
			t.Parallel()
			input := testCommandFile(t, source.String())
			out := filepath.Join(t.TempDir(), "result.json")
			want := []string{}
			for _, name := range names {
				for _, test := range fixture.Cases {
					if test.Input == name && test.Match {
						want = append(want, name)
					}
				}
			}
			status, stdout, stderr := testCommandCall(t, context.Background(), input, "--match="+fixture.Pattern, "--out="+out, "--seed=0")
			data, err := os.ReadFile(out)
			var stage struct{ Passed, Failed, Ignored []string }
			if err != nil {
				t.Fatal(err)
			}
			if err = json.Unmarshal(data, &stage); err != nil {
				t.Fatal(err)
			}
			wantStatus := 0
			if len(want) == 0 {
				wantStatus = 1
			}
			if status != wantStatus || stdout != "" || stderr != "" || !slices.Equal(stage.Passed, want) || len(stage.Failed)+len(stage.Ignored) != 0 {
				t.Fatal(status, stdout, stderr, stage, want)
			}
		})
	}
}
