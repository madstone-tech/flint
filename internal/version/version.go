// Package version holds build-time metadata, populated via -ldflags -X at
// build/release time (see Taskfile.yml and .goreleaser.yaml).
package version

import (
	"fmt"
	"runtime"
)

// These are overwritten at build time with:
//
//	-X github.com/madstone-tech/flint/internal/version.Version=...
//
// For plain `go build`/`go run` they keep their default values.
var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
	BuiltBy = "source"
)

// String returns a human-readable, multi-line version summary.
func String() string {
	return fmt.Sprintf(
		"flint %s\n  commit:   %s\n  built:    %s\n  built by: %s\n  go:       %s %s/%s",
		Version, Commit, Date, BuiltBy,
		runtime.Version(), runtime.GOOS, runtime.GOARCH,
	)
}

// Short returns the version string on its own, for example the `--version`
// output on one line.
func Short() string {
	return Version
}
