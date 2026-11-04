package cli

import (
	"context"
	"os"
	"strings"
	"testing"
)

// verifyApalacheTemporalPrompt is the warning and question printed before an
// Apalache run with --temporal, in the order upstream prints them.
const verifyApalacheTemporalPrompt = "\n  WARNING: Apalache has experimental support for temporal properties and might give incorrect results.\n  Consider using --backend tlc, which fully supports temporal properties.\n\nDo you want to proceed with Apalache anyway? (y/N) "

// yesStdin answers every confirmation prompt with "y". It holds no state, so parallel
// tests can share it safely.
type yesStdin struct{}

func (yesStdin) Read(p []byte) (int, error) { return copy(p, "y\n"), nil }

func TestMain(m *testing.M) {
	verifyStdin = yesStdin{}
	os.Exit(m.Run())
}

func TestVerifyApalacheTemporalConfirmation(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		answer string
		want   bool
	}{{"y\n", true}, {"Y\n", true}, {" y \n", true}, {"n\n", false}, {"\n", false}, {"", false}, {"yes\n", false}} {
		var stderr strings.Builder
		if got := verifyConfirmApalacheTemporal(context.Background(), strings.NewReader(tc.answer), &stderr); got != tc.want {
			t.Fatalf("answer %q: got %v want %v", tc.answer, got, tc.want)
		}
		if !strings.Contains(stderr.String(), "WARNING: Apalache has experimental support") ||
			!strings.HasSuffix(stderr.String(), "Do you want to proceed with Apalache anyway? (y/N) ") {
			t.Fatalf("prompt text = %q", stderr.String())
		}
	}
}
