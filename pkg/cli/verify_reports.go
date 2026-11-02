package cli

import (
	"context"
	"fmt"

	quint "github.com/glycerine/quint-go/pkg"
	"github.com/glycerine/quint-go/pkg/backend/apalache"
)

// verifyReportInvariants adds ordinary predicate attribution to an existing
// violation. Reporting failures retain the checker verdict and remain visible.
func verifyReportInvariants(ctx context.Context, compiled quint.CompilationResult, labels []string, stage *verifyStage) {
	if stage == nil || stage.Outcome != apalache.CheckViolation || len(labels) == 0 {
		return
	}
	violated, err := apalache.FindViolatedInvariants(ctx, compiled, labels, stage.Trace)
	if err != nil {
		stage.Warnings = append(stage.Warnings, fmt.Sprintf("could not identify violated invariants: %v", err))
		return
	}
	for _, index := range violated {
		stage.ViolatedInvariants = append(stage.ViolatedInvariants, labels[index])
	}
}
