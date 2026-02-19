package cli

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseDefaultsToHelp(t *testing.T) {
	parsed, err := Parse(nil)
	require.NoError(t, err)
	require.True(t, parsed.ShowHelp)
	require.Equal(t, CommandHelp, parsed.Command)
}

func TestParseCommandWithConfig(t *testing.T) {
	parsed, err := Parse([]string{"--config", "/tmp/sotto.conf", "doctor"})
	require.NoError(t, err)
	require.Equal(t, CommandDoctor, parsed.Command)
	require.Equal(t, "/tmp/sotto.conf", parsed.ConfigPath)
	require.False(t, parsed.ShowHelp)
}

func TestParseArgMatrix(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		wantErr  string
		wantCmd  Command
		wantHelp bool
		wantPath string
	}{
		{
			name:     "help short flag",
			args:     []string{"-h"},
			wantCmd:  CommandHelp,
			wantHelp: true,
		},
		{
			name:     "help long flag",
			args:     []string{"--help"},
			wantCmd:  CommandHelp,
			wantHelp: true,
		},
		{
			name:     "version flag",
			args:     []string{"--version"},
			wantCmd:  CommandVersion,
			wantHelp: false,
		},
		{
			name:    "config after command",
			args:    []string{"status", "--config", "/tmp/cfg"},
			wantErr: "unexpected arguments after command",
		},
		{
			name:    "missing config path",
			args:    []string{"--config"},
			wantErr: "requires a path",
		},
		{
			name:    "unknown flag",
			args:    []string{"--bogus"},
			wantErr: "unknown flag",
		},
		{
			name:    "unknown command",
			args:    []string{"bogus"},
			wantErr: "unknown command",
		},
		{
			name:    "extra args after command",
			args:    []string{"doctor", "extra"},
			wantErr: "unexpected arguments",
		},
		{
			name:     "valid cancel command",
			args:     []string{"cancel"},
			wantCmd:  CommandCancel,
			wantHelp: false,
		},
		{
			name:     "valid stop with config",
			args:     []string{"--config", "/tmp/cfg", "stop"},
			wantCmd:  CommandStop,
			wantHelp: false,
			wantPath: "/tmp/cfg",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			parsed, err := Parse(tc.args)
			if tc.wantErr != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.wantErr)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tc.wantCmd, parsed.Command)
			require.Equal(t, tc.wantHelp, parsed.ShowHelp)
			require.Equal(t, tc.wantPath, parsed.ConfigPath)
		})
	}
}

func TestHelpTextIncludesCoreCommands(t *testing.T) {
	text := HelpText("sotto")
	require.Contains(t, text, "toggle")
	require.Contains(t, text, "stop")
	require.Contains(t, text, "cancel")
	require.Contains(t, text, "doctor")
	require.Contains(t, text, "--config PATH")
}
