// Package backend provides metadata shared by the in-process Go checkers.
package backend

import (
	"runtime/debug"
	"strings"
)

const UnknownVersion = "unknown"

type ModuleVersionStatus string

const (
	ModuleVersioned ModuleVersionStatus = "versioned"
	ModuleLocal     ModuleVersionStatus = "local"
	ModuleUnknown   ModuleVersionStatus = "unknown"
)

// ModuleVersion identifies the source selected for a linked dependency. Version
// and Sum describe the effective replacement when one is present. A local
// workspace or replacement has an unknown version; RequestedVersion retains
// the original module requirement without presenting it as the linked version.
type ModuleVersion struct {
	Path             string              `json:"path"`
	Version          string              `json:"version"`
	Status           ModuleVersionStatus `json:"status"`
	Sum              string              `json:"sum,omitempty"`
	Replacement      string              `json:"replacement,omitempty"`
	RequestedVersion string              `json:"requestedVersion,omitempty"`
}

// VersionInfo reports linked module identities and an optional build-time
// checker revision. Module versions and checksums do not imply an exact source
// revision. Unknown revisions remain explicit, including local workspace builds.
type VersionInfo struct {
	Backend  string          `json:"backend"`
	Revision string          `json:"revision"`
	Modules  []ModuleVersion `json:"modules"`
}

// ReadVersionInfo reads the executable's Go build metadata. revision is an
// explicit checker revision supplied by its build, not the executable's VCS
// revision. No filesystem, environment, network, or checker initialization is
// needed. Missing dependency metadata is reported as unknown.
func ReadVersionInfo(name, revision string, modulePaths ...string) VersionInfo {
	info, _ := debug.ReadBuildInfo()
	return versionInfo(info, name, revision, modulePaths)
}

func versionInfo(info *debug.BuildInfo, name, revision string, modulePaths []string) VersionInfo {
	revision = strings.TrimSpace(revision)
	if revision == "" {
		revision = UnknownVersion
	}
	result := VersionInfo{Backend: name, Revision: revision, Modules: make([]ModuleVersion, 0, len(modulePaths))}
	for _, path := range modulePaths {
		entry := ModuleVersion{Path: path, Version: UnknownVersion, Status: ModuleUnknown}
		module := findModule(info, path)
		if module != nil {
			selected := module
			if module.Replace != nil {
				entry.RequestedVersion = module.Version
				selected = module.Replace
				entry.Replacement = selected.Path
			}
			if selected.Version == "" || selected.Version == "(devel)" {
				entry.Status = ModuleLocal
			} else {
				entry.Version, entry.Sum, entry.Status = selected.Version, selected.Sum, ModuleVersioned
			}
		}
		result.Modules = append(result.Modules, entry)
	}
	return result
}

func findModule(info *debug.BuildInfo, path string) *debug.Module {
	if info == nil {
		return nil
	}
	if info.Main.Path == path {
		return &info.Main
	}
	for _, module := range info.Deps {
		if module != nil && module.Path == path {
			return module
		}
	}
	return nil
}
