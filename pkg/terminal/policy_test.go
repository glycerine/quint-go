package terminal

import (
	"bytes"
	"context"
	"io"
	"os"
	"strings"
	"testing"
)

type fdWriter struct{ bytes.Buffer }

func (*fdWriter) Fd() uintptr { return 42 }

func TestPolicyCachesColorButQueriesLiveStdoutWidth(t *testing.T) {
	columns, calls := 120, 0
	env := map[string]string{"TERM": "xterm-256color"}
	output := &fdWriter{}
	probe := terminalProbe{
		env: env, platform: "linux",
		isTTY: func(fd int) bool {
			if fd != 42 {
				t.Fatalf("TTY fd = %d", fd)
			}
			return true
		},
		columns: func(fd int) int {
			if fd != 42 {
				t.Fatalf("width fd = %d", fd)
			}
			calls++
			return columns
		},
	}
	policy := newPolicy(Options{}, output, probe)
	if calls != 0 || policy.ColorLevel() != 2 || policy.Width() != 120 {
		t.Fatalf("initial policy: level=%d width=%d calls=%d", policy.ColorLevel(), policy.Width(), calls)
	}
	env["TERM"] = "dumb"
	columns = 27
	if policy.ColorLevel() != 2 || policy.Width() != 27 {
		t.Fatal("color was not cached or width was not live")
	}
	columns = 0
	if policy.Width() != 80 || output.Len() != 0 {
		t.Fatal("missing width did not fall back without writing")
	}
	columns = -1
	if policy.Width() != 80 {
		t.Fatal("negative width must fall back")
	}
}

func TestPolicyExplicitOverridesOwnValuesAndPreserveContext(t *testing.T) {
	type markerKey struct{}
	base, cancel := context.WithCancel(context.WithValue(context.Background(), markerKey{}, "kept"))
	defer cancel()
	level := 1
	ctx := WithOptions(base, Options{Width: 19, ColorLevel: &level})
	level = 0
	policy := PolicyFromContext(ctx)
	if policy.Width() != 19 || policy.ColorLevel() != 1 || !strings.Contains(policy.Style("x", "red"), "\x1b[31m") {
		t.Fatal("explicit options were not copied")
	}
	ctx = Bind(ctx, io.Discard)
	policy = PolicyFromContext(ctx)
	if policy.Width() != 19 || policy.ColorLevel() != 1 {
		t.Fatal("bind lost overrides")
	}
	if rebound := Bind(ctx, &fdWriter{}); rebound != ctx {
		t.Fatal("bind is not idempotent")
	}
	plain := 0
	nested := WithOptions(ctx, Options{Width: 11, ColorLevel: &plain})
	if got := PolicyFromContext(Bind(nested, io.Discard)); got.Width() != 11 || got.Style("x", "red") != "x" {
		t.Fatal("nested explicit no-color failed")
	}
	if ctx.Value(markerKey{}) != "kept" {
		t.Fatal("context value lost")
	}
	cancel()
	if ctx.Err() != context.Canceled {
		t.Fatal("context cancellation lost")
	}
}

func TestPolicyPlainDefaultsAndForcedRedirectedColor(t *testing.T) {
	var absent *Policy
	if absent.Width() != 80 || absent.ColorLevel() != 0 || absent.Style("x", "red") != "x" {
		t.Fatal("nil policy must be plain")
	}
	if policy := PolicyFromContext(context.Background()); policy.Width() != 80 || policy.ColorLevel() != 0 {
		t.Fatal("unbound library context must be plain")
	}
	probe := terminalProbe{env: map[string]string{"FORCE_COLOR": "1", "COLUMNS": "3"}, platform: "linux"}
	forced := newPolicy(Options{}, &bytes.Buffer{}, probe)
	if forced.ColorLevel() != 1 || forced.Width() != 80 {
		t.Fatal("forced redirected color or ignored COLUMNS mismatch")
	}
	zero := 0
	if got := newPolicy(Options{ColorLevel: &zero}, io.Discard, probe); got.ColorLevel() != 0 {
		t.Fatal("explicit no-color must override FORCE_COLOR")
	}
	probe.env = map[string]string{"TERM": "xterm"}
	if got := newPolicy(Options{Width: -1}, io.Discard, probe); got.ColorLevel() != 0 || got.Width() != 80 {
		t.Fatal("ordinary redirected output must be plain with default width")
	}
}

func TestHostPolicyRegularFileAndDiscard(t *testing.T) {
	t.Setenv("FORCE_COLOR", "0")
	file, err := os.CreateTemp(t.TempDir(), "stdout")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	policy := PolicyFromContext(Bind(context.Background(), file))
	if policy.Width() != 80 || policy.ColorLevel() != 0 {
		t.Fatal("regular file was treated as a terminal")
	}
	if got := PolicyFromContext(Bind(context.Background(), io.Discard)); got.Width() != 80 || got.ColorLevel() != 0 {
		t.Fatal("discard policy mismatch")
	}
}
