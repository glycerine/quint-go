package quint

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTraceWriter(t *testing.T) {
	state, err := ParseExpression(`Rec("x", 123456789012345678901234567890)`)
	if err != nil {
		t.Fatal(err)
	}
	directory := t.TempDir()
	for _, test := range []struct {
		name, template, traceName, suffix string
		count                             int
		metadata                          bool
	}{
		{"single", "trace.itf", "", "trace.itf", 1, false},
		{"multiple", "trace{seq}.itf", "", "trace3.itf", 2, true},
		{"named", "{test}-{seq}.itf", "spec", "spec-3.itf", 2, false},
		{"automatic", "trace.itf", "", "trace3.itf", 2, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			// A placeholder-free parent avoids upstream's first-dot path rule here.
			w := TraceWriter{Template: filepath.Join(directory, test.template), TraceCount: test.count, Metadata: test.metadata, Header: ITFHeader{Source: "spec.qnt", CreatedAt: "fixed", Timestamp: 42, Status: "old"}}
			filename, err := w.Write(3, "violation", []string{"x"}, []Expr{state}, test.traceName)
			if err != nil {
				t.Fatal(err)
			}
			if filename != filepath.Join(directory, test.suffix) {
				t.Fatal(filename)
			}
			data, err := os.ReadFile(filename)
			if err != nil {
				t.Fatal(err)
			}
			trace, err := DecodeITFTrace(data)
			if err != nil {
				t.Fatal(err)
			}
			if len(trace.States) != 1 || !bytes.Contains(data, []byte("123456789012345678901234567890")) || !bytes.Contains(trace.Metadata, []byte(`"status":"violation"`)) || !bytes.Contains(trace.Metadata, []byte(`"timestamp":42`)) {
				t.Fatal(string(data))
			}
			expectedVars := 1
			if test.metadata {
				expectedVars = 3
			}
			if len(trace.Vars) != expectedVars {
				t.Fatal(trace.Vars)
			}
			if _, err = w.Write(3, "success", []string{"x"}, []Expr{}, test.traceName); err != nil {
				t.Fatal(err)
			}
			data, _ = os.ReadFile(filename)
			trace, err = DecodeITFTrace(data)
			if err != nil || len(trace.States) != 0 {
				t.Fatal(string(data), err)
			}
		})
	}
}

func TestTraceWriterErrors(t *testing.T) {
	invalid := []Expr{{Kind: "name", Name: "symbolic"}}
	if path, err := (TraceWriter{}).Write(0, "", nil, invalid, ""); path != "" || err != nil {
		t.Fatal(path, err)
	}
	filename := filepath.Join(t.TempDir(), "trace.itf")
	w := TraceWriter{Template: filename}
	if path, err := w.Write(7, "", nil, invalid, ""); path != filename || err == nil || !strings.Contains(err.Error(), "ITF conversion failed on 7") {
		t.Fatal(path, err)
	}
	if _, err := os.Stat(filename); !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}
	w.Template = filepath.Join(t.TempDir(), "missing", "trace.itf")
	if _, err := w.Write(0, "", nil, nil, ""); !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}
}
