package cli

import (
	"fmt"
	"io"
	"strings"
)

// cliWriteHelp uses checked-in output from the upstream option builders, with
// the backend descriptions adjusted to the implementations shipped in Go.
func cliWriteHelp(writer io.Writer, command string) error {
	text := cliHelpTexts[command]
	written, err := io.WriteString(writer, text)
	if err == nil && written != len(text) {
		err = io.ErrShortWrite
	}
	return err
}

func cliPrintHelp(stdout, stderr io.Writer, command string) int {
	if err := cliWriteHelp(stdout, command); err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return 1
	}
	return 0
}

// Parser failures are printed by yargs without the diagnostic renderer's
// "error:" prefix. Source and runtime failures use their own renderers.
func cliPrintArgumentError(stderr io.Writer, err error) int {
	fmt.Fprintln(stderr, err)
	return 1
}

func cliUnknownArguments(names []string) error {
	if len(names) == 1 {
		return fmt.Errorf("Unknown argument: %s", names[0])
	}
	return fmt.Errorf("Unknown arguments: %s", strings.Join(names, ", "))
}

func cliInvalidChoice(name, value string, choices ...string) error {
	quoted := make([]string, len(choices))
	for i, choice := range choices {
		quoted[i] = fmt.Sprintf("%q", choice)
	}
	return fmt.Errorf("Invalid values:\n  Argument: %s, Given: %q, Choices: %s", name, value, strings.Join(quoted, ", "))
}
