package version

import "runtime"

var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)

func String() string {
	return "sotto " + Version + " (commit=" + Commit + ", date=" + Date + ", go=" + runtime.Version() + ")"
}
