package quint

import (
	"errors"
	"fmt"
	"github.com/antlr4-go/antlr/v4"
	"strings"
)

// SyntaxIssue describes a lexer/parser error at a zero-based UTF-16 position.
// Code is populated for Quint grammar actions that emit a QNT diagnostic.
type SyntaxIssue struct {
	Code     string
	Message  string
	Position Position
}

// SyntaxErrors preserves all errors reported during syntax recognition.
// Semantic conversion errors remain Diagnostic values.
type SyntaxErrors struct {
	Source  string // Optional source name supplied by source-aware entry points.
	Context string
	Issues  []SyntaxIssue
}

func (e *SyntaxErrors) Error() string {
	messages := make([]string, len(e.Issues))
	for i, issue := range e.Issues {
		messages[i] = fmt.Sprintf("%d:%d: %s", issue.Position.Line+1, issue.Position.Col, issue.Message)
	}
	return "invalid " + e.Context + ": " + strings.Join(messages, "; ")
}

type effectSyntaxErrors struct {
	*antlr.DefaultErrorListener
	text    string
	issues  []SyntaxIssue
	onIssue func(interface{}, int, int, string, Position)
}

func newSyntaxErrors(text string) *effectSyntaxErrors {
	return &effectSyntaxErrors{DefaultErrorListener: antlr.NewDefaultErrorListener(), text: text}
}
func (e *effectSyntaxErrors) SyntaxError(recognizer antlr.Recognizer, offending interface{}, line, column int, message string, exception antlr.RecognitionException) {
	if e.onIssue != nil {
		offending, line, column, message = recoveryLookaheadSyntaxIssue(recognizer, offending, line, column, message, exception)
	}
	// ANTLR Go counts runes; Quint/LSP positions count UTF-16 code units.
	pos := Position{}
	runeColumn := 0
	for _, r := range e.text {
		if pos.Line == line-1 && runeColumn == column {
			break
		}
		width := 1
		if r > 0xffff {
			width = 2
		}
		pos.Index += width
		if r == '\n' {
			pos.Line++
			pos.Col = 0
			runeColumn = 0
		} else {
			pos.Col += width
			runeColumn++
		}
	}
	issue := SyntaxIssue{Message: message, Position: pos}
	if len(message) >= 8 && strings.HasPrefix(message, "[QNT") && message[7] == ']' {
		issue.Code = message[1:7]
	}
	e.issues = append(e.issues, issue)
	if e.onIssue != nil {
		e.onIssue(offending, line, column, message, pos)
	}
}
func (e *effectSyntaxErrors) trailing(token antlr.Token) {
	e.SyntaxError(nil, token, token.GetLine(), token.GetColumn(), "unexpected trailing input", nil)
}
func (e *effectSyntaxErrors) err(context string) error {
	return &SyntaxErrors{Context: context, Issues: append([]SyntaxIssue(nil), e.issues...)}
}

func withSyntaxSource(err error, name string) error {
	var syntax *SyntaxErrors
	if errors.As(err, &syntax) {
		syntax.Source = name
	}
	return err
}
