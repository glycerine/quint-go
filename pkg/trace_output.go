package quint

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/glycerine/insdict"
)

const (
	TraceTestPlaceholder     = "{test}"
	TraceSequencePlaceholder = "{seq}"
)

// ExpandOutputTemplate replaces sequence placeholders or, with autoAppend, adds
// the index before the first dot in the entire path, matching upstream exactly.
func ExpandOutputTemplate(template string, index int, autoAppend bool) string {
	sequence := strconv.Itoa(index)
	if strings.Contains(template, TraceSequencePlaceholder) {
		return strings.ReplaceAll(template, TraceSequencePlaceholder, sequence)
	}
	if !autoAppend {
		return template
	}
	if dot := strings.IndexByte(template, '.'); dot >= 0 {
		return template[:dot] + sequence + template[dot:]
	}
	return template + sequence
}

// ExpandNamedOutputTemplate substitutes the test name before sequence expansion.
// Names follow JavaScript replacement-string dollar substitutions, as upstream.
func ExpandNamedOutputTemplate(template, name string, index int, autoAppend bool) string {
	return ExpandOutputTemplate(replaceTraceTestName(template, name), index, autoAppend)
}

// ITFHeader supplies the source/status and two caller-owned clock values used by
// upstream addItfHeader. CreatedAt is the display date text; Timestamp is Unix
// milliseconds. Keeping clocks outside this function makes output reproducible
// and allows the CLI to choose its date/timezone presentation.
type ITFHeader struct {
	Source    string
	Status    string
	CreatedAt string
	Timestamp int64
}

// AddITFHeader adds Quint's standard trace metadata. Existing #meta replaces the
// generated header, as upstream's object spread does. Loop indexes, parameters,
// custom fields, raw values, and property enumeration order are preserved.
func AddITFHeader(trace []byte, header ITFHeader) (json.RawMessage, error) {
	data := bytes.TrimSpace(trace)
	if len(data) == 0 || data[0] != '{' || !json.Valid(data) {
		return nil, fmt.Errorf("ITF header requires a trace JSON object")
	}
	fields, err := itfObject(data)
	if err != nil {
		return nil, err
	}
	metadata, err := json.Marshal(struct {
		Format            string `json:"format"`
		FormatDescription string `json:"format-description"`
		Source            string `json:"source"`
		Status            string `json:"status"`
		Description       string `json:"description"`
		Timestamp         int64  `json:"timestamp"`
	}{"ITF", "https://apalache-mc.org/docs/adr/015adr-trace.html", header.Source, header.Status, "Created by Quint on " + header.CreatedAt, header.Timestamp})
	if err != nil {
		return nil, err
	}
	output := insdict.NewDict[string, json.RawMessage]()
	output.Set("#meta", metadata)
	for name, value := range fields.All() {
		output.Set(name, value)
	}
	return json.Marshal(itfEncodedObject{output})
}

func replaceTraceTestName(template, name string) string {
	var out strings.Builder
	for from := 0; ; {
		offset := strings.Index(template[from:], TraceTestPlaceholder)
		if offset < 0 {
			out.WriteString(template[from:])
			break
		}
		index := from + offset
		end := index + len(TraceTestPlaceholder)
		out.WriteString(template[from:index])
		for i := 0; i < len(name); i++ {
			if name[i] == '$' && i+1 < len(name) {
				switch name[i+1] {
				case '$':
					out.WriteByte('$')
					i++
					continue
				case '&':
					out.WriteString(TraceTestPlaceholder)
					i++
					continue
				case '`':
					out.WriteString(template[:index])
					i++
					continue
				case '\'':
					out.WriteString(template[end:])
					i++
					continue
				}
			}
			out.WriteByte(name[i])
		}
		from = end
	}
	return out.String()
}
