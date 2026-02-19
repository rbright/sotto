package indicator

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rbright/sotto/internal/config"
	"github.com/stretchr/testify/require"
)

func TestHyprNotifyDispatchAndFocusedMonitorTracking(t *testing.T) {
	argsFile := filepath.Join(t.TempDir(), "hypr-args.log")
	t.Setenv("HYPR_ARGS_FILE", argsFile)
	installHyprctlStub(t, `
if [[ "${1:-}" == "-j" && "${2:-}" == "monitors" ]]; then
  echo '[{"name":"DP-1","focused":true}]'
  exit 0
fi
printf '%s\n' "$*" >> "${HYPR_ARGS_FILE}"
`)

	cfg := config.Default().Indicator
	cfg.SoundEnable = false
	cfg.Enable = true
	cfg.TextRecording = "Recording"
	cfg.TextProcessing = "Transcribing"
	cfg.TextError = "Speech error"

	notify := NewHyprNotify(cfg, nil)
	notify.ShowRecording(context.Background())
	notify.ShowTranscribing(context.Background())
	notify.ShowError(context.Background(), "")
	notify.Hide(context.Background())

	require.Equal(t, "DP-1", notify.FocusedMonitor())

	data, err := os.ReadFile(argsFile)
	require.NoError(t, err)
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	require.Len(t, lines, 4)
	require.Equal(t, "--quiet dispatch notify 1 300000 rgb(89b4fa) Recording", lines[0])
	require.Equal(t, "--quiet dispatch notify 1 300000 rgb(cba6f7) Transcribing", lines[1])
	require.Equal(t, "--quiet dispatch notify 3 1600 rgb(f38ba8) Speech error", lines[2])
	require.Equal(t, "--quiet dispatch dismissnotify", lines[3])
}

func TestHyprNotifyShowErrorUsesProvidedTextAndDefaultTimeout(t *testing.T) {
	argsFile := filepath.Join(t.TempDir(), "hypr-args.log")
	t.Setenv("HYPR_ARGS_FILE", argsFile)
	installHyprctlStub(t, `
printf '%s\n' "$*" >> "${HYPR_ARGS_FILE}"
`)

	cfg := config.Default().Indicator
	cfg.SoundEnable = false
	cfg.ErrorTimeoutMS = 0 // exercises fallback to 1200ms

	notify := NewHyprNotify(cfg, nil)
	notify.ShowError(context.Background(), "custom error")

	data, err := os.ReadFile(argsFile)
	require.NoError(t, err)
	require.Equal(t, "--quiet dispatch notify 3 1200 rgb(f38ba8) custom error\n", string(data))
}

func TestHyprNotifyDisabledSkipsHyprctlDispatch(t *testing.T) {
	argsFile := filepath.Join(t.TempDir(), "hypr-args.log")
	t.Setenv("HYPR_ARGS_FILE", argsFile)
	installHyprctlStub(t, `
printf '%s\n' "$*" >> "${HYPR_ARGS_FILE}"
`)

	cfg := config.Default().Indicator
	cfg.Enable = false
	cfg.SoundEnable = false

	notify := NewHyprNotify(cfg, nil)
	notify.ShowRecording(context.Background())
	notify.ShowTranscribing(context.Background())
	notify.ShowError(context.Background(), "ignored")
	notify.Hide(context.Background())

	_, err := os.Stat(argsFile)
	require.Error(t, err)
	require.True(t, os.IsNotExist(err))
}

func TestFocusedMonitorStaysEmptyWhenQueryFails(t *testing.T) {
	installHyprctlStub(t, `
exit 1
`)

	cfg := config.Default().Indicator
	cfg.Enable = true
	cfg.SoundEnable = false

	notify := NewHyprNotify(cfg, nil)
	notify.ShowRecording(context.Background())
	require.Empty(t, notify.FocusedMonitor())
}

func installHyprctlStub(t *testing.T, body string) {
	t.Helper()

	dir := t.TempDir()
	path := filepath.Join(dir, "hyprctl")
	script := "#!/usr/bin/env bash\nset -euo pipefail\n" + body + "\n"
	require.NoError(t, os.WriteFile(path, []byte(script), 0o755))
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))
}
