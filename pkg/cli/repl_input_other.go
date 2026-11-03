//go:build !(aix || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris)

package cli

import (
	"context"
	"os"
)

// No terminal keepalive is advertised without an interruptible file adapter.
func newREPLFileInput(context.Context, *os.File) (replFileInput, bool, error) {
	return nil, false, nil
}
