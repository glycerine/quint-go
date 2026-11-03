package cli

import (
	"context"
	"io"
	"strings"

	quint "github.com/glycerine/quint-go/pkg"
	"github.com/glycerine/quint-go/pkg/terminal"
)

// TerminalOptions provides explicit per-invocation presentation overrides.
// A nil ColorLevel detects stdout capabilities; an explicit zero disables color.
type TerminalOptions = terminal.Options

// WithTerminalOptions selects terminal presentation without changing command
// flags or taking ownership of the supplied writers.
func WithTerminalOptions(ctx context.Context, options TerminalOptions) context.Context {
	return terminal.WithOptions(ctx, options)
}

func terminalBind(ctx context.Context, stdout io.Writer) context.Context {
	return terminal.Bind(ctx, stdout)
}

func terminalWidth(ctx context.Context) int {
	return terminal.PolicyFromContext(ctx).Width()
}

func terminalStyle(ctx context.Context, text string, styles ...string) string {
	return terminal.PolicyFromContext(ctx).Style(text, styles...)
}

func terminalDiagnosticOptions(ctx context.Context, option quint.DiagnosticRenderOptions) quint.DiagnosticRenderOptions {
	option.Decorate = terminal.PolicyFromContext(ctx).Style
	return option
}

func terminalREPLHelp(ctx context.Context, text string) string {
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		if !strings.HasPrefix(line, ".") {
			continue
		}
		end := strings.IndexAny(line, "\t =[")
		if end < 0 {
			end = len(line)
		}
		lines[i] = terminalStyle(ctx, line[:end], "red") + strings.ReplaceAll(line[end:], "[module]", terminalStyle(ctx, "[module]", "gray"))
	}
	return strings.Join(lines, "\n")
}
