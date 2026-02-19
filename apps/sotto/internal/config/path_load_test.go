package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestResolvePathPrecedence(t *testing.T) {
	explicit := "/tmp/custom.conf"
	resolved, err := ResolvePath(explicit)
	require.NoError(t, err)
	require.Equal(t, explicit, resolved)

	xdg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)
	resolved, err = ResolvePath("")
	require.NoError(t, err)
	require.Equal(t, filepath.Join(xdg, "sotto", "config.conf"), resolved)

	t.Setenv("XDG_CONFIG_HOME", "")
	home := t.TempDir()
	t.Setenv("HOME", home)
	resolved, err = ResolvePath("")
	require.NoError(t, err)
	require.Equal(t, filepath.Join(home, ".config", "sotto", "config.conf"), resolved)
}

func TestLoadMissingConfigUsesDefaultsWithWarning(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.conf")

	loaded, err := Load(path)
	require.NoError(t, err)
	require.Equal(t, path, loaded.Path)
	require.False(t, loaded.Exists)
	require.Equal(t, Default(), loaded.Config)
	require.NotEmpty(t, loaded.Warnings)
	require.Contains(t, loaded.Warnings[0].Message, "not found")
}

func TestLoadExistingConfigParsesAndValidates(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.conf")
	contents := `
riva_grpc = 127.0.0.1:50051
riva_http = 127.0.0.1:9000
audio.input = default
audio.fallback = default
paste.enable = false
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

func TestLoadParseErrorIncludesPath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "broken.conf")
	require.NoError(t, os.WriteFile(path, []byte("bad line"), 0o600))

	_, err := Load(path)
	require.Error(t, err)
	require.Contains(t, err.Error(), "parse config")
	require.Contains(t, err.Error(), path)
}
