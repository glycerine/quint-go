package main

import (
	"context"
	"os"

	"github.com/glycerine/quint-go/pkg/cli"
)

func main() {
	code := cli.ExecuteWithSignals(context.Background(), os.Args[1:], os.Stdin, os.Stdout, os.Stderr)
	os.Exit(code)
}
