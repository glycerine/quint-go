package apalache

import (
	"encoding/json"
	"testing"

	cmd "github.com/glycerine/apalache-go/pkg/shai/cmdexecutor"
)

// BenchmarkCheckConfigHandoff measures the checker-input handoff: validating
// options, adapting the compiled IR, and marshalling the Apalache config. It
// does not start the checker service.
func BenchmarkCheckConfigHandoff(b *testing.B) {
	compiled := compileSource(b, "module M { action init=true action step=true val safe=true }")
	options := CheckOptions{MaxSteps: 5}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := CheckConfig(compiled, nil, options); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkDecodeCheckResponseSafe measures decoding of a successful checker
// response (the response-side half of the handoff).
func BenchmarkDecodeCheckResponseSafe(b *testing.B) {
	response := &cmd.CmdResponse{Result: &cmd.CmdResponse_Success{Success: `{"modules":[{"name":"M"}]}`}}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := decodeCheckResponse(response); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkDecodeCheckResponseViolation measures decoding of a bounded-check
// failure carrying a counterexample payload.
func BenchmarkDecodeCheckResponseViolation(b *testing.B) {
	errorData, _ := json.Marshal(map[string]any{
		"checking_result": "Error",
		"counterexamples": []any{map[string]any{"states": []any{map[string]any{"x": 1}}}},
	})
	payload, _ := json.Marshal(map[string]any{"pass_name": "BoundedChecker", "error_data": json.RawMessage(errorData)})
	response := &cmd.CmdResponse{Result: &cmd.CmdResponse_Failure{Failure: &cmd.CmdError{
		ErrorType: cmd.CmdErrorType_PASS_FAILURE,
		Data:      string(payload),
	}}}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := decodeCheckResponse(response); err != nil {
			b.Fatal(err)
		}
	}
}
