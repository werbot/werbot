// Package version holds build metadata, set via -ldflags -X.
package version

// Build metadata; defaults apply to non-release builds.
var (
	Version   = "dev"
	Commit    = "none"
	BuildDate = "unknown"
)
