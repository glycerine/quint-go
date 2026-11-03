package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestVerifyRejectsIneffectiveRandomTransitions(t *testing.T) {
	t.Parallel()
	input := verifyTestSource(t, "module M { action init=true action step=true val safe=true }")
	directory := t.TempDir()
	config := filepath.Join(directory, "checker.json")
	if err := os.WriteFile(config, []byte(`{"checker":{"tuning":{"search.simulation":"true"}}}`), 0600); err != nil {
		t.Fatal(err)
	}
	// The CLI's explicit default false overrides a loaded true before the
	// executor validates which behavior its effective configuration requests.
	var stdout, stderr bytes.Buffer
	args := []string{input, "--max-steps=0", "--apalache-config=" + config}
	if code := commandVerify(context.Background(), args, &stdout, &stderr); code != 0 || !strings.Contains(stdout.String(), "[ok]") || stderr.Len() != 0 {
		t.Fatal("loaded simulation flag bypassed CLI precedence", code, stdout.String(), stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	if code := commandVerify(context.Background(), append(args, "--random-transitions"), &stdout, &stderr); code != 1 || stdout.Len() != 0 || !strings.Contains(stderr.String(), "unsupported Go Apalache CHECK option") || !strings.Contains(stderr.String(), "--random-transitions") {
		t.Fatal("unsupported sampling became a verification verdict", code, stdout.String(), stderr.String())
	}
}
