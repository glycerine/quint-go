// Package apalache connects Quint to the existing Go Apalache command pipeline.
package apalache

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/glycerine/apalache-go/shai"
	cmd "github.com/glycerine/apalache-go/shai/cmdexecutor"
)

// Executor uses the production command service in-process, without a server or JVM.
// Config is the same JSON configuration accepted by the Go Shai service.
// Cancellation observed before Run returns takes precedence over a backend
// response, including a response that completed concurrently with cancellation.
type Executor struct{ service *shai.CmdExecutorService }

func New() *Executor { return &Executor{service: shai.NewCmdExecutorService()} }

func (e *Executor) Run(ctx context.Context, command cmd.Cmd, config json.RawMessage) (*cmd.CmdResponse, error) {
	if !json.Valid(config) {
		return nil, fmt.Errorf("invalid Apalache configuration JSON")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	response, err := e.service.Run(ctx, &cmd.CmdRequest{Cmd: command, Config: string(config)})
	// The service can encode solver cancellation as an ordinary failure payload.
	// Preserve caller cancellation as a Go context error instead of losing its
	// identity inside that JSON, or accepting a concurrent success response.
	if contextError := ctx.Err(); contextError != nil {
		return nil, contextError
	}
	return response, err
}
