package apalache

import (
	"fmt"
	"strings"
)

// checkerTuningError explains g3 unknown-parameter rejections. Scala Apalache
// may accept such keys in its own FineTuningParser table, but the linked g3
// solver does not expose them, so the Go service cannot apply them. The
// rejection stays; only the message changes so it does not read as a Go
// parser refusing a key Scala knows.
func checkerTuningError(err error) error {
	if err == nil {
		return nil
	}
	msg := err.Error()
	if strings.Contains(msg, "unknown parameter") || strings.Contains(msg, "unknown module") {
		return fmt.Errorf("g3 does not expose this checker tuning key, so it cannot be applied (Scala Apalache may accept it): %w", err)
	}
	return err
}
