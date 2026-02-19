package output

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestBuildPasteShortcut(t *testing.T) {
	t.Parallel()

	t.Run("builds payload", func(t *testing.T) {
		got, err := buildPasteShortcut("SUPER,V", "0xabc")
		require.NoError(t, err)
		require.Equal(t, "SUPER,V,address:0xabc", got)
	})

	t.Run("rejects empty shortcut", func(t *testing.T) {
		_, err := buildPasteShortcut("", "0xabc")
		require.Error(t, err)
		require.Contains(t, err.Error(), "shortcut")
	})

	t.Run("rejects empty address", func(t *testing.T) {
		_, err := buildPasteShortcut("CTRL,V", "")
		require.Error(t, err)
		require.Contains(t, err.Error(), "address")
	})
}

func TestDefaultPasteDispatchesShortcut(t *testing.T) {
	argsFile := filepath.Join(t.TempDir(), "hypr-args.log")
	t.Setenv("HYPR_ARGS_FILE", argsFile)
	t.Setenv("HYPR_ACTIVEWINDOW_JSON", `{"address":"0xabc","class":"ghostty","initialClass":"ghostty"}`)
	installHyprctlPasteStub(t)

	err := defaultPaste(context.Background(), "SUPER,V")
	require.NoError(t, err)

	data, err := os.ReadFile(argsFile)
	require.NoError(t, err)
	require.Contains(t, string(data), "--quiet dispatch sendshortcut SUPER,V,address:0xabc")
}

func TestActiveWindowWithRetryHonorsContextCancel(t *testing.T) {
	emptyPathDir := t.TempDir()
	t.Setenv("PATH", emptyPathDir)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := activeWindowWithRetry(ctx, 3, 10*time.Millisecond)
	require.ErrorIs(t, err, context.Canceled)
}

func TestDefaultPasteFailsWhenActiveWindowAddressMissing(t *testing.T) {
	t.Setenv("HYPR_ACTIVEWINDOW_JSON", `{"address":"","class":"brave-browser"}`)
	installHyprctlPasteStub(t)

	err := defaultPaste(context.Background(), "CTRL,V")
	require.Error(t, err)
	require.Contains(t, err.Error(), "empty address")
}

func installHyprctlPasteStub(t *testing.T) {
	t.Helper()

	dir := t.TempDir()
	path := filepath.Join(dir, "hyprctl")
	script := `#!/usr/bin/env bash
set -euo pipefail
if [[ "${1:-}" == "-j" && "${2:-}" == "activewindow" ]]; then
  if [[ -n "${HYPR_ACTIVEWINDOW_JSON:-}" ]]; then
    echo "${HYPR_ACTIVEWINDOW_JSON}"
  else
    echo '{"address":"0xabc","class":"brave-browser","initialClass":"brave-browser"}'
  fi
  exit 0
fi
printf '%s\n' "$*" >> "${HYPR_ARGS_FILE}"
`
	require.NoError(t, os.WriteFile(path, []byte(strings.TrimSpace(script)+"\n"), 0o755))
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))
}
