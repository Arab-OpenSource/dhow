// Package buildinfo exposes the version stamped into Dhow binaries at build time.
package buildinfo

import "runtime/debug"

// Set with -ldflags "-X github.com/Arab-OpenSource/dhow/internal/buildinfo.version=v1.2.3".
var (
	version = "dev"
	commit  = ""
)

// Info describes the running binary.
type Info struct {
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	GoVersion string `json:"goVersion"`
}

// Get returns the build information, falling back to VCS data embedded by the Go toolchain.
func Get() Info {
	info := Info{Version: version, Commit: commit}
	if bi, ok := debug.ReadBuildInfo(); ok {
		info.GoVersion = bi.GoVersion
		if info.Commit == "" {
			for _, s := range bi.Settings {
				if s.Key == "vcs.revision" {
					info.Commit = s.Value
				}
			}
		}
	}
	return info
}
