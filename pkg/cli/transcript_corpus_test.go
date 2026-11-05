package cli

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"
)

// transcriptDirective is one `<!-- !test ... -->` marker from an upstream
// integration document. Only the fields this runner evaluates are kept.
type transcriptDirective struct {
	kind string
	name string
	body string
}

// transcriptCase is an executable `in` block with its expected results.
type transcriptCase struct {
	name     string
	command  string
	exit     int
	stdout   *string
	stderr   *string
	hasCheck bool
}

var transcriptMarker = regexp.MustCompile(`^<!-- !test (in|out|err|exit|check|program)(?: (.*?))? -->$`)

// parseUpstreamTranscript reads the txm-style markers used by quint/integration-tests.
// An `exit` marker attaches to the immediately preceding in/check block when only
// blank lines separate them; otherwise it applies to the next in/check marker. Out and
// err blocks attach to the in/check case with the same name.
func parseUpstreamTranscript(t *testing.T, path string) []transcriptCase {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	var lines []string
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 1<<20), 1<<24)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err = scanner.Err(); err != nil {
		t.Fatal(err)
	}
	type record struct {
		transcriptDirective
		exit    int
		blockAt int
	}
	var records []record
	pendingExit := -1
	for index := 0; index < len(lines); index++ {
		match := transcriptMarker.FindStringSubmatch(lines[index])
		if match == nil {
			continue
		}
		kind, name := match[1], match[2]
		switch kind {
		case "program":
			continue
		case "exit":
			code, err := strconv.Atoi(name)
			if err != nil {
				t.Fatalf("%s: bad exit marker %q", path, lines[index])
			}
			last := len(records) - 1
			if last >= 0 && (records[last].kind == "in" || records[last].kind == "check") &&
				records[last].exit == -1 && blankBetween(lines, records[last].blockAt, index) {
				records[last].exit = code
			} else {
				pendingExit = code
			}
			continue
		}
		body, next := transcriptBlock(lines, index+1)
		rec := record{transcriptDirective: transcriptDirective{kind: kind, name: name, body: body}, exit: -1, blockAt: next}
		if kind == "in" || kind == "check" {
			if pendingExit != -1 {
				rec.exit = pendingExit
				pendingExit = -1
			}
		}
		records = append(records, rec)
		index = next - 1
	}
	var cases []transcriptCase
	index := map[string]int{}
	for _, rec := range records {
		switch rec.kind {
		case "in", "check":
			name := rec.name
			if name == "" {
				name = "command " + strconv.Itoa(len(cases)+1)
			}
			exit := rec.exit
			if exit == -1 {
				exit = 0
			}
			index[name] = len(cases)
			cases = append(cases, transcriptCase{name: name, command: rec.body, exit: exit, hasCheck: rec.kind == "check"})
		case "out", "err":
			position, ok := index[rec.name]
			if !ok {
				continue
			}
			text := strings.TrimRight(rec.body, "\n")
			if rec.kind == "out" {
				cases[position].stdout = &text
			} else {
				cases[position].stderr = &text
			}
		}
	}
	return cases
}

// blankBetween reports whether every line in [from, to) is blank.
func blankBetween(lines []string, from, to int) bool {
	for index := from; index < to && index < len(lines); index++ {
		if strings.TrimSpace(lines[index]) != "" {
			return false
		}
	}
	return true
}

// transcriptBlock returns the fenced or four-space-indented text starting after a marker.
func transcriptBlock(lines []string, start int) (string, int) {
	position := start
	for position < len(lines) && strings.TrimSpace(lines[position]) == "" {
		position++
	}
	if position >= len(lines) {
		return "", position
	}
	var out []string
	if strings.HasPrefix(lines[position], "```") {
		position++
		for position < len(lines) && !strings.HasPrefix(lines[position], "```") {
			out = append(out, lines[position])
			position++
		}
		return strings.Join(out, "\n"), position + 1
	}
	if !strings.HasPrefix(lines[position], "    ") {
		return "", start
	}
	for position < len(lines) && strings.HasPrefix(lines[position], "    ") {
		out = append(out, strings.TrimPrefix(lines[position], "    "))
		position++
	}
	return strings.Join(out, "\n"), position
}

// transcriptBinary builds the command-line binary once and returns a PATH that exposes it as quint.
func transcriptBinary(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	bin := filepath.Join(dir, "quint")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	build := exec.CommandContext(ctx, "go", "build", "-o", bin, "github.com/glycerine/quint-go/pkg/cmd/quint")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build quint: %v\n%s", err, output)
	}
	return dir
}

