package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"

	"github.com/glycerine/insdict"
	quint "github.com/glycerine/quint-go/pkg"
)

// verifyFrontendFailure retains the source snapshot needed by the common CLI
// reporter. It is presentation metadata and never enters the stage JSON.
type verifyFrontendFailure struct {
	sourceCode *insdict.Dict[string, string]
}

func verifyFrontendSources(sources map[string]string) *verifyFrontendFailure {
	code := insdict.NewDict[string, string]()
	paths := make([]string, 0, len(sources))
	for path := range sources {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	for _, path := range paths {
		code.Set(path, sources[path])
	}
	return &verifyFrontendFailure{sourceCode: code}
}

func verifyFrontendLoadingError(input string, err error) error {
	if errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("file %s does not exist", input)
	}
	return fmt.Errorf("file %s could not be opened due to %v", input, err)
}

// Frontend failures use outputResult's source reporter even at verbosity zero.
// The backend result reporter owns checker failures and successful results.
func verifyFrontendOutputContext(ctx context.Context, options verifyOptions, stage verifyStage, stderr io.Writer) (handled bool, code int) {
	if stage.Frontend == nil || stage.BackendResult || stage.Status == "ok" {
		return false, 0
	}
	if options.out != "" || options.outITF != "" {
		return true, 1
	}
	write := func(text string) bool {
		text += "\n"
		n, err := io.WriteString(stderr, text)
		return err == nil && n == len(text)
	}
	duplicate := false
	for _, message := range cliUniqueErrorMessages(stage.Errors) {
		rendered := quint.RenderErrorMessage(message, stage.Frontend.sourceCode,
			terminalDiagnosticOptions(ctx, quint.DiagnosticRenderOptions{InclusiveEnd: true}))
		if !write(rendered) {
			return true, 1
		}
		duplicate = duplicate || rendered == "error: "+stage.Explanation
	}
	if stage.Explanation != "" && !duplicate {
		write("error: " + stage.Explanation)
	}
	return true, 1
}
