package backend

import (
	"encoding/json"
	"reflect"
	"runtime/debug"
	"testing"
)

func TestVersionInfoUsesSelectedModuleSources(t *testing.T) {
	t.Parallel()
	info := &debug.BuildInfo{
		Main: debug.Module{Path: "example/quint", Version: "v9.0.0"},
		Deps: []*debug.Module{
			{Path: "example/released", Version: "v1.2.3", Sum: "h1:release"},
			{Path: "example/workspace", Version: "(devel)"},
			{Path: "example/local", Version: "v1.0.0", Sum: "h1:requested", Replace: &debug.Module{Path: "../checker"}},
			{Path: "example/fork", Version: "v2.0.0", Sum: "h1:requested", Replace: &debug.Module{Path: "example/replacement", Version: "v3.1.0", Sum: "h1:effective"}},
			{Path: "example/pseudo", Version: "v0.0.0-20260102030405-abcdef012345"},
			nil,
		},
		Settings: []debug.BuildSetting{{Key: "vcs.revision", Value: "quint-revision-is-not-a-checker-revision"}},
	}
	paths := []string{"example/fork", "example/workspace", "example/released", "example/local", "example/missing", "example/pseudo", "example/quint"}
	got := versionInfo(info, "checker", "", paths)
	want := VersionInfo{Backend: "checker", Revision: UnknownVersion, Modules: []ModuleVersion{
		{Path: "example/fork", Version: "v3.1.0", Status: ModuleVersioned, Sum: "h1:effective", Replacement: "example/replacement", RequestedVersion: "v2.0.0"},
		{Path: "example/workspace", Version: UnknownVersion, Status: ModuleLocal},
		{Path: "example/released", Version: "v1.2.3", Status: ModuleVersioned, Sum: "h1:release"},
		{Path: "example/local", Version: UnknownVersion, Status: ModuleLocal, Replacement: "../checker", RequestedVersion: "v1.0.0"},
		{Path: "example/missing", Version: UnknownVersion, Status: ModuleUnknown},
		{Path: "example/pseudo", Version: "v0.0.0-20260102030405-abcdef012345", Status: ModuleVersioned},
		{Path: "example/quint", Version: "v9.0.0", Status: ModuleVersioned},
	}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v; want %+v", got, want)
	}
	// Results own their slices and values; callers cannot change a future report
	// or the build metadata used to construct one.
	got.Modules[0].Version = "changed"
	again := versionInfo(info, "checker", "", paths)
	if !reflect.DeepEqual(again, want) || info.Deps[3].Replace.Version != "v3.1.0" {
		t.Fatal("version report aliases mutable build metadata", again)
	}
}

func TestVersionInfoMissingMetadataAndExplicitRevision(t *testing.T) {
	t.Parallel()
	for _, info := range []*debug.BuildInfo{nil, {}} {
		for _, revision := range []string{"", "  ", " exact-checker-revision "} {
			got := versionInfo(info, "checker", revision, []string{"example/checker"})
			wantRevision := UnknownVersion
			if revision == " exact-checker-revision " {
				wantRevision = "exact-checker-revision"
			}
			if got.Revision != wantRevision || len(got.Modules) != 1 || got.Modules[0].Version != UnknownVersion || got.Modules[0].Status != ModuleUnknown {
				t.Fatal(got)
			}
			encoded, err := json.Marshal(got)
			if err != nil || !json.Valid(encoded) {
				t.Fatal(string(encoded), err)
			}
		}
	}
	got := ReadVersionInfo("checker", "explicit", "example/module-not-linked-by-this-test")
	if got.Backend != "checker" || got.Revision != "explicit" || got.Modules[0].Status != ModuleUnknown {
		t.Fatal(got)
	}
}
