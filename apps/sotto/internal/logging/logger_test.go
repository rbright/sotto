package logging

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestResolveLogPathUsesXDGStateHome(t *testing.T) {
	xdgStateHome := t.TempDir()
	t.Setenv("XDG_STATE_HOME", xdgStateHome)
	t.Setenv("HOME", t.TempDir())

	path, err := resolveLogPath()
	require.NoError(t, err)
	require.Equal(t, filepath.Join(xdgStateHome, "sotto", "log.jsonl"), path)
}

func TestResolveLogPathFallsBackToHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv("XDG_STATE_HOME", "")
	t.Setenv("HOME", home)

	path, err := resolveLogPath()
	require.NoError(t, err)
	require.Equal(t, filepath.Join(home, ".local", "state", "sotto", "log.jsonl"), path)
}

func TestNewCreatesWritableJSONLogFile(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	runtime, err := New()
	require.NoError(t, err)

	runtime.Logger.Info("unit-test-log", "component", "logging")
	require.NoError(t, runtime.Close())

	contents, err := os.ReadFile(runtime.Path)
	require.NoError(t, err)
	require.Contains(t, string(contents), `"msg":"unit-test-log"`)
	require.Contains(t, string(contents), `"component":"logging"`)

	stat, err := os.Stat(runtime.Path)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o600), stat.Mode().Perm())
}
