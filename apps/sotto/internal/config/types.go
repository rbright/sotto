// Package config resolves, parses, validates, and defaults sotto configuration.
package config

// Config is the fully materialized runtime configuration used by sotto.
type Config struct {
	RivaGRPC       string
	RivaHTTP       string
	RivaHealthPath string
	Audio          AudioConfig
	Paste          PasteConfig
	ASR            ASRConfig
	Transcript     TranscriptConfig
	Indicator      IndicatorConfig
	Clipboard      CommandConfig
	PasteCmd       CommandConfig
	Vocab          VocabConfig
	Debug          DebugConfig
}

// AudioConfig controls preferred and fallback input-source selection.
type AudioConfig struct {
	Input    string
	Fallback string
}

// PasteConfig controls post-commit paste behavior.
type PasteConfig struct {
	Enable   bool
	Shortcut string
}

// ASRConfig controls request-level hints passed to Riva.
type ASRConfig struct {
	AutomaticPunctuation bool
	LanguageCode         string
	Model                string
}

// TranscriptConfig controls transcript assembly formatting.
type TranscriptConfig struct {
	TrailingSpace bool
}

// IndicatorConfig controls visual indicator and audio cue behavior.
type IndicatorConfig struct {
	Enable            bool
	Backend           string
	DesktopAppName    string
	SoundEnable       bool
	SoundStartFile    string
	SoundStopFile     string
	SoundCompleteFile string
	SoundCancelFile   string
	Height            int
	TextRecording     string
	TextProcessing    string
	TextError         string
	ErrorTimeoutMS    int
}

// CommandConfig stores a raw command string and its parsed argv form.
type CommandConfig struct {
	Raw  string
	Argv []string
}

// VocabConfig controls enabled speech phrase sets and dedupe limits.
type VocabConfig struct {
	GlobalSets []string
	Sets       map[string]VocabSet
	MaxPhrases int
}

// VocabSet is one named phrase group with a shared boost value.
type VocabSet struct {
	Name    string
	Boost   float64
	Phrases []string
}

// DebugConfig controls optional debug artifact output.
type DebugConfig struct {
	EnableAudioDump bool
	EnableGRPCDump  bool
}

// Warning is a non-fatal parse/validation message.
type Warning struct {
	Line    int
	Message string
}

// SpeechPhrase is the normalized phrase payload sent to ASR adapters.
type SpeechPhrase struct {
	Phrase string
	Boost  float32
}
