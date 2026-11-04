package cli

import (
	"strings"
	"testing"
)

// The upstream verification transcript prints a synthesized @type comment for
// every typed TLA+ declaration. The Go checker must keep those annotations.
func TestCompileTLAPlusPrintsDeclarationTypes(t *testing.T) {
	t.Parallel()
	path := coreTestFile(t, "typed.qnt", "module M {\n  var x: int\n  pure def f(a) = a\n  action init = x' = f(0)\n  action step = x' = x + 1\n  val inv = x >= 0\n}")
	code, stdout, stderr := coreInvoke(t, "compile", path, "--target=tlaplus", "--verbosity=0")
	if code != 0 || stderr != "" {
		t.Fatal(code, stdout, stderr)
	}
	for _, want := range []string{
		"VARIABLE\n  (*\n    @type: Int;\n  *)\n  x\n",
		"(*\n  @type: ((a) => a);\n*)\nf(a_5) == a_5\n",
		"(*\n  @type: (() => Bool);\n*)\ninv == x >= 0\n",
		"(*\n  @type: (() => Bool);\n*)\ninit == x = f(0)\n",
	} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("missing %q in:\n%s", want, stdout)
		}
	}
}

// Preparation failures print source-located diagnostics, then the upstream explanation.
func TestCompileTLAPlusReportsPreparationDiagnostics(t *testing.T) {
	t.Parallel()
	path := coreTestFile(t, "reused.qnt", "module M {\n  var x: int\n  action A = x' = 0\n  action init = A\n  action step = A\n}")
	code, stdout, stderr := coreInvoke(t, "compile", path, "--target=tlaplus", "--verbosity=0")
	if code != 1 || stdout != "" {
		t.Fatal(code, stdout, stderr)
	}
	if !strings.Contains(stderr, " Error [QNT409]: Action A is used both for init and for step") ||
		!strings.Contains(stderr, "at ") || !strings.HasSuffix(stderr, "\nerror: Failed to convert init to predicate\n") {
		t.Fatal(stderr)
	}
}

// Assignment steps use the Apalache `:=` form, matching upstream
// verification.md. Temporal definitions and unbounded set assignments are
// emitted as-is rather than rejected, also matching upstream output.
func TestCompileTLAPlusAssignmentAndTemporalShapesMatchUpstream(t *testing.T) {
	t.Parallel()
	path := coreTestFile(t, "shapes.qnt", "module M {\n  var x: int\n  var s: Set[int]\n  action init = all { x' = 0, s' = Set(1) }\n  action step = all { x' = x + 1, s' = Nat }\n  temporal live = eventually(x == 2)\n}")
	code, stdout, stderr := coreInvoke(t, "compile", path, "--target=tlaplus", "--verbosity=0")
	if code != 0 || stderr != "" {
		t.Fatal(code, stdout, stderr)
	}
	for _, want := range []string{
		"step == x' := x + 1 /\\ s' := Nat\n",
		"live == <>(x = 2)\n",
		"init == x = 0 /\\ s = {1}\n",
	} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("missing %q in:\n%s", want, stdout)
		}
	}
}
