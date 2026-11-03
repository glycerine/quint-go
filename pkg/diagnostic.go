package quint

import (
	"encoding/json"
	"strings"
)

type Diagnostic struct {
	Code      string          `json:"code"`
	Message   string          `json:"message"`
	Reference *ID             `json:"reference,omitempty"`
	Trace     []ID            `json:"trace,omitempty"`
	Data      *DiagnosticData `json:"data,omitempty"`
}

func (d Diagnostic) Error() string { return "[" + d.Code + "] " + d.Message }

// MarshalJSON preserves the difference between an absent stack and an explicit
// empty stack, as serialized by the TypeScript CLI and revived Rust errors.
func (d Diagnostic) MarshalJSON() ([]byte, error) {
	var trace *[]ID
	if d.Trace != nil {
		trace = &d.Trace
	}
	return json.Marshal(struct {
		Code      string          `json:"code"`
		Message   string          `json:"message"`
		Reference *ID             `json:"reference,omitempty"`
		Trace     *[]ID           `json:"trace,omitempty"`
		Data      *DiagnosticData `json:"data,omitempty"`
	}{d.Code, d.Message, d.Reference, trace, d.Data})
}

type ErrorMessage struct {
	Explanation    string     `json:"explanation"`
	Locations      []Location `json:"locs"`
	TraceLocations []Location `json:"traceLocs,omitempty"`
}

// MarshalJSON retains an explicitly empty resolved stack. It occurs when the
// original stack has caller frames but none has an available source location.
func (e ErrorMessage) MarshalJSON() ([]byte, error) {
	locations := e.Locations
	if locations == nil {
		locations = []Location{}
	}
	var trace *[]Location
	if e.TraceLocations != nil {
		trace = &e.TraceLocations
	}
	return json.Marshal(struct {
		Explanation    string      `json:"explanation"`
		Locations      []Location  `json:"locs"`
		TraceLocations *[]Location `json:"traceLocs,omitempty"`
	}{e.Explanation, locations, trace})
}

// ResolveDiagnostic follows upstream: the first trace ID takes precedence over
// Reference even when its location is unavailable. Remaining unknown frames
// are omitted while known frames retain their order.
func ResolveDiagnostic(sources SourceMap, d Diagnostic) ErrorMessage {
	result := ErrorMessage{Explanation: d.Error(), Locations: []Location{}}
	reference := d.Reference
	if len(d.Trace) > 0 {
		reference = &d.Trace[0]
	}
	if reference != nil && *reference != (ID{}) {
		if loc, ok := sources[*reference]; ok {
			result.Locations = append(result.Locations, loc)
		}
	}
	if len(d.Trace) > 1 {
		result.TraceLocations = []Location{}
	}
	for _, id := range d.Trace[minimum(1, len(d.Trace)):] {
		if loc, ok := sources[id]; ok {
			result.TraceLocations = append(result.TraceLocations, loc)
		}
	}
	return result
}
func minimum(a, b int) int {
	if a < b {
		return a
	}
	return b
}

type ErrorTree struct {
	Message  *string     `json:"message,omitempty"`
	Location string      `json:"location"`
	Children []ErrorTree `json:"children"`
}

func ErrorLeaf(location, message string) ErrorTree {
	return ErrorTree{Message: &message, Location: location, Children: []ErrorTree{}}
}
func WrapError(location string, child ErrorTree) ErrorTree {
	if child.Location == location {
		return child
	}
	return ErrorTree{Location: location, Children: []ErrorTree{child}}
}
func (e ErrorTree) String() string {
	var out strings.Builder
	for i, child := range e.Children {
		if i > 0 {
			out.WriteString("and\n")
		}
		out.WriteString(child.String())
	}
	if e.Message != nil && *e.Message != "" {
		out.WriteString(*e.Message)
		out.WriteByte('\n')
	}
	out.WriteString(e.Location)
	out.WriteByte('\n')
	return out.String()
}

// DiagnosticData carries machine-readable fixes and nested inference errors.
type DiagnosticData struct {
	Fix   *DiagnosticFix `json:"fix,omitempty"`
	Trace *ErrorTree     `json:"trace,omitempty"`
}
type DiagnosticFix struct {
	Kind        string `json:"kind"`
	Original    string `json:"original"`
	Replacement string `json:"replacement"`
}
