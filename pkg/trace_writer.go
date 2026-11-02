package quint

import (
	"fmt"
	"os"
)

// TraceWriter connects simulation/test trace callbacks to ITF files, following
// upstream prepareOnTrace. Header clock values are supplied by the caller.
// An empty Template disables output, including value conversion.
type TraceWriter struct {
	Template   string
	TraceCount int
	Metadata   bool
	Header     ITFHeader
}

// Write encodes and writes one trace. A nonempty name enables {test} expansion.
// The returned filename is empty when output is disabled. Like upstream, files
// are overwritten and parent directories must already exist. Errors are returned
// to the CLI so it can report them without printing from the library.
func (w TraceWriter) Write(index int, status string, vars []string, states []Expr, name string) (string, error) {
	if w.Template == "" {
		return "", nil
	}
	filename := ExpandOutputTemplate(w.Template, index, w.TraceCount > 1)
	if name != "" {
		filename = ExpandNamedOutputTemplate(w.Template, name, index, w.TraceCount > 1)
	}
	trace, err := EncodeITFTrace(vars, states, w.Metadata)
	if err != nil {
		return filename, fmt.Errorf("ITF conversion failed on %d: %w", index, err)
	}
	header := w.Header
	header.Status = status
	data, err := AddITFHeader(trace, header)
	if err != nil {
		return filename, fmt.Errorf("ITF header on %d: %w", index, err)
	}
	if err := os.WriteFile(filename, data, 0666); err != nil {
		return filename, fmt.Errorf("writing ITF trace %d: %w", index, err)
	}
	return filename, nil
}
