package cli

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/dlclark/regexp2"
)

type testRegexNullableFixture struct {
	Pattern string
	Cases   []struct {
		Input    string
		Match    bool
		Captures []*string
	}
}

func testRegexNullableFixtures(t *testing.T) []testRegexNullableFixture {
	t.Helper()
	data, err := os.ReadFile("testdata/test_regex_nullable.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixtures []testRegexNullableFixture
	if err := json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	return fixtures
}

func TestJavaScriptRegexNullableReference(t *testing.T) {
	t.Parallel()
	for _, fixture := range testRegexNullableFixtures(t) {
		compiled, err := testCompileJavaScriptRegex(fixture.Pattern)
		if err != nil {
			t.Fatal(fixture.Pattern, err)
		}
		for _, tc := range fixture.Cases {
			got, err := compiled.MatchString(context.Background(), tc.Input)
			if err != nil || got != tc.Match {
				t.Fatalf("/%s/ on %q=%v,%v; want %v", fixture.Pattern, tc.Input, got, err, tc.Match)
			}
			// Verify original capture slots as well as selection. A discarded
			// empty iteration must restore undefined rather than retain "".
			match, err := compiled.compiled.FindStringMatch(tc.Input)
			if err != nil {
				t.Fatal(fixture.Pattern, err)
			}
			if match == nil {
				continue
			}
			captures := make([]*string, len(tc.Captures))
			for n := range captures {
				group := match.GroupByNumber(n)
				if group != nil && len(group.Captures) > 0 {
					value := group.String()
					captures[n] = &value
				}
			}
			if !reflect.DeepEqual(captures, tc.Captures) {
				t.Fatalf("/%s/ on %q captures %v; want %v", fixture.Pattern, tc.Input, captures, tc.Captures)
			}
		}
	}
}

func TestTestCommandNullableCaptureSelection(t *testing.T) {
	t.Parallel()
	patterns := []string{`^(?:(?=(a)))?\1$`, `^(a?)+\1$`, `^(a?){2}\1$`, `^(?:(a)?(b)?){2}\1$`, `(?<=^(?:(a?))+?)\1$`, `^(?!((a)?(b)?)+?\1$)[ab]*$`}
	names := []string{"a", "aa", "b", "aba", "abba", "aaa", "bb"}
	var source strings.Builder
	source.WriteString("module M {")
	for _, name := range names {
		source.WriteString(" val " + name + "=true")
	}
	source.WriteByte('}')
	fixtures := testRegexNullableFixtures(t)
	for _, pattern := range patterns {
		t.Run(pattern, func(t *testing.T) {
			t.Parallel()
			var fixture *testRegexNullableFixture
			for i := range fixtures {
				if fixtures[i].Pattern == pattern {
					fixture = &fixtures[i]
					break
				}
			}
			if fixture == nil {
				t.Fatal("missing actual upstream fixture", pattern)
			}
			want := []string{}
			for _, name := range names {
				for _, tc := range fixture.Cases {
					if tc.Input == name && tc.Match {
						want = append(want, name)
					}
				}
			}
			input := testCommandFile(t, source.String())
			out := filepath.Join(t.TempDir(), "result.json")
			status, stdout, stderr := testCommandCall(t, context.Background(), input, "--match="+pattern, "--out="+out, "--seed=0")
			data, err := os.ReadFile(out)
			if err != nil {
				t.Fatal(err, status, stdout, stderr)
			}
			var stage struct{ Passed, Failed, Ignored []string }
			if err := json.Unmarshal(data, &stage); err != nil {
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

func TestJavaScriptRegexProgressBoundsAndLegacyEscapes(t *testing.T) {
	t.Parallel()
	for _, pattern := range []string{
		strings.Repeat("(", 18) + "a?" + strings.Repeat(")+", 18),
		"(" + strings.Repeat("a", 32720) + "|)+",
		strings.Repeat("a", 65520) + "()",
	} {
		if _, err := testCompileJavaScriptRegex(pattern); err == nil || !strings.Contains(err.Error(), "translation exceeds 65536 bytes") {
			t.Fatalf("expansion of %d source bytes must fail explicitly before native compilation: %v", len(pattern), err)
		}
	}
	for _, tc := range []struct{ source, want string }{
		{"1", `\x01`}, {"18", `\x018`}, {"1234", `\x534`},
		{"400", `\x200`}, {"777", `\x3f7`}, {"89", `\x389`}, {"9", `\x39`},
	} {
		if got := testRegexLegacyDecimal(tc.source); got != tc.want {
			t.Errorf("%s=%s; want %s", tc.source, got, tc.want)
		}
	}
	for _, tc := range []struct {
		source                      string
		present, repeated, optional bool
	}{
		{"", false, false, false}, {"a", false, false, false}, {"*", true, true, true}, {"+?", true, true, true}, {"?", true, false, true},
		{"{2}", true, true, false}, {"{0,1}?", true, false, true}, {"{2,}", true, true, true}, {"{00,2}", true, true, true},
		{"{", false, false, false}, {"{,2}", false, false, false}, {"{+1}", false, false, false}, {"{2,x}", false, false, false},
		{"{2,1}", false, false, false}, {"{999999999999999999999999999999}", false, false, false}, {"{0,99999999999999999999999999}", false, false, false},
	} {
		present, repeated, optional := testRegexQuantifier(tc.source)
		if present != tc.present || repeated != tc.repeated || optional != tc.optional {
			t.Errorf("%s=%v,%v,%v", tc.source, present, repeated, optional)
		}
	}
}

func TestJavaScriptRegexNativeFailureIsolation(t *testing.T) {
	t.Parallel()
	// This is deliberately the unadapted native form of a formerly crashing
	// valid JS pattern. The production translation now matches it correctly.
	for _, input := range []string{"", "a", "aa"} {
		native := regexp2.MustCompile(`^(?!(?:a?)+?)$`, regexp2.ECMAScript)
		native.MatchTimeout = testRegexMatchTimeout
		matcher := testJavaScriptRegex{compiled: native}
		matched, err := matcher.MatchString(context.Background(), input)
		if err != nil && strings.Contains(err.Error(), "unsupported --match") {
			if matched {
				t.Fatal("native failure was reported as a match")
			}
			return
		}
	}
	t.Fatal("raw native panic regression no longer exercises failure isolation")
}
