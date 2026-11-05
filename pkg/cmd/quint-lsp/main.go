package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/glycerine/quint-go/pkg/lsp"
)

func main() {
	os.Exit(run())
}

func run() int {
	logger := lsp.NewEnvironmentLogger()
	defer logger.Close()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return runWith(ctx, logger, os.Stdin, os.Stdout, os.Stderr)
}

// runWith serves LSP traffic until input ends, the context is cancelled, or the
// server fails. Cancellation, including a signal, is a clean exit.
func runWith(ctx context.Context, logger *lsp.Logger, input io.Reader, output, errOutput io.Writer) int {
	if err := lsp.NewServer(lsp.Options{Logger: logger}).Serve(ctx, input, output); err != nil {
		if ctx.Err() != nil {
			return 0
		}
		logger.Log("ERROR", "Language server stopped: %s", err)
		fmt.Fprintln(errOutput, err)
		return 1
	}
	return 0
}