// transcriptCaseTimeout returns the per-case limit. QUINT_CORPUS_CASE_TIMEOUT
// (a Go duration such as "45m") overrides the 5-minute default for slow cases.
func transcriptCaseTimeout(t *testing.T) time.Duration {
	t.Helper()
	raw := os.Getenv("QUINT_CORPUS_CASE_TIMEOUT")
	if raw == "" {
		return 5 * time.Minute
	}
	d, err := time.ParseDuration(raw)
	if err != nil || d <= 0 {
		t.Fatalf("bad QUINT_CORPUS_CASE_TIMEOUT %q", raw)
	}
	return d
}

// runTranscriptCase executes one command in the upstream quint directory.
func runTranscriptCase(t *testing.T, root, pathDir string, c transcriptCase) (stdout, stderr string, code int) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), transcriptCaseTimeout(t))
	defer cancel()
	cmd := exec.CommandContext(ctx, "bash", "-c", c.command)
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "PATH="+pathDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	var outBuffer, errBuffer bytes.Buffer
	cmd.Stdout = &outBuffer
	cmd.Stderr = &errBuffer
	err := cmd.Run()
	var exitErr *exec.ExitError
	if err != nil && !errors.As(err, &exitErr) {
		t.Fatal(err)
	}
	code = 0
	if exitErr != nil {
		code = exitErr.ExitCode()
	}
	return strings.TrimRight(outBuffer.String(), "\n"), strings.TrimRight(errBuffer.String(), "\n"), code
}

// replStreamDeviation: upstream's TypeScript REPL writes "syntax error" and
// "static analysis error" blocks to stdout. The Go REPL writes them to stderr,
// which the cli tests depend on, so Go keeps its stream split.
const replStreamDeviation = "Go REPL writes error blocks to stderr; upstream TypeScript writes them to stdout (Go keeps the stderr contract)"

// replPromptDeviation: the Go REPL banner and ">>> " prompt-echo layout differ
// from the upstream TypeScript transcript text.
const replPromptDeviation = "Go REPL banner and prompt-echo format differ from the upstream TypeScript transcript"

