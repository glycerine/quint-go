//go:build windows

package terminal

import (
	"fmt"

	"golang.org/x/sys/windows"
)

func hostOSRelease() string {
	version := windows.RtlGetVersion()
	return fmt.Sprintf("%d.%d.%d", version.MajorVersion, version.MinorVersion, version.BuildNumber)
}
