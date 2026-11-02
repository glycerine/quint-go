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
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := lsp.NewServer(lsp.Options{}).Serve(ctx, os.Stdin, os.Stdout); err != nil {
		if ctx.Err() != nil {
			return
		}
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