// transcriptKnownDeviations lists upstream transcript cases that the Go port
// documents as deliberate or blocked differences. Keys are "file/case name".
// Each case still runs, and fails the test if it starts matching upstream.
var transcriptKnownDeviations = map[string]string{
	"integration-tests/verification.md/can specify --server-endpoint":                                "in-process Go backend has no server endpoint (see PLAN_PORT.md)",
	"integration-tests/verification.md/TLC violation":                                                "P9.12: --max-steps is rejected for --backend tlc; exposing it is a user decision",
	"integration-tests/runtime/typescript/repl.md/repl works after name error":                       replStreamDeviation,
	"integration-tests/runtime/typescript/repl.md/repl works after unknown type alias error":         replStreamDeviation,
	"integration-tests/runtime/typescript/repl.md/repl works after type application arity error":     replStreamDeviation,
	"integration-tests/runtime/typescript/repl.md/repl works after conflict":                         replStreamDeviation,
	"integration-tests/runtime/typescript/repl.md/repl works after type errors":                      replStreamDeviation,
	"integration-tests/runtime/typescript/repl.md/regression 428":                                    replStreamDeviation,
	"integration-tests/runtime/typescript/repl.md/repl malformed expressions":                        replStreamDeviation,
	"integration-tests/runtime/typescript/repl.md/repl loads a file and a module":                    replPromptDeviation,
	"integration-tests/runtime/typescript/repl.md/repl loads module that is not the last":            replPromptDeviation,
	"integration-tests/runtime/typescript/repl.md/repl resolves imports relative to the loaded file": replPromptDeviation,
	"integration-tests/runtime/typescript/repl.md/repl saves a file with .save and loads it back":    replPromptDeviation,
	"integration-tests/runtime/typescript/repl.md/repl loads a module directly":                      replPromptDeviation,
	"integration-tests/runtime/typescript/repl.md/repl evaluates coin":                               replPromptDeviation,
	"integration-tests/runtime/typescript/repl.md/compile imports":                                   replPromptDeviation,
	"integration-tests/runtime/typescript/repl.md/compile instances":                                 replPromptDeviation,
	"integration-tests/runtime/typescript/repl.md/repl with instance vars":                           replPromptDeviation,
	"integration-tests/runtime/typescript/repl.md/repl diff":                                         replPromptDeviation,
	"integration-tests/verification.md/can convert ApalacheCompliation.qnt to TLA+":                  "P0.4 / pinned printer: Variants! qualifier and line wrapping differ from upstream",
	"integration-tests/runtime/typescript/test.md/test runs":                                         "Go repro hint names --backend=go and anchors --match=^name$; upstream hint is --seed=0x1 --match=name",
	"integration-tests/runtime/typescript/test.md/test empty trace":                                  "Go repro hint names --backend=go and anchors --match=^name$; upstream hint is --seed=0x1 --match=name",
	"integration-tests/runtime/typescript/test.md/verbose test":                                      "Go repro hint names --backend=go and anchors --match=^name$; upstream hint is --seed=0x1 --match=name",
	"integration-tests/runtime/typescript/test.md/test compile error":                                "Go repro hint names --backend=go and anchors --match=^name$; upstream hint is --seed=0x1 --match=name",
	"integration-tests/runtime/typescript/test.md/then error messages":                               "Go repro hint names --backend=go and anchors --match=^name$; upstream hint is --seed=0x1 --match=name",
	"integration-tests/runtime/rust/run.md/run finds violation":                                      "Rust reproduce hint is '--seed=<decimal> --backend=rust'; Go prints hex seed with --backend=go. Traces match after normalizing seeds",
	"integration-tests/runtime/rust/run.md/run finds violation with metadata":                        "seed/hint representation only (Rust decimal seed, --backend=rust); trace content matches after normalizing seeds",
	"integration-tests/runtime/rust/run.md/run finds example":                                        "seed/hint representation only (Rust decimal seed, --backend=rust); trace content matches after normalizing seeds",
	"integration-tests/runtime/rust/run.md/run uninitialized":                                        "seed/hint representation only (Rust decimal seed, --backend=rust); trace content matches after normalizing seeds",
	"integration-tests/runtime/rust/run.md/runtime error prints trace":                               "seed/hint representation only (Rust decimal seed, --backend=rust); trace content matches after normalizing seeds",
	"integration-tests/runtime/rust/run.md/witnesses":                                                "seed/hint representation only (Rust decimal seed, --backend=rust); trace content matches after normalizing seeds",
	"integration-tests/runtime/rust/run.md/multiple invariants":                                      "seed/hint representation only (Rust decimal seed, --backend=rust); trace content matches after normalizing seeds",
	"integration-tests/runtime/rust/run.md/rust backend debug":                                       "Rust prints '[DEBUG] ...' for q::debug; Go prints '> ...' (output format, not evaluation)",
	"integration-tests/runtime/rust/run.md/pending diagnostics flushed on runtime error":             "Rust prints '[DEBUG] ...' for q::debug; Go prints '> ...' (output format, not evaluation)",
	"integration-tests/runtime/rust/run.md/run with n-traces itf violation":                          "Rust default backend counts n-traces differently: seed 123 yields 3 violation files upstream; Go and TypeScript (verified with compiled CLI) yield 1. Default-backend gap for user decision",
	"integration-tests/runtime/rust/run.md/gradualPonzi - Run progressInv":                           "Rust backend reports a violation on seed 0x3154c258ec659ee6; Go and TypeScript backend report ok (verified with compiled CLI). Rust-backend behavior, not Go",
	"integration-tests/runtime/rust/run.md/run can hide fields":                                      "Rust backend trace differs on seed 0xcc198528dea8b (State 1 deposit vs withdraw, different length); Go matches the TypeScript backend apart from the hint line",
	"integration-tests/runtime/rust/run.md/run finds violation with metadata on bank":                "Rust backend trace differs on seed 0x1 (State 1 withdraw on charlie vs deposit on alice); Go matches the TypeScript backend apart from the hint line",
	"integration-tests/runtime/typescript/run.md/run finds violation":                                "Go reproduce hint is 'Use --seed=0x<hex> --backend=go'; upstream TypeScript hint names --backend=typescript (only the hint line differs)",
	"integration-tests/runtime/typescript/run.md/run finds violation with metadata":                  "Go reproduce hint is 'Use --seed=0x<hex> --backend=go'; upstream TypeScript hint names --backend=typescript (only the hint line differs)",
	"integration-tests/runtime/typescript/run.md/run finds violation with metadata on bank":          "Go reproduce hint is 'Use --seed=0x<hex> --backend=go'; upstream TypeScript hint names --backend=typescript (only the hint line differs)",
	"integration-tests/runtime/typescript/run.md/run can hide fields":                                "Go reproduce hint is 'Use --seed=0x<hex> --backend=go'; upstream TypeScript hint names --backend=typescript (only the hint line differs)",
	"integration-tests/runtime/typescript/run.md/run finds example":                                  "Go reproduce hint is 'Use --seed=0x<hex> --backend=go'; upstream TypeScript hint names --backend=typescript (only the hint line differs)",
	"integration-tests/runtime/typescript/run.md/run finds overflow":                                 "Go reproduce hint is 'Use --seed=0x<hex> --backend=go'; upstream TypeScript hint names --backend=typescript (only the hint line differs)",
	"integration-tests/runtime/typescript/run.md/run shows calls":                                    "Go reproduce hint is 'Use --seed=0x<hex> --backend=go'; upstream TypeScript hint names --backend=typescript (only the hint line differs)",
	"integration-tests/runtime/typescript/run.md/run uninitialized":                                  "Go reproduce hint is 'Use --seed=0x<hex> --backend=go'; upstream TypeScript hint names --backend=typescript (only the hint line differs)",
	"integration-tests/runtime/typescript/run.md/witnesses":                                          "Go reproduce hint is 'Use --seed=0x<hex> --backend=go'; upstream TypeScript hint names --backend=typescript (only the hint line differs)",
	"integration-tests/runtime/typescript/run.md/multiple invariants":                                "Go reproduce hint is 'Use --seed=0x<hex> --backend=go'; upstream TypeScript hint names --backend=typescript (only the hint line differs)",
}

