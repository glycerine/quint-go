package apalache

import (
	"context"
	"encoding/json"
	"fmt"

	cmd "github.com/glycerine/apalache-go/shai/cmdexecutor"
	quint "github.com/glycerine/quint-go/pkg"
)

// PreparationError retains source-referenced diagnostics before checking or TLA
// generation. Explanation may describe a backend preparation restriction;
// omitting it retains the init-to-predicate explanation.
type PreparationError struct {
	Diagnostics []quint.Diagnostic
	Explanation string
}

func (e *PreparationError) Error() string {
	if e.Explanation != "" {
		return fmt.Sprintf("%s: %v", e.Explanation, e.Diagnostics)
	}
	return fmt.Sprintf("failed to convert init to predicate: %v", e.Diagnostics)
}

// CommandFailure retains the command service's error category and raw payload.
// Callers can interpret pass-specific JSON without losing backend information.
type CommandFailure struct {
	Type string
	Data string
}

func (e *CommandFailure) Error() string { return e.Type + ": " + e.Data }

// TLA converts a compiled specification using Go Apalache's production TLA
// command. It removes run definitions, converts init assignments to predicates,
// and applies Quint's printer fixes. Verification input uses MarshalInput
// directly and retains assignments and run definitions.
func (e *Executor) TLA(ctx context.Context, compiled quint.CompilationResult) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	module, diagnostics := quint.ConvertInit(quint.RemoveRunDefinitions(compiled.MainModule), compiled.Table, compiled.Analysis.Modes)
	if len(diagnostics) > 0 {
		return "", &PreparationError{Diagnostics: diagnostics}
	}
	prepared := compiled
	prepared.MainModule = module
	prepared, err := adaptTLAIntegerDivision(prepared)
	if err != nil {
		return "", err
	}
	input, err := MarshalInput(prepared)
	if err != nil {
		return "", err
	}
	config, err := json.Marshal(struct {
		Source quintSource `json:"source"`
	}{quintSource{Kind: "string", Format: "qnt", Content: string(input)}})
	if err != nil {
		return "", err
	}
	response, err := e.Run(ctx, cmd.Cmd_TLA, config)
	if err != nil {
		return "", err
	}
	return decodeTLAResponse(response)
}

func decodeTLAResponse(response *cmd.CmdResponse) (string, error) {
	if response == nil {
		return "", fmt.Errorf("Go Apalache returned no TLA response")
	}
	switch result := response.Result.(type) {
	case *cmd.CmdResponse_Failure:
		if result.Failure == nil {
			return "", fmt.Errorf("Go Apalache returned an empty TLA failure")
		}
		return "", &CommandFailure{Type: result.Failure.GetErrorType().String(), Data: result.Failure.GetData()}
	case *cmd.CmdResponse_Success:
		var tla *string
		if err := json.Unmarshal([]byte(result.Success), &tla); err != nil {
			return "", fmt.Errorf("invalid Go Apalache TLA response: %w", err)
		}
		if tla == nil {
			return "", fmt.Errorf("Go Apalache TLA response must contain a string")
		}
		return quint.PostprocessTLA(*tla), nil
	default:
		return "", fmt.Errorf("Go Apalache TLA response has no result")
	}
}

// quintSource matches the Go checker's string-source configuration contract.
type quintSource struct {
	Kind    string `json:"kind"`
	Format  string `json:"format"`
	Content string `json:"content"`
}
