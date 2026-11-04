package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	quint "github.com/glycerine/quint-go/pkg"
	"github.com/glycerine/quint-go/pkg/backend/apalache"
)

type failingVerifyBackend struct{ t *testing.T }

func (b failingVerifyBackend) Verify(context.Context, quint.CompilationResult, json.RawMessage, apalache.VerificationOptions) (apalache.VerificationResult, error) {
	b.t.Fatal("checker must not run for an invalid configuration")
	return apalache.VerificationResult{}, nil
}

// A legacy Apalache configuration is rejected with upstream's key diagnostic before any checker runs.
func TestVerifyRejectsLegacyApalacheConfigBeforeChecking(t *testing.T) {
	t.Parallel()
	input := verifyTestSource(t, verifyTestModel)
	config := filepath.Join(t.TempDir(), "legacy.json")
	if err := os.WriteFile(config, []byte(`{"input":{"source":{"type":"string"}}}`), 0600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	code := verifyWithBackend(context.Background(), []string{input, "--apalache-config=" + config}, &stdout, &stderr, failingVerifyBackend{t})
	if code != 1 || stdout.Len() != 0 || stderr.String() != "error: $.input: Unknown configuration key.\n" {
		t.Fatal(code, stdout.String(), stderr.String())
	}
}
