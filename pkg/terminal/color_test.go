package terminal

import (
	"encoding/json"
	"os"
	"reflect"
	"strconv"
	"testing"
)

func TestUpstreamTerminalColorPolicyAndStyles(t *testing.T) {
	t.Parallel()
	var fixtures struct {
		Policy []struct {
			Name, Platform, OSRelease string
			Env                       map[string]string
			Args                      []string
			StdoutTTY, StderrTTY      bool
			StdoutLevel, StderrLevel  int
		}
		Styles []struct {
			Name, Text, Output string
			Level              int
			Style              []string
		}
	}
	data, err := os.ReadFile("../testdata/parser/terminal_policy.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	if len(fixtures.Policy) != 42 || len(fixtures.Styles) != 68 {
		t.Fatal("unexpected saved fixture counts", len(fixtures.Policy), len(fixtures.Styles))
	}
	for _, fixture := range fixtures.Policy {
		t.Run(fixture.Name, func(t *testing.T) {
			input := ColorInput{Args: fixture.Args, Env: fixture.Env, Platform: fixture.Platform, OSRelease: fixture.OSRelease, IsTTY: fixture.StdoutTTY}
			before, _ := json.Marshal(input)
			if got := ColorLevel(input); got != fixture.StdoutLevel {
				t.Fatal("stdout", got, "expected", fixture.StdoutLevel)
			}
			input.IsTTY = fixture.StderrTTY
			if got := ColorLevel(input); got != fixture.StderrLevel {
				t.Fatal("stderr", got, "expected", fixture.StderrLevel)
			}
			input.IsTTY = fixture.StdoutTTY
			after, _ := json.Marshal(input)
			if !reflect.DeepEqual(before, after) {
				t.Fatal("policy mutated caller-owned arguments or environment")
			}
		})
	}
	for _, fixture := range fixtures.Styles {
		t.Run(fixture.Name+"/"+strconv.Itoa(fixture.Level), func(t *testing.T) {
			if got := Style(fixture.Text, fixture.Level, fixture.Style...); got != fixture.Output {
				t.Fatalf("Go %q; Chalk %q", got, fixture.Output)
			}
		})
	}
}

func TestColorPolicyFlagAndEnvironmentBoundaries(t *testing.T) {
	t.Parallel()
	for _, example := range []struct {
		name  string
		input ColorInput
		want  int
	}{
		{"colors-alias", ColorInput{Args: []string{"--colors"}}, 1},
		{"true-alias", ColorInput{Args: []string{"--color=true"}}, 1},
		{"always-alias", ColorInput{Args: []string{"--color=always"}}, 1},
		{"no-colors-alias", ColorInput{Args: []string{"--no-colors", "--color"}, IsTTY: true}, 0},
		{"false-alias", ColorInput{Args: []string{"--color=false", "--color=full"}, IsTTY: true}, 0},
		{"never-alias", ColorInput{Args: []string{"--color=never", "--color"}, IsTTY: true}, 0},
		{"16m-alias", ColorInput{Args: []string{"--color=16m"}}, 3},
		{"disabled-after-terminator", ColorInput{Args: []string{"--color", "--", "--no-color"}}, 1},
		{"force-decimal-prefix", ColorInput{Env: map[string]string{"FORCE_COLOR": "\uFEFF  +2suffix"}}, 2},
		{"force-clamped-positive-overflow", ColorInput{Env: map[string]string{"FORCE_COLOR": "9999999999999999999999999999999"}}, 3},
		{"force-hex-prefix-is-zero", ColorInput{Env: map[string]string{"FORCE_COLOR": "0x2", "TERM": "xterm"}, IsTTY: true}, 0},
		{"force-negative-preserved", ColorInput{Env: map[string]string{"FORCE_COLOR": "-1"}}, -1},
		{"force-nan-bypasses-stream-guard", ColorInput{Env: map[string]string{"FORCE_COLOR": "NaN", "TERM": "xterm"}}, 1},
		{"force-nan-overrides-disable", ColorInput{Args: []string{"--no-color"}, Env: map[string]string{"FORCE_COLOR": "NaN", "COLORTERM": "truecolor"}}, 3},
		{"term-case-insensitive", ColorInput{Env: map[string]string{"TERM": "XTERM-256COLOR"}, IsTTY: true}, 2},
		{"term-color-substring", ColorInput{Env: map[string]string{"TERM": "my-color-terminal"}, IsTTY: true}, 1},
		{"teamcity-leading-zero-positive", ColorInput{Env: map[string]string{"TEAMCITY_VERSION": "9.001.0"}, IsTTY: true}, 1},
		{"teamcity-leading-zero-zero", ColorInput{Env: map[string]string{"TEAMCITY_VERSION": "9.000.0"}, IsTTY: true}, 0},
		{"teamcity-major-ten", ColorInput{Env: map[string]string{"TEAMCITY_VERSION": "10.0.0"}, IsTTY: true}, 1},
		{"iterm-missing-version", ColorInput{Env: map[string]string{"TERM_PROGRAM": "iTerm.app"}, IsTTY: true}, 2},
		{"windows-missing-build", ColorInput{Platform: "win32", OSRelease: "10", IsTTY: true}, 1},
		{"windows-newer-major", ColorInput{Platform: "win32", OSRelease: "11.0.14931", IsTTY: true}, 3},
	} {
		t.Run(example.name, func(t *testing.T) {
			if got := ColorLevel(example.input); got != example.want {
				t.Fatal(got, "expected", example.want)
			}
		})
	}
}

