package config

import (
	"strings"
	"testing"
)

func TestParseValidConfig(t *testing.T) {
	input := `
# comment
riva_grpc = 127.0.0.1:50051
riva_http = "127.0.0.1:9000"
audio.input = "Elgato"
paste.enable = true
vocab.global = core, team

vocabset core {
  boost = 14
  phrases = [ "Sotto", "Hyprland" ]
}

vocabset team {
  boost = 18
  phrases = [ "Sotto", "Riva" ]
}
`

	cfg, warnings, err := Parse(input, Default())
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if cfg.RivaGRPC != "127.0.0.1:50051" {
		t.Fatalf("unexpected riva_grpc: %s", cfg.RivaGRPC)
	}
	if cfg.Audio.Input != "Elgato" {
		t.Fatalf("unexpected audio.input: %s", cfg.Audio.Input)
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

func TestParseUnknownKeyFails(t *testing.T) {
	_, _, err := Parse(`foo.bar = 1`, Default())
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "unknown key") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseLineNumberOnError(t *testing.T) {
	_, _, err := Parse("\n\nthis is bad", Default())
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "line 3") {
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
	cfg, _, err := Parse(`paste_cmd = "mycmd --name 'hello world'"`, Default())
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
	cfg, _, err := Parse(`paste.shortcut = "SUPER,V"`, Default())
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if cfg.Paste.Shortcut != "SUPER,V" {
		t.Fatalf("unexpected paste.shortcut: %q", cfg.Paste.Shortcut)
	}
}

func TestParseSingleQuotedStrings(t *testing.T) {
	cfg, _, err := Parse(`
indicator.text_recording = 'Recording active'
clipboard_cmd = 'wl-copy --trim-newline'
`, Default())
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if cfg.Indicator.TextRecording != "Recording active" {
		t.Fatalf("unexpected indicator.text_recording: %q", cfg.Indicator.TextRecording)
	}
	if strings.Join(cfg.Clipboard.Argv, "|") != "wl-copy|--trim-newline" {
		t.Fatalf("unexpected clipboard argv: %#v", cfg.Clipboard.Argv)
	}
}

func TestParseRejectsUnterminatedSingleQuotedString(t *testing.T) {
	_, _, err := Parse(`indicator.text_recording = 'Recording`, Default())
	if err == nil {
		t.Fatal("expected parse error")
	}
	if !strings.Contains(err.Error(), "closing single quote") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseIndicatorSoundEnable(t *testing.T) {
	cfg, _, err := Parse(`indicator.sound_enable = false`, Default())
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if cfg.Indicator.SoundEnable {
		t.Fatalf("expected indicator.sound_enable=false")
	}
}

func TestParseIndicatorSoundFiles(t *testing.T) {
	cfg, _, err := Parse(`
indicator.sound_start_file = /tmp/start.wav
indicator.sound_stop_file = /tmp/stop.wav
indicator.sound_complete_file = /tmp/complete.wav
indicator.sound_cancel_file = /tmp/cancel.wav
`, Default())
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if cfg.Indicator.SoundStartFile != "/tmp/start.wav" {
		t.Fatalf("unexpected start file: %q", cfg.Indicator.SoundStartFile)
	}
	if cfg.Indicator.SoundStopFile != "/tmp/stop.wav" {
		t.Fatalf("unexpected stop file: %q", cfg.Indicator.SoundStopFile)
	}
	if cfg.Indicator.SoundCompleteFile != "/tmp/complete.wav" {
		t.Fatalf("unexpected complete file: %q", cfg.Indicator.SoundCompleteFile)
	}
	if cfg.Indicator.SoundCancelFile != "/tmp/cancel.wav" {
		t.Fatalf("unexpected cancel file: %q", cfg.Indicator.SoundCancelFile)
	}
}

func TestParseUnterminatedVocabSetReportsStartLine(t *testing.T) {
	_, _, err := Parse(`
vocabset internal {
  boost = 10
`, Default())
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "line 2") {
		t.Fatalf("expected vocabset start line in error, got %v", err)
	}
}
