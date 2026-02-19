package main

import (
	"os"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMainHelp(t *testing.T) {
	output, err := runMainSubprocess(t, "--help")
	require.NoError(t, err, string(output))
	require.Contains(t, string(output), "Usage:")
}

func TestMainInvalidCommandExitsNonZero(t *testing.T) {
	output, err := runMainSubprocess(t, "not-a-command")
	require.Error(t, err)

	exitErr, ok := err.(*exec.ExitError)
	require.True(t, ok)
	require.Equal(t, 2, exitErr.ExitCode())
	require.Contains(t, string(output), "unknown command")
}

func TestMainHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}

	args := os.Args
	dashIndex := -1
	for i, arg := range args {
		if arg == "--" {
			dashIndex = i
			break
		}
	}

	os.Args = []string{"sotto"}
	if dashIndex >= 0 && dashIndex+1 < len(args) {
		os.Args = append(os.Args, args[dashIndex+1:]...)
	}

	main()
}

func runMainSubprocess(t *testing.T, args ...string) ([]byte, error) {
	t.Helper()

	cmdArgs := []string{"-test.run=TestMainHelperProcess", "--"}
	cmdArgs = append(cmdArgs, args...)

	cmd := exec.Command(os.Args[0], cmdArgs...)
	cmd.Env = append(os.Environ(), "GO_WANT_HELPER_PROCESS=1")
	return cmd.CombinedOutput()
}
