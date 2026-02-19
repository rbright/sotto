package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestResolvePathPrecedence(t *testing.T) {
	explicit := "/tmp/custom.jsonc"
	resolved, err := ResolvePath(explicit)
	require.NoError(t, err)
	require.Equal(t, explicit, resolved)

	xdg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)
	resolved, err = ResolvePath("")
	require.NoError(t, err)
	require.Equal(t, filepath.Join(xdg, "sotto", "config.jsonc"), resolved)

	t.Setenv("XDG_CONFIG_HOME", "")
	home := t.TempDir()
	t.Setenv("HOME", home)
	resolved, err = ResolvePath("")
	require.NoError(t, err)
	require.Equal(t, filepath.Join(home, ".config", "sotto", "config.jsonc"), resolved)
}

func TestLoadMissingConfigUsesDefaultsWithWarning(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.jsonc")

	loaded, err := Load(path)
	require.NoError(t, err)
	require.Equal(t, path, loaded.Path)
	require.False(t, loaded.Exists)
	require.Equal(t, Default(), loaded.Config)
	require.NotEmpty(t, loaded.Warnings)
	require.Contains(t, loaded.Warnings[0].Message, "not found")
}

func TestLoadExistingJSONCParsesAndValidates(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.jsonc")
	contents := `
{
  "riva": {
    "grpc": "127.0.0.1:50051",
    "http": "127.0.0.1:9000"
  },
  "audio": {
    "input": "default",
    "fallback": "default"
  },
  "paste": {
    "enable": false
  }
}
`
	require.NoError(t, os.WriteFile(path, []byte(contents), 0o600))

	loaded, err := Load(path)
	require.NoError(t, err)
	require.True(t, loaded.Exists)
	require.Equal(t, path, loaded.Path)
	require.Equal(t, "127.0.0.1:50051", loaded.Config.RivaGRPC)
	require.Equal(t, "127.0.0.1:9000", loaded.Config.RivaHTTP)
	require.False(t, loaded.Config.Paste.Enable)
}

func TestLoadImplicitPathFallsBackToLegacyConfigConf(t *testing.T) {
	xdg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)

	legacyPath := filepath.Join(xdg, "sotto", "config.conf")
	require.NoError(t, os.MkdirAll(filepath.Dir(legacyPath), 0o700))
	require.NoError(t, os.WriteFile(legacyPath, []byte("paste.enable = false\n"), 0o600))

	loaded, err := Load("")
	require.NoError(t, err)
	require.True(t, loaded.Exists)
	require.Equal(t, legacyPath, loaded.Path)
	require.False(t, loaded.Config.Paste.Enable)
	require.NotEmpty(t, loaded.Warnings)
	require.Contains(t, loaded.Warnings[0].Message, "legacy config path")
}

func TestLoadParseErrorIncludesPath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "broken.jsonc")
	require.NoError(t, os.WriteFile(path, []byte("{ not-json }"), 0o600))

	_, err := Load(path)
	require.Error(t, err)
	require.Contains(t, err.Error(), "parse config")
	require.Contains(t, err.Error(), path)
}
