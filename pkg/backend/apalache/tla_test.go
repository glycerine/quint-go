package apalache

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	cmd "github.com/glycerine/apalache-go/pkg/shai/cmdexecutor"
	quint "github.com/glycerine/quint-go/pkg"
)

func TestTLAResponseDecoding(t *testing.T) {
	data, _ := json.Marshal("a == <A>_v")
	got, err := decodeTLAResponse(&cmd.CmdResponse{Result: &cmd.CmdResponse_Success{Success: string(data)}})
	if err != nil || got != "a == <<A>>_v" {
		t.Fatal(got, err)
	}
	for _, response := range []*cmd.CmdResponse{
		nil, {},
		{Result: &cmd.CmdResponse_Success{Success: "{"}},
		{Result: &cmd.CmdResponse_Success{Success: "null"}},
		{Result: &cmd.CmdResponse_Success{Success: "42"}},
		{Result: &cmd.CmdResponse_Failure{}},
	} {
		if _, err := decodeTLAResponse(response); err == nil {
			t.Fatal("malformed response accepted", response)
		}
	}
	payload := `{"pass_name":"Transpiler","error_data":{"message":"unsupported"}}`
	_, err = decodeTLAResponse(&cmd.CmdResponse{Result: &cmd.CmdResponse_Failure{Failure: &cmd.CmdError{ErrorType: cmd.CmdErrorType_PASS_FAILURE, Data: payload}}})
	var failure *CommandFailure
	if !errors.As(err, &failure) || failure.Type != "PASS_FAILURE" || failure.Data != payload || !strings.Contains(err.Error(), "Transpiler") {
		t.Fatal(err)
	}
}

func TestTLAPreparationFailureAndCancellation(t *testing.T) {
	compiled := compileSource(t, "module M { var x:int action common=x'=0 action init=common action step=common val safe=true }")
	before, _ := json.Marshal(compiled.MainModule)
	_, err := New().TLA(context.Background(), compiled)
	var preparation *PreparationError
	if !errors.As(err, &preparation) || len(preparation.Diagnostics) != 1 || preparation.Diagnostics[0].Code != quint.QNT409 {
		t.Fatal(err)
	}
	after, _ := json.Marshal(compiled.MainModule)
	if string(before) != string(after) {
		t.Fatal("failed preparation changed module")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := New().TLA(ctx, quint.CompilationResult{}); err != context.Canceled {
		t.Fatal(err)
	}
}
