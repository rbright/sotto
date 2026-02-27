package config

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseValidJSONCConfig(t *testing.T) {
	input := `
{
  // local endpoints
  "riva": {
    "grpc": "127.0.0.1:50051",
    "http": "127.0.0.1:9000"
  },
  "audio": {
    "input": "Elgato"
  },
  "paste": {
    "enable": true,
    "shortcut": "SUPER,V"
  },
  "vocab": {
    "global": ["core", "team"],
    "sets": {
      "core": {
        "boost": 14,
        "phrases": ["Sotto", "Hyprland"]
      },
      "team": {
        "boost": 18,
        "phrases": ["Sotto", "Riva"]
      }
    }
  },
}
`

	cfg, warnings, err := Parse(input, Default())
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if cfg.RivaGRPC != "127.0.0.1:50051" {
		t.Fatalf("unexpected riva.grpc: %s", cfg.RivaGRPC)
	}
	if cfg.Audio.Input != "Elgato" {
		t.Fatalf("unexpected audio.input: %s", cfg.Audio.Input)
	}
	if cfg.Paste.Shortcut != "SUPER,V" {
		t.Fatalf("unexpected paste.shortcut: %s", cfg.Paste.Shortcut)
	}
	if len(warnings) == 0 {
		t.Fatalf("expected dedupe warning for repeated phrase")
	}

	phrases, _, err := BuildSpeechPhrases(cfg)
	if err != nil {
		t.Fatalf("BuildSpeechPhrases() error = %v", err)
	}
	if len(phrases) != 3 {
		t.Fatalf("expected 3 unique phrases, got %d", len(phrases))
	}

	for _, p := range phrases {
		if p.Phrase == "Sotto" && p.Boost != 18 {
			t.Fatalf("expected highest boost retained for Sotto; got %v", p.Boost)
		}
	}
}

func TestParseLegacyFormatStillSupportedWithWarning(t *testing.T) {
	cfg, warnings, err := Parse(`
riva_grpc = 127.0.0.1:50051
paste.enable = false
`, Default())
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if cfg.RivaGRPC != "127.0.0.1:50051" {
		t.Fatalf("unexpected riva_grpc: %s", cfg.RivaGRPC)
	}
	if cfg.Paste.Enable {
		t.Fatalf("expected paste.enable=false")
	}

	found := false
	for _, w := range warnings {
		if strings.Contains(w.Message, "legacy") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected legacy format warning, warnings=%+v", warnings)
	}
}

func TestParseJSONCUnknownKeyFails(t *testing.T) {
	_, _, err := Parse(`{"foo": {"bar": 1}}`, Default())
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseJSONCLineNumberOnError(t *testing.T) {
	_, _, err := Parse(`
{
  "riva": {
    "grpc": "127.0.0.1:50051"
    "http": "127.0.0.1:9000"
  }
}
`, Default())
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "line") {
		t.Fatalf("expected line number in error, got %v", err)
	}
}

func TestValidateMissingVocabSetReference(t *testing.T) {
	cfg := Default()
	cfg.Vocab.GlobalSets = []string{"missing"}

	if _, err := Validate(cfg); err == nil {
		t.Fatal("expected error for missing vocab set")
	}
}

func TestValidateMaxPhraseLimit(t *testing.T) {
	cfg := Default()
	cfg.Vocab.MaxPhrases = 1
	cfg.Vocab.GlobalSets = []string{"team"}
	cfg.Vocab.Sets["team"] = VocabSet{
		Name:    "team",
		Boost:   10,
		Phrases: []string{"one", "two"},
	}

	_, err := Validate(cfg)
	if err == nil {
		t.Fatal("expected max phrase limit error")
	}
	if !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseCommandArgvQuoted(t *testing.T) {
	cfg, _, err := Parse(`{"paste_cmd":"mycmd --name 'hello world'"}`, Default())
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	got := strings.Join(cfg.PasteCmd.Argv, "|")
	want := "mycmd|--name|hello world"
	if got != want {
		t.Fatalf("unexpected argv parse: got %q want %q", got, want)
	}
}

func TestParsePasteShortcut(t *testing.T) {
	cfg, _, err := Parse(`{"paste":{"shortcut":"SUPER,V"}}`, Default())
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if cfg.Paste.Shortcut != "SUPER,V" {
		t.Fatalf("unexpected paste.shortcut: %q", cfg.Paste.Shortcut)
	}
}

func TestParseTranscriptCapitalizeSentencesJSONC(t *testing.T) {
	cfg, _, err := Parse(`{"transcript":{"capitalize_sentences":false}}`, Default())
	require.NoError(t, err)
	require.False(t, cfg.Transcript.CapitalizeSentences)
}

func TestParseTranscriptCapitalizeSentencesLegacy(t *testing.T) {
	cfg, _, err := Parse("transcript.capitalize_sentences = false\n", Default())
	require.NoError(t, err)
	require.False(t, cfg.Transcript.CapitalizeSentences)
}

func TestParseIndicatorBackend(t *testing.T) {
	cfg, _, err := Parse(`
{
  "indicator": {
    "backend": "desktop",
    "desktop_app_name": "sotto-indicator"
  }
}
`, Default())
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if cfg.Indicator.Backend != "desktop" {
		t.Fatalf("expected indicator.backend=desktop, got %q", cfg.Indicator.Backend)
	}
	if cfg.Indicator.DesktopAppName != "sotto-indicator" {
		t.Fatalf("unexpected indicator.desktop_app_name: %q", cfg.Indicator.DesktopAppName)
	}
}

func TestParseIndicatorSoundEnable(t *testing.T) {
	cfg, _, err := Parse(`{"indicator":{"sound_enable":false}}`, Default())
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if cfg.Indicator.SoundEnable {
		t.Fatalf("expected indicator.sound_enable=false")
	}
}

func TestParseIndicatorTextKeysRejected(t *testing.T) {
	_, _, err := Parse(`{"indicator":{"text_recording":"Recording"}}`, Default())
	require.Error(t, err)
	require.Contains(t, err.Error(), "unknown field")
}

func TestParseIndicatorSoundFileKeysRejected(t *testing.T) {
	_, _, err := Parse(`{"indicator":{"sound_start_file":"/tmp/start.wav"}}`, Default())
	require.Error(t, err)
	require.Contains(t, err.Error(), "unknown field")
}

func TestParseInitializesNilVocabMap(t *testing.T) {
	base := Default()
	base.Vocab.Sets = nil

	cfg, _, err := Parse(`
{
  "vocab": {
    "sets": {
      "team": {
        "boost": 10,
        "phrases": ["sotto"]
      }
    }
  }
}
`, base)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if cfg.Vocab.Sets == nil {
		t.Fatal("expected vocab map to be initialized")
	}
	if _, ok := cfg.Vocab.Sets["team"]; !ok {
		t.Fatalf("expected parsed vocab set to be present")
	}
}
