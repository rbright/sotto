package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseArgv(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    []string
		wantErr string
	}{
		{name: "empty", input: "", want: nil},
		{name: "simple", input: "wl-copy --trim-newline", want: []string{"wl-copy", "--trim-newline"}},
		{name: "quoted spaces", input: `mycmd --name "hello world"`, want: []string{"mycmd", "--name", "hello world"}},
		{name: "single quote", input: `mycmd --name 'hello world'`, want: []string{"mycmd", "--name", "hello world"}},
		{name: "escaped space", input: `mycmd hello\ world`, want: []string{"mycmd", "hello world"}},
		{name: "leading comment", input: `# wl-copy --trim-newline`, want: nil},
		{name: "unterminated quote", input: `mycmd "oops`, wantErr: "unterminated quote"},
		{name: "unterminated escape", input: `mycmd hello\`, wantErr: "unterminated escape"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseArgv(tc.input)
			if tc.wantErr != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestMustParseArgvPanicsOnInvalidInput(t *testing.T) {
	require.Panics(t, func() {
		_ = mustParseArgv(`mycmd "unterminated`)
	})
}
