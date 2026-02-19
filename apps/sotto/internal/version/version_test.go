package version

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStringIncludesBuildMetadata(t *testing.T) {
	originalVersion := Version
	originalCommit := Commit
	originalDate := Date
	t.Cleanup(func() {
		Version = originalVersion
		Commit = originalCommit
		Date = originalDate
	})

	Version = "1.2.3"
	Commit = "abc123"
	Date = "2026-02-18"

	got := String()
	require.Contains(t, got, "sotto 1.2.3")
	require.Contains(t, got, "commit=abc123")
	require.Contains(t, got, "date=2026-02-18")
	require.Contains(t, got, "go=")
}