// nonGoBackendFlag matches `--backend=typescript`, `--backend typescript`, and the
// same forms for rust, including the trailing-space form used before a line break.
var nonGoBackendFlag = regexp.MustCompile(`\s--backend(=|\s+)(typescript|rust)\b`)

// stripNonGoBackend removes non-Go backend flags from a command line. The second
// result reports whether the command pinned a non-Go backend.
func stripNonGoBackend(command string) (string, bool) {
	if !nonGoBackendFlag.MatchString(" " + command) {
		return command, false
	}
	return strings.TrimSpace(nonGoBackendFlag.ReplaceAllString(" "+command, "")), true
}

// TestUpstreamIntegrationTranscripts runs the executable `in` cases of selected upstream
// transcript documents against the Go binary. Set QUINT_CORPUS_TESTS=1 to run it.
func TestUpstreamIntegrationTranscripts(t *testing.T) {
	if os.Getenv("QUINT_CORPUS_TESTS") == "" {
		t.Skip("upstream transcript corpus; set QUINT_CORPUS_TESTS=1")
	}
	root, err := filepath.Abs(filepath.Join("..", "..", "quint"))
	if err != nil {
		t.Fatal(err)
	}
	pathDir := transcriptBinary(t)
	files := strings.Split(os.Getenv("QUINT_TRANSCRIPTS"), ",")
	if len(files) == 1 && files[0] == "" {
		files = []string{"integration-tests/lang/io.md", "integration-tests/verification.md"}
	}
	for _, file := range files {
		cases := parseUpstreamTranscript(t, filepath.Join(root, file))
		for _, c := range cases {
			name := file + "/" + c.name
			if stripped, pinned := stripNonGoBackend(c.command); pinned {
				// The Go port only implements the Go backend. The upstream case pins a
				// different backend, so run the same command on the Go backend with that
				// flag removed and check it against the upstream expectations.
				c.command = stripped
				name += " (backend flag stripped)"
			}
			t.Run(name, func(t *testing.T) {
				stdout, stderr, code := runTranscriptCase(t, root, pathDir, c)
				if reason, known := transcriptKnownDeviations[file+"/"+c.name]; known {
					// A known deviation must still deviate. If it now matches upstream,
					// remove its entry so the case is checked strictly.
					matches := code == c.exit &&
						(c.stdout == nil || stdout == *c.stdout) &&
						(c.stderr == nil || stderr == *c.stderr)
					if matches {
						t.Errorf("known deviation now matches upstream; remove it from transcriptKnownDeviations (%s)", reason)
					}
					t.Skip("known deviation: " + reason)
				}
				if code != c.exit {
					t.Errorf("exit = %d, want %d\nstdout:\n%s\nstderr:\n%s", code, c.exit, stdout, stderr)
				}
				if c.stdout != nil && stdout != *c.stdout {
					t.Errorf("stdout mismatch\n--- got\n%s\n--- want\n%s", stdout, *c.stdout)
				}
				if c.stderr != nil && stderr != *c.stderr {
					t.Errorf("stderr mismatch\n--- got\n%s\n--- want\n%s", stderr, *c.stderr)
				}
			})
		}
	}
}
