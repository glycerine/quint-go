package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"

	quint "github.com/glycerine/quint-go/pkg"
)

// REPLLoopOptions controls the stream-based REPL. A zero Verbosity suppresses
// banners and prompts; expression results and diagnostics are still printed.
// The input reader belongs to the caller and is never closed by RunREPL.
type REPLLoopOptions struct {
	Quiet     bool
	Verbosity int
	Require   string
	Main      string
	Commands  []string
	Seed      *quint.Integer
}

func commandREPL(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	options, help, err := replParseOptions(args)
	if err != nil {
		return cliPrintArgumentError(stderr, err)
	}
	if help != "" {
		return cliPrintHelp(stdout, stderr, "repl")
	}
	return runREPLCommandInput(ctx, stdout, stderr, options)
}

var replVerbosity = regexp.MustCompile(`^\.verbosity\s*=?\s*([0-5])$`)
var replSeed = regexp.MustCompile(`^\.seed\s*=?\s*(0x[0-9a-f]+|[0-9]*)$`)

// runREPL owns one sequential session; each host controls how input arrives.
func runREPL(ctx context.Context, next replNextInput, stdout, stderr io.Writer, options REPLLoopOptions) int {
	if next == nil || stdout == nil || stderr == nil {
		return 2
	}
	if err := ctx.Err(); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if options.Verbosity < 0 || options.Verbosity > 5 {
		fmt.Fprintln(stderr, "repl: verbosity must be between 0 and 5")
		return 2
	}
	level := options.Verbosity
	if options.Quiet {
		level = 0
	}
	session := quint.NewREPLSession()
	if options.Seed != nil {
		session = quint.NewREPLSessionWithSeed(*options.Seed)
	} else {
		random, err := quint.NewRandomRNG()
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		session.SetSeed(random.State())
	}
	var writeErr error
	ctx = terminalBind(ctx, stdout)
	stdout = &replOutputWriter{writer: stdout, err: &writeErr}
	ctx = quint.WithRuntimeHooks(ctx, quint.RuntimeHooks{DebugWriter: stdout})
	out := func(text string) {
		if writeErr == nil {
			_, writeErr = io.WriteString(stdout, text)
		}
	}
	report := func(err error) {
		if writeErr == nil {
			_, writeErr = io.WriteString(stderr, terminalStyle(ctx, session.RenderErrorContext(ctx, err)+"\n", "red"))
		}
	}
	if level > 0 {
		out(terminalStyle(ctx, "Quint Go REPL", "gray") + terminalStyle(ctx, "\nType \".exit\" to exit, or \".help\" for more information\n", "gray"))
	}
	pending := ""
	braces, parens, comments := 0, 0, 0
	recycling := false
	renderResult := func(result quint.REPLResult) error {
		switch result.Kind {
		case "declaration":
			out("\n")
		case "value":
			formatted, err := session.FormatValueContext(ctx, result.Value, terminalWidth(ctx))
			if err != nil {
				return err
			}
			out(formatted + "\n")
			if err := replPrintStateDiff(ctx, stdout, result, level); err != nil {
				return err
			}
			if len(result.Missing) > 0 {
				out(terminalStyle(ctx, "[warning] some variables are undefined: "+strings.Join(result.Missing, ", ")+"\n", "yellow"))
			}
		}
		return writeErr
	}
	evaluate := func(text string) {
		evaluationContext, recorder, err := executionREPLContext(ctx, session, level)
		if err != nil {
			report(err)
			return
		}
		result, evaluationError := session.Evaluate(evaluationContext, text)
		if evaluationError == nil {
			if err := renderResult(result); err != nil {
				report(err)
				return
			}
		}
		if err := executionPrintREPLFrames(ctx, stdout, session, recorder); err != nil {
			report(err)
			return
		}
		if evaluationError != nil {
			report(evaluationError)
		}
	}
	replay := func(filename, main string, reload bool) error {
		replayContext, hooks, err := executionREPLReplay(ctx, session, func() int { return level }, out, stdout, report, renderResult, &writeErr)
		if err != nil {
			return err
		}
		if reload {
			return session.ReloadWithReplay(replayContext, hooks)
		}
		return session.LoadWithReplay(replayContext, filename, main, hooks)
	}
	if options.Require != "" {
		file, main := options.Require, options.Main
		if i := strings.LastIndex(file, "::"); i >= 0 {
			main, file = file[i+2:], file[:i]
		}
		if err := replay(file, main, false); err != nil {
			report(err)
			return 1
		}
	}
	// A command returns true only when the caller explicitly exits the loop.
	consume := func(line string) bool {
		if strings.HasPrefix(line, ".") && !strings.HasPrefix(line, "... ") {
			fields := strings.Fields(line)
			if len(fields) == 0 {
				return false
			}
			command := fields[0]
			if strings.HasPrefix(command, ".verbosity") {
				match := replVerbosity.FindStringSubmatch(line)
				if match == nil {
					report(errors.New(".verbosity requires a level from 0 to 5"))
				} else {
					level = int(match[1][0] - '0')
					if level > 0 {
						out(terminalStyle(ctx, ".verbosity="+match[1]+"\n", "gray"))
					}
				}
				return false
			}
			if strings.HasPrefix(command, ".seed") {
				match := replSeed.FindStringSubmatch(line)
				if match == nil {
					report(errors.New(".seed requires an integer, or no argument"))
				} else if match[1] == "" {
					out(terminalStyle(ctx, ".seed="+session.Seed().String()+"\n", "gray"))
				} else {
					seed, err := quint.ParseSeed(match[1])
					if err != nil {
						report(err)
					} else {
						session.SetSeed(seed)
						if level > 0 {
							out(terminalStyle(ctx, ".seed="+seed.String()+"\n", "gray"))
						}
					}
				}
				return false
			}
			switch command {
			case ".exit", ".quit":
				pending = ""
				return true
			case ".help":
				out(terminalREPLHelp(ctx, ".clear\tClear definitions and expression history\n.exit\tExit the REPL\n.help\tPrint this help message\n.load <filename> [module]\tLoad a file and optionally import a module\n.reload\tReload the last loaded file\n.save <filename>\tSave definitions and expression history\n.state\tPrint current variable values\n.history\tPrint accepted input history\n.verbosity=[0-5]\tSet output level\n.seed[=<number>]\tSet or get the random seed\n\nFinish multiline input with an empty line.\n"))
			case ".clear":
				session.Reset()
				pending, braces, parens, comments, recycling = "", 0, 0, 0, false
				out("\n")
			case ".load":
				if len(fields) < 2 || len(fields) > 3 {
					report(errors.New(".load requires a filename and optional module name"))
				} else {
					main := ""
					if len(fields) == 3 {
						main = fields[2]
					}
					if err := replay(fields[1], main, false); err != nil {
						report(err)
					}
				}
			case ".reload":
				if err := replay("", "", true); err != nil {
					report(err)
				}
			case ".state":
				state, err := session.State()
				if err != nil {
					report(err)
				} else {
					formatted, err := session.FormatValueContext(ctx, state, terminalWidth(ctx))
					if err != nil {
						report(err)
					} else {
						out(formatted + "\n")
					}
				}
			case ".history":
				for _, text := range session.History() {
					out(text + "\n")
				}
			case ".save":
				replSaveCommand(ctx, session, fields, out)
			default:
				report(fmt.Errorf("Unexpected command: %s\nType .help to see the list of commands", line))
			}
			return false
		}
		openBraces, openParens, openComments := replCountBraces(line)
		braces, parens, comments = braces+openBraces, parens+openParens, comments+openComments
		if pending == "" {
			recycling = strings.HasPrefix(strings.TrimSpace(line), ">>> ")
			if braces > 0 || parens > 0 || comments > 0 || recycling {
				pending = replTrimDecorations(line)
			} else if strings.TrimSpace(line) != "" {
				evaluate(line + "\n")
			}
			return false
		}
		trimmed := strings.TrimSpace(line)
		continuingPaste := strings.HasPrefix(trimmed, "... ")
		if (trimmed == "" && braces <= 0 && parens <= 0 && comments <= 0) || (recycling && !continuingPaste) {
			evaluate(pending)
			pending, recycling = "", false
		} else {
			pending += "\n" + replTrimDecorations(trimmed)
		}
		return false
	}
	for _, command := range options.Commands {
		if writeErr != nil {
			return 1
		}
		if err := ctx.Err(); err != nil {
			report(err)
			return 1
		}
		if consume(command) {
			if writeErr != nil {
				return 1
			}
			return 0
		}
	}
	for {
		if writeErr != nil {
			return 1
		}
		if err := ctx.Err(); err != nil {
			report(err)
			return 1
		}
		if level > 0 {
			if pending != "" {
				out("... ")
			} else {
				out(">>> ")
			}
		}
		if writeErr != nil {
			return 1
		}
		event, err := next(ctx)
		if err != nil {
			report(err)
			return 1
		}
		if event.Kind == REPLInputEOF {
			break
		}
		if event.Kind == REPLInputInterrupt {
			pending, braces, parens, comments, recycling = "", 0, 0, 0, false
			out(terminalStyle(ctx, " <cancelled>\n", "yellow"))
			continue
		}
		if consume(event.Line) {
			break
		}
	}
	if err := ctx.Err(); err != nil {
		report(err)
		return 1
	}
	if pending != "" {
		report(errors.New("Unexpected end of input in multiline expression; finish it with an empty line"))
		return 1
	}
	if writeErr != nil {
		return 1
	}
	return 0
}

