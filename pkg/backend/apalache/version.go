package apalache

import "github.com/glycerine/quint-go/pkg/backend"

// CheckerRevision may be set at build time with -ldflags=-X=.../apalache.CheckerRevision=REVISION.
// It must identify the actual Go Apalache source used by the build. Leaving it
// unset reports an unknown revision, including local workspace builds.
var CheckerRevision string

// VersionInfo reports Go Apalache and its linked SMT implementation without
// starting a checker. The exact checker revision is optional build metadata;
// neither a module version nor Quint's own revision substitutes for it.
func VersionInfo() backend.VersionInfo {
	return backend.ReadVersionInfo("apalache-go", CheckerRevision,
		"github.com/glycerine/apalache-go", "github.com/glycerine/g3/pkg")
}
