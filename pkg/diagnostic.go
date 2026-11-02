package quint

import (
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

type ErrorMessage struct {
	Explanation    string     `json:"explanation"`
	Locations      []Location `json:"locs"`
	TraceLocations []Location `json:"traceLocs,omitempty"`
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