func TestNamedStyleChainsAndModifiers(t *testing.T) {
	t.Parallel()
	for _, example := range []struct {
		name        string
		open, close int
	}{
		{"reset", 0, 0}, {"bold", 1, 22}, {"dim", 2, 22}, {"italic", 3, 23}, {"underline", 4, 24},
		{"inverse", 7, 27}, {"hidden", 8, 28}, {"strikethrough", 9, 29},
		{"black", 30, 39}, {"cyan", 36, 39}, {"white", 37, 39}, {"grey", 90, 39},
		{"blackBright", 90, 39}, {"redBright", 91, 39}, {"greenBright", 92, 39}, {"yellowBright", 93, 39},
		{"blueBright", 94, 39}, {"magentaBright", 95, 39}, {"cyanBright", 96, 39}, {"whiteBright", 97, 39},
		{"bgBlack", 40, 49}, {"bgRed", 41, 49}, {"bgGreen", 42, 49}, {"bgYellow", 43, 49},
		{"bgBlue", 44, 49}, {"bgMagenta", 45, 49}, {"bgCyan", 46, 49}, {"bgWhite", 47, 49},
		{"bgBlackBright", 100, 49}, {"bgRedBright", 101, 49}, {"bgGreenBright", 102, 49}, {"bgYellowBright", 103, 49},
		{"bgBlueBright", 104, 49}, {"bgMagentaBright", 105, 49}, {"bgCyanBright", 106, 49}, {"bgWhiteBright", 107, 49},
		{"bgGray", 100, 49}, {"bgGrey", 100, 49},
	} {
		if got, want := Style("x", 1, example.name), ansiCode(example.open)+"x"+ansiCode(example.close); got != want {
			t.Fatalf("%s: %q, expected %q", example.name, got, want)
		}
	}
	if got := Style("\x1b[22mtext\x1b[24m", 1, "dim", "underline"); got != "\x1b[2m\x1b[4m\x1b[22m\x1b[2mtext\x1b[24m\x1b[4m\x1b[24m\x1b[22m" {
		t.Fatalf("nested modifier reset: %q", got)
	}
	if got := Style("text", 0, "red", "visible"); got != "" {
		t.Fatal("disabled visible chain", got)
	}
	if got := Style("text", 1, "visible"); got != "text" {
		t.Fatal("visible-only chain", got)
	}
	if got := Style("text", 1, "unknown"); got != "text" {
		t.Fatal("unknown style changed text", got)
	}
	if got := Style("text", 1); got != "text" {
		t.Fatal("empty chain changed text", got)
	}
	if got := Style("left\rright", 1, "red"); got != "\x1b[31mleft\rright\x1b[39m" {
		t.Fatalf("standalone carriage return changed: %q", got)
	}
}
