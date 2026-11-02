package cli

import (
	"context"
	"strings"

	quint "github.com/glycerine/quint-go/pkg"
)

func replSaveCommand(ctx context.Context, session *quint.REPLSession, fields []string, out func(string)) {
	if len(fields) < 2 {
		out(".save requires a filename\n")
		return
	}
	if err := session.Save(ctx, fields[1]); err != nil {
		out(err.Error() + "\n")
		return
	}
	out("Session saved to: " + fields[1] + "\n")
}

func replReplayOutput(level func() int, out func(string), report func(error), render func(quint.REPLResult), writeErr *error) quint.REPLReplayHooks {
	first := true
	return quint.REPLReplayHooks{
		Before: func(filename, input string) error {
			if level() > 0 {
				if first {
					out("Evaluating expression history in " + filename + "\n")
				}
				out(">>> " + strings.ReplaceAll(input, "\n", "\n... ") + "\n")
			}
			first = false
			return *writeErr
		},
		After: func(entry quint.REPLReplayResult) error {
			if entry.Err != nil {
				report(entry.Err)
			} else {
				render(entry.Result)
			}
			return *writeErr
		},
	}
}
