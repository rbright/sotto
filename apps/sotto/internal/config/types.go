package config

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

type AudioConfig struct {
	Input    string
	Fallback string
}

type PasteConfig struct {
	Enable   bool
	Shortcut string
}

type ASRConfig struct {
	AutomaticPunctuation bool
	LanguageCode         string
	Model                string
}

type TranscriptConfig struct {
	TrailingSpace bool
}

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

type CommandConfig struct {
	Raw  string
	Argv []string
}

type VocabConfig struct {
	GlobalSets []string
	Sets       map[string]VocabSet
	MaxPhrases int
}

type VocabSet struct {
	Name    string
	Boost   float64
	Phrases []string
}

type DebugConfig struct {
	EnableAudioDump bool
	EnableGRPCDump  bool
}

type Warning struct {
	Line    int
	Message string
}

type SpeechPhrase struct {
	Phrase string
	Boost  float32
}
