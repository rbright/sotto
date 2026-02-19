package hypr

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestQueryActiveWindowAndFocusedMonitor(t *testing.T) {
	argsFile := filepath.Join(t.TempDir(), "hypr-args.log")
	t.Setenv("HYPR_ARGS_FILE", argsFile)
	installHyprctlStub(t, `
if [[ "${1:-}" == "-j" && "${2:-}" == "activewindow" ]]; then
  echo '{"address":" 0xabc ","class":" brave-browser ","initialClass":" Brave "}'
  exit 0
fi
if [[ "${1:-}" == "-j" && "${2:-}" == "monitors" ]]; then
  echo '[{"name":"HDMI-A-1","focused":false},{"name":" DP-1 ","focused":true}]'
  exit 0
fi
printf '%s\n' "$*" >> "${HYPR_ARGS_FILE}"
`)

	window, err := QueryActiveWindow(context.Background())
	require.NoError(t, err)
	require.Equal(t, "0xabc", window.Address)
	require.Equal(t, "brave-browser", window.Class)
	require.Equal(t, "Brave", window.InitialClass)

	monitor, err := QueryFocusedMonitor(context.Background())
	require.NoError(t, err)
	require.Equal(t, "DP-1", monitor)
}

func TestQueryActiveWindowRejectsEmptyAddress(t *testing.T) {
	installHyprctlStub(t, `
if [[ "${1:-}" == "-j" && "${2:-}" == "activewindow" ]]; then
  echo '{"address":"","class":"brave"}'
  exit 0
fi
echo '[]'
`)

	_, err := QueryActiveWindow(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "empty address")
}

func TestSendShortcutRequiresNonEmptyPayload(t *testing.T) {
	err := SendShortcut(context.Background(), " ")
	require.Error(t, err)
	require.Contains(t, err.Error(), "non-empty payload")
}

func TestNotifyAndDismissUseHyprctlDispatch(t *testing.T) {
	argsFile := filepath.Join(t.TempDir(), "hypr-args.log")
	t.Setenv("HYPR_ARGS_FILE", argsFile)
	installHyprctlStub(t, `
printf '%s\n' "$*" >> "${HYPR_ARGS_FILE}"
`)

	err := Notify(context.Background(), 3, 1200, "", "Speech recognition error")
	require.NoError(t, err)

	err = DismissNotify(context.Background())
	require.NoError(t, err)

	data, err := os.ReadFile(argsFile)
	require.NoError(t, err)
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	require.Len(t, lines, 2)
	require.Equal(t, "--quiet dispatch notify 3 1200 rgb(89b4fa) Speech recognition error", lines[0])
	require.Equal(t, "--quiet dispatch dismissnotify", lines[1])
}

func TestSendShortcutReturnsCombinedOutputOnFailure(t *testing.T) {
	installHyprctlStub(t, `
echo 'boom from hyprctl' >&2
exit 1
`)

	err := SendShortcut(context.Background(), "CTRL,V,address:0xabc")
	require.Error(t, err)
	require.Contains(t, err.Error(), "boom from hyprctl")
}

func installHyprctlStub(t *testing.T, body string) {
	t.Helper()

	dir := t.TempDir()
	path := filepath.Join(dir, "hyprctl")
	script := "#!/usr/bin/env bash\nset -euo pipefail\n" + body + "\n"
	require.NoError(t, os.WriteFile(path, []byte(script), 0o755))
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))
}
