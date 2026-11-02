package quint

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"
	"unicode"
)

const replEmptyModuleText = "module __repl__ {\n  \n}"

type replSessionArchive struct {
	text        string
	close       int
	expressions []string
	pending     []string
	ready       bool
}

func (archive *replSessionArchive) ensure() {
	if !archive.ready {
		archive.text, archive.close, archive.ready = replEmptyModuleText, len(replEmptyModuleText)-1, true
	}
}

func (archive *replSessionArchive) declaration(text string) {
	archive.ensure()
	prefix := archive.text[:archive.close]
	if len(prefix) > 0 && len(text) > 0 && !unicode.IsSpace(rune(prefix[len(prefix)-1])) && !unicode.IsSpace(rune(text[0])) {
		text = "\n" + text
	}
	insertion := text + "\n"
	archive.text = prefix + insertion + archive.text[archive.close:]
	archive.close += len(insertion)
}

// REPLReplayResult describes one saved expression, including accepted runtime
// failures. Results and lexical closures retain the loaded session's owners.
type REPLReplayResult struct {
	File   string
	Input  string
	Result REPLResult
	Err    error
}

// REPLReplayHooks lets an embedding print prompts before evaluation and values
// or errors afterwards. Returning an error stops replay without evaluating the
// remaining expressions. Nil callbacks replay silently.
type REPLReplayHooks struct {
	Before func(filename, input string) error
	After  func(REPLReplayResult) error
}

// Load reads a project and replays its saved expression comments exactly once.
// Runtime or static failures of individual expressions do not prevent later
// expressions from replaying; source-loading and compilation failures return.
func (s *REPLSession) Load(ctx context.Context, path, main string) error {
	return s.LoadWithReplay(ctx, path, main, REPLReplayHooks{})
}

func (s *REPLSession) LoadWithReplay(ctx context.Context, path, main string, hooks REPLReplayHooks) error {
	if err := s.loadProject(ctx, path, main); err != nil {
		return err
	}
	pending := s.saved.pending
	s.saved.pending = nil
	for _, input := range pending {
		if err := ctx.Err(); err != nil {
			return err
		}
		if hooks.Before != nil {
			if err := hooks.Before(path, input); err != nil {
				return err
			}
		}
		result, err := s.Evaluate(ctx, input)
		if hooks.After != nil {
			if hookError := hooks.After(REPLReplayResult{File: path, Input: input, Result: result, Err: err}); hookError != nil {
				return hookError
			}
		}
		if err := ctx.Err(); err != nil {
			return err
		}
	}
	return nil
}

func (s *REPLSession) ReloadWithReplay(ctx context.Context, hooks REPLReplayHooks) error {
	if s.loaded == "" {
		return fmt.Errorf("Nothing to reload. Use: .load filename [moduleName].")
	}
	return s.LoadWithReplay(ctx, s.loaded, s.loadMain, hooks)
}

// SavedText returns a source snapshot without evaluating expressions, reading
// files, advancing randomness, or changing the session. Its declaration text
// and ordinary comments retain the inputs' original whitespace and line ends.
func (s *REPLSession) SavedText() (string, error) {
	if s == nil || s.ids == nil {
		return "", fmt.Errorf("REPL session is not initialized")
	}
	main := s.main
	if main == "" {
		main = "__repl__"
	}
	text := s.saved.text
	annotation := replMainAnnotation.FindStringSubmatchIndex(text)
	if annotation == nil {
		text = "// @mainModule " + main + "\n" + text
	} else if text[annotation[2]:annotation[3]] != main {
		text = text[:annotation[2]] + main + text[annotation[3]:]
	}
	for _, expression := range s.saved.expressions {
		if strings.Contains(expression, "*/") {
			return "", fmt.Errorf("cannot save an expression containing */ in the REPL session comment format")
		}
	}
	var output strings.Builder
	output.WriteString(text)
	for index, expression := range s.saved.expressions {
		if index > 0 {
			output.WriteByte('\n')
		}
		output.WriteString("/*! ")
		output.WriteString(expression)
		output.WriteString(" !*/\n")
	}
	return output.String(), nil
}

// Save writes the current source snapshot. A serialization or cancellation
// error leaves the destination untouched; saving never changes the reload path.
func (s *REPLSession) Save(ctx context.Context, path string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	text, err := s.SavedText()
	if err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(text), 0666)
}

