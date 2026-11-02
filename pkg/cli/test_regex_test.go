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
	unsupported := map[string]bool{
		"pattern_045": true, // JavaScript nested character class, not subtraction.
		"pattern_079": true, "pattern_080": true, "pattern_081": true,
		"pattern_082": true, "pattern_083": true, "pattern_085": true,
		"pattern_086": true, "pattern_109": true, // Non-ASCII/escaped capture names.
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
			if fixture.Invalid || unsupported[fixture.Name] {
				if err == nil {
					t.Fatalf("accepted invalid/unsupported JavaScript expression %q", fixture.Pattern)
				}
				if !fixture.Invalid && !strings.Contains(err.Error(), "unsupported --match") {
					t.Fatalf("unsupported JavaScript form needs an explicit limit: %v", err)
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
			if elapsed := time.Since(start); elapsed > time.Second {
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
	names := []string{"a", "b", "aba", "abb", "abc", "Afoo", "foo", "Test", "fooTest"}
	var source strings.Builder
	source.WriteString("module M {")
	for _, name := range names {
		source.WriteString(" val " + name + "=true")
	}
	source.WriteByte('}')
	for _, fixture := range fixtures {
		if !slices.Contains([]string{"default_suffix", "pattern_001", "pattern_002", "pattern_009", "pattern_014", "pattern_020", "pattern_055", "pattern_058"}, fixture.Name) {
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
