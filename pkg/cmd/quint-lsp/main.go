package main

import (
	"context"
	"fmt"
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
	if err := lsp.NewServer(lsp.Options{Logger: logger}).Serve(ctx, os.Stdin, os.Stdout); err != nil {
		if ctx.Err() != nil {
			return 0
		}
		logger.Log("ERROR", "Language server stopped: %s", err)
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}
