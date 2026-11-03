// Package terminal provides deterministic terminal presentation policies.
package terminal

import (
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

// ColorInput contains the process and stream facts used by supports-color 7.
// Args excludes the executable; Platform uses Node names such as "win32".
// Environment presence matters independently of an environment value's text.
type ColorInput struct {
	Args                []string
	Env                 map[string]string
	Platform, OSRelease string
	IsTTY               bool
}

var (
	term256Pattern       = regexp.MustCompile(`(?i)-256(color)?$`)
	termColorPattern     = regexp.MustCompile(`(?i)^(screen|xterm|vt100|vt220|rxvt)|color|ansi|cygwin|linux`)
	teamcityColorPattern = regexp.MustCompile(`^(9\.(0*[1-9]\d*)\.|\d{2,}\.)`)
)

// ColorLevel follows supports-color 7.2.0's stream policy. In particular,
// FORCE_COLOR overrides ordinary color flags, flags stop at --, and NO_COLOR
// has no special meaning in this version of the upstream dependency.
func ColorLevel(input ColorInput) int {
	hasFlag := func(name string) bool {
		for _, arg := range input.Args {
			if arg == "--" {
				break
			}
			if arg == "--"+name {
				return true
			}
		}
		return false
	}
	force, defined, valid := 0, false, true
	if hasFlag("no-color") || hasFlag("no-colors") || hasFlag("color=false") || hasFlag("color=never") {
		defined = true
	} else if hasFlag("color") || hasFlag("colors") || hasFlag("color=true") || hasFlag("color=always") {
		force, defined = 1, true
	}
	if value, present := input.Env["FORCE_COLOR"]; present {
		defined = true
		switch value {
		case "true", "":
			force, valid = 1, true
		case "false":
			force, valid = 0, true
		default:
			force, valid = colorParseInt(value)
			if force > 3 {
				force = 3
			}
		}
	}
	if defined && valid && force == 0 {
		return 0
	}
	if hasFlag("color=16m") || hasFlag("color=full") || hasFlag("color=truecolor") {
		return 3
	}
	if hasFlag("color=256") {
		return 2
	}
	if !input.IsTTY && !defined {
		return 0
	}
	minimum := 0
	if valid {
		minimum = force
	}
	if input.Env["TERM"] == "dumb" {
		return minimum
	}
	if input.Platform == "win32" {
		parts := strings.Split(input.OSRelease, ".")
		if len(parts) >= 3 && colorNumber(parts[0]) >= 10 && colorNumber(parts[2]) >= 10586 {
			if colorNumber(parts[2]) >= 14931 {
				return 3
			}
			return 2
		}
		return 1
	}
	if _, present := input.Env["CI"]; present {
		for _, name := range []string{"TRAVIS", "CIRCLECI", "APPVEYOR", "GITLAB_CI", "GITHUB_ACTIONS", "BUILDKITE"} {
			if _, present := input.Env[name]; present {
				return 1
			}
		}
		if input.Env["CI_NAME"] == "codeship" {
			return 1
		}
		return minimum
	}
	if version, present := input.Env["TEAMCITY_VERSION"]; present {
		if teamcityColorPattern.MatchString(version) {
			return 1
		}
		return 0
	}
	if input.Env["COLORTERM"] == "truecolor" {
		return 3
	}
	switch input.Env["TERM_PROGRAM"] {
	case "iTerm.app":
		version, valid := colorParseInt(strings.Split(input.Env["TERM_PROGRAM_VERSION"], ".")[0])
		if valid && version >= 3 {
			return 3
		}
		return 2
	case "Apple_Terminal":
		return 2
	}
	if term256Pattern.MatchString(input.Env["TERM"]) {
		return 2
	}
	if termColorPattern.MatchString(input.Env["TERM"]) {
		return 1
	}
	if _, present := input.Env["COLORTERM"]; present {
		return 1
	}
	return minimum
}

// JavaScript parseInt(_, 10) accepts decimal prefixes and leading whitespace.
// A failed parse remains distinguishable from zero for FORCE_COLOR precedence.
func colorParseInt(text string) (int, bool) {
	text = strings.TrimLeftFunc(text, func(r rune) bool { return unicode.IsSpace(r) || r == '\uFEFF' })
	end := 0
	if len(text) > 0 && (text[0] == '+' || text[0] == '-') {
		end++
	}
	start := end
	for end < len(text) && text[end] >= '0' && text[end] <= '9' {
		end++
	}
	if start == end {
		return 0, false
	}
	value, _ := strconv.ParseInt(text[:end], 10, strconv.IntSize)
	// ParseInt returns the appropriate signed bound on overflow. The positive
	// bound is subsequently clamped to three by the color policy.
	return int(value), true
}

func colorNumber(text string) float64 {
	value, _ := strconv.ParseFloat(strings.TrimSpace(text), 64)
	return value
}

type ansiStyle struct{ open, close string }

// Style applies Chalk 4's static named-style chains. Unknown names are ignored;
// RGB/hex conversion and tagged templates are outside this named-style API.
func Style(text string, level int, styles ...string) string {
	visible := false
	chain := make([]ansiStyle, 0, len(styles))
	open, close := "", ""
	for _, name := range styles {
		if name == "visible" {
			visible = true
			continue
		}
		style, ok := namedANSIStyle(name)
		if !ok {
			continue
		}
		chain = append(chain, style)
		open += style.open
		close = style.close + close
	}
	if level <= 0 || text == "" {
		if visible {
			return ""
		}
		return text
	}
	if len(chain) == 0 {
		return text
	}
	if strings.ContainsRune(text, '\x1b') {
		for i := len(chain) - 1; i >= 0; i-- {
			style := chain[i]
			text = strings.ReplaceAll(text, style.close, style.close+style.open)
		}
	}
	if strings.ContainsRune(text, '\n') {
		var lines strings.Builder
		start := 0
		for i := 0; i < len(text); i++ {
			if text[i] != '\n' {
				continue
			}
			end := i
			crlf := i > start && text[i-1] == '\r'
			if crlf {
				end--
			}
			lines.WriteString(text[start:end])
			lines.WriteString(close)
			if crlf {
				lines.WriteByte('\r')
			}
			lines.WriteByte('\n')
			lines.WriteString(open)
			start = i + 1
		}
		lines.WriteString(text[start:])
		text = lines.String()
	}
	return open + text + close
}

func namedANSIStyle(name string) (ansiStyle, bool) {
	var foreground, background, modifier int
	foreground, background, modifier = -1, -1, -1
	switch name {
	case "black", "red", "green", "yellow", "blue", "magenta", "cyan", "white":
		foreground = styleColorOffset(name)
	case "blackBright", "redBright", "greenBright", "yellowBright", "blueBright", "magentaBright", "cyanBright", "whiteBright":
		foreground = 60 + styleColorOffset(strings.TrimSuffix(name, "Bright"))
	case "gray", "grey":
		foreground = 60
	case "bgBlack", "bgRed", "bgGreen", "bgYellow", "bgBlue", "bgMagenta", "bgCyan", "bgWhite":
		background = styleColorOffset(strings.ToLower(name[2:3]) + name[3:])
	case "bgBlackBright", "bgRedBright", "bgGreenBright", "bgYellowBright", "bgBlueBright", "bgMagentaBright", "bgCyanBright", "bgWhiteBright":
		background = 60 + styleColorOffset(strings.TrimSuffix(strings.ToLower(name[2:3])+name[3:], "Bright"))
	case "bgGray", "bgGrey":
		background = 60
	case "reset":
		modifier = 0
	case "bold":
		modifier = 1
	case "dim":
		modifier = 2
	case "italic":
		modifier = 3
	case "underline":
		modifier = 4
	case "inverse":
		modifier = 7
	case "hidden":
		modifier = 8
	case "strikethrough":
		modifier = 9
	default:
		return ansiStyle{}, false
	}
	if foreground >= 0 {
		return ansiStyle{open: ansiCode(30 + foreground), close: ansiCode(39)}, true
	}
	if background >= 0 {
		return ansiStyle{open: ansiCode(40 + background), close: ansiCode(49)}, true
	}
	closing := modifier + 20
	if modifier == 0 {
		closing = 0
	} else if modifier == 1 {
		closing = 22
	}
	return ansiStyle{open: ansiCode(modifier), close: ansiCode(closing)}, true
}

func styleColorOffset(name string) int {
	switch name {
	case "black":
		return 0
	case "red":
		return 1
	case "green":
		return 2
	case "yellow":
		return 3
	case "blue":
		return 4
	case "magenta":
		return 5
	case "cyan":
		return 6
	default:
		return 7
	}
}

func ansiCode(code int) string { return "\x1b[" + strconv.Itoa(code) + "m" }
