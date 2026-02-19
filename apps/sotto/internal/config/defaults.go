package config

func Default() Config {
	clipboard := "wl-copy --trim-newline"

	return Config{
		RivaGRPC:       "127.0.0.1:50051",
		RivaHTTP:       "127.0.0.1:9000",
		RivaHealthPath: "/v1/health/ready",
		Audio: AudioConfig{
			Input:    "default",
			Fallback: "default",
		},
		Paste: PasteConfig{Enable: true, Shortcut: "CTRL,V"},
		ASR: ASRConfig{
			AutomaticPunctuation: true,
			LanguageCode:         "en-US",
			Model:                "",
		},
		Transcript: TranscriptConfig{TrailingSpace: true},
		Indicator: IndicatorConfig{
			Enable:            true,
			SoundEnable:       true,
			SoundStartFile:    "",
			SoundStopFile:     "",
			SoundCompleteFile: "",
			SoundCancelFile:   "",
			Height:            28,
			TextRecording:     "Recording…",
			TextProcessing:    "Transcribing…",
			TextError:         "Speech recognition error",
			ErrorTimeoutMS:    1600,
		},
		Clipboard: CommandConfig{Raw: clipboard, Argv: mustParseArgv(clipboard)},
		Vocab: VocabConfig{
			GlobalSets: nil,
			Sets:       map[string]VocabSet{},
			MaxPhrases: 1024,
		},
		Debug: DebugConfig{},
	}
}
