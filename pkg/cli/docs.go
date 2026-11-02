package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/glycerine/insdict"
	quint "github.com/glycerine/quint-go/pkg"
)

const docsHelp = "Usage: quint docs <input> [--out <file>]\n\nProduces Markdown documentation from docstrings in a Quint specification.\n\nOptions:\n  --out <file>  write the documentation stage as JSON\n  --help        show this help\n"

type docsOptions struct{ input, out string }

// docsNamedEntries emits an insertion-ordered object rather than serializing
// insdict's implementation details or depending on Go map iteration order.
type docsNamedEntries struct {
	entries *insdict.Dict[string, quint.DocumentationEntry]
}

func (named docsNamedEntries) MarshalJSON() ([]byte, error) {
	var out bytes.Buffer
	out.WriteByte('{')
	comma := false
	for name, entry := range named.entries.All() {
		if comma {
			out.WriteByte(',')
		}
		comma = true
		key, _ := json.Marshal(name)
		value, err := json.Marshal(entry)
		if err != nil {
			return nil, err
		}
		out.Write(key)
		out.WriteByte(':')
		out.Write(value)
	}
	out.WriteByte('}')
	return out.Bytes(), nil
}

type docsModules struct {
	entries *insdict.Dict[string, docsNamedEntries]
}

func (modules docsModules) MarshalJSON() ([]byte, error) {
	var out bytes.Buffer
	out.WriteByte('{')
	comma := false
	for name, entries := range modules.entries.All() {
		if comma {
			out.WriteByte(',')
		}
		comma = true
		key, _ := json.Marshal(name)
		value, err := json.Marshal(entries)
		if err != nil {
			return nil, err
		}
		out.Write(key)
		out.WriteByte(':')
		out.Write(value)
	}
	out.WriteByte('}')
	return out.Bytes(), nil
}

type docsStage struct {
	Stage         string                `json:"stage"`
	Warnings      *[]any                `json:"warnings,omitempty"`
	Errors        *[]quint.ErrorMessage `json:"errors,omitempty"`
	Documentation *docsModules          `json:"documentation,omitempty"`
}

func commandDocs(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	options, help, err := docsParseOptions(args)
	if help {
		if _, err = io.WriteString(stdout, docsHelp); err != nil {
			fmt.Fprintln(stderr, "error:", err)
			return 1
		}
		return 0
	}
	if err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return 1
	}
	stage := docsStage{Stage: "loading"}
	fail := func(message string, diagnostics []quint.ErrorMessage) int {
		stage.Errors = &diagnostics
		if options.out != "" {
			if err := docsWriteStage(options.out, stage); err != nil {
				fmt.Fprintln(stderr, "error:", err)
			}
		} else {
			for _, diagnostic := range diagnostics {
				fmt.Fprintln(stderr, "error:", diagnostic.Explanation)
			}
			fmt.Fprintln(stderr, "error:", message)
		}
		return 1
	}
	if err = ctx.Err(); err != nil {
		return fail(err.Error(), []quint.ErrorMessage{})
	}
	path, err := filepath.Abs(options.input)
	if err != nil {
		return fail(err.Error(), []quint.ErrorMessage{})
	}
	data, err := os.ReadFile(path)
	if err != nil {
		message := fmt.Sprintf("file %s could not be opened due to %v", options.input, err)
		if errors.Is(err, os.ErrNotExist) {
			message = fmt.Sprintf("file %s does not exist", options.input)
		}
		return fail(message, []quint.ErrorMessage{})
	}
	warnings := []any{}
	stage.Stage, stage.Warnings = "documentation", &warnings
	parsed, parseErr := quint.ParseModulesWithLocations(path, string(data))
	documentation := docsModules{entries: insdict.NewDict[string, docsNamedEntries]()}
	for _, module := range parsed.Modules {
		if err = ctx.Err(); err != nil {
			return fail(err.Error(), []quint.ErrorMessage{})
		}
		entries := quint.ProduceDocs(module)
		parts := []string{}
		for _, entry := range entries.All() {
			parts = append(parts, quint.DocumentationMarkdown(entry))
		}
		// Upstream's documentation stage prints Markdown independently of
		// --out, which additionally writes the machine-readable stage.
		if _, err = fmt.Fprintf(stdout, "# Documentation for %s\n\n%s\n", module.Name, strings.Join(parts, "\n\n")); err != nil {
			return fail(err.Error(), []quint.ErrorMessage{})
		}
		documentation.entries.Set(module.Name, docsNamedEntries{entries: entries})
	}
	if parseErr != nil {
		return fail("parsing failed", docsParseDiagnostics(parseErr, parsed.Sources, path))
	}
	if err = ctx.Err(); err != nil {
		return fail(err.Error(), []quint.ErrorMessage{})
	}
	stage.Documentation = &documentation
	if options.out != "" {
		if err = docsWriteStage(options.out, stage); err != nil {
			fmt.Fprintln(stderr, "error:", err)
			return 1
		}
	}
	return 0
}

func docsParseOptions(args []string) (docsOptions, bool, error) {
	var options docsOptions
	seenOut, positionalOnly := false, false
	for i := 0; i < len(args); i++ {
		argument := args[i]
		if !positionalOnly {
			switch {
			case argument == "--help" || argument == "-h":
				return options, true, nil
			case argument == "--":
				positionalOnly = true
				continue
			case argument == "--out" || strings.HasPrefix(argument, "--out="):
				if seenOut {
					return options, false, fmt.Errorf("--out can not be specified more than once")
				}
				seenOut = true
				if strings.HasPrefix(argument, "--out=") {
					options.out = strings.TrimPrefix(argument, "--out=")
				} else {
					i++
					if i == len(args) || strings.HasPrefix(args[i], "--") {
						return options, false, fmt.Errorf("--out requires a filename")
					}
					options.out = args[i]
				}
				if options.out == "" {
					return options, false, fmt.Errorf("--out requires a filename")
				}
				continue
			case strings.HasPrefix(argument, "-"):
				return options, false, fmt.Errorf("unknown option %s", argument)
			}
		}
		if options.input != "" {
			return options, false, fmt.Errorf("unexpected argument %s", argument)
		}
		options.input = argument
	}
	if options.input == "" {
		return options, false, fmt.Errorf("docs requires an input file")
	}
	return options, false, nil
}

func docsWriteStage(path string, stage docsStage) error {
	encoded, err := json.Marshal(stage)
	if err != nil {
		return err
	}
	return os.WriteFile(path, encoded, 0o666)
}

func docsParseDiagnostics(err error, sources quint.SourceMap, path string) []quint.ErrorMessage {
	var syntax *quint.SyntaxErrors
	if errors.As(err, &syntax) {
		diagnostics := make([]quint.ErrorMessage, 0, len(syntax.Issues))
		for _, issue := range syntax.Issues {
			message := issue.Message
			if !strings.HasPrefix(message, "[QNT") {
				code := issue.Code
				if code == "" {
					code = "QNT000"
				}
				message = "[" + code + "] " + message
			}
			diagnostics = append(diagnostics, quint.ErrorMessage{Explanation: message, Locations: []quint.Location{{Source: path, Start: issue.Position}}})
		}
		return diagnostics
	}
	var diagnostic quint.Diagnostic
	if errors.As(err, &diagnostic) {
		return []quint.ErrorMessage{quint.ResolveDiagnostic(sources, diagnostic)}
	}
	return []quint.ErrorMessage{{Explanation: err.Error(), Locations: []quint.Location{}}}
}
