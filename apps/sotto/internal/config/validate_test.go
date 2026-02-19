package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBuildSpeechPhrasesSortedAndHighestBoostWins(t *testing.T) {
	cfg := Default()
	cfg.Vocab.GlobalSets = []string{"core", "team"}
	cfg.Vocab.Sets["core"] = VocabSet{Name: "core", Boost: 10, Phrases: []string{"beta", "alpha"}}
	cfg.Vocab.Sets["team"] = VocabSet{Name: "team", Boost: 20, Phrases: []string{"alpha", "gamma"}}

	phrases, warnings, err := BuildSpeechPhrases(cfg)
	require.NoError(t, err)
	require.Len(t, warnings, 1)
	require.Equal(t, []SpeechPhrase{
		{Phrase: "alpha", Boost: 20},
		{Phrase: "beta", Boost: 10},
		{Phrase: "gamma", Boost: 20},
	}, phrases)
}

func TestValidateRejectsInvalidCoreFields(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*Config)
		wantErr string
	}{
		{name: "empty riva grpc", mutate: func(c *Config) { c.RivaGRPC = "" }, wantErr: "riva_grpc"},
		{name: "empty riva http", mutate: func(c *Config) { c.RivaHTTP = "" }, wantErr: "riva_http"},
		{name: "bad health path", mutate: func(c *Config) { c.RivaHealthPath = "v1/health" }, wantErr: "must start"},
		{name: "empty language", mutate: func(c *Config) { c.ASR.LanguageCode = "" }, wantErr: "language_code"},
		{name: "invalid indicator height", mutate: func(c *Config) { c.Indicator.Height = 0 }, wantErr: "indicator.height"},
		{name: "negative error timeout", mutate: func(c *Config) { c.Indicator.ErrorTimeoutMS = -1 }, wantErr: "error_timeout"},
		{name: "invalid max phrases", mutate: func(c *Config) { c.Vocab.MaxPhrases = 0 }, wantErr: "vocab.max_phrases"},
		{name: "empty clipboard argv", mutate: func(c *Config) { c.Clipboard.Argv = nil }, wantErr: "clipboard_cmd"},
		{name: "paste command raw but empty argv", mutate: func(c *Config) {
			c.Paste.Enable = true
			c.PasteCmd.Raw = "mycmd"
			c.PasteCmd.Argv = nil
		}, wantErr: "paste_cmd"},
		{name: "missing paste shortcut when using default paste", mutate: func(c *Config) {
			c.Paste.Enable = true
			c.PasteCmd = CommandConfig{}
			c.Paste.Shortcut = ""
		}, wantErr: "paste.shortcut"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := Default()
			tc.mutate(&cfg)

			_, err := Validate(cfg)
			require.Error(t, err)
			require.Contains(t, err.Error(), tc.wantErr)
		})
	}
}
