// Package version exposes build metadata used by `sotto version`.
package version

import "runtime"

var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)

// String returns build metadata in the user-facing version output format.
func String() string {
	return "sotto " + Version + " (commit=" + Commit + ", date=" + Date + ", go=" + runtime.Version() + ")"
}
