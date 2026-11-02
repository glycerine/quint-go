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
type Executor struct{ service *shai.CmdExecutorService }

func New() *Executor { return &Executor{service: shai.NewCmdExecutorService()} }

func (e *Executor) Run(ctx context.Context, command cmd.Cmd, config json.RawMessage) (*cmd.CmdResponse, error) {
	if !json.Valid(config) {
		return nil, fmt.Errorf("invalid Apalache configuration JSON")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return e.service.Run(ctx, &cmd.CmdRequest{Cmd: command, Config: string(config)})
}
