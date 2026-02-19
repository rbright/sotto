package output

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rbright/sotto/internal/config"
	"github.com/stretchr/testify/require"
)

func TestRunCommandWithInputWritesStdin(t *testing.T) {
	scriptPath := writeStdinCaptureScript(t)
	outputPath := filepath.Join(t.TempDir(), "stdin.txt")

	err := runCommandWithInput(context.Background(), []string{scriptPath, outputPath}, "hello from sotto")
	require.NoError(t, err)

	data, err := os.ReadFile(outputPath)
	require.NoError(t, err)
	require.Equal(t, "hello from sotto", string(data))
}

func TestRunCommandWithInputRejectsEmptyArgv(t *testing.T) {
	err := runCommandWithInput(context.Background(), nil, "payload")
	require.Error(t, err)
	require.Contains(t, err.Error(), "argv cannot be empty")
}

func TestCommitterCommitWritesClipboardWhenPasteDisabled(t *testing.T) {
	scriptPath := writeStdinCaptureScript(t)
	clipboardPath := filepath.Join(t.TempDir(), "clipboard.txt")

	cfg := config.Default()
	cfg.Paste.Enable = false
	cfg.Clipboard = config.CommandConfig{Argv: []string{scriptPath, clipboardPath}}

	committer := NewCommitter(cfg, nil)
	err := committer.Commit(context.Background(), "captured transcript")
	require.NoError(t, err)

	data, err := os.ReadFile(clipboardPath)
	require.NoError(t, err)
	require.Equal(t, "captured transcript", string(data))
}

func TestCommitterCommitSkipsEmptyTranscript(t *testing.T) {
	scriptPath := writeStdinCaptureScript(t)
	clipboardPath := filepath.Join(t.TempDir(), "clipboard.txt")

	cfg := config.Default()
	cfg.Paste.Enable = false
	cfg.Clipboard = config.CommandConfig{Argv: []string{scriptPath, clipboardPath}}

	committer := NewCommitter(cfg, nil)
	err := committer.Commit(context.Background(), "")
	require.NoError(t, err)

	_, statErr := os.Stat(clipboardPath)
	require.Error(t, statErr)
	require.True(t, os.IsNotExist(statErr))
}

func TestCommitterCommitReturnsErrorWhenClipboardCommandFails(t *testing.T) {
	failScript := writeFailScript(t, "clipboard failed")

	cfg := config.Default()
	cfg.Paste.Enable = false
	cfg.Clipboard = config.CommandConfig{Argv: []string{failScript}}

	committer := NewCommitter(cfg, nil)
	err := committer.Commit(context.Background(), "captured transcript")
	require.Error(t, err)
	require.Contains(t, err.Error(), "set clipboard")
}

func TestCommitterCommitPasteCmdFailureDoesNotFailCommit(t *testing.T) {
	clipboardScript := writeStdinCaptureScript(t)
	clipboardPath := filepath.Join(t.TempDir(), "clipboard.txt")
	pasteFailScript := writeFailScript(t, "paste failed")

	cfg := config.Default()
	cfg.Clipboard = config.CommandConfig{Argv: []string{clipboardScript, clipboardPath}}
	cfg.Paste.Enable = true
	cfg.PasteCmd = config.CommandConfig{Argv: []string{pasteFailScript}}

	committer := NewCommitter(cfg, nil)
	err := committer.Commit(context.Background(), "captured transcript")
	require.NoError(t, err)

	data, readErr := os.ReadFile(clipboardPath)
	require.NoError(t, readErr)
	require.Equal(t, "captured transcript", string(data))
}

func TestCommitterCommitDefaultPasteFailureDoesNotFailCommit(t *testing.T) {
	clipboardScript := writeStdinCaptureScript(t)
	clipboardPath := filepath.Join(t.TempDir(), "clipboard.txt")

	argsFile := filepath.Join(t.TempDir(), "hypr-args.log")
	t.Setenv("HYPR_ARGS_FILE", argsFile)
	installHyprctlDefaultPasteFailStub(t)

	cfg := config.Default()
	cfg.Clipboard = config.CommandConfig{Argv: []string{clipboardScript, clipboardPath}}
	cfg.Paste.Enable = true
	cfg.PasteCmd = config.CommandConfig{}

	committer := NewCommitter(cfg, nil)
	err := committer.Commit(context.Background(), "captured transcript")
	require.NoError(t, err)

	data, readErr := os.ReadFile(clipboardPath)
	require.NoError(t, readErr)
	require.Equal(t, "captured transcript", string(data))
}

func writeStdinCaptureScript(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	path := filepath.Join(dir, "capture-stdin.sh")
	script := `#!/usr/bin/env bash
set -euo pipefail
cat > "$1"
`
	require.NoError(t, os.WriteFile(path, []byte(script), 0o755))
	return path
}

func writeFailScript(t *testing.T, message string) string {
	t.Helper()

	dir := t.TempDir()
	path := filepath.Join(dir, "fail.sh")
	script := "#!/usr/bin/env bash\nset -euo pipefail\necho " + "\"" + message + "\"" + " >&2\nexit 1\n"
	require.NoError(t, os.WriteFile(path, []byte(script), 0o755))
	return path
}

func installHyprctlDefaultPasteFailStub(t *testing.T) {
	t.Helper()

	dir := t.TempDir()
	path := filepath.Join(dir, "hyprctl")
	script := `#!/usr/bin/env bash
set -euo pipefail
if [[ "${1:-}" == "-j" && "${2:-}" == "activewindow" ]]; then
  echo '{"address":"0xabc","class":"brave-browser","initialClass":"brave-browser"}'
  exit 0
fi
if [[ "${1:-}" == "--quiet" && "${2:-}" == "dispatch" && "${3:-}" == "sendshortcut" ]]; then
  echo "sendshortcut failed" >&2
  exit 1
fi
printf '%s\n' "$*" >> "${HYPR_ARGS_FILE}"
`
	require.NoError(t, os.WriteFile(path, []byte(strings.TrimSpace(script)+"\n"), 0o755))
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))
}