// Runtime debug output shares the REPL stream's failure state, including short
// writes. A failed debug write stops the loop before any later evaluation.
type replOutputWriter struct {
	writer io.Writer
	err    *error
}

func (w *replOutputWriter) Write(data []byte) (int, error) {
	if *w.err != nil {
		return 0, *w.err
	}
	n, err := w.writer.Write(data)
	if err == nil && n < len(data) {
		err = io.ErrShortWrite
	}
	*w.err = err
	return n, err
}

func replTrimDecorations(line string) string {
	trimmed := strings.TrimLeft(line, " \t")
	if strings.HasPrefix(trimmed, ">>> ") || strings.HasPrefix(trimmed, "... ") {
		return trimmed[4:]
	}
	return line
}

// Keep the TypeScript REPL's line-based delimiter counting and blank-line
// completion rule, including its intentionally simple treatment of strings.
func replCountBraces(line string) (braces, parens, comments int) {
	for i := 0; i < len(line); i++ {
		switch line[i] {
		case '{':
			braces++
		case '}':
			braces--
		case '(':
			parens++
		case ')':
			parens--
		case '/':
			if i+1 < len(line) && line[i+1] == '*' {
				comments++
				i++
			}
		case '*':
			if i+1 < len(line) && line[i+1] == '/' {
				comments--
				i++
			}
		}
	}
	return
}
