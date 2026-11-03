package tlc

import "github.com/glycerine/quint-go/pkg/backend"

// CheckerRevision may be set at build time with -ldflags=-X=.../tlc.CheckerRevision=REVISION.
// It must identify the actual native Go TLC source used by the build. Leaving it
// unset reports an unknown revision, including local workspace builds.
var CheckerRevision string

// VersionInfo reports native Go TLC and the Go Apalache TLA printer dependency
// without initializing either checker. Native TLC's embedding-binary build date
// and VCS revision do not identify the linked TLC dependency.
func VersionInfo() backend.VersionInfo {
	return backend.ReadVersionInfo("tlc-go", CheckerRevision,
		"github.com/glycerine/tlago", "github.com/glycerine/apalache-go")
}
