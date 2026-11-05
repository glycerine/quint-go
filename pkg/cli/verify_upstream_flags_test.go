package cli

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// Every option the upstream verify command declares must either parse in the Go
// verifier or be rejected by an explicit policy. The rejected set is the
// documented Scala-server/version surface, which the in-process backend lacks.
func TestVerifyAcceptsUpstreamOptionsExceptDocumentedRejections(t *testing.T) {
	source, err := os.ReadFile("../../quint/src/cli.ts")
	if err != nil {
		t.Fatal(err)
	}
	text := string(source)
	_, rest, ok := strings.Cut(text, "const verifyCmd = {")
	if !ok {
		t.Fatal("missing upstream verify command")
	}
	body, _, ok := strings.Cut(rest, "// Timeouts are postponed")
	if !ok {
		t.Fatal("missing upstream verify command terminator")
	}
	// The shared compile options are declared by compileOpts, not verifyCmd.
	_, compileRest, ok := strings.Cut(text, "const compileOpts = (yargs: any) =>")
	if !ok {
		t.Fatal("missing upstream compile options")
	}
	compileBody, _, _ := strings.Cut(compileRest, "\nconst ")
	names := map[string]bool{}
	optionName := regexp.MustCompile(`\.option\('([a-z-]+)'`)
	for _, src := range []string{body, compileBody} {
		for _, m := range optionName.FindAllStringSubmatch(src, -1) {
			names[m[1]] = true
		}
	}
	if len(names) < 15 {
		t.Fatalf("upstream verify option extraction is incomplete: %v", names)
	}
	rejected := map[string]string{
		"apalache-version": "unsupported by the in-process Go Apalache backend",
		"server-endpoint":  "unsupported by the in-process Go Apalache backend",
	}
	for name := range names {
		_, _, err := verifyParseOptions([]string{"--" + name + "=x", "spec.qnt"}, nil, nil)
		if err == nil {
			continue
		}
		msg := err.Error()
		if want, isRejected := rejected[name]; isRejected {
			if !strings.Contains(msg, want) {
				t.Errorf("--%s rejected with unexpected message: %v", name, err)
			}
			continue
		}
		if strings.Contains(msg, "Unknown argument") || strings.Contains(msg, "not defined") {
			t.Errorf("upstream verify option --%s is not accepted: %v", name, err)
		}
	}
	for name := range rejected {
		if !names[name] {
			t.Errorf("documented rejection --%s is no longer an upstream verify option; remove it", name)
		}
	}
}