var replMainAnnotation = regexp.MustCompile(`^// @mainModule[\t ]+(\w+)(?:\r\n?|\n|$)`)
var replArchiveLineEnds = strings.NewReplacer("\r\n", "\r\n", "\r", "\n")

// newREPLArchive verifies the chosen module's closing brace in the original
// source. Inserting here avoids the upstream last-character truncation bug.
func newREPLArchive(path, text, main string, synthetic bool) (replSessionArchive, error) {
	text, expressions := replExtractExpressions(text)
	archive := replSessionArchive{text: text, expressions: []string{}, pending: expressions, ready: true}
	if synthetic {
		archive.text += replEmptyModuleText
	}
	// Replacing bare CR preserves byte offsets, including CRLF pairs. The
	// verified parser span therefore still indexes the original archive.
	parserText := replArchiveLineEnds.Replace(archive.text)
	modules, sources, err := ParseModulesWithSource(path, parserText)
	if err != nil {
		return replSessionArchive{}, err
	}
	file, err := NewSourceFile(path, parserText)
	if err != nil {
		return replSessionArchive{}, err
	}
	for _, module := range modules {
		if module.Name != main {
			continue
		}
		location := sources[module.ID]
		if location.End == nil {
			break
		}
		end, err := file.ByteOffset(location.End.Line, location.End.Col)
		if err == nil && end > 0 && archive.text[end-1] == '}' {
			archive.close = end - 1
			return archive, nil
		}
	}
	return replSessionArchive{}, fmt.Errorf("cannot locate the closing brace of REPL module %s", main)
}

// replExtractExpressions recognizes actual block comments, rather than marker
// text inside strings or unrelated comments. Generated comment terminators and
// separators are removed so saving a loaded session cannot duplicate replay.
func replExtractExpressions(text string) (string, []string) {
	var source strings.Builder
	expressions := []string{}
	removedComment := false
	writeSource := func(fragment string) {
		// Removing a comment must retain its role as a token separator.
		// Delay this until actual source follows, so trailing saved history
		// does not introduce whitespace on each load/save round trip.
		if removedComment && source.Len() > 0 && len(fragment) > 0 && !unicode.IsSpace(rune(source.String()[source.Len()-1])) && !unicode.IsSpace(rune(fragment[0])) {
			source.WriteByte(' ')
		}
		removedComment = false
		source.WriteString(fragment)
	}
	for index := 0; index < len(text); {
		if text[index] == '"' {
			end := index + 1
			for end < len(text) {
				if text[end] == '\\' && end+1 < len(text) {
					end += 2
					continue
				}
				end++
				if text[end-1] == '"' {
					break
				}
			}
			writeSource(text[index:end])
			index = end
			continue
		}
		if strings.HasPrefix(text[index:], "//") || strings.HasPrefix(text[index:], "#!") {
			end := strings.IndexAny(text[index:], "\r\n")
			if end < 0 {
				writeSource(text[index:])
				break
			}
			end += index + 1
			writeSource(text[index:end])
			index = end
			continue
		}
		if strings.HasPrefix(text[index:], "/*") {
			end := strings.Index(text[index+2:], "*/")
			if end < 0 {
				writeSource(text[index:])
				break
			}
			end += index + 4
			comment := text[index:end]
			if strings.HasPrefix(comment, "/*! ") && strings.HasSuffix(comment, " !*/") {
				expressions = append(expressions, comment[4:len(comment)-4])
				removedComment = true
				index = end
				if strings.HasPrefix(text[index:], "\r\n") {
					index += 2
				} else if strings.HasPrefix(text[index:], "\n") {
					index++
				}
				if strings.HasPrefix(text[index:], "\n/*! ") {
					index++
				}
				continue
			}
			writeSource(comment)
			index = end
			continue
		}
		writeSource(text[index : index+1])
		index++
	}
	return source.String(), expressions
}

// The main file is read once, retaining its raw text for saving while exposing
// the existing normalized resolver input to the parser and diagnostics.
type replMainSourceResolver struct {
	SourceResolver
	path, text string
	code       map[string]string
}

func (resolver replMainSourceResolver) Load(path SourceLookupPath) (string, error) {
	if path.NormalizedPath != resolver.path {
		return resolver.SourceResolver.Load(path)
	}
	text := strings.ReplaceAll(strings.ReplaceAll(resolver.text, "\r\n", "\n"), "\r", "\n")
	resolver.code[path.NormalizedPath] = text
	return text, nil
}
