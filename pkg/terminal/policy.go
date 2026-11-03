// Package terminal supplies Quint's optional terminal presentation policy.
// It does not own, wrap, close, or write to the caller's output stream.
package terminal

import (
	"context"
	"io"
	"os"
	"runtime"
	"strings"

	"golang.org/x/term"
)

// Options overrides the terminal settings of one invocation. Width values above
// zero override the live stdout width. A nil ColorLevel selects automatic color;
// an explicit zero disables it, including when FORCE_COLOR is set.
type Options struct {
	Width      int
	ColorLevel *int
}

type optionsKey struct{}
type policyKey struct{}

// Policy caches color detection while querying the stdout width when needed.
// Policies are immutable and can be shared by concurrent evaluations.
type Policy struct {
	level int
	width func() int
}

// WithOptions gives a context caller-owned terminal overrides. The supplied
// color value is copied, so subsequent changes to its pointer have no effect.
func WithOptions(ctx context.Context, options Options) context.Context {
	if options.ColorLevel != nil {
		level := *options.ColorLevel
		options.ColorLevel = &level
	}
	ctx = context.WithValue(ctx, optionsKey{}, options)
	// A nested override must supersede an already bound parent's policy.
	return context.WithValue(ctx, policyKey{}, (*Policy)(nil))
}

// Bind captures automatic color settings for stdout and keeps its size query
// live. Bind is idempotent, including when stdout is subsequently wrapped by a
// writer that records write failures. Error output uses this same stdout policy.
func Bind(ctx context.Context, stdout io.Writer) context.Context {
	if policy, _ := ctx.Value(policyKey{}).(*Policy); policy != nil {
		return ctx
	}
	options, _ := ctx.Value(optionsKey{}).(Options)
	policy := newPolicy(options, stdout, hostProbe())
	return context.WithValue(ctx, policyKey{}, policy)
}

// PolicyFromContext returns a bound policy, or a plain policy with any explicit
// context overrides. Its nil-safe methods also make an absent policy plain.
func PolicyFromContext(ctx context.Context) *Policy {
	if policy, _ := ctx.Value(policyKey{}).(*Policy); policy != nil {
		return policy
	}
	options, _ := ctx.Value(optionsKey{}).(Options)
	policy := &Policy{}
	if options.ColorLevel != nil {
		policy.level = *options.ColorLevel
	}
	if options.Width > 0 {
		policy.width = func() int { return options.Width }
	}
	return policy
}

// Width reports stdout's current positive column count, or Quint's 80-column
// fallback. It intentionally does not inspect COLUMNS or the stderr stream.
func (policy *Policy) Width() int {
	if policy != nil && policy.width != nil {
		if width := policy.width(); width > 0 {
			return width
		}
	}
	return 80
}

// ColorLevel is the cached Chalk-compatible color capability (zero is plain).
func (policy *Policy) ColorLevel() int {
	if policy == nil {
		return 0
	}
	return policy.level
}

// Style applies Chalk-compatible named ANSI styles when color is enabled.
func (policy *Policy) Style(text string, styles ...string) string {
	return Style(text, policy.ColorLevel(), styles...)
}

type terminalProbe struct {
	env       map[string]string
	args      []string
	platform  string
	osRelease string
	isTTY     func(int) bool
	columns   func(int) int
}

func hostProbe() terminalProbe {
	env := make(map[string]string)
	for _, item := range os.Environ() {
		if key, value, found := strings.Cut(item, "="); found {
			env[key] = value
		}
	}
	platform := runtime.GOOS
	if platform == "windows" {
		platform = "win32"
	}
	return terminalProbe{
		env: env, args: os.Args[1:], platform: platform, osRelease: hostOSRelease(),
		isTTY: term.IsTerminal,
		columns: func(fd int) int {
			columns, _, err := term.GetSize(fd)
			if err != nil {
				return 0
			}
			return columns
		},
	}
}

func newPolicy(options Options, stdout io.Writer, probe terminalProbe) *Policy {
	policy := &Policy{}
	if output, ok := stdout.(interface{ Fd() uintptr }); ok {
		fd := int(output.Fd())
		policy.level = ColorLevel(ColorInput{Args: probe.args, Env: probe.env, Platform: probe.platform, OSRelease: probe.osRelease, IsTTY: probe.isTTY(fd)})
		policy.width = func() int { return probe.columns(fd) }
	} else {
		policy.level = ColorLevel(ColorInput{Args: probe.args, Env: probe.env, Platform: probe.platform, OSRelease: probe.osRelease})
	}
	if options.ColorLevel != nil {
		policy.level = *options.ColorLevel
	}
	if options.Width > 0 {
		policy.width = func() int { return options.Width }
	}
	return policy
}
