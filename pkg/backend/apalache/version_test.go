package apalache

import (
	"testing"

	"github.com/glycerine/quint-go/pkg/backend"
)

func TestVersionInfoIdentifiesGoCheckerAndSolver(t *testing.T) {
	t.Parallel()
	info := VersionInfo()
	if info.Backend != "apalache-go" || info.Revision == "" || len(info.Modules) != 2 || info.Modules[0].Path != "github.com/glycerine/apalache-go" || info.Modules[1].Path != "github.com/glycerine/g3/pkg" {
		t.Fatal(info)
	}
	for _, module := range info.Modules {
		if module.Status != backend.ModuleVersioned && module.Version != backend.UnknownVersion {
			t.Fatal("unknown/local source acquired a false release version", module)
		}
	}
}
