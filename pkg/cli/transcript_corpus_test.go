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

// runTranscriptCase executes one command in the upstream quint directory.
func runTranscriptCase(t *testing.T, root, pathDir string, c transcriptCase) (stdout, stderr string, code int) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
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

// transcriptKnownDeviations lists upstream transcript cases that the Go port
// documents as deliberate or blocked differences. Keys are "file/case name".
// Each case still runs, and fails the test if it starts matching upstream.
var transcriptKnownDeviations = map[string]string{
	"integration-tests/verification.md/can specify --server-endpoint":               "in-process Go backend has no server endpoint (see PLAN_PORT.md)",
	"integration-tests/verification.md/TLC violation":                               "P9.12: --max-steps is rejected for --backend tlc; exposing it is a user decision",
	"integration-tests/verification.md/can convert ApalacheCompliation.qnt to TLA+": "P0.4 / pinned printer: Variants! qualifier and line wrapping differ from upstream",
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
			if strings.Contains(c.command, "--backend=typescript") || strings.Contains(c.command, "--backend=rust") ||
				strings.Contains(c.command, "--backend typescript") || strings.Contains(c.command, "--backend rust") {
				// The Go port only implements the Go backend; these cases are out of scope.
				t.Run(file+"/"+c.name+" (out of scope: non-Go backend)", func(t *testing.T) { t.Skip("non-Go backend") })
				continue
			}
			t.Run(file+"/"+c.name, func(t *testing.T) {
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
